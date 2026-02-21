// Package flatpak 桌面实例管理（KasmVNC 启动/停止/状态）
package flatpak

import (
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
	cmd         *exec.Cmd       // Xkasmvnc 进程
	openboxCmd  *exec.Cmd       // openbox 进程
	pid         int
	startedAt   time.Time
	running     bool
	runningApps map[string]*RunningApp // app_id -> RunningApp
	appCmds     map[string]*exec.Cmd   // app_id -> 进程
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
		"-interface", "127.0.0.1",
		"-BlacklistTimeout", "0",
		"-FreeKeyMappings",
		"-SendCutText",
		"-AcceptCutText",
		"-MaxCutText", "10485760",
	}

	cmd := exec.Command(binary, args...)
	cmd.Env = d.buildEnv()

	// 捕获输出用于调试
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Xkasmvnc: %w", err)
	}

	d.cmd = cmd
	d.pid = cmd.Process.Pid
	d.startedAt = time.Now()

	// 等待端口就绪
	if !d.waitForPort(wsPort, 15*time.Second) {
		cmd.Process.Kill()
		cmd.Wait()
		d.cmd = nil
		d.pid = 0
		d.running = false
		return fmt.Errorf("KasmVNC port %d not ready after timeout", wsPort)
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
	go d.watchProcess()

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
		// 在释放锁的情况下 Wait，避免与 watchProcess 竞争
		cmd := d.cmd
		d.cmd = nil
		d.pid = 0
		d.mu.Unlock()
		cmd.Wait()
		d.mu.Lock()
	} else {
		d.cmd = nil
		d.pid = 0
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

// LaunchApp 在桌面中启动 Flatpak 应用
func (d *Desktop) LaunchApp(appID, name string, args []string) error {
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

	// flatpak run APP_ID [ARGS...]
	cmdArgs := []string{"run", appID}
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}

	cmd := exec.Command("flatpak", cmdArgs...)
	cmd.Env = append(d.buildEnv(),
		fmt.Sprintf("DISPLAY=:%d", display),
		"PULSE_SERVER=unix:/run/pulse/native",
	)

	d.logger.Info("launching flatpak app",
		zap.String("app_id", appID),
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
		d.logger.Info("flatpak app exited", zap.String("app_id", appID))
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

// ensureOpenboxConfig 确保 openbox 配置文件存在
func (d *Desktop) ensureOpenboxConfig() {
	os.MkdirAll(d.dataDir, 0755)
	configFile := filepath.Join(d.dataDir, "openbox-rc.xml")

	if _, err := os.Stat(configFile); err == nil {
		return // 已存在
	}

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
</openbox_config>
`
	os.WriteFile(configFile, []byte(config), 0644)
}

// waitForPort 等待端口就绪
func (d *Desktop) waitForPort(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// watchProcess 监控 KasmVNC 进程退出
func (d *Desktop) watchProcess() {
	d.mu.RLock()
	cmd := d.cmd
	d.mu.RUnlock()

	if cmd == nil {
		return
	}
	cmd.Wait()

	d.mu.Lock()
	// 只有当 cmd 仍然是我们监控的那个时才更新状态
	// （Stop 可能已经替换了 d.cmd）
	if d.cmd == cmd {
		d.running = false
		d.pid = 0
		d.cmd = nil
		d.logger.Warn("KasmVNC process exited unexpectedly")
	}
	d.mu.Unlock()
}
