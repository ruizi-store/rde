// Package flatpak 桌面实例管理（KasmVNC 启动/停止/状态）
package flatpak

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Desktop KasmVNC 桌面实例管理器
type Desktop struct {
	logger    *zap.Logger
	dataDir   string
	config    DesktopConfig
	installer *Installer

	mu          sync.RWMutex
	cmd         *exec.Cmd // Xkasmvnc 进程
	openboxCmd  *exec.Cmd // openbox 进程
	pid         int
	startedAt   time.Time
	running     bool
	runningApps map[string]*RunningApp // app_id -> RunningApp
	appCmds     map[string]*exec.Cmd   // app_id -> 进程
	outputBuf   *bytes.Buffer          // Xkasmvnc 输出缓冲
}

// NewDesktop 创建桌面管理器
func NewDesktop(logger *zap.Logger, dataDir string, installer *Installer) *Desktop {
	return &Desktop{
		logger:      logger,
		dataDir:     dataDir,
		installer:   installer,
		config:      DefaultDesktopConfig(),
		runningApps: make(map[string]*RunningApp),
		appCmds:     make(map[string]*exec.Cmd),
	}
}

// SetConfig 更新配置
func (d *Desktop) SetConfig(config DesktopConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config = config
}

// GetConfig 获取配置
func (d *Desktop) GetConfig() DesktopConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}

// IsRunning 检查桌面是否正在运行
func (d *Desktop) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// Start 启动 KasmVNC 桌面实例
func (d *Desktop) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.startLocked()
}

// startLocked 内部启动（调用方必须持有 mu 锁）
func (d *Desktop) startLocked() error {
	if d.running {
		return nil // 已在运行
	}

	if !d.installer.IsInstalled() {
		return fmt.Errorf("KasmVNC not installed")
	}

	display := d.config.Display
	wsPort := d.config.WebSocketPort
	resolution := d.config.DefaultResolution

	d.logger.Info("starting KasmVNC desktop",
		zap.Int("display", display),
		zap.Int("ws_port", wsPort),
		zap.String("resolution", resolution),
	)

	// 确保 openbox 配置存在
	d.ensureOpenboxConfig()

	// 启动 Xkasmvnc
	binary := d.installer.GetBinaryPath()
	args := []string{
		fmt.Sprintf(":%d", display),
		"-geometry", resolution,
		"-depth", "24",
		"-websocketPort", strconv.Itoa(wsPort),
		"-rfbport", strconv.Itoa(wsPort + 100), // rfb = ws + 100
		"-AlwaysShared",
		"-SecurityTypes", "None",
		"-DisableBasicAuth",
		"-httpd", "/usr/share/kasmvnc/www",
		"-interface", "127.0.0.1",
		"-publicIP", "127.0.0.1", // 跳过 STUN 查询，避免启动超时
		"-BlacklistTimeout", "0",
		"-FreeKeyMappings",
		"-SendCutText",
		"-AcceptCutText",
	}

	cmd := exec.Command(binary, args...)
	cmd.Env = d.buildEnv()

	// 捕获输出用于调试和错误诊断
	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Xkasmvnc: %w", err)
	}

	d.cmd = cmd
	d.pid = cmd.Process.Pid
	d.startedAt = time.Now()
	d.outputBuf = &outputBuf

	// 监控进程退出（用于提前检测崩溃）
	processDone := make(chan error, 1)
	go func() {
		processDone <- cmd.Wait()
	}()

	// 等待端口就绪（同时检测进程提前退出）
	if err := d.waitForPortOrExit(wsPort, 30*time.Second, processDone); err != nil {
		// 收集已产生的输出用于诊断
		output := strings.TrimSpace(outputBuf.String())
		d.logger.Error("KasmVNC failed to start",
			zap.Error(err),
			zap.String("output", output),
		)
		cmd.Process.Kill()
		// 等一下让 processDone goroutine 退出
		select {
		case <-processDone:
		case <-time.After(2 * time.Second):
		}
		d.cmd = nil
		d.pid = 0
		d.running = false
		d.outputBuf = nil
		if output != "" {
			return fmt.Errorf("%w\nKasmVNC output:\n%s", err, output)
		}
		return err
	}

	d.running = true
	d.logger.Info("KasmVNC desktop started",
		zap.Int("pid", d.pid),
		zap.Int("display", display),
		zap.Int("ws_port", wsPort),
	)

	// 启动 openbox 窗口管理器
	go d.startOpenbox(display)

	// 监控进程退出
	go d.watchProcess(processDone)

	return nil
}

// Stop 停止桌面
func (d *Desktop) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopLocked()
}

// stopLocked 内部停止（调用方必须持有 mu 锁）
func (d *Desktop) stopLocked() {
	if !d.running {
		return
	}

	d.logger.Info("stopping KasmVNC desktop")

	// 标记为非运行，让 watchProcess 退出后不做额外操作
	d.running = false

	// 停止所有运行中的应用
	for id, cmd := range d.appCmds {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		delete(d.appCmds, id)
		delete(d.runningApps, id)
	}

	// 停止 openbox
	if d.openboxCmd != nil && d.openboxCmd.Process != nil {
		d.openboxCmd.Process.Kill()
		d.openboxCmd = nil
	}

	// 停止 KasmVNC
	if d.cmd != nil && d.cmd.Process != nil {
		d.cmd.Process.Kill()
		d.cmd = nil
		d.pid = 0
		d.outputBuf = nil
	} else {
		d.cmd = nil
		d.pid = 0
		d.outputBuf = nil
	}

	d.logger.Info("KasmVNC desktop stopped")
}

// Restart 重启桌面
func (d *Desktop) Restart() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopLocked()
	return d.startLocked()
}

// GetStatus 获取桌面状态
func (d *Desktop) GetStatus() *DesktopStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	status := &DesktopStatus{
		Running:        d.running,
		Display:        d.config.Display,
		WebSocketPort:  d.config.WebSocketPort,
		PID:            d.pid,
		Resolution:     d.config.DefaultResolution,
		KasmVNCVersion: d.installer.GetInstalledVersion(),
		RunningApps:    make([]string, 0),
	}

	if d.running {
		status.Uptime = int64(time.Since(d.startedAt).Seconds())
		status.VNCURL = fmt.Sprintf("/api/v1/flatpak/vnc/vnc.html?autoconnect=true&resize=remote&path=api/v1/flatpak/vnc/websockify&password=")
		for appID := range d.runningApps {
			status.RunningApps = append(status.RunningApps, appID)
		}
	}

	return status
}

// LaunchApp 在桌面中启动 Flatpak 应用（以指定用户身份）
func (d *Desktop) LaunchApp(appID, name string, args []string, username string, uid int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return fmt.Errorf("desktop not running")
	}

	// 检查是否已在运行
	if _, exists := d.runningApps[appID]; exists {
		return nil // 已在运行
	}

	display := d.config.Display
	runtimeDir := fmt.Sprintf("/run/user/%d", uid)

	// 确保 XDG_RUNTIME_DIR 存在
	os.MkdirAll(runtimeDir, 0700)
	os.Chown(runtimeDir, uid, uid)

	// KasmVNC 无 GPU 硬件加速，为 Electron 应用准备配置
	d.ensureElectronConfig(appID, username, uid)

	// 清理可能残留的 FUSE 挂载（上次 document-portal 未正常退出留下的）
	docMountPath := filepath.Join(runtimeDir, "doc")
	exec.Command("fusermount3", "-u", docMountPath).Run()
	os.RemoveAll(docMountPath)

	// 构建 flatpak run 命令参数
	flatpakArgs := []string{"run"}

	// 对 Electron 应用：绕过 zypak 直接运行 electron 二进制，
	// 并添加 --no-sandbox --disable-gpu 以兼容 KasmVNC 无 GPU 环境
	if ecfg, ok := electronAppOverrides[appID]; ok {
		flatpakArgs = append(flatpakArgs, "--command="+ecfg.command)
		flatpakArgs = append(flatpakArgs, appID)
		flatpakArgs = append(flatpakArgs, ecfg.flags...)
	} else {
		flatpakArgs = append(flatpakArgs, appID)
	}

	if len(args) > 0 {
		flatpakArgs = append(flatpakArgs, args...)
	}

	// 使用 su -l 以目标用户身份运行（完整 login 环境: HOME, XDG_DATA_DIRS 等）
	// dbus-run-session 会自动启动临时 dbus-daemon 并设置 DBUS_SESSION_BUS_ADDRESS
	// 解决 Electron 等应用需要 D-Bus session bus 的问题
	flatpakCmd := fmt.Sprintf("DISPLAY=:%d XDG_RUNTIME_DIR=%s PULSE_SERVER=unix:/run/pulse/native dbus-run-session -- flatpak %s",
		display, runtimeDir, strings.Join(flatpakArgs, " "))

	cmd := exec.Command("su", "-l", username, "-s", "/bin/bash", "-c", flatpakCmd)

	// 捕获输出用于诊断应用启动失败
	var appOutput bytes.Buffer
	cmd.Stdout = &appOutput
	cmd.Stderr = &appOutput

	d.logger.Info("launching flatpak app as user",
		zap.String("app_id", appID),
		zap.String("username", username),
		zap.Int("uid", uid),
		zap.Int("display", display),
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", appID, err)
	}

	d.appCmds[appID] = cmd
	d.runningApps[appID] = &RunningApp{
		AppID:     appID,
		Name:      name,
		PID:       cmd.Process.Pid,
		StartedAt: time.Now(),
	}

	// 监控应用进程退出
	go func() {
		cmd.Wait()
		d.mu.Lock()
		delete(d.appCmds, appID)
		delete(d.runningApps, appID)
		d.mu.Unlock()
		output := strings.TrimSpace(appOutput.String())
		if output != "" {
			// 只记录最后 2000 字符避免日志过大
			if len(output) > 2000 {
				output = output[len(output)-2000:]
			}
			d.logger.Info("flatpak app exited",
				zap.String("app_id", appID),
				zap.String("output", output))
		} else {
			d.logger.Info("flatpak app exited", zap.String("app_id", appID))
		}
	}()

	return nil
}

// GetRunningApps 获取运行中的应用
func (d *Desktop) GetRunningApps() []*RunningApp {
	d.mu.RLock()
	defer d.mu.RUnlock()

	apps := make([]*RunningApp, 0, len(d.runningApps))
	for _, app := range d.runningApps {
		apps = append(apps, app)
	}
	return apps
}

// ===== 内部方法 =====

// electronAppOverrides KasmVNC 环境下 Electron 应用的命令覆盖配置
// 绕过 zypak wrapper（在无 GPU 环境下会导致 int3 trap 崩溃），
// 直接运行 electron 二进制并使用 --no-sandbox --disable-gpu
var electronAppOverrides = map[string]struct {
	command string   // flatpak --command= 覆盖（直接运行 electron 二进制）
	flags   []string // Electron/Chromium 命令行标志
}{
	"com.visualstudio.code": {
		command: "/app/extra/vscode/code",
		flags:   []string{"--no-sandbox", "--disable-gpu", "--disable-gpu-compositing"},
	},
}

// electronAppConfigs Electron 应用的配置文件（禁用 GPU 硬件加速）
var electronAppConfigs = map[string]struct {
	subDir   string
	filename string
	content  string
}{
	"com.visualstudio.code": {
		subDir:   "config/Code",
		filename: "argv.json",
		content:  `{"enable-proposed-api":[],"disable-hardware-acceleration":true}`,
	},
}

// ensureElectronConfig 为 Electron 应用创建 argv.json 禁用硬件加速（KasmVNC 无 GPU）
func (d *Desktop) ensureElectronConfig(appID, username string, uid int) {
	cfg, ok := electronAppConfigs[appID]
	if !ok {
		return
	}

	homeDir := fmt.Sprintf("/home/%s", username)
	configDir := filepath.Join(homeDir, ".var/app", appID, cfg.subDir)
	configFile := filepath.Join(configDir, cfg.filename)

	// 已存在则跳过
	if _, err := os.Stat(configFile); err == nil {
		return
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		d.logger.Warn("failed to create electron config dir", zap.String("dir", configDir), zap.Error(err))
		return
	}

	if err := os.WriteFile(configFile, []byte(cfg.content), 0644); err != nil {
		d.logger.Warn("failed to write electron config", zap.String("file", configFile), zap.Error(err))
		return
	}

	// 修正所有权
	os.Chown(configDir, uid, uid)
	os.Chown(configFile, uid, uid)
	d.logger.Info("created electron config for KasmVNC",
		zap.String("app_id", appID),
		zap.String("file", configFile),
	)
}

// buildEnv 构建环境变量
func (d *Desktop) buildEnv() []string {
	env := os.Environ()
	// 过滤掉可能干扰的 X11 相关环境变量
	var filtered []string
	for _, e := range env {
		key := strings.SplitN(e, "=", 2)[0]
		switch key {
		case "DISPLAY", "XAUTHORITY", "WAYLAND_DISPLAY", "XDG_SESSION_TYPE",
			"XDG_CURRENT_DESKTOP", "GDMSESSION", "DESKTOP_SESSION":
			continue
		default:
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// startOpenbox 启动 openbox 窗口管理器
func (d *Desktop) startOpenbox(display int) {
	// 等待一小段时间确保 X server 就绪
	time.Sleep(500 * time.Millisecond)

	configFile := filepath.Join(d.dataDir, "openbox-rc.xml")
	args := []string{"--config-file", configFile}

	cmd := exec.Command("openbox", args...)
	cmd.Env = append(d.buildEnv(), fmt.Sprintf("DISPLAY=:%d", display))

	if err := cmd.Start(); err != nil {
		d.logger.Warn("failed to start openbox", zap.Error(err))
		return
	}

	d.mu.Lock()
	d.openboxCmd = cmd
	d.mu.Unlock()

	cmd.Wait()
}

// ensureOpenboxConfig 确保 openbox 配置文件存在并保持最新
func (d *Desktop) ensureOpenboxConfig() {
	os.MkdirAll(d.dataDir, 0755)
	configFile := filepath.Join(d.dataDir, "openbox-rc.xml")

	// 写入默认配置
	config := `<?xml version="1.0" encoding="UTF-8"?>
<openbox_config xmlns="http://openbox.org/3.4/rc"
                xmlns:xi="http://www.w3.org/2001/XInclude">
  <resistance>
    <strength>10</strength>
    <screen_edge_strength>20</screen_edge_strength>
  </resistance>
  <focus>
    <focusNew>yes</focusNew>
    <followMouse>no</followMouse>
    <focusLast>yes</focusLast>
    <underMouse>no</underMouse>
    <focusDelay>200</focusDelay>
    <raiseOnFocus>no</raiseOnFocus>
  </focus>
  <placement>
    <policy>Smart</policy>
    <center>yes</center>
    <monitor>Primary</monitor>
    <primaryMonitor>1</primaryMonitor>
  </placement>
  <theme>
    <name>Clearlooks</name>
    <titleLayout>NLIMC</titleLayout>
    <keepBorder>yes</keepBorder>
    <animateIconify>yes</animateIconify>
    <font place="ActiveWindow"><name>sans</name><size>10</size></font>
    <font place="InactiveWindow"><name>sans</name><size>10</size></font>
  </theme>
  <desktops>
    <number>1</number>
    <firstdesk>1</firstdesk>
  </desktops>
  <keyboard>
    <keybind key="A-F4"><action name="Close"/></keybind>
    <keybind key="A-Tab"><action name="NextWindow"/></keybind>
    <keybind key="A-F11"><action name="ToggleFullscreen"/></keybind>
  </keyboard>
  <mouse>
    <context name="Frame">
      <mousebind button="A-Left" action="Press"><action name="Focus"/><action name="Raise"/></mousebind>
      <mousebind button="A-Left" action="Drag"><action name="Move"/></mousebind>
      <mousebind button="A-Right" action="Drag"><action name="Resize"/></mousebind>
    </context>
    <context name="Titlebar">
      <mousebind button="Left" action="Press"><action name="Focus"/><action name="Raise"/></mousebind>
      <mousebind button="Left" action="Drag"><action name="Move"/></mousebind>
      <mousebind button="Left" action="DoubleClick"><action name="ToggleMaximize"/></mousebind>
    </context>
    <context name="Close">
      <mousebind button="Left" action="Click"><action name="Close"/></mousebind>
    </context>
    <context name="Iconify">
      <mousebind button="Left" action="Click"><action name="Iconify"/></mousebind>
    </context>
    <context name="Maximize">
      <mousebind button="Left" action="Click"><action name="ToggleMaximize"/></mousebind>
    </context>
    <context name="Client">
      <mousebind button="Left" action="Press"><action name="Focus"/><action name="Raise"/></mousebind>
      <mousebind button="Middle" action="Press"><action name="Focus"/><action name="Raise"/></mousebind>
      <mousebind button="Right" action="Press"><action name="Focus"/><action name="Raise"/></mousebind>
    </context>
  </mouse>
  <applications>
    <!-- KasmVNC 环境: 所有窗口默认最大化且去掉标题栏 -->
    <application class="*">
      <maximized>yes</maximized>
      <decor>no</decor>
    </application>
  </applications>
</openbox_config>
`
	os.WriteFile(configFile, []byte(config), 0644)
}

// waitForPortOrExit 等待端口就绪，同时检测进程是否提前退出
func (d *Desktop) waitForPortOrExit(port int, timeout time.Duration, processDone <-chan error) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// 检测进程是否已退出
		select {
		case exitErr := <-processDone:
			if exitErr != nil {
				return fmt.Errorf("KasmVNC process exited: %w", exitErr)
			}
			return fmt.Errorf("KasmVNC process exited unexpectedly with code 0")
		default:
		}

		// 尝试连接端口
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("KasmVNC port %d not ready after %v timeout", port, timeout)
}

// watchProcess 监控 KasmVNC 进程退出
func (d *Desktop) watchProcess(processDone <-chan error) {
	exitErr := <-processDone

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		d.running = false
		d.pid = 0
		d.cmd = nil
		if exitErr != nil {
			d.logger.Warn("KasmVNC process exited unexpectedly", zap.Error(exitErr))
		} else {
			d.logger.Warn("KasmVNC process exited unexpectedly with code 0")
		}
	}
}
