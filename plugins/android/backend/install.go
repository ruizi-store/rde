// Package android 安装向导
package android

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// InstallWizard 安装向导
type InstallWizard struct {
	mu          sync.RWMutex
	config      *InstallConfig
	steps       []*StepInfo
	currentStep int
	isRunning   bool
	cancelFunc  context.CancelFunc
	listeners   []func(*StepInfo)
}

// NewInstallWizard 创建安装向导
func NewInstallWizard(config *InstallConfig) *InstallWizard {
	if config == nil {
		config = &InstallConfig{
			DockerImage:      "redroid/redroid:16.0.0-latest",
			ContainerName:    "ruizios-android",
			BinderModulePath: "/var/lib/rde/plugins/android/binder-modules/binder",
			ADBPort:          5555,
			DataVolume:       "ruizios-android-data",
		}
	}

	return &InstallWizard{
		config:      config,
		steps:       make([]*StepInfo, 0),
		currentStep: -1,
		isRunning:   false,
		listeners:   make([]func(*StepInfo), 0),
	}
}

// OnStepUpdate 注册步骤更新监听器
func (w *InstallWizard) OnStepUpdate(listener func(*StepInfo)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.listeners = append(w.listeners, listener)
}

func (w *InstallWizard) notifyListeners(step *StepInfo) {
	w.mu.RLock()
	listeners := make([]func(*StepInfo), len(w.listeners))
	copy(listeners, w.listeners)
	w.mu.RUnlock()

	for _, listener := range listeners {
		listener(step)
	}
}

// CheckEnvironment 检查环境状态
func (w *InstallWizard) CheckEnvironment() *EnvironmentStatus {
	status := &EnvironmentStatus{
		DKMSInstalled:    CheckDKMSInstalled(),
		HeadersInstalled: CheckLinuxHeadersInstalled(),
		BinderLoaded:     CheckBinderModuleLoaded(),
		DockerInstalled:  CheckDockerInstalled(),
		KernelVersion:    GetKernelVersion(),
		RequiredSteps:    make([]InstallStep, 0),
	}

	status.BinderInstalled = w.checkBinderDKMSInstalled()

	if status.DockerInstalled {
		status.ImageExists = CheckDockerImageExists(w.config.DockerImage)
		status.ContainerExists = CheckContainerExists(w.config.ContainerName)
		status.ContainerRunning = CheckContainerRunning(w.config.ContainerName)
	}

	// 确定需要的步骤
	if !status.BinderLoaded {
		if !status.DKMSInstalled {
			status.RequiredSteps = append(status.RequiredSteps, StepInstallDKMS)
		}
		if !status.HeadersInstalled {
			status.RequiredSteps = append(status.RequiredSteps, StepInstallHeaders)
		}
		if !status.BinderInstalled {
			status.RequiredSteps = append(status.RequiredSteps, StepInstallBinder)
		}
		status.RequiredSteps = append(status.RequiredSteps, StepLoadBinder)
	}
	if !status.DockerInstalled {
		status.RequiredSteps = append(status.RequiredSteps, StepInstallDocker)
	}
	if !status.ImageExists {
		status.RequiredSteps = append(status.RequiredSteps, StepPullImage)
	}
	if !status.ContainerRunning && !status.ContainerExists {
		status.RequiredSteps = append(status.RequiredSteps, StepStartContainer)
	}

	status.IsReady = status.BinderLoaded && status.DockerInstalled &&
		status.ImageExists && (status.ContainerRunning || status.ContainerExists)
	status.OnlyNeedStartContainer = status.BinderLoaded && status.DockerInstalled &&
		status.ImageExists && status.ContainerExists && !status.ContainerRunning

	return status
}

func (w *InstallWizard) checkBinderDKMSInstalled() bool {
	out, err := runCommand("dkms", "status")
	if err != nil {
		return false
	}
	return strings.Contains(out, "binder")
}

// GetSteps 获取当前步骤列表
func (w *InstallWizard) GetSteps() []*StepInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()
	steps := make([]*StepInfo, len(w.steps))
	copy(steps, w.steps)
	return steps
}

// GetCurrentStep 获取当前步骤
func (w *InstallWizard) GetCurrentStep() *StepInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.currentStep >= 0 && w.currentStep < len(w.steps) {
		return w.steps[w.currentStep]
	}
	return nil
}

// IsRunning 是否正在运行
func (w *InstallWizard) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isRunning
}

// Start 启动安装
func (w *InstallWizard) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.isRunning {
		w.mu.Unlock()
		return fmt.Errorf("安装已在进行中")
	}

	ctx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel
	w.isRunning = true

	w.mu.Unlock()
	status := w.CheckEnvironment()
	w.mu.Lock()

	w.steps = make([]*StepInfo, 0)
	for _, step := range status.RequiredSteps {
		info := &StepInfo{
			Step:   step,
			Status: StatusPending,
		}
		switch step {
		case StepInstallDKMS:
			info.Title = "安装 DKMS"
			info.Description = "安装动态内核模块支持工具"
		case StepInstallHeaders:
			info.Title = "安装内核头文件"
			info.Description = fmt.Sprintf("安装 Linux %s 内核头文件", status.KernelVersion)
		case StepInstallBinder:
			info.Title = "安装 Binder 模块"
			info.Description = "通过 DKMS 编译并安装 Android Binder 内核模块"
		case StepLoadBinder:
			info.Title = "加载 Binder 模块"
			info.Description = "将 Binder 模块加载到内核"
		case StepInstallDocker:
			info.Title = "安装 Docker"
			info.Description = "安装 Docker 容器运行时"
		case StepPullImage:
			info.Title = "拉取 Android 镜像"
			info.Description = fmt.Sprintf("下载 %s 镜像", w.config.DockerImage)
		case StepStartContainer:
			info.Title = "启动容器"
			info.Description = "启动 Android 容器实例"
		}
		w.steps = append(w.steps, info)
	}

	w.steps = append(w.steps, &StepInfo{
		Step:        StepCompleted,
		Status:      StatusPending,
		Title:       "安装完成",
		Description: "Android 环境已就绪",
	})

	w.currentStep = 0
	w.mu.Unlock()

	go w.runInstallation(ctx)

	return nil
}

// Cancel 取消安装
func (w *InstallWizard) Cancel() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancelFunc != nil {
		w.cancelFunc()
	}
}

func (w *InstallWizard) runInstallation(ctx context.Context) {
	defer func() {
		w.mu.Lock()
		w.isRunning = false
		w.mu.Unlock()
	}()

	for {
		w.mu.RLock()
		if w.currentStep >= len(w.steps) {
			w.mu.RUnlock()
			break
		}
		step := w.steps[w.currentStep]
		w.mu.RUnlock()

		select {
		case <-ctx.Done():
			w.updateStepStatus(step, StatusFailed, "安装已取消")
			return
		default:
		}

		w.updateStepStatus(step, StatusInProgress, "")
		err := w.executeStep(ctx, step)
		if err != nil {
			w.updateStepStatus(step, StatusFailed, err.Error())
			return
		}
		w.updateStepStatus(step, StatusCompleted, "")

		w.mu.Lock()
		w.currentStep++
		w.mu.Unlock()
	}
}

func (w *InstallWizard) updateStepStatus(step *StepInfo, status StepStatus, errorMsg string) {
	now := time.Now()
	step.Status = status
	step.Error = errorMsg

	switch status {
	case StatusInProgress:
		step.StartedAt = &now
		step.Progress = 0
	case StatusCompleted:
		step.FinishedAt = &now
		step.Progress = 100
	case StatusFailed:
		step.FinishedAt = &now
	}

	w.notifyListeners(step)
}

func (w *InstallWizard) executeStep(ctx context.Context, step *StepInfo) error {
	kernelVersion := GetKernelVersion()

	switch step.Step {
	case StepInstallDKMS:
		if err := runCmd("apt-get", "install", "-y", "dkms"); err != nil {
			return fmt.Errorf("安装 dkms 失败: %w", err)
		}

	case StepInstallHeaders:
		if CheckKernelHeadersExist() {
			return nil
		}
		headersPkg := "linux-headers-" + kernelVersion
		if err := runCmd("apt-get", "install", "-y", headersPkg); err != nil {
			// 回退尝试通用包
			fallbackPkgs := []string{"linux-headers-amd64", "linux-headers-generic"}
			for _, pkg := range fallbackPkgs {
				if err2 := runCmd("apt-get", "install", "-y", pkg); err2 == nil {
					return nil
				}
			}
			return fmt.Errorf("无法安装内核头文件 (%s): %w", headersPkg, err)
		}

	case StepInstallBinder:
		if _, err := os.Stat(w.config.BinderModulePath); os.IsNotExist(err) {
			return fmt.Errorf("Binder 模块源码不存在: %s", w.config.BinderModulePath)
		}

		dstDir := "/usr/src/anbox-binder-1"
		os.RemoveAll(dstDir)
		if err := copyDir(w.config.BinderModulePath, dstDir); err != nil {
			return fmt.Errorf("复制 binder 源码失败: %w", err)
		}

		_ = runCmd("dkms", "remove", "anbox-binder/1", "--all")

		if err := runCmd("dkms", "install", "anbox-binder/1"); err != nil {
			return fmt.Errorf("dkms install 失败: %w", err)
		}

	case StepLoadBinder:
		if err := runCmd("modprobe", "binder_linux"); err != nil {
			return fmt.Errorf("加载 binder_linux 模块失败: %w", err)
		}

		// 开机自启动
		_ = os.WriteFile("/etc/modules-load.d/binder.conf", []byte("binder_linux\n"), 0644)

		// 安装 udev 规则
		udevRules := filepath.Join(filepath.Dir(w.config.BinderModulePath), "99-anbox.rules")
		if _, err := os.Stat(udevRules); err == nil {
			data, _ := os.ReadFile(udevRules)
			if len(data) > 0 {
				_ = os.WriteFile("/etc/udev/rules.d/99-anbox.rules", data, 0644)
				_ = runCmd("udevadm", "control", "--reload-rules")
				_ = runCmd("udevadm", "trigger")
			}
		}

		// 确保 /dev/binder 权限
		for i := 0; i < 5; i++ {
			if _, err := os.Stat("/dev/binder"); err == nil {
				_ = os.Chmod("/dev/binder", 0666)
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

	case StepInstallDocker:
		if err := runCmd("apt-get", "install", "-y", "docker.io"); err != nil {
			return fmt.Errorf("Docker 安装失败: %w", err)
		}
		for i := 0; i < 10; i++ {
			if CheckDockerInstalled() {
				break
			}
			time.Sleep(time.Second)
		}

	case StepPullImage:
		if err := runCmd("docker", "pull", w.config.DockerImage); err != nil {
			return fmt.Errorf("镜像拉取失败: %w", err)
		}

	case StepStartContainer:
		if CheckContainerRunning(w.config.ContainerName) {
			return nil
		}

		// 容器已存在但未运行，直接启动
		if CheckContainerExists(w.config.ContainerName) {
			if err := runCmd("docker", "start", w.config.ContainerName); err != nil {
				return fmt.Errorf("启动已有容器失败: %w", err)
			}
			time.Sleep(3 * time.Second)
			adbAddr := fmt.Sprintf("localhost:%d", w.config.ADBPort)
			runCommand("adb", "connect", adbAddr)
			return nil
		}

		args := []string{
			"run", "-d",
			"--name", w.config.ContainerName,
			"--privileged",
			"-p", fmt.Sprintf("%d:5555", w.config.ADBPort),
		}
		if w.config.DataVolume != "" {
			args = append(args, "-v", fmt.Sprintf("%s:/data", w.config.DataVolume))
		}
		args = append(args, w.config.DockerImage)

		if err := runCmd("docker", args...); err != nil {
			return fmt.Errorf("容器启动失败: %w", err)
		}

		// 等待容器启动并自动连接 ADB
		time.Sleep(3 * time.Second)
		adbAddr := fmt.Sprintf("localhost:%d", w.config.ADBPort)
		runCommand("adb", "connect", adbAddr)

	case StepCompleted:
		// 完成
	}

	return nil
}

// ========== 辅助函数 ==========

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return err
		}

		return os.Chmod(dstPath, info.Mode())
	})
}
