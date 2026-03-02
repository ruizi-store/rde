// Package system 提供系统信息服务
package system

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ruizi-store/rde/backend/core/module"
	"github.com/ruizi-store/rde/backend/model"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"go.uber.org/zap"
)

// Service 系统信息服务
type Service struct {
	logger   *zap.Logger
	version  string
	dataPath string
	eventBus module.EventBus
}

// NewService 创建系统服务
func NewService(logger *zap.Logger, version, dataPath string) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		logger:   logger,
		version:  version,
		dataPath: dataPath,
	}
}

// SetEventBus 设置事件总线
func (s *Service) SetEventBus(eventBus module.EventBus) {
	s.eventBus = eventBus
}

// publishEvent 发布事件
func (s *Service) publishEvent(eventType string, data interface{}) {
	if s.eventBus != nil {
		s.eventBus.Publish(eventType, data)
	}
}

// GetSystemInfo 获取系统信息
func (s *Service) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	info, err := host.Info()
	if err != nil {
		return nil, err
	}

	return &SystemInfo{
		Hostname:      info.Hostname,
		OS:            info.OS,
		Platform:      info.Platform,
		Arch:          runtime.GOARCH,
		KernelVersion: info.KernelVersion,
		Uptime:        info.Uptime,
		BootTime:      info.BootTime,
		Procs:         info.Procs,
	}, nil
}

// GetCPUInfo 获取 CPU 信息
func (s *Service) GetCPUInfo(ctx context.Context) (*CPUInfo, error) {
	infos, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	// 获取 CPU 使用率
	percentages, err := cpu.Percent(0, false)
	usage := 0.0
	if err == nil && len(percentages) > 0 {
		usage = percentages[0]
	}

	// 获取核心数
	physicalCores, _ := cpu.Counts(false)
	logicalCores, _ := cpu.Counts(true)

	result := &CPUInfo{
		Cores:       physicalCores,
		Threads:     logicalCores,
		Usage:       parseFloat(usage, 1),
		Temperature: getCPUTemperature(),
	}

	if len(infos) > 0 {
		result.ModelName = infos[0].ModelName
		result.MHz = infos[0].Mhz
		result.CacheSize = infos[0].CacheSize
	}

	return result, nil
}

// GetMemoryInfo 获取内存信息
func (s *Service) GetMemoryInfo(ctx context.Context) (*MemoryInfo, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	swap, _ := mem.SwapMemory()

	result := &MemoryInfo{
		Total:       vm.Total,
		Used:        vm.Used,
		Free:        vm.Free,
		Available:   vm.Available,
		UsedPercent: parseFloat(vm.UsedPercent, 1),
	}

	if swap != nil {
		result.SwapTotal = swap.Total
		result.SwapUsed = swap.Used
		result.SwapFree = swap.Free
	}

	return result, nil
}

// GetDiskInfo 获取磁盘信息
func (s *Service) GetDiskInfo(ctx context.Context, path string) (*DiskInfo, error) {
	if path == "" {
		path = "/"
		if runtime.GOOS == "windows" {
			path = "C:"
		}
	}

	usage, err := disk.Usage(path)
	if err != nil {
		return nil, err
	}

	return &DiskInfo{
		Path:        usage.Path,
		Total:       usage.Total,
		Used:        usage.Used,
		Free:        usage.Free,
		UsedPercent: parseFloat(usage.UsedPercent, 1),
		FSType:      usage.Fstype,
		MountPoint:  path,
	}, nil
}

// GetAllDisks 获取所有磁盘信息
func (s *Service) GetAllDisks(ctx context.Context) ([]DiskInfo, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var disks []DiskInfo
	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}

		disks = append(disks, DiskInfo{
			Path:        p.Device,
			Total:       usage.Total,
			Used:        usage.Used,
			Free:        usage.Free,
			UsedPercent: parseFloat(usage.UsedPercent, 1),
			FSType:      p.Fstype,
			MountPoint:  p.Mountpoint,
		})
	}

	return disks, nil
}

// GetNetworkInterfaces 获取网络接口列表
func (s *Service) GetNetworkInterfaces(ctx context.Context, physicalOnly bool) ([]NetworkInterface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	stats, _ := psnet.IOCounters(true)
	statsMap := make(map[string]psnet.IOCountersStat)
	for _, s := range stats {
		statsMap[s.Name] = s
	}

	var result []NetworkInterface
	for _, iface := range interfaces {
		// 过滤物理网卡
		if physicalOnly && !isPhysicalInterface(iface.Name) {
			continue
		}

		ni := NetworkInterface{
			Name:       iface.Name,
			MacAddress: iface.HardwareAddr.String(),
			State:      "down",
		}

		// 检查接口状态
		if iface.Flags&net.FlagUp != 0 {
			ni.State = "up"
		}

		// 获取 IP 地址
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				ip, _, _ := net.ParseCIDR(addr.String())
				if ip != nil {
					if ip.To4() != nil {
						ni.IPv4 = append(ni.IPv4, ip.String())
					} else {
						ni.IPv6 = append(ni.IPv6, ip.String())
					}
				}
			}
		}

		// 获取流量统计
		if stat, ok := statsMap[iface.Name]; ok {
			ni.BytesSent = stat.BytesSent
			ni.BytesRecv = stat.BytesRecv
		}

		result = append(result, ni)
	}

	return result, nil
}

// GetNetworkStats 获取网络统计
func (s *Service) GetNetworkStats(ctx context.Context) ([]NetworkStats, error) {
	stats, err := psnet.IOCounters(true)
	if err != nil {
		return nil, err
	}

	var result []NetworkStats
	for _, stat := range stats {
		result = append(result, NetworkStats{
			Interface:   stat.Name,
			BytesSent:   stat.BytesSent,
			BytesRecv:   stat.BytesRecv,
			PacketsSent: stat.PacketsSent,
			PacketsRecv: stat.PacketsRecv,
			ErrorsIn:    stat.Errin,
			ErrorsOut:   stat.Errout,
		})
	}

	return result, nil
}

// GetDeviceInfo 获取设备信息
func (s *Service) GetDeviceInfo(ctx context.Context) (*DeviceInfo, error) {
	info := &DeviceInfo{
		OSVersion: s.version,
		Port:      3080,
	}

	// 获取主机名
	hostInfo, err := host.Info()
	if err == nil {
		info.DeviceName = hostInfo.Hostname
	}

	// 获取 IP 地址
	interfaces, _ := s.GetNetworkInterfaces(ctx, true)
	for _, iface := range interfaces {
		if iface.State == "up" && len(iface.IPv4) > 0 {
			info.LanIPv4 = append(info.LanIPv4, iface.IPv4...)
		}
	}

	// 获取 MAC 地址
	mac, err := s.GetMacAddress(ctx)
	if err == nil {
		info.MacAddress = mac
	}

	// 读取设备型号和序列号
	osRelease := readOSRelease()
	info.DeviceModel = osRelease["MODEL"]
	info.DeviceSN = osRelease["SN"]

	return info, nil
}

// GetResourceUsage 获取当前资源使用情况
func (s *Service) GetResourceUsage(ctx context.Context) (*ResourceUsage, error) {
	usage := &ResourceUsage{
		Timestamp: time.Now(),
	}

	// CPU
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		usage.CPUUsage = parseFloat(cpuPercent[0], 1)
	}

	// Memory
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		usage.MemoryUsage = parseFloat(memInfo.UsedPercent, 1)
	}

	// Disk
	diskInfo, err := disk.Usage("/")
	if err == nil {
		usage.DiskUsage = parseFloat(diskInfo.UsedPercent, 1)
	}

	return usage, nil
}

// GetCPUTemperature 获取 CPU 温度
func (s *Service) GetCPUTemperature(ctx context.Context) (float64, error) {
	temp := getCPUTemperature()
	if temp == -1 {
		return 0, fmt.Errorf("temperature sensor not available")
	}
	return float64(temp), nil
}

// getCPUTemperature 内部函数获取 CPU 温度
func getCPUTemperature() int {
	// 尝试从 /sys/class/thermal 读取
	paths := []string{
		"/sys/class/thermal/thermal_zone0/temp",
		"/sys/class/hwmon/hwmon0/temp1_input",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			temp, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil {
				return temp / 1000 // 转换为摄氏度
			}
		}
	}

	return -1 // 不可用
}

// GetMacAddress 获取 MAC 地址
func (s *Service) GetMacAddress(ctx context.Context) (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	// 优先获取物理网卡
	for _, iface := range interfaces {
		if isPhysicalInterface(iface.Name) && len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String(), nil
		}
	}

	// 获取任意网卡
	for _, iface := range interfaces {
		if len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String(), nil
		}
	}

	// 使用主机名作为后备
	hostname, _ := os.Hostname()
	if hostname != "" {
		return hostname, nil
	}

	return "unknown", nil
}

// GetTimeZone 获取时区
func (s *Service) GetTimeZone(ctx context.Context) *TimeZoneInfo {
	zone, offset := time.Now().Zone()
	return &TimeZoneInfo{
		Name:   zone,
		Offset: offset,
	}
}

// GetTopProcesses 获取 CPU/内存占用最高的进程
func (s *Service) GetTopProcesses(ctx context.Context, limit int, sortBy string) ([]ProcessInfo, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var result []ProcessInfo
	for _, p := range procs {
		info := ProcessInfo{PID: p.Pid}

		info.Name, _ = p.Name()
		info.CPUPercent, _ = p.CPUPercent()
		info.MemPercent, _ = p.MemoryPercent()
		statuses, _ := p.Status()
		if len(statuses) > 0 {
			info.Status = statuses[0]
		}
		info.Username, _ = p.Username()
		info.CreateTime, _ = p.CreateTime()

		result = append(result, info)
	}

	// 简单排序（按 CPU 或内存）
	if sortBy == "memory" {
		sortByMemory(result)
	} else {
		sortByCPU(result)
	}

	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result, nil
}

// Reboot 重启系统
func (s *Service) Reboot(ctx context.Context) error {
	s.logger.Info("System reboot requested")

	// 发布系统重启事件
	s.publishEvent("system.reboot", map[string]interface{}{
		"time": time.Now().Format(time.RFC3339),
	})

	cmd := exec.Command("reboot")
	return cmd.Run()
}

// Shutdown 关机
func (s *Service) Shutdown(ctx context.Context) error {
	s.logger.Info("System shutdown requested")

	// 发布系统关机事件
	s.publishEvent("system.shutdown", map[string]interface{}{
		"time": time.Now().Format(time.RFC3339),
	})

	cmd := exec.Command("shutdown", "-h", "now")
	return cmd.Run()
}

// GetLogs 获取系统日志
func (s *Service) GetLogs(ctx context.Context, logFile string, lines int) ([]string, error) {
	if logFile == "" {
		logFile = filepath.Join(s.dataPath, "logs", "rde.log")
	}

	file, err := os.Open(logFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var result []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	// 返回最后 N 行
	if lines > 0 && lines < len(result) {
		result = result[len(result)-lines:]
	}

	return result, scanner.Err()
}

// GetHealthStatus 获取健康状态
func (s *Service) GetHealthStatus(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		Status:      "healthy",
		Checks:      make(map[string]string),
		LastChecked: time.Now(),
	}

	// CPU 检查
	cpuPercent, _ := cpu.Percent(0, false)
	if len(cpuPercent) > 0 {
		if cpuPercent[0] > 90 {
			status.Checks["cpu"] = "critical"
			status.Status = "critical"
		} else if cpuPercent[0] > 70 {
			status.Checks["cpu"] = "warning"
			if status.Status == "healthy" {
				status.Status = "warning"
			}
		} else {
			status.Checks["cpu"] = "ok"
		}
	}

	// 内存检查
	memInfo, _ := mem.VirtualMemory()
	if memInfo != nil {
		if memInfo.UsedPercent > 90 {
			status.Checks["memory"] = "critical"
			status.Status = "critical"
		} else if memInfo.UsedPercent > 80 {
			status.Checks["memory"] = "warning"
			if status.Status == "healthy" {
				status.Status = "warning"
			}
		} else {
			status.Checks["memory"] = "ok"
		}
	}

	// 磁盘检查
	diskInfo, _ := disk.Usage("/")
	if diskInfo != nil {
		if diskInfo.UsedPercent > 95 {
			status.Checks["disk"] = "critical"
			status.Status = "critical"
		} else if diskInfo.UsedPercent > 85 {
			status.Checks["disk"] = "warning"
			if status.Status == "healthy" {
				status.Status = "warning"
			}
		} else {
			status.Checks["disk"] = "ok"
		}
	}

	return status
}

// 辅助函数

func parseFloat(f float64, precision int) float64 {
	format := fmt.Sprintf("%%.%df", precision)
	str := fmt.Sprintf(format, f)
	result, _ := strconv.ParseFloat(str, 64)
	return result
}

func isPhysicalInterface(name string) bool {
	// 排除虚拟接口
	virtuals := []string{"lo", "docker", "veth", "br-", "virbr", "vnet"}
	for _, v := range virtuals {
		if strings.HasPrefix(name, v) {
			return false
		}
	}
	return true
}

func readOSRelease() map[string]string {
	result := make(map[string]string)
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return result
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := parts[0]
			value := strings.Trim(parts[1], "\"")
			result[key] = value
		}
	}
	return result
}

func sortByCPU(procs []ProcessInfo) {
	for i := 0; i < len(procs)-1; i++ {
		for j := i + 1; j < len(procs); j++ {
			if procs[j].CPUPercent > procs[i].CPUPercent {
				procs[i], procs[j] = procs[j], procs[i]
			}
		}
	}
}

func sortByMemory(procs []ProcessInfo) {
	for i := 0; i < len(procs)-1; i++ {
		for j := i + 1; j < len(procs); j++ {
			if procs[j].MemPercent > procs[i].MemPercent {
				procs[i], procs[j] = procs[j], procs[i]
			}
		}
	}
}

// SSHStatus SSH 服务状态
type SSHStatus struct {
	Running bool   `json:"running"`
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Message string `json:"message,omitempty"`
}

// GetSSHStatus 获取 SSH 服务状态
func (s *Service) GetSSHStatus(ctx context.Context) (*SSHStatus, error) {
	status := &SSHStatus{
		Port: 22,
	}

	// 检查服务是否运行
	checkCmd := exec.Command("systemctl", "is-active", "ssh")
	if output, err := checkCmd.CombinedOutput(); err == nil {
		status.Running = strings.TrimSpace(string(output)) == "active"
	} else {
		// 尝试 sshd
		checkCmd = exec.Command("systemctl", "is-active", "sshd")
		if output, err := checkCmd.CombinedOutput(); err == nil {
			status.Running = strings.TrimSpace(string(output)) == "active"
		}
	}

	// 检查是否开机自启
	enabledCmd := exec.Command("systemctl", "is-enabled", "ssh")
	if output, err := enabledCmd.CombinedOutput(); err == nil {
		status.Enabled = strings.TrimSpace(string(output)) == "enabled"
	} else {
		enabledCmd = exec.Command("systemctl", "is-enabled", "sshd")
		if output, err := enabledCmd.CombinedOutput(); err == nil {
			status.Enabled = strings.TrimSpace(string(output)) == "enabled"
		}
	}

	// 读取端口配置
	if content, err := os.ReadFile("/etc/ssh/sshd_config"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Port ") {
				if port, err := strconv.Atoi(strings.TrimPrefix(trimmed, "Port ")); err == nil {
					status.Port = port
				}
			}
		}
	}

	return status, nil
}

// EnableSSH 启用 SSH 服务
func (s *Service) EnableSSH(ctx context.Context) error {
	s.logger.Info("Enabling SSH service")

	// 启动服务
	startCmd := exec.Command("systemctl", "start", "ssh")
	if output, err := startCmd.CombinedOutput(); err != nil {
		// 尝试 sshd
		startCmd = exec.Command("systemctl", "start", "sshd")
		if output2, err2 := startCmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("failed to start SSH: %s / %s", string(output), string(output2))
		}
	}

	s.logger.Info("SSH service enabled")

	// 发布 SSH 启用事件
	s.publishEvent("system.ssh.enabled", map[string]interface{}{
		"time": time.Now().Format(time.RFC3339),
	})

	return nil
}

// DisableSSH 禁用 SSH 服务
func (s *Service) DisableSSH(ctx context.Context) error {
	s.logger.Info("Disabling SSH service")

	// 停止服务
	stopCmd := exec.Command("systemctl", "stop", "ssh")
	if output, err := stopCmd.CombinedOutput(); err != nil {
		stopCmd = exec.Command("systemctl", "stop", "sshd")
		if output2, err2 := stopCmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("failed to stop SSH: %s / %s", string(output), string(output2))
		}
	}

	s.logger.Info("SSH service disabled")

	// 发布 SSH 禁用事件
	s.publishEvent("system.ssh.disabled", map[string]interface{}{
		"time": time.Now().Format(time.RFC3339),
	})

	return nil
}

// SetSSHAutoStart 设置 SSH 开机自启
func (s *Service) SetSSHAutoStart(ctx context.Context, enabled bool) error {
	if enabled {
		s.logger.Info("Enabling SSH autostart")
		enableCmd := exec.Command("systemctl", "enable", "ssh")
		if _, err := enableCmd.CombinedOutput(); err != nil {
			enableCmd = exec.Command("systemctl", "enable", "sshd")
			if _, err := enableCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to enable SSH autostart: %v", err)
			}
		}
	} else {
		s.logger.Info("Disabling SSH autostart")
		disableCmd := exec.Command("systemctl", "disable", "ssh")
		disableCmd.Run()
		disableCmd = exec.Command("systemctl", "disable", "sshd")
		disableCmd.Run()
	}
	return nil
}

// RemoteAccessSettings 远程访问设置
type RemoteAccessSettings struct {
	SSHEnabled      bool `json:"ssh_enabled"`
	SSHRunning      bool `json:"ssh_running"`
	SSHPort         int  `json:"ssh_port"`
	TerminalEnabled bool `json:"terminal_enabled"`
}

// remoteAccessConfigFile 配置文件路径
func (s *Service) remoteAccessConfigFile() string {
	return filepath.Join(s.dataPath, "remote_access.json")
}

// GetRemoteAccessSettings 获取远程访问设置
func (s *Service) GetRemoteAccessSettings(ctx context.Context) (*RemoteAccessSettings, error) {
	settings := &RemoteAccessSettings{
		SSHEnabled:      false,
		TerminalEnabled: false,
		SSHPort:         22,
	}

	// 读取配置文件
	configFile := s.remoteAccessConfigFile()
	configExists := true
	if data, err := os.ReadFile(configFile); err == nil {
		if err := json.Unmarshal(data, settings); err != nil {
			s.logger.Warn("Failed to parse remote access config", zap.Error(err))
		}
	} else {
		configExists = false
	}

	// 获取 SSH 实际运行状态
	sshStatus, err := s.GetSSHStatus(ctx)
	if err == nil {
		settings.SSHRunning = sshStatus.Running
		settings.SSHPort = sshStatus.Port

		// 配置文件不存在时，从系统实际状态初始化
		if !configExists {
			settings.SSHEnabled = sshStatus.Running || sshStatus.Enabled
			// 持久化，避免下次再不一致
			if saveErr := s.saveRemoteAccessSettings(settings); saveErr != nil {
				s.logger.Warn("Failed to save initial remote access settings", zap.Error(saveErr))
			}
		}
	}

	return settings, nil
}

// SetSSHEnabled 设置 SSH 启用状态
func (s *Service) SetSSHEnabled(ctx context.Context, enabled bool) error {
	settings, err := s.GetRemoteAccessSettings(ctx)
	if err != nil {
		return err
	}

	settings.SSHEnabled = enabled

	// 根据启用状态控制 SSH 服务
	if enabled {
		if err := s.EnableSSH(ctx); err != nil {
			return fmt.Errorf("failed to enable SSH service: %w", err)
		}
		// 设置开机自启
		if err := s.SetSSHAutoStart(ctx, true); err != nil {
			s.logger.Warn("Failed to set SSH autostart", zap.Error(err))
		}
	} else {
		if err := s.DisableSSH(ctx); err != nil {
			return fmt.Errorf("failed to disable SSH service: %w", err)
		}
		// 禁用开机自启
		if err := s.SetSSHAutoStart(ctx, false); err != nil {
			s.logger.Warn("Failed to disable SSH autostart", zap.Error(err))
		}
	}

	// 保存配置
	return s.saveRemoteAccessSettings(settings)
}

// SetTerminalEnabled 设置终端启用状态
func (s *Service) SetTerminalEnabled(ctx context.Context, enabled bool) error {
	settings, err := s.GetRemoteAccessSettings(ctx)
	if err != nil {
		return err
	}

	settings.TerminalEnabled = enabled

	// 保存配置
	return s.saveRemoteAccessSettings(settings)
}

// IsTerminalEnabled 检查终端是否启用
func (s *Service) IsTerminalEnabled() bool {
	configFile := s.remoteAccessConfigFile()
	data, err := os.ReadFile(configFile)
	if err != nil {
		return false // 默认关闭
	}

	var settings RemoteAccessSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}

	return settings.TerminalEnabled
}

// saveRemoteAccessSettings 保存远程访问设置
func (s *Service) saveRemoteAccessSettings(settings *RemoteAccessSettings) error {
	configFile := s.remoteAccessConfigFile()

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	s.logger.Info("Remote access settings saved",
		zap.Bool("sshEnabled", settings.SSHEnabled),
		zap.Bool("terminalEnabled", settings.TerminalEnabled),
	)

	return nil
}

// ==================== 代理设置 ====================

// proxyConfigFile 获取代理配置文件路径
func (s *Service) proxyConfigFile() string {
	return filepath.Join(s.dataPath, "config", "proxy.json")
}

// GetProxyConfig 获取代理配置
func (s *Service) GetProxyConfig(ctx context.Context) (*model.SystemProxyConfig, error) {
	configFile := s.proxyConfigFile()
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 返回默认配置
			return &model.SystemProxyConfig{
				Mode:    model.ProxyModeOff,
				NoProxy: "localhost,127.0.0.1,*.local",
			}, nil
		}
		return nil, fmt.Errorf("failed to read proxy config: %w", err)
	}

	var config model.SystemProxyConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse proxy config: %w", err)
	}

	// 不返回密码明文
	if config.Auth != nil {
		config.Auth.Password = ""
	}

	return &config, nil
}

// SaveProxyConfig 保存代理配置
func (s *Service) SaveProxyConfig(ctx context.Context, config *model.SystemProxyConfig) error {
	configFile := s.proxyConfigFile()

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 如果密码为空但有旧配置，保留旧密码
	if config.Auth != nil && config.Auth.Password == "" {
		oldConfig, _ := s.getProxyConfigInternal()
		if oldConfig != nil && oldConfig.Auth != nil {
			config.Auth.Password = oldConfig.Auth.Password
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal proxy config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write proxy config: %w", err)
	}

	s.logger.Info("Proxy config saved", zap.String("mode", string(config.Mode)))

	// 应用代理配置到系统
	if err := s.applyProxyToSystem(config); err != nil {
		s.logger.Warn("Failed to apply proxy to system", zap.Error(err))
	}

	return nil
}

// getProxyConfigInternal 内部获取代理配置（包含密码）
func (s *Service) getProxyConfigInternal() (*model.SystemProxyConfig, error) {
	configFile := s.proxyConfigFile()
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var config model.SystemProxyConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// applyProxyToSystem 应用代理配置到系统
func (s *Service) applyProxyToSystem(config *model.SystemProxyConfig) error {
	proxyScriptPath := "/etc/profile.d/rde-proxy.sh"

	if config.Mode == model.ProxyModeOff {
		// 删除代理脚本
		if err := os.Remove(proxyScriptPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove proxy script: %w", err)
		}
		// 关闭时清理 Docker 代理和 Mirror
		s.removeDockerProxy()
		s.removeDockerMirror()
		return nil
	}

	// 生成系统代理环境变量脚本
	var content strings.Builder
	content.WriteString("#!/bin/bash\n")
	content.WriteString("# RDE System Proxy Configuration\n")
	content.WriteString("# Auto-generated, do not edit manually\n\n")

	if config.HttpProxy != "" {
		content.WriteString(fmt.Sprintf("export http_proxy=\"%s\"\n", config.HttpProxy))
		content.WriteString(fmt.Sprintf("export HTTP_PROXY=\"%s\"\n", config.HttpProxy))
	}
	if config.HttpsProxy != "" {
		content.WriteString(fmt.Sprintf("export https_proxy=\"%s\"\n", config.HttpsProxy))
		content.WriteString(fmt.Sprintf("export HTTPS_PROXY=\"%s\"\n", config.HttpsProxy))
	}
	if config.Socks5 != "" {
		content.WriteString(fmt.Sprintf("export all_proxy=\"%s\"\n", config.Socks5))
		content.WriteString(fmt.Sprintf("export ALL_PROXY=\"%s\"\n", config.Socks5))
	}
	if config.NoProxy != "" {
		content.WriteString(fmt.Sprintf("export no_proxy=\"%s\"\n", config.NoProxy))
		content.WriteString(fmt.Sprintf("export NO_PROXY=\"%s\"\n", config.NoProxy))
	}

	if err := os.WriteFile(proxyScriptPath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("failed to write proxy script: %w", err)
	}

	// Docker Hub Registry Mirror
	if config.DockerMirror != "" {
		if err := s.configureDockerMirror(config.DockerMirror); err != nil {
			s.logger.Warn("Failed to configure Docker mirror", zap.Error(err))
		} else {
			s.logger.Info("Docker mirror configured", zap.String("mirror", config.DockerMirror))
		}
	} else {
		s.removeDockerMirror()
	}

	// Docker daemon HTTP/HTTPS 代理（用于拉取 ghcr.io 等非 Hub 镜像）
	if config.DockerProxyEnabled {
		if err := s.configureDockerProxy(config); err != nil {
			s.logger.Warn("Failed to configure Docker proxy", zap.Error(err))
		} else {
			s.logger.Info("Docker proxy configured")
		}
	} else {
		s.removeDockerProxy()
	}

	return nil
}

// configureDockerMirror 配置 Docker daemon 的 registry-mirrors
func (s *Service) configureDockerMirror(mirrorURL string) error {
	daemonConfigPath := "/etc/docker/daemon.json"

	// 读取现有配置
	var daemonConfig map[string]interface{}
	data, err := os.ReadFile(daemonConfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read daemon.json: %w", err)
		}
		daemonConfig = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &daemonConfig); err != nil {
			return fmt.Errorf("parse daemon.json: %w", err)
		}
	}

	// 设置 registry-mirrors（合并而非覆盖，避免与其他镜像源冲突）
	existingMirrors := []string{}
	if existing, ok := daemonConfig["registry-mirrors"]; ok {
		if arr, ok := existing.([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok && s != mirrorURL {
					existingMirrors = append(existingMirrors, s)
				}
			}
		}
	}
	// 新镜像放在最前面（优先使用）
	daemonConfig["registry-mirrors"] = append([]string{mirrorURL}, existingMirrors...)

	// 写回配置
	newData, err := json.MarshalIndent(daemonConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal daemon.json: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(daemonConfigPath), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(daemonConfigPath, newData, 0644); err != nil {
		return fmt.Errorf("write daemon.json: %w", err)
	}

	// 重载 Docker daemon 配置（SIGHUP，不重启容器）
	cmd := exec.Command("systemctl", "reload", "docker")
	if err := cmd.Run(); err != nil {
		s.logger.Warn("Failed to reload docker daemon", zap.Error(err))
	}

	return nil
}

// removeDockerMirror 移除 Docker daemon 的 registry-mirrors
func (s *Service) removeDockerMirror() {
	daemonConfigPath := "/etc/docker/daemon.json"
	data, err := os.ReadFile(daemonConfigPath)
	if err != nil {
		return
	}

	var daemonConfig map[string]interface{}
	if json.Unmarshal(data, &daemonConfig) != nil {
		return
	}

	if _, ok := daemonConfig["registry-mirrors"]; !ok {
		return
	}

	delete(daemonConfig, "registry-mirrors")
	newData, _ := json.MarshalIndent(daemonConfig, "", "  ")
	if err := os.WriteFile(daemonConfigPath, newData, 0644); err != nil {
		s.logger.Warn("Failed to update daemon.json", zap.Error(err))
		return
	}
	exec.Command("systemctl", "reload", "docker").Run()
}

// configureDockerProxy 配置 Docker daemon 的 HTTP/HTTPS 代理
// 写入 /etc/systemd/system/docker.service.d/http-proxy.conf
func (s *Service) configureDockerProxy(config *model.SystemProxyConfig) error {
	proxyDir := "/etc/systemd/system/docker.service.d"
	proxyFile := filepath.Join(proxyDir, "http-proxy.conf")

	if err := os.MkdirAll(proxyDir, 0755); err != nil {
		return fmt.Errorf("create docker proxy dir: %w", err)
	}

	var content strings.Builder
	content.WriteString("[Service]\n")
	if config.HttpProxy != "" {
		content.WriteString(fmt.Sprintf("Environment=\"HTTP_PROXY=%s\"\n", config.HttpProxy))
	}
	if config.HttpsProxy != "" {
		content.WriteString(fmt.Sprintf("Environment=\"HTTPS_PROXY=%s\"\n", config.HttpsProxy))
	}
	if config.NoProxy != "" {
		content.WriteString(fmt.Sprintf("Environment=\"NO_PROXY=%s\"\n", config.NoProxy))
	}

	if err := os.WriteFile(proxyFile, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("write docker proxy config: %w", err)
	}

	// 重载 systemd 并重启 Docker
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		s.logger.Warn("Failed to daemon-reload", zap.Error(err))
	}
	if err := exec.Command("systemctl", "restart", "docker").Run(); err != nil {
		return fmt.Errorf("restart docker: %w", err)
	}

	return nil
}

// removeDockerProxy 移除 Docker daemon 的 HTTP 代理配置
func (s *Service) removeDockerProxy() {
	proxyFile := "/etc/systemd/system/docker.service.d/http-proxy.conf"
	if err := os.Remove(proxyFile); err != nil {
		if !os.IsNotExist(err) {
			s.logger.Warn("Failed to remove docker proxy config", zap.Error(err))
		}
		return
	}

	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "restart", "docker").Run()
	s.logger.Info("Docker proxy config removed")
}

// TestProxy 测试代理连接
func (s *Service) TestProxy(ctx context.Context, req *model.ProxyTestRequest) (*model.ProxyTestResponse, error) {
	if req.ProxyUrl == "" {
		return &model.ProxyTestResponse{
			Success: false,
			Message: "代理地址不能为空",
		}, nil
	}

	if req.TestUrl == "" {
		req.TestUrl = "https://www.google.com"
	}

	start := time.Now()

	// 使用 curl 测试代理
	cmd := exec.CommandContext(ctx, "curl", "-x", req.ProxyUrl, "-s", "-o", "/dev/null", "-w", "%{http_code}", "--connect-timeout", "10", req.TestUrl)
	output, err := cmd.Output()
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return &model.ProxyTestResponse{
			Success: false,
			Message: "连接失败: " + err.Error(),
			Latency: latency,
		}, nil
	}

	statusCode := strings.TrimSpace(string(output))
	if statusCode == "200" || statusCode == "301" || statusCode == "302" {
		return &model.ProxyTestResponse{
			Success: true,
			Message: fmt.Sprintf("连接成功 (HTTP %s)", statusCode),
			Latency: latency,
		}, nil
	}

	return &model.ProxyTestResponse{
		Success: false,
		Message: fmt.Sprintf("连接失败 (HTTP %s)", statusCode),
		Latency: latency,
	}, nil
}
