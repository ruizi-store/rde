// Package system 提供系统信息模块
package system

import (
	"time"
)

// SystemInfo 系统信息
type SystemInfo struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Platform     string `json:"platform"`
	Arch         string `json:"arch"`
	KernelVersion string `json:"kernel_version"`
	Uptime       uint64 `json:"uptime"`       // 秒
	BootTime     uint64 `json:"boot_time"`    // Unix 时间戳
	Procs        uint64 `json:"procs"`        // 进程数
}

// CPUInfo CPU 信息
type CPUInfo struct {
	ModelName   string  `json:"model_name"`
	Cores       int     `json:"cores"`       // 物理核心数
	Threads     int     `json:"threads"`     // 逻辑核心数
	MHz         float64 `json:"mhz"`
	CacheSize   int32   `json:"cache_size"`  // KB
	Usage       float64 `json:"usage"`       // 百分比
	Temperature int     `json:"temperature"` // 摄氏度，-1 表示不可用
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Total       uint64  `json:"total"`       // 字节
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	Available   uint64  `json:"available"`
	UsedPercent float64 `json:"used_percent"`
	SwapTotal   uint64  `json:"swap_total"`
	SwapUsed    uint64  `json:"swap_used"`
	SwapFree    uint64  `json:"swap_free"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`       // 字节
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
	FSType      string  `json:"fs_type"`
	MountPoint  string  `json:"mount_point"`
}

// NetworkInterface 网络接口信息
type NetworkInterface struct {
	Name       string   `json:"name"`
	MacAddress string   `json:"mac_address"`
	IPv4       []string `json:"ipv4"`
	IPv6       []string `json:"ipv6"`
	BytesSent  uint64   `json:"bytes_sent"`
	BytesRecv  uint64   `json:"bytes_recv"`
	State      string   `json:"state"` // up, down
}

// NetworkStats 网络统计
type NetworkStats struct {
	Interface string `json:"interface"`
	BytesSent uint64 `json:"bytes_sent"`
	BytesRecv uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
	ErrorsIn    uint64 `json:"errors_in"`
	ErrorsOut   uint64 `json:"errors_out"`
}

// DeviceInfo 设备信息
type DeviceInfo struct {
	OSVersion   string   `json:"os_version"`
	DeviceName  string   `json:"device_name"`
	DeviceModel string   `json:"device_model"`
	DeviceSN    string   `json:"device_sn"`
	LanIPv4     []string `json:"lan_ipv4"`
	MacAddress  string   `json:"mac_address"`
	Initialized bool     `json:"initialized"`
	Port        int      `json:"port"`
}

// ResourceUsage 资源使用情况（用于实时监控）
type ResourceUsage struct {
	Timestamp   time.Time `json:"timestamp"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	DiskUsage   float64   `json:"disk_usage"`
	NetworkRx   uint64    `json:"network_rx"` // bytes/s
	NetworkTx   uint64    `json:"network_tx"` // bytes/s
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float32 `json:"mem_percent"`
	Status     string  `json:"status"`
	Username   string  `json:"username"`
	CreateTime int64   `json:"create_time"`
}

// TimeZoneInfo 时区信息
type TimeZoneInfo struct {
	Name   string `json:"name"`
	Offset int    `json:"offset"` // UTC 偏移（秒）
}

// PowerInfo 电源信息
type PowerInfo struct {
	CPUPower    string `json:"cpu_power,omitempty"`
	BatteryPct  int    `json:"battery_pct,omitempty"`  // -1 表示无电池
	IsCharging  bool   `json:"is_charging,omitempty"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	Status      string            `json:"status"` // healthy, warning, critical
	Checks      map[string]string `json:"checks"` // 各项检查结果
	LastChecked time.Time         `json:"last_checked"`
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
}
