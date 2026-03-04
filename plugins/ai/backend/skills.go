// Package ai NAS 技能服务 - 提供系统工具和 Function Calling 支持
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SkillsService 技能服务
type SkillsService struct {
	logger  *zap.Logger
	dataDir string
}

// NewSkillsService 创建技能服务
func NewSkillsService(logger *zap.Logger, dataDir string) *SkillsService {
	return &SkillsService{
		logger:  logger,
		dataDir: dataDir,
	}
}

// ==================== 存储分析 ====================

// DiskUsageInfo 磁盘使用信息
type DiskUsageInfo struct {
	FileSystem string `json:"filesystem"`
	Size       string `json:"size"`
	Used       string `json:"used"`
	Available  string `json:"available"`
	UsePercent string `json:"use_percent"`
	MountPoint string `json:"mount_point"`
	Total      string `json:"total,omitempty"`
}

// FolderSize 文件夹大小
type FolderSize struct {
	Path string `json:"path"`
	Size string `json:"size"`
}

// LargeFile 大文件
type LargeFile struct {
	Path    string `json:"path"`
	Size    string `json:"size"`
	ModTime string `json:"mod_time"`
}

// StorageAnalysis 存储分析结果
type StorageAnalysis struct {
	DiskUsage  []DiskUsageInfo `json:"disk_usage"`
	TopFolders []FolderSize    `json:"top_folders"`
	LargeFiles []LargeFile     `json:"large_files"`
	Timestamp  string          `json:"timestamp"`
}

// AnalyzeDiskUsage 分析磁盘使用
func (s *SkillsService) AnalyzeDiskUsage() ([]DiskUsageInfo, error) {
	output, err := exec.Command("df", "-h", "--output=source,size,used,avail,pcent,target").Output()
	if err != nil {
		return nil, err
	}

	var result []DiskUsageInfo
	lines := strings.Split(string(output), "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		if strings.HasPrefix(fields[0], "tmpfs") || strings.HasPrefix(fields[0], "devtmpfs") ||
			strings.HasPrefix(fields[0], "udev") || strings.HasPrefix(fields[0], "overlay") {
			continue
		}
		result = append(result, DiskUsageInfo{
			FileSystem: fields[0],
			Size:       fields[1],
			Used:       fields[2],
			Available:  fields[3],
			UsePercent: fields[4],
			MountPoint: fields[5],
			Total:      fields[1],
		})
	}
	return result, nil
}

// AnalyzeTopFolders 分析最大的文件夹
func (s *SkillsService) AnalyzeTopFolders(path string, limit int) ([]FolderSize, error) {
	if path == "" {
		path = "/"
	}
	if limit <= 0 {
		limit = 10
	}

	cmd := exec.Command("du", "-h", "--max-depth=1", path)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	done := make(chan error)
	go func() { done <- cmd.Run() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("分析超时")
	}

	var folders []FolderSize
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] != path {
			folders = append(folders, FolderSize{Size: fields[0], Path: fields[1]})
		}
	}

	// 按大小排序（简化：取前 limit 个）
	if len(folders) > limit {
		folders = folders[:limit]
	}
	return folders, nil
}

// FindLargeFiles 查找大文件
func (s *SkillsService) FindLargeFiles(path string, minSizeMB, limit int) ([]LargeFile, error) {
	if path == "" {
		path = "/"
	}
	var err error
	if path, err = validatePath(path); err != nil {
		return nil, err
	}
	if minSizeMB <= 0 {
		minSizeMB = 100
	}
	if limit <= 0 {
		limit = 20
	}

	sizeArg := fmt.Sprintf("+%dM", minSizeMB)
	cmd := exec.Command("find", path, "-type", "f", "-size", sizeArg, "-printf", "%s %T+ %p\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	done := make(chan error)
	go func() { done <- cmd.Run() }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("搜索超时")
	}

	var files []LargeFile
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line == "" || len(files) >= limit {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}
		sizeBytes, _ := strconv.ParseInt(parts[0], 10, 64)
		files = append(files, LargeFile{
			Path:    parts[2],
			Size:    formatBytes(sizeBytes),
			ModTime: parts[1],
		})
	}
	return files, nil
}

// GetStorageAnalysis 获取完整存储分析
func (s *SkillsService) GetStorageAnalysis(path string) (*StorageAnalysis, error) {
	analysis := &StorageAnalysis{Timestamp: time.Now().Format("2006-01-02 15:04:05")}

	// 并行执行三个独立的磁盘分析任务
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		if diskUsage, err := s.AnalyzeDiskUsage(); err == nil {
			analysis.DiskUsage = diskUsage
		}
	}()

	go func() {
		defer wg.Done()
		if topFolders, err := s.AnalyzeTopFolders(path, 10); err == nil {
			analysis.TopFolders = topFolders
		}
	}()

	go func() {
		defer wg.Done()
		if largeFiles, err := s.FindLargeFiles(path, 100, 20); err == nil {
			analysis.LargeFiles = largeFiles
		}
	}()

	wg.Wait()
	return analysis, nil
}

// ==================== 文件搜索 ====================

// FileSearchResult 文件搜索结果
type FileSearchResult struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Size    string `json:"size"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

// SearchFiles 搜索文件
func (s *SkillsService) SearchFiles(path, pattern, fileType string, minSizeMB, limit int) ([]FileSearchResult, error) {
	if path == "" {
		path = "/"
	}
	var err error
	if path, err = validatePath(path); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}

	args := []string{path}
	if pattern != "" {
		args = append(args, "-iname", pattern)
	}
	if fileType == "dir" {
		args = append(args, "-type", "d")
	} else {
		args = append(args, "-type", "f")
	}
	if minSizeMB > 0 {
		args = append(args, "-size", fmt.Sprintf("+%dM", minSizeMB))
	}
	args = append(args, "-printf", "%s %T+ %p\n")

	cmd := exec.Command("find", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	done := make(chan error)
	go func() { done <- cmd.Run() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("搜索超时")
	}

	var results []FileSearchResult
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line == "" || len(results) >= limit {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}
		sizeBytes, _ := strconv.ParseInt(parts[0], 10, 64)
		results = append(results, FileSearchResult{
			Path:    parts[2],
			Name:    filepath.Base(parts[2]),
			Size:    formatBytes(sizeBytes),
			ModTime: parts[1],
			IsDir:   fileType == "dir",
		})
	}
	return results, nil
}

// ==================== 系统信息 ====================

// SystemInfo 系统信息
type SystemInfo struct {
	Hostname    string  `json:"hostname"`
	OS          string  `json:"os"`
	Kernel      string  `json:"kernel"`
	Arch        string  `json:"arch"`
	CPUCores    int     `json:"cpu_cores"`
	CPUModel    string  `json:"cpu_model"`
	MemTotal    string  `json:"mem_total"`
	MemUsed     string  `json:"mem_used"`
	MemPercent  float64 `json:"mem_percent"`
	Uptime      string  `json:"uptime"`
	LoadAvg     string  `json:"load_avg"`
}

// GetSystemInfo 获取系统信息
func (s *SkillsService) GetSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{}

	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				info.OS = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			}
		}
	}
	if output, err := exec.Command("uname", "-r").Output(); err == nil {
		info.Kernel = strings.TrimSpace(string(output))
	}
	if output, err := exec.Command("uname", "-m").Output(); err == nil {
		info.Arch = strings.TrimSpace(string(output))
	}

	// CPU
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		cores := 0
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "processor") {
				cores++
			}
			if strings.HasPrefix(line, "model name") && info.CPUModel == "" {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					info.CPUModel = strings.TrimSpace(parts[1])
				}
			}
		}
		info.CPUCores = cores
	}

	// Memory
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		var total, available int64
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fmt.Sscanf(line, "MemTotal: %d kB", &total)
			}
			if strings.HasPrefix(line, "MemAvailable:") {
				fmt.Sscanf(line, "MemAvailable: %d kB", &available)
			}
		}
		totalBytes := total * 1024
		usedBytes := (total - available) * 1024
		info.MemTotal = formatBytes(totalBytes)
		info.MemUsed = formatBytes(usedBytes)
		if total > 0 {
			info.MemPercent = float64(total-available) / float64(total) * 100
		}
	}

	// Uptime
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			seconds, _ := strconv.ParseFloat(fields[0], 64)
			days := int(seconds) / 86400
			hours := (int(seconds) % 86400) / 3600
			mins := (int(seconds) % 3600) / 60
			info.Uptime = fmt.Sprintf("%dd %dh %dm", days, hours, mins)
		}
	}

	// Load Average
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			info.LoadAvg = strings.Join(fields[:3], " ")
		}
	}

	return info, nil
}

// ==================== Docker 状态 ====================

// DockerContainer Docker 容器
type DockerContainer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Ports   string `json:"ports"`
	Created string `json:"created"`
}

// DockerStatus Docker 状态
type DockerStatus struct {
	Running    bool              `json:"running"`
	Containers []DockerContainer `json:"containers"`
	Version    string            `json:"version"`
}

// GetDockerStatus 获取 Docker 状态
func (s *SkillsService) GetDockerStatus() (*DockerStatus, error) {
	status := &DockerStatus{}

	if output, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output(); err == nil {
		status.Version = strings.TrimSpace(string(output))
		status.Running = true
	}

	if output, err := exec.Command("docker", "ps", "-a", "--format",
		"{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.State}}\t{{.Ports}}\t{{.CreatedAt}}").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line == "" {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) >= 7 {
				status.Containers = append(status.Containers, DockerContainer{
					ID:      fields[0][:12],
					Name:    fields[1],
					Image:   fields[2],
					Status:  fields[3],
					State:   fields[4],
					Ports:   fields[5],
					Created: fields[6],
				})
			}
		}
	}

	return status, nil
}

// ==================== SMART 信息 ====================

// SmartInfo SMART 硬盘信息
type SmartInfo struct {
	Device      string `json:"device"`
	Model       string `json:"model"`
	Serial      string `json:"serial"`
	Health      string `json:"health"`
	Temperature string `json:"temperature"`
	PowerOnHrs  string `json:"power_on_hours"`
}

// GetSmartInfo 获取 SMART 信息
func (s *SkillsService) GetSmartInfo() ([]SmartInfo, error) {
	output, err := exec.Command("sh", "-c", "lsblk -d -o NAME,TYPE | grep disk | awk '{print $1}'").Output()
	if err != nil {
		return nil, err
	}

	var results []SmartInfo
	disks := strings.Fields(string(output))

	for _, disk := range disks {
		info := SmartInfo{Device: "/dev/" + disk}
		smartOutput, err := exec.Command("sudo", "smartctl", "-i", "-H", "-A", "/dev/"+disk).Output()
		if err != nil {
			info.Health = "unknown"
			results = append(results, info)
			continue
		}

		text := string(smartOutput)
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "Device Model") || strings.Contains(line, "Model Number") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					info.Model = strings.TrimSpace(parts[1])
				}
			}
			if strings.Contains(line, "Serial Number") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					info.Serial = strings.TrimSpace(parts[1])
				}
			}
			if strings.Contains(line, "SMART overall-health") {
				if strings.Contains(line, "PASSED") {
					info.Health = "PASSED"
				} else {
					info.Health = "FAILED"
				}
			}
			if strings.Contains(line, "Temperature_Celsius") || strings.Contains(line, "Temperature") {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					info.Temperature = fields[len(fields)-1] + "°C"
				}
			}
			if strings.Contains(line, "Power_On_Hours") {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					info.PowerOnHrs = fields[len(fields)-1] + "h"
				}
			}
		}
		results = append(results, info)
	}
	return results, nil
}

// ==================== RAID 状态 ====================

// RaidInfo RAID 信息
type RaidInfo struct {
	Name    string   `json:"name"`
	Level   string   `json:"level"`
	Status  string   `json:"status"`
	Devices []string `json:"devices"`
	State   string   `json:"state"`
}

// GetRaidStatus 获取 RAID 状态
func (s *SkillsService) GetRaidStatus() ([]RaidInfo, error) {
	var raids []RaidInfo

	// mdadm RAID
	if data, err := os.ReadFile("/proc/mdstat"); err == nil {
		raids = append(raids, s.parseMdstat(string(data))...)
	}

	// ZFS
	if output, err := exec.Command("zpool", "status").Output(); err == nil {
		raids = append(raids, s.parseZpoolStatus(string(output))...)
	}

	return raids, nil
}

func (s *SkillsService) parseMdstat(content string) []RaidInfo {
	var raids []RaidInfo
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if !strings.Contains(line, " : ") {
			continue
		}
		parts := strings.SplitN(line, " : ", 2)
		if len(parts) < 2 {
			continue
		}
		raid := RaidInfo{Name: strings.TrimSpace(parts[0])}

		// 解析 RAID 级别和设备
		info := parts[1]
		fields := strings.Fields(info)
		if len(fields) > 1 {
			raid.Level = fields[1]
			for _, f := range fields[2:] {
				if !strings.Contains(f, "[") {
					raid.Devices = append(raid.Devices, f)
				}
			}
		}

		// 检查状态
		if i+1 < len(lines) {
			nextLine := lines[i+1]
			if strings.Contains(nextLine, "[UU") {
				raid.Status = "active"
				raid.State = "healthy"
			} else if strings.Contains(nextLine, "_") {
				raid.Status = "degraded"
				raid.State = "degraded"
			}
		}
		raids = append(raids, raid)
	}
	return raids
}

func (s *SkillsService) parseZpoolStatus(content string) []RaidInfo {
	var raids []RaidInfo
	var current *RaidInfo

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pool:") {
			if current != nil {
				raids = append(raids, *current)
			}
			current = &RaidInfo{
				Name:  strings.TrimSpace(strings.TrimPrefix(line, "pool:")),
				Level: "zfs",
			}
		}
		if current != nil && strings.HasPrefix(line, "state:") {
			current.State = strings.TrimSpace(strings.TrimPrefix(line, "state:"))
			current.Status = strings.ToLower(current.State)
		}
	}
	if current != nil {
		raids = append(raids, *current)
	}
	return raids
}

// ==================== 网络状态 ====================

// NASNetworkInterface 网络接口
type NASNetworkInterface struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	IPv4    string `json:"ipv4,omitempty"`
	IPv6    string `json:"ipv6,omitempty"`
	MAC     string `json:"mac"`
	Speed   string `json:"speed,omitempty"`
	RxBytes string `json:"rx_bytes"`
	TxBytes string `json:"tx_bytes"`
}

// NASNetworkStatus 网络状态
type NASNetworkStatus struct {
	Interfaces   []NASNetworkInterface `json:"interfaces"`
	Gateway      string                `json:"gateway"`
	DNS          []string              `json:"dns"`
	Connectivity bool                  `json:"connectivity"`
}

// GetNetworkStatus 获取网络状态
func (s *SkillsService) GetNetworkStatus() (*NASNetworkStatus, error) {
	status := &NASNetworkStatus{}

	// 使用 ip 命令获取接口
	output, err := exec.Command("ip", "-j", "addr").Output()
	if err != nil {
		return s.getNetworkStatusFallback()
	}

	var interfaces []struct {
		IfName    string `json:"ifname"`
		OperState string `json:"operstate"`
		Address   string `json:"address"`
		AddrInfo  []struct {
			Family string `json:"family"`
			Local  string `json:"local"`
		} `json:"addr_info"`
	}

	if err := json.Unmarshal(output, &interfaces); err != nil {
		return s.getNetworkStatusFallback()
	}

	for _, iface := range interfaces {
		if iface.IfName == "lo" {
			continue
		}

		ni := NASNetworkInterface{
			Name:  iface.IfName,
			State: iface.OperState,
			MAC:   iface.Address,
		}

		for _, addr := range iface.AddrInfo {
			if addr.Family == "inet" && ni.IPv4 == "" {
				ni.IPv4 = addr.Local
			} else if addr.Family == "inet6" && ni.IPv6 == "" && !strings.HasPrefix(addr.Local, "fe80") {
				ni.IPv6 = addr.Local
			}
		}

		// 获取流量统计
		rxPath := fmt.Sprintf("/sys/class/net/%s/statistics/rx_bytes", iface.IfName)
		txPath := fmt.Sprintf("/sys/class/net/%s/statistics/tx_bytes", iface.IfName)
		if data, err := os.ReadFile(rxPath); err == nil {
			rx, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
			ni.RxBytes = formatBytes(rx)
		}
		if data, err := os.ReadFile(txPath); err == nil {
			tx, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
			ni.TxBytes = formatBytes(tx)
		}

		// 获取速度
		speedPath := fmt.Sprintf("/sys/class/net/%s/speed", iface.IfName)
		if speed, err := os.ReadFile(speedPath); err == nil {
			speedVal := strings.TrimSpace(string(speed))
			if speedVal != "-1" && speedVal != "" {
				ni.Speed = speedVal + " Mbps"
			}
		}

		status.Interfaces = append(status.Interfaces, ni)
	}

	// 获取网关
	if output, err := exec.Command("ip", "route", "show", "default").Output(); err == nil {
		if match := regexp.MustCompile(`default via (\S+)`).FindStringSubmatch(string(output)); len(match) > 1 {
			status.Gateway = match[1]
		}
	}

	// 获取 DNS
	if data, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "nameserver") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					status.DNS = append(status.DNS, fields[1])
				}
			}
		}
	}

	// 测试连通性
	if err := exec.Command("ping", "-c", "1", "-W", "2", "8.8.8.8").Run(); err == nil {
		status.Connectivity = true
	}

	return status, nil
}

// getNetworkStatusFallback 回退方法获取网络状态
func (s *SkillsService) getNetworkStatusFallback() (*NASNetworkStatus, error) {
	status := &NASNetworkStatus{}

	cmd := exec.Command("ip", "addr")
	output, _ := cmd.Output()

	lines := strings.Split(string(output), "\n")
	var currentIface *NASNetworkInterface
	for _, line := range lines {
		if match := regexp.MustCompile(`^\d+:\s+(\S+):`).FindStringSubmatch(line); len(match) > 1 {
			if currentIface != nil && currentIface.Name != "lo" {
				status.Interfaces = append(status.Interfaces, *currentIface)
			}
			currentIface = &NASNetworkInterface{Name: match[1]}
			if strings.Contains(line, "UP") {
				currentIface.State = "UP"
			} else {
				currentIface.State = "DOWN"
			}
		}
		if currentIface != nil {
			if match := regexp.MustCompile(`inet (\S+)`).FindStringSubmatch(line); len(match) > 1 {
				currentIface.IPv4 = match[1]
			}
			if match := regexp.MustCompile(`link/ether (\S+)`).FindStringSubmatch(line); len(match) > 1 {
				currentIface.MAC = match[1]
			}
		}
	}
	if currentIface != nil && currentIface.Name != "lo" {
		status.Interfaces = append(status.Interfaces, *currentIface)
	}

	return status, nil
}

// ==================== 共享状态 ====================

// ShareInfo 共享信息
type ShareInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"` // smb, nfs
	ReadOnly bool   `json:"read_only"`
}

// ShareStatus 共享状态
type ShareStatus struct {
	SMBRunning  bool        `json:"smb_running"`
	NFSRunning  bool        `json:"nfs_running"`
	Shares      []ShareInfo `json:"shares"`
	ActiveUsers int         `json:"active_users"`
}

// GetShareStatus 获取共享状态
func (s *SkillsService) GetShareStatus() (*ShareStatus, error) {
	status := &ShareStatus{}

	// 检查 SMB
	if err := exec.Command("systemctl", "is-active", "smbd").Run(); err == nil {
		status.SMBRunning = true
	} else if err := exec.Command("systemctl", "is-active", "smb").Run(); err == nil {
		status.SMBRunning = true
	}

	// 检查 NFS
	if err := exec.Command("systemctl", "is-active", "nfs-server").Run(); err == nil {
		status.NFSRunning = true
	} else if err := exec.Command("systemctl", "is-active", "nfs-kernel-server").Run(); err == nil {
		status.NFSRunning = true
	}

	// 解析 SMB 共享
	if data, err := os.ReadFile("/etc/samba/smb.conf"); err == nil {
		status.Shares = append(status.Shares, s.parseSmbConf(string(data))...)
	}

	// 解析 NFS 共享
	if data, err := os.ReadFile("/etc/exports"); err == nil {
		status.Shares = append(status.Shares, s.parseExports(string(data))...)
	}

	// 活跃 SMB 用户
	if output, err := exec.Command("smbstatus", "-b").Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		count := 0
		inData := false
		for _, line := range lines {
			if strings.HasPrefix(line, "----") {
				inData = true
				continue
			}
			if inData && strings.TrimSpace(line) != "" {
				count++
			}
		}
		status.ActiveUsers = count
	}

	return status, nil
}

func (s *SkillsService) parseSmbConf(content string) []ShareInfo {
	var shares []ShareInfo
	var current *ShareInfo

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.Trim(line, "[]")
			if name == "global" || name == "printers" || name == "print$" {
				current = nil
				continue
			}
			current = &ShareInfo{Name: name, Type: "smb"}
			shares = append(shares, *current)
		}
		if current != nil {
			kv := strings.SplitN(line, "=", 2)
			if len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				val := strings.TrimSpace(kv[1])
				idx := len(shares) - 1
				switch key {
				case "path":
					shares[idx].Path = val
				case "read only":
					shares[idx].ReadOnly = strings.ToLower(val) == "yes"
				}
			}
		}
	}
	return shares
}

func (s *SkillsService) parseExports(content string) []ShareInfo {
	var shares []ShareInfo
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			share := ShareInfo{
				Path: fields[0],
				Type: "nfs",
				Name: filepath.Base(fields[0]),
			}
			if strings.Contains(line, "ro") {
				share.ReadOnly = true
			}
			shares = append(shares, share)
		}
	}
	return shares
}

// ==================== 中级功能 ====================

// UserSession 用户会话
type UserSession struct {
	User      string `json:"user"`
	Host      string `json:"host"`
	LoginTime string `json:"login_time"`
	Type      string `json:"type"` // ssh, smb, local
}

// GetUserSessions 获取用户会话
func (s *SkillsService) GetUserSessions() ([]UserSession, error) {
	var sessions []UserSession

	if output, err := exec.Command("who").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				session := UserSession{
					User:      fields[0],
					LoginTime: strings.Join(fields[2:4], " "),
					Type:      "ssh",
				}
				if len(fields) > 4 {
					session.Host = strings.Trim(fields[4], "()")
				}
				if session.Host == "" || session.Host == ":0" {
					session.Type = "local"
				}
				sessions = append(sessions, session)
			}
		}
	}

	if output, err := exec.Command("smbstatus", "-b").Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		inData := false
		for _, line := range lines {
			if strings.HasPrefix(line, "----") {
				inData = true
				continue
			}
			if inData {
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					sessions = append(sessions, UserSession{User: fields[1], Host: fields[3], Type: "smb"})
				}
			}
		}
	}

	return sessions, nil
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Enabled bool   `json:"enabled"`
}

// GetServiceStatus 获取常用服务状态
func (s *SkillsService) GetServiceStatus() ([]ServiceInfo, error) {
	services := []string{
		"smbd", "smb", "nmbd", "nfs-server", "nfs-kernel-server",
		"docker", "containerd", "sshd", "ssh",
		"nginx", "apache2", "httpd",
		"mariadb", "mysql", "postgresql",
	}

	var result []ServiceInfo
	checked := make(map[string]bool)

	for _, svc := range services {
		if checked[svc] {
			continue
		}
		cmd := exec.Command("systemctl", "is-active", svc)
		status := "inactive"
		if output, err := cmd.Output(); err == nil {
			status = strings.TrimSpace(string(output))
		}
		if status == "inactive" || status == "unknown" {
			continue
		}
		enabled := exec.Command("systemctl", "is-enabled", svc).Run() == nil
		result = append(result, ServiceInfo{Name: svc, Status: status, Enabled: enabled})
		checked[svc] = true
	}
	return result, nil
}

// TemperatureInfo 温度信息
type TemperatureInfo struct {
	Label       string  `json:"label"`
	Temperature float64 `json:"temperature"`
	Unit        string  `json:"unit"`
}

// GetTemperatures 获取温度信息
func (s *SkillsService) GetTemperatures() ([]TemperatureInfo, error) {
	var result []TemperatureInfo

	if output, err := exec.Command("sensors").Output(); err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if match := regexp.MustCompile(`(.+?):\s+\+?([0-9.]+)°C`).FindStringSubmatch(line); len(match) > 2 {
				temp, _ := strconv.ParseFloat(match[2], 64)
				result = append(result, TemperatureInfo{Label: strings.TrimSpace(match[1]), Temperature: temp, Unit: "°C"})
			}
		}
	}

	hwmonDirs, _ := filepath.Glob("/sys/class/hwmon/hwmon*/temp*_input")
	for _, path := range hwmonDirs {
		if data, err := os.ReadFile(path); err == nil {
			temp, _ := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
			temp /= 1000
			label := "Unknown"
			labelPath := strings.Replace(path, "_input", "_label", 1)
			if labelData, err := os.ReadFile(labelPath); err == nil {
				label = strings.TrimSpace(string(labelData))
			}
			result = append(result, TemperatureInfo{Label: label, Temperature: temp, Unit: "°C"})
		}
	}
	return result, nil
}

// ScheduledTask 定时任务
type ScheduledTask struct {
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	User     string `json:"user"`
}

// GetScheduledTasks 获取定时任务
func (s *SkillsService) GetScheduledTasks() ([]ScheduledTask, error) {
	var tasks []ScheduledTask

	if data, err := os.ReadFile("/etc/crontab"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 7 {
				tasks = append(tasks, ScheduledTask{
					Schedule: strings.Join(fields[:5], " "),
					User:     fields[5],
					Command:  strings.Join(fields[6:], " "),
				})
			}
		}
	}

	cronDirs := []string{"/etc/cron.d"}
	for _, dir := range cronDirs {
		files, _ := os.ReadDir(dir)
		for _, file := range files {
			if data, err := os.ReadFile(filepath.Join(dir, file.Name())); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					fields := strings.Fields(line)
					if len(fields) >= 7 {
						tasks = append(tasks, ScheduledTask{
							Schedule: strings.Join(fields[:5], " "),
							User:     fields[5],
							Command:  strings.Join(fields[6:], " "),
						})
					}
				}
			}
		}
	}
	return tasks, nil
}

// UpdateInfo 更新信息
type UpdateInfo struct {
	Available       int      `json:"available"`
	SecurityUpdates int      `json:"security_updates"`
	LastCheck       string   `json:"last_check"`
	Packages        []string `json:"packages,omitempty"`
}

// GetUpdateStatus 获取更新状态
func (s *SkillsService) GetUpdateStatus() (*UpdateInfo, error) {
	info := &UpdateInfo{}

	if _, err := exec.LookPath("apt"); err == nil {
		cmd := exec.Command("apt", "list", "--upgradable")
		if output, err := cmd.Output(); err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				if strings.Contains(line, "upgradable") {
					info.Available++
					parts := strings.Split(line, "/")
					if len(parts) > 0 {
						info.Packages = append(info.Packages, parts[0])
					}
					if strings.Contains(line, "security") {
						info.SecurityUpdates++
					}
				}
			}
		}
		info.LastCheck = time.Now().Format("2006-01-02 15:04:05")
		return info, nil
	}

	if _, err := exec.LookPath("dnf"); err == nil {
		cmd := exec.Command("dnf", "check-update", "-q")
		if output, _ := cmd.Output(); len(output) > 0 {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			info.Available = len(lines)
		}
		info.LastCheck = time.Now().Format("2006-01-02 15:04:05")
		return info, nil
	}

	if _, err := exec.LookPath("pacman"); err == nil {
		cmd := exec.Command("pacman", "-Qu")
		if output, _ := cmd.Output(); len(output) > 0 {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			info.Available = len(lines)
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					info.Packages = append(info.Packages, fields[0])
				}
			}
		}
		info.LastCheck = time.Now().Format("2006-01-02 15:04:05")
		return info, nil
	}

	return info, nil
}

// ==================== 高级工具 ====================

// ShutdownResult 关机结果
type ShutdownResult struct {
	Scheduled bool   `json:"scheduled"`
	Time      string `json:"time"`
	Message   string `json:"message"`
	CancelCmd string `json:"cancel_cmd,omitempty"`
}

// ScheduleShutdown 定时关机
func (s *SkillsService) ScheduleShutdown(minutes int, reboot bool) (*ShutdownResult, error) {
	if minutes < 0 {
		return nil, fmt.Errorf("时间不能为负数")
	}

	var cmd *exec.Cmd
	var action string

	if minutes == 0 {
		if reboot {
			cmd = exec.Command("shutdown", "-r", "now")
			action = "立即重启"
		} else {
			cmd = exec.Command("shutdown", "-h", "now")
			action = "立即关机"
		}
	} else {
		timeArg := fmt.Sprintf("+%d", minutes)
		if reboot {
			cmd = exec.Command("shutdown", "-r", timeArg)
			action = fmt.Sprintf("%d 分钟后重启", minutes)
		} else {
			cmd = exec.Command("shutdown", "-h", timeArg)
			action = fmt.Sprintf("%d 分钟后关机", minutes)
		}
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "must be root") {
			return nil, fmt.Errorf("权限不足，需要管理员权限执行关机命令")
		}
		return nil, fmt.Errorf("执行失败: %s", errMsg)
	}

	result := &ShutdownResult{
		Scheduled: true,
		Time:      action,
		Message:   fmt.Sprintf("已计划 %s", action),
	}
	if minutes > 0 {
		result.CancelCmd = "shutdown -c"
	}
	return result, nil
}

// CancelShutdown 取消定时关机
func (s *SkillsService) CancelShutdown() error {
	cmd := exec.Command("shutdown", "-c")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "No scheduled shutdown") {
			return fmt.Errorf("当前没有计划的关机任务")
		}
		return fmt.Errorf("取消失败: %s", errMsg)
	}
	return nil
}

// FileSearchResult2 增强版文件搜索结果
type FileSearchResult2 struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Size    string `json:"size"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
	Match   string `json:"match,omitempty"`
}

// SearchFilesByName 按文件名搜索
func (s *SkillsService) SearchFilesByName(searchPath, keyword string, maxResults int) ([]FileSearchResult2, error) {
	if searchPath == "" {
		searchPath = "/"
	}
	var err error
	if searchPath, err = validatePath(searchPath); err != nil {
		return nil, err
	}
	if err = validateSearchKeyword(keyword); err != nil {
		return nil, err
	}
	if maxResults <= 0 {
		maxResults = 50
	}

	// 过滤关键字中的 glob 特殊字符（保留 * 和 ?）
	keyword = strings.ReplaceAll(keyword, "[", "")
	keyword = strings.ReplaceAll(keyword, "]", "")
	pattern := fmt.Sprintf("*%s*", keyword)
	cmd := exec.Command("find", searchPath, "-iname", pattern, "-type", "f", "-printf", "%s %T+ %p\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	done := make(chan error)
	go func() { done <- cmd.Run() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("搜索超时")
	}

	var results []FileSearchResult2
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line == "" || len(results) >= maxResults {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}
		sizeBytes, _ := strconv.ParseInt(parts[0], 10, 64)
		results = append(results, FileSearchResult2{
			Path: parts[2], Name: filepath.Base(parts[2]),
			Size: formatBytes(sizeBytes), ModTime: parts[1],
		})
	}
	return results, nil
}

// SearchFilesByExtension 按后缀名搜索
func (s *SkillsService) SearchFilesByExtension(searchPath, extension string, maxResults int) ([]FileSearchResult2, error) {
	if searchPath == "" {
		searchPath = "/"
	}
	var err error
	if searchPath, err = validatePath(searchPath); err != nil {
		return nil, err
	}
	if maxResults <= 0 {
		maxResults = 50
	}
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}

	pattern := fmt.Sprintf("*%s", extension)
	cmd := exec.Command("find", searchPath, "-iname", pattern, "-type", "f", "-printf", "%s %T+ %p\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	done := make(chan error)
	go func() { done <- cmd.Run() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("搜索超时")
	}

	var results []FileSearchResult2
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line == "" || len(results) >= maxResults {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}
		sizeBytes, _ := strconv.ParseInt(parts[0], 10, 64)
		results = append(results, FileSearchResult2{
			Path: parts[2], Name: filepath.Base(parts[2]),
			Size: formatBytes(sizeBytes), ModTime: parts[1],
		})
	}
	return results, nil
}

// ContentSearchResult 内容搜索结果
type ContentSearchResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// SearchFilesByContent 按内容搜索
func (s *SkillsService) SearchFilesByContent(searchPath, keyword string, maxResults int) ([]ContentSearchResult, error) {
	if searchPath == "" {
		searchPath = "/"
	}
	var err error
	if searchPath, err = validatePath(searchPath); err != nil {
		return nil, err
	}
	if err = validateSearchKeyword(keyword); err != nil {
		return nil, err
	}
	if maxResults <= 0 {
		maxResults = 50
	}

	// 使用 -e 和 -- 防止关键字被解析为 grep 参数
	cmd := exec.Command("grep", "-rInH",
		"--include=*.{txt,log,conf,cfg,ini,json,xml,yaml,yml,md,sh,py,go,js,ts,html,css}",
		"-e", keyword, "--", searchPath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	done := make(chan error)
	go func() { done <- cmd.Run() }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("搜索超时")
	}

	var results []ContentSearchResult
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line == "" || len(results) >= maxResults {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		lineNum, _ := strconv.Atoi(parts[1])
		content := parts[2]
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		results = append(results, ContentSearchResult{
			File: parts[0], Line: lineNum, Content: strings.TrimSpace(content),
		})
	}
	return results, nil
}

// ScriptExecutionResult 脚本执行结果
type ScriptExecutionResult struct {
	Success  bool   `json:"success"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Duration string `json:"duration"`
}

// ExecuteScript 执行用户脚本（有安全限制）
func (s *SkillsService) ExecuteScript(script, language string, timeout int) (*ScriptExecutionResult, error) {
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 300 {
		timeout = 300
	}

	// 安全检查：黑名单 + 危险模式
	dangerousPatterns := []string{
		"rm -rf /", "rm -rf /*", "mkfs", "dd if=", ":(){ ",
		"chmod 777 /", "chown -R", "> /dev/sd",
		"shutdown", "reboot", "init 0", "init 6", "halt", "poweroff",
		"passwd", "useradd", "userdel", "usermod",
		"sudo", "su -", "su root", "pkill", "killall",
		"iptables", "nftables", "mount", "umount",
		"fdisk", "parted", "cryptsetup",
	}
	scriptLower := strings.ToLower(script)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(scriptLower, strings.ToLower(pattern)) {
			return nil, fmt.Errorf("脚本包含危险命令 '%s'，已拒绝执行", pattern)
		}
	}

	// 检测 Shell 注入模式：反引号、$()、进程替代、管道到危险命令
	shellInjectionPatterns := regexp.MustCompile("(?i)` .+`|\\$\\(.+\\)|/dev/tcp|/dev/udp|>\\s*/etc/|eval\\s|exec\\s")
	if shellInjectionPatterns.MatchString(script) {
		return nil, fmt.Errorf("脚本包含不安全的 Shell 模式，已拒绝执行")
	}

	var ext, interpreter string
	switch strings.ToLower(language) {
	case "bash", "sh", "shell":
		ext, interpreter = ".sh", "bash"
	case "python", "py":
		ext, interpreter = ".py", "python3"
	case "node", "js", "javascript":
		ext, interpreter = ".js", "node"
	default:
		ext, interpreter = ".sh", "bash"
	}

	tmpFile, err := os.CreateTemp("", "ai-script-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(script); err != nil {
		return nil, fmt.Errorf("写入脚本失败: %w", err)
	}
	tmpFile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	startTime := time.Now()
	cmd := exec.CommandContext(ctx, interpreter, tmpFile.Name())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = os.TempDir()
	// 限制环境变量，防止通过环境变量注入
	cmd.Env = []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=" + os.TempDir(),
		"LANG=C.UTF-8",
	}

	result := &ScriptExecutionResult{}
	err = cmd.Run()
	result.Duration = time.Since(startTime).String()

	if ctx.Err() == context.DeadlineExceeded {
		result.Success = false
		result.Error = fmt.Sprintf("脚本执行超时（%d秒）", timeout)
		result.ExitCode = -1
		return result, nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Success = false
		result.Error = stderr.String()
	} else {
		result.Success = true
	}

	output := stdout.String()
	if len(output) > 10000 {
		output = output[:10000] + "\n... (输出过长，已截断)"
	}
	result.Output = output
	return result, nil
}

// ==================== 安全辅助函数 ====================

// validatePath 验证并清理路径，防止路径遍历和访问敏感目录
func validatePath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("路径必须为绝对路径: %s", path)
	}
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("禁止路径遍历: %s", path)
	}
	sensitivePaths := []string{"/proc", "/sys", "/dev"}
	for _, sp := range sensitivePaths {
		if cleaned == sp || strings.HasPrefix(cleaned, sp+"/") {
			return "", fmt.Errorf("禁止访问系统目录: %s", sp)
		}
	}
	return cleaned, nil
}

// validateSearchKeyword 验证搜索关键字（防止 grep 参数注入）
func validateSearchKeyword(keyword string) error {
	if keyword == "" {
		return fmt.Errorf("关键字不能为空")
	}
	if len(keyword) > 500 {
		return fmt.Errorf("关键字过长")
	}
	return nil
}

// ==================== 工具函数 ====================

func parseSizeToBytes(size string) int64 {
	size = strings.ToUpper(strings.TrimSpace(size))
	re := regexp.MustCompile(`^([0-9.]+)([KMGTP]?)B?$`)
	matches := re.FindStringSubmatch(size)
	if len(matches) < 2 {
		return 0
	}
	val, _ := strconv.ParseFloat(matches[1], 64)
	if len(matches) > 2 {
		switch matches[2] {
		case "K":
			val *= 1024
		case "M":
			val *= 1024 * 1024
		case "G":
			val *= 1024 * 1024 * 1024
		case "T":
			val *= 1024 * 1024 * 1024 * 1024
		case "P":
			val *= 1024 * 1024 * 1024 * 1024 * 1024
		}
	}
	return int64(val)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ==================== 技能执行器 ====================

// ExecuteSkill 执行技能
func (s *SkillsService) ExecuteSkill(req *SkillRequest) *SkillResponse {
	switch req.SkillID {
	case "storage-analyzer":
		return s.executeStorageAnalyzer(req.Action, req.Arguments)
	case "file-manager":
		return s.executeFileManager(req.Action, req.Arguments)
	case "system-diagnosis":
		return s.executeSystemDiagnosis(req.Action, req.Arguments)
	default:
		return &SkillResponse{Success: false, Error: fmt.Sprintf("未知的技能: %s", req.SkillID)}
	}
}

func (s *SkillsService) executeStorageAnalyzer(action string, params map[string]interface{}) *SkillResponse {
	switch action {
	case "analyze", "disk_usage", "":
		path, _ := params["path"].(string)
		analysis, err := s.GetStorageAnalysis(path)
		if err != nil {
			return &SkillResponse{Success: false, Error: err.Error()}
		}
		summary := s.generateStorageAnalysisSummary(analysis)
		return &SkillResponse{Success: true, Data: analysis, Summary: summary}

	case "top_folders":
		path, _ := params["path"].(string)
		limit := 10
		if l, ok := params["limit"].(float64); ok {
			limit = int(l)
		}
		folders, err := s.AnalyzeTopFolders(path, limit)
		if err != nil {
			return &SkillResponse{Success: false, Error: err.Error()}
		}
		return &SkillResponse{Success: true, Data: folders}

	case "large_files":
		path, _ := params["path"].(string)
		minSize := 100
		if m, ok := params["min_size_mb"].(float64); ok {
			minSize = int(m)
		}
		files, err := s.FindLargeFiles(path, minSize, 20)
		if err != nil {
			return &SkillResponse{Success: false, Error: err.Error()}
		}
		return &SkillResponse{Success: true, Data: files}

	default:
		return &SkillResponse{Success: false, Error: "未知操作"}
	}
}

func (s *SkillsService) executeFileManager(action string, params map[string]interface{}) *SkillResponse {
	switch action {
	case "search":
		path, _ := params["path"].(string)
		pattern, _ := params["pattern"].(string)
		fileType, _ := params["type"].(string)
		minSize := 0
		if m, ok := params["min_size_mb"].(float64); ok {
			minSize = int(m)
		}
		results, err := s.SearchFiles(path, pattern, fileType, minSize, 50)
		if err != nil {
			return &SkillResponse{Success: false, Error: err.Error()}
		}
		return &SkillResponse{Success: true, Data: results}
	default:
		return &SkillResponse{Success: false, Error: "未知操作"}
	}
}

func (s *SkillsService) executeSystemDiagnosis(action string, params map[string]interface{}) *SkillResponse {
	switch action {
	case "info", "":
		info, err := s.GetSystemInfo()
		if err != nil {
			return &SkillResponse{Success: false, Error: err.Error()}
		}
		return &SkillResponse{Success: true, Data: info}
	default:
		return &SkillResponse{Success: false, Error: "未知操作"}
	}
}

func (s *SkillsService) generateStorageAnalysisSummary(analysis *StorageAnalysis) string {
	var sb strings.Builder
	sb.WriteString("## 📊 存储分析报告\n\n")
	sb.WriteString(fmt.Sprintf("分析时间: %s\n\n", analysis.Timestamp))

	if len(analysis.DiskUsage) > 0 {
		sb.WriteString("### 💾 磁盘使用情况\n\n| 挂载点 | 总容量 | 已使用 | 可用 | 使用率 |\n|--------|--------|--------|------|--------|\n")
		for _, disk := range analysis.DiskUsage {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				disk.MountPoint, disk.Total, disk.Used, disk.Available, disk.UsePercent))
		}
		sb.WriteString("\n")
	}

	if len(analysis.TopFolders) > 0 {
		sb.WriteString("### 📁 占用空间最大的文件夹\n\n| 文件夹 | 大小 |\n|--------|------|\n")
		for i, folder := range analysis.TopFolders {
			if i >= 10 {
				break
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", folder.Path, folder.Size))
		}
		sb.WriteString("\n")
	}

	if len(analysis.LargeFiles) > 0 {
		sb.WriteString("### 📄 大文件 (>100MB)\n\n| 文件 | 大小 |\n|------|------|\n")
		for i, file := range analysis.LargeFiles {
			if i >= 10 {
				break
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", file.Path, file.Size))
		}
		if len(analysis.LargeFiles) > 10 {
			sb.WriteString(fmt.Sprintf("\n*还有 %d 个大文件未显示*\n", len(analysis.LargeFiles)-10))
		}
	}

	return sb.String()
}

// FormatSkillResultForAI 格式化技能结果供 AI 使用
func (s *SkillsService) FormatSkillResultForAI(resp *SkillResponse) string {
	if !resp.Success {
		return fmt.Sprintf("执行失败: %s", resp.Error)
	}
	if resp.Summary != "" {
		return resp.Summary
	}
	data, _ := json.MarshalIndent(resp.Data, "", "  ")
	return string(data)
}

// ==================== Function Calling 支持 ====================

// GetToolDefinitions 获取所有可用技能的 Tool 定义（OpenAI 格式）
func (s *SkillsService) GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{Type: "function", Function: FunctionDef{
			Name: "get_system_info", Description: "获取本机系统信息，包括操作系统、内核、CPU、内存、负载等",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "analyze_storage", Description: "分析磁盘存储使用情况，包括磁盘容量、大文件夹和大文件列表",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string", "description": "要分析的路径，默认为 /"},
			}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "search_files", Description: "在指定目录中搜索文件，支持通配符模式匹配",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string", "description": "搜索路径，默认为 /"},
				"pattern": map[string]interface{}{"type": "string", "description": "文件名匹配模式，如 *.log、config*"},
				"type":    map[string]interface{}{"type": "string", "description": "文件类型: file 或 dir", "enum": []string{"file", "dir"}},
			}, "required": []string{"pattern"}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_disk_usage", Description: "获取磁盘分区使用情况列表",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "find_large_files", Description: "查找指定目录下的大文件",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"path":        map[string]interface{}{"type": "string", "description": "搜索路径，默认为 /"},
				"min_size_mb": map[string]interface{}{"type": "integer", "description": "最小文件大小（MB），默认 100"},
			}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_docker_status", Description: "获取 Docker 容器状态列表",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_smart_info", Description: "获取硬盘 SMART 健康信息",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_raid_status", Description: "获取 RAID 磁盘阵列状态",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_network_status", Description: "获取网络状态，包括接口、IP、网关、DNS 等",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_share_status", Description: "获取文件共享状态，包括 SMB 和 NFS",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_user_sessions", Description: "获取当前登录的用户会话",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_service_status", Description: "获取常用服务的运行状态",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_temperatures", Description: "获取系统温度信息",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_scheduled_tasks", Description: "获取系统定时任务列表（crontab）",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "get_update_status", Description: "检查系统可用更新",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "schedule_shutdown", Description: "定时关机或重启系统",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"minutes": map[string]interface{}{"type": "integer", "description": "延迟时间（分钟），0表示立即执行"},
				"reboot":  map[string]interface{}{"type": "boolean", "description": "是否重启而非关机"},
			}, "required": []string{"minutes"}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "cancel_shutdown", Description: "取消已计划的定时关机或重启",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "search_files_by_name", Description: "搜索文件名中包含指定关键字的文件",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"keyword": map[string]interface{}{"type": "string", "description": "要搜索的关键字"},
				"path":    map[string]interface{}{"type": "string", "description": "搜索路径，默认为 /"},
			}, "required": []string{"keyword"}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "search_files_by_extension", Description: "搜索指定后缀名的文件",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"extension": map[string]interface{}{"type": "string", "description": "文件后缀名，如 log、txt、mp4"},
				"path":      map[string]interface{}{"type": "string", "description": "搜索路径，默认为 /"},
			}, "required": []string{"extension"}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "search_files_by_content", Description: "搜索文件内容中包含指定关键字的文件",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"keyword": map[string]interface{}{"type": "string", "description": "要搜索的关键字"},
				"path":    map[string]interface{}{"type": "string", "description": "搜索路径，默认为 /"},
			}, "required": []string{"keyword"}},
		}},
		{Type: "function", Function: FunctionDef{
			Name: "execute_script", Description: "执行用户脚本，支持 Bash/Python/Node.js，有安全限制",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"script":   map[string]interface{}{"type": "string", "description": "脚本代码"},
				"language": map[string]interface{}{"type": "string", "description": "脚本语言", "enum": []string{"bash", "python", "node"}},
				"timeout":  map[string]interface{}{"type": "integer", "description": "超时时间（秒），默认30"},
			}, "required": []string{"script", "language"}},
		}},
	}
}

// ExecuteToolCall 执行工具调用并返回结果
func (s *SkillsService) ExecuteToolCall(name string, argsJSON string) string {
	s.logger.Info("Executing tool call", zap.String("name", name), zap.String("args", argsJSON))

	var args map[string]interface{}
	if argsJSON != "" && argsJSON != "{}" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			args = make(map[string]interface{})
		}
	} else {
		args = make(map[string]interface{})
	}

	var resp *SkillResponse

	switch name {
	case "get_system_info":
		resp = s.executeSystemDiagnosis("info", args)
	case "analyze_storage":
		resp = s.executeStorageAnalyzer("analyze", args)
	case "get_disk_usage":
		diskUsage, err := s.AnalyzeDiskUsage()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: diskUsage}
		}
	case "find_large_files":
		resp = s.executeStorageAnalyzer("large_files", args)
	case "search_files":
		resp = s.executeFileManager("search", args)
	case "get_docker_status":
		status, err := s.GetDockerStatus()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: status}
		}
	case "get_smart_info":
		info, err := s.GetSmartInfo()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: info}
		}
	case "get_raid_status":
		status, err := s.GetRaidStatus()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: status}
		}
	case "get_network_status":
		status, err := s.GetNetworkStatus()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: status}
		}
	case "get_share_status":
		status, err := s.GetShareStatus()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: status}
		}
	case "get_user_sessions":
		sessions, err := s.GetUserSessions()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: sessions}
		}
	case "get_service_status":
		status, err := s.GetServiceStatus()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: status}
		}
	case "get_temperatures":
		temps, err := s.GetTemperatures()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: temps}
		}
	case "get_scheduled_tasks":
		tasks, err := s.GetScheduledTasks()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: tasks}
		}
	case "get_update_status":
		status, err := s.GetUpdateStatus()
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: status}
		}
	case "schedule_shutdown":
		minutes := 0
		if m, ok := args["minutes"].(float64); ok {
			minutes = int(m)
		}
		reboot, _ := args["reboot"].(bool)
		result, err := s.ScheduleShutdown(minutes, reboot)
		if err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: result, Summary: result.Message}
		}
	case "cancel_shutdown":
		if err := s.CancelShutdown(); err != nil {
			resp = &SkillResponse{Success: false, Error: err.Error()}
		} else {
			resp = &SkillResponse{Success: true, Data: map[string]string{"message": "已取消定时关机"}}
		}
	case "search_files_by_name":
		keyword, _ := args["keyword"].(string)
		path, _ := args["path"].(string)
		if keyword == "" {
			resp = &SkillResponse{Success: false, Error: "缺少必需参数: keyword"}
		} else {
			results, err := s.SearchFilesByName(path, keyword, 50)
			if err != nil {
				resp = &SkillResponse{Success: false, Error: err.Error()}
			} else {
				resp = &SkillResponse{Success: true, Data: results,
					Summary: fmt.Sprintf("找到 %d 个文件名包含 \"%s\" 的文件", len(results), keyword)}
			}
		}
	case "search_files_by_extension":
		extension, _ := args["extension"].(string)
		path, _ := args["path"].(string)
		if extension == "" {
			resp = &SkillResponse{Success: false, Error: "缺少必需参数: extension"}
		} else {
			results, err := s.SearchFilesByExtension(path, extension, 50)
			if err != nil {
				resp = &SkillResponse{Success: false, Error: err.Error()}
			} else {
				resp = &SkillResponse{Success: true, Data: results,
					Summary: fmt.Sprintf("找到 %d 个 .%s 文件", len(results), strings.TrimPrefix(extension, "."))}
			}
		}
	case "search_files_by_content":
		keyword, _ := args["keyword"].(string)
		path, _ := args["path"].(string)
		if keyword == "" {
			resp = &SkillResponse{Success: false, Error: "缺少必需参数: keyword"}
		} else {
			results, err := s.SearchFilesByContent(path, keyword, 50)
			if err != nil {
				resp = &SkillResponse{Success: false, Error: err.Error()}
			} else {
				resp = &SkillResponse{Success: true, Data: results,
					Summary: fmt.Sprintf("找到 %d 个包含 \"%s\" 的文件", len(results), keyword)}
			}
		}
	case "execute_script":
		script, _ := args["script"].(string)
		language, _ := args["language"].(string)
		timeout := 30
		if t, ok := args["timeout"].(float64); ok {
			timeout = int(t)
		}
		if script == "" {
			resp = &SkillResponse{Success: false, Error: "缺少必需参数: script"}
		} else {
			result, err := s.ExecuteScript(script, language, timeout)
			if err != nil {
				resp = &SkillResponse{Success: false, Error: err.Error()}
			} else {
				var summary string
				if result.Success {
					summary = fmt.Sprintf("脚本执行成功（耗时 %s）\n\n输出：\n```\n%s\n```", result.Duration, result.Output)
				} else {
					summary = fmt.Sprintf("脚本执行失败（退出码 %d）\n\n错误：\n```\n%s\n```", result.ExitCode, result.Error)
				}
				resp = &SkillResponse{Success: true, Data: result, Summary: summary}
			}
		}
	default:
		return fmt.Sprintf("未知工具: %s", name)
	}

	return s.FormatSkillResultForAI(resp)
}
