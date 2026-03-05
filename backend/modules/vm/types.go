// Package vm 虚拟机类型定义
package vm

import "time"

// VMStatus 虚拟机状态
type VMStatus string

const (
	VMStatusRunning  VMStatus = "running"
	VMStatusStopped  VMStatus = "stopped"
	VMStatusPaused   VMStatus = "paused"
	VMStatusCreating VMStatus = "creating"
	VMStatusError    VMStatus = "error"
)

// VM 虚拟机
type VM struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	Status       VMStatus      `json:"status"`
	CPU          int           `json:"cpu"`
	Memory       int64         `json:"memory"` // MB
	DiskSize     int64         `json:"disk_size,omitempty"` // GB
	DiskPath     string        `json:"disk_path,omitempty"`
	ISOPath      string        `json:"iso_path,omitempty"`
	VNCPort      int           `json:"vnc_port,omitempty"`
	SSHPort      int           `json:"ssh_port,omitempty"`
	QMPSocket    string        `json:"qmp_socket,omitempty"` // QMP Unix socket 路径
	PortForwards []PortForward `json:"port_forwards,omitempty"` // 端口转发配置
	USBDevices   []USBDevice   `json:"usb_devices,omitempty"`   // USB 设备直通
	NetworkMode  string        `json:"network_mode,omitempty"`  // user, bridge, nat
	BridgeIface  string        `json:"bridge_iface,omitempty"`  // 桥接网卡名
	// 性能优化选项
	CPUModel     string `json:"cpu_model,omitempty"`     // host, host-passthrough, qemu64 等
	EnableHuge   bool   `json:"enable_huge,omitempty"`   // 启用大页内存
	IOThread     bool   `json:"io_thread,omitempty"`     // 启用 I/O 线程
	CPUPinning   []int  `json:"cpu_pinning,omitempty"`   // CPU 绑定核心
	VirtioBlk    bool   `json:"virtio_blk,omitempty"`    // 使用 virtio-blk 驱动
	// P5: 自动化配置
	AutoStart    bool   `json:"auto_start,omitempty"`    // 系统启动时自动运行
	StartOrder   int    `json:"start_order,omitempty"`   // 启动顺序 (小优先)
	StartDelay   int    `json:"start_delay,omitempty"`   // 启动延迟秒数
	OS           string `json:"os,omitempty"` // 操作系统名称
	OSType       string `json:"os_type,omitempty"`
	Arch         string `json:"arch,omitempty"`
	Accelerator  string `json:"accelerator,omitempty"` // kvm, tcg
	Network      string `json:"network,omitempty"`
	Display      string `json:"display,omitempty"`
	PID          int    `json:"pid,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// USBDevice USB 设备
type USBDevice struct {
	VendorID  string `json:"vendor_id"`  // USB Vendor ID (如 0x1234)
	ProductID string `json:"product_id"` // USB Product ID (如 0x5678)
	Name      string `json:"name,omitempty"`
	Bus       int    `json:"bus,omitempty"`
	Device    int    `json:"device,omitempty"`
}

// PortForward 端口转发配置
type PortForward struct {
	Name      string `json:"name,omitempty"`      // 名称，如 SSH, HTTP
	Protocol  string `json:"protocol,omitempty"` // tcp, udp
	HostPort  int    `json:"host_port"`           // 主机端口
	GuestPort int    `json:"guest_port"`          // 客户机端口
}

// VMConfig 虚拟机配置
type VMConfig struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	CPU         int    `json:"cpu"`
	Memory      int64  `json:"memory"` // MB
	DiskSize    int64  `json:"disk_size,omitempty"` // GB
	DiskPath    string `json:"disk_path,omitempty"`
	ISOPath     string `json:"iso_path,omitempty"`
	OSType      string `json:"os_type,omitempty"`
	Arch        string `json:"arch,omitempty"`
	Accelerator string `json:"accelerator,omitempty"`
	Network     string `json:"network,omitempty"`
	Display     string `json:"display,omitempty"`
}

// Snapshot 快照
type Snapshot struct {
	ID        string    `json:"id"`
	VMID      string    `json:"vm_id"`
	Name      string    `json:"name"`
	Tag       string    `json:"tag,omitempty"`
	Size      int64     `json:"size,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateVMRequest 创建虚拟机请求
type CreateVMRequest struct {
	Name         string        `json:"name" binding:"required"`
	Description  string        `json:"description,omitempty"`
	CPU          int           `json:"cpu"`
	Memory       int64         `json:"memory"`
	DiskSize     int64         `json:"disk_size"`
	ISOPath      string        `json:"iso_path,omitempty"`
	OSType       string        `json:"os_type,omitempty"`
	Arch         string        `json:"arch,omitempty"`
	Template     string        `json:"template,omitempty"`
	PortForwards []PortForward `json:"port_forwards,omitempty"`
	USBDevices   []USBDevice   `json:"usb_devices,omitempty"`
	NetworkMode  string        `json:"network_mode,omitempty"` // user, bridge
	BridgeIface  string        `json:"bridge_iface,omitempty"`
	// 性能优化
	CPUModel   string `json:"cpu_model,omitempty"`   // host, host-passthrough
	EnableHuge bool   `json:"enable_huge,omitempty"` // 大页内存
	IOThread   bool   `json:"io_thread,omitempty"`   // I/O 线程
	CPUPinning []int  `json:"cpu_pinning,omitempty"` // CPU 绑定
}

// UpdateVMRequest 更新虚拟机请求
type UpdateVMRequest struct {
	Name         string        `json:"name,omitempty"`
	Description  string        `json:"description,omitempty"`
	CPU          int           `json:"cpu,omitempty"`
	Memory       int64         `json:"memory,omitempty"`
	PortForwards []PortForward `json:"port_forwards,omitempty"`
	USBDevices   []USBDevice   `json:"usb_devices,omitempty"`
	NetworkMode  string        `json:"network_mode,omitempty"`
	BridgeIface  string        `json:"bridge_iface,omitempty"`
	// 性能优化
	CPUModel   string `json:"cpu_model,omitempty"`
	EnableHuge *bool  `json:"enable_huge,omitempty"`
	IOThread   *bool  `json:"io_thread,omitempty"`
	CPUPinning []int  `json:"cpu_pinning,omitempty"`
}

// CreateSnapshotRequest 创建快照请求
type CreateSnapshotRequest struct {
	Name string `json:"name" binding:"required"`
	Tag  string `json:"tag,omitempty"`
}

// ImportDiskRequest 导入磁盘请求
type ImportDiskRequest struct {
	Path   string `json:"path" binding:"required"`
	Format string `json:"format,omitempty"` // qcow2, raw, vmdk
}

// ResizeDiskRequest 调整磁盘大小请求
type ResizeDiskRequest struct {
	Size int64 `json:"size" binding:"required"` // GB
}

// VMTemplate 虚拟机模板
type VMTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	OS          string    `json:"os,omitempty"` // 操作系统名称
	OSType      string    `json:"os_type"`
	Arch        string    `json:"arch"`
	CPU         int       `json:"cpu"`
	Memory      int64     `json:"memory"`
	DiskSize    int64     `json:"disk_size"`
	BaseDisk    string    `json:"base_disk,omitempty"`  // 基础磁盘路径（自定义模板）
	IsCustom    bool      `json:"is_custom,omitempty"`  // 是否为自定义模板
	CreatedAt   time.Time `json:"created_at,omitempty"` // 创建时间
}

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	VMID        string `json:"vm_id" binding:"required"` // 从哪个 VM 创建模板
}

// BackupInfo 备份信息
type BackupInfo struct {
	ID          string    `json:"id"`
	VMID        string    `json:"vm_id"`
	VMName      string    `json:"vm_name"`
	Name        string    `json:"name,omitempty"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Compressed  bool      `json:"compressed"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description,omitempty"`
}

// CreateBackupRequest 创建备份请求
type CreateBackupRequest struct {
	VMID        string `json:"vm_id" binding:"required"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Compress    bool   `json:"compress,omitempty"`
}

// RestoreBackupRequest 还原备份请求
type RestoreBackupRequest struct {
	BackupID string `json:"backup_id" binding:"required"`
	NewName  string `json:"new_name,omitempty"` // 可选，还原为新 VM
}

// ISO ISO 镜像
type ISO struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// VMStats 虚拟机统计（基础）
type VMStats struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage int64   `json:"memory_usage"`
	DiskRead    int64   `json:"disk_read"`
	DiskWrite   int64   `json:"disk_write"`
	NetRx       int64   `json:"net_rx"`
	NetTx       int64   `json:"net_tx"`
}

// VMStatsDetail 虚拟机资源统计详情
type VMStatsDetail struct {
	VMID        string    `json:"vm_id"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryUsed  int64     `json:"memory_used"`
	MemoryTotal int64     `json:"memory_total"`
	DiskRead    int64     `json:"disk_read"`
	DiskWrite   int64     `json:"disk_write"`
	NetRx       int64     `json:"net_rx"`
	NetTx       int64     `json:"net_tx"`
	Uptime      int64     `json:"uptime"`
	Timestamp   time.Time `json:"timestamp"`
}

// VNCToken VNC 连接令牌
type VNCToken struct {
	Token     string    `json:"token"`
	VMID      string    `json:"vm_id"`
	VNCPort   int       `json:"vnc_port"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ISOFile ISO 文件信息（含修改时间）
type ISOFile struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// QEMUProcess QEMU 进程信息
type QEMUProcess struct {
	PID     int
	VMID    string
	VNCPort int
	SSHPort int
	Cmd     string
}

// ==================== P3: 批量操作和存储 ====================

// BatchResult 批量操作结果
type BatchResult struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// StorageInfo 存储使用信息
type StorageInfo struct {
	TotalSpace     int64 `json:"total_space"`      // 总空间 (bytes)
	UsedSpace      int64 `json:"used_space"`       // 已用空间
	FreeSpace      int64 `json:"free_space"`       // 可用空间
	VMDiskUsage    int64 `json:"vm_disk_usage"`    // VM 磁盘占用
	ISOUsage       int64 `json:"iso_usage"`        // ISO 占用
	BackupUsage    int64 `json:"backup_usage"`     // 备份占用
	TemplateUsage  int64 `json:"template_usage"`   // 模板占用
	SnapshotUsage  int64 `json:"snapshot_usage"`   // 快照占用
	VMCount        int   `json:"vm_count"`         // VM 数量
	RunningVMCount int   `json:"running_vm_count"` // 运行中 VM 数量
}

// ==================== P5: 导入导出和自动化 ====================

// ExportFormat 导出格式
type ExportFormat string

const (
	ExportFormatOVA   ExportFormat = "ova"   // OVA 打包格式
	ExportFormatQCOW2 ExportFormat = "qcow2" // 仅磁盘
	ExportFormatRAW   ExportFormat = "raw"   // 原始格式
)

// ExportVMRequest 导出 VM 请求
type ExportVMRequest struct {
	VMID        string       `json:"vm_id" binding:"required"`
	Format      ExportFormat `json:"format,omitempty"`      // 默认 ova
	IncludeISO  bool         `json:"include_iso,omitempty"` // 是否包含 ISO
	Compress    bool         `json:"compress,omitempty"`    // 是否压缩
}

// ExportResult 导出结果
type ExportResult struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Format   string `json:"format"`
	Checksum string `json:"checksum,omitempty"` // SHA256
}

// ImportVMRequest 导入 VM 请求
type ImportVMRequest struct {
	Path        string `json:"path" binding:"required"` // 文件路径或 URL
	Name        string `json:"name,omitempty"`          // 新名称
	Description string `json:"description,omitempty"`
}

// AutoStartConfig 自动启动配置
type AutoStartConfig struct {
	VMID       string `json:"vm_id"`
	Enabled    bool   `json:"enabled"`
	Order      int    `json:"order"`       // 启动顺序
	Delay      int    `json:"delay"`       // 延迟秒数
}

// LiveSnapshotRequest 实时快照请求
type LiveSnapshotRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	IncludeRAM  bool   `json:"include_ram,omitempty"` // 是否包含内存状态
}

// ==================== P6: SSH 终端集成 ====================

// VMSSHConfig VM SSH 配置
type VMSSHConfig struct {
	Username string `json:"username,omitempty"` // 默认用户名
	Port     int    `json:"port,omitempty"`     // Guest SSH 端口（默认 22）
}

// VMSSHConnectRequest VM SSH 连接请求
type VMSSHConnectRequest struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	AuthType   string `json:"auth_type" binding:"required,oneof=password key"` // password | key
	Cols       uint16 `json:"cols,omitempty"`
	Rows       uint16 `json:"rows,omitempty"`
}

// VMSSHSession VM SSH 会话信息
type VMSSHSession struct {
	ID        string `json:"id"`
	VMID      string `json:"vm_id"`
	VMName    string `json:"vm_name"`
	Host      string `json:"host"`     // 连接地址(localhost)
	Port      int    `json:"port"`     // 转发后的端口
	Username  string `json:"username"`
	CreatedAt int64  `json:"created_at"`
}

// ==================== P6: 资源监控仪表板 ====================

// VMResourceStats VM 资源统计
type VMResourceStats struct {
	VMID      string `json:"vm_id"`
	Timestamp int64  `json:"timestamp"`

	// CPU 统计
	CPU VMCPUStats `json:"cpu"`

	// 内存统计
	Memory VMMemoryStats `json:"memory"`

	// 磁盘统计
	Disks []VMDiskStats `json:"disks"`

	// 网络统计
	Networks []VMNetworkStats `json:"networks"`
}

// VMCPUStats CPU 统计
type VMCPUStats struct {
	UsagePercent float64 `json:"usage_percent"` // CPU 使用率百分比
	CPUTime      uint64  `json:"cpu_time"`      // CPU 时间（纳秒）
	VCPUs        int     `json:"vcpus"`         // vCPU 数量
}

// VMMemoryStats 内存统计
type VMMemoryStats struct {
	Total       uint64  `json:"total"`        // 配置的总内存(MB)
	Used        uint64  `json:"used"`         // 已使用内存(MB)
	Available   uint64  `json:"available"`    // 可用内存(MB)
	UsedPercent float64 `json:"used_percent"` // 内存使用率
	Balloon     uint64  `json:"balloon"`      // Balloon 实际大小
}

// VMDiskStats 磁盘统计
type VMDiskStats struct {
	Device      string  `json:"device"`        // 设备名
	Path        string  `json:"path"`          // 磁盘路径
	BytesRead   uint64  `json:"bytes_read"`    // 读取字节数
	BytesWrite  uint64  `json:"bytes_written"` // 写入字节数
	OpsRead     uint64  `json:"ops_read"`      // 读操作数
	OpsWrite    uint64  `json:"ops_written"`   // 写操作数
	ReadSpeed   float64 `json:"read_speed"`    // 读取速度 (B/s)
	WriteSpeed  float64 `json:"write_speed"`   // 写入速度 (B/s)
}

// VMNetworkStats 网络统计
type VMNetworkStats struct {
	Device      string  `json:"device"`        // 网络设备名
	BytesRx     uint64  `json:"bytes_rx"`      // 接收字节数
	BytesTx     uint64  `json:"bytes_tx"`      // 发送字节数
	PacketsRx   uint64  `json:"packets_rx"`    // 接收包数
	PacketsTx   uint64  `json:"packets_tx"`    // 发送包数
	RxSpeed     float64 `json:"rx_speed"`      // 接收速度 (B/s)
	TxSpeed     float64 `json:"tx_speed"`      // 发送速度 (B/s)
}

// VMResourceHistory 资源历史记录（用于图表）
type VMResourceHistory struct {
	VMID       string            `json:"vm_id"`
	Period     string            `json:"period"` // 1m, 5m, 1h, 1d
	DataPoints []VMResourceStats `json:"data_points"`
}
