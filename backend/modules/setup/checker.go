package setup

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"go.uber.org/zap"
)

// Checker 系统检查器
type Checker struct {
	logger *zap.Logger
}

// NewChecker 创建系统检查器
func NewChecker(logger *zap.Logger) *Checker {
	return &Checker{logger: logger}
}

// CheckSystem 执行完整的系统检查
func (c *Checker) CheckSystem() (*SystemCheckResult, error) {
	result := &SystemCheckResult{
		Dependencies: c.checkDependencies(),
		Ports:        c.checkPorts(),
		DiskSpace:    c.checkDiskSpace("/"),
		AllPassed:    true,
	}

	// 判断是否全部通过
	for _, dep := range result.Dependencies {
		if dep.Required && !dep.Installed {
			result.AllPassed = false
			break
		}
	}

	for _, port := range result.Ports {
		if port.InUse {
			result.AllPassed = false
			break
		}
	}

	if !result.DiskSpace.Sufficient {
		result.AllPassed = false
	}

	return result, nil
}

// checkDependencies 检查系统依赖
func (c *Checker) checkDependencies() []DependencyCheck {
	// 定义需要检查的依赖
	// 注意：docker, xpra, scrcpy 等已移至套件系统，由套件自行管理依赖
	deps := []struct {
		name     string
		cmd      string
		args     []string
		required bool
	}{
		{"systemd", "systemctl", []string{"--version"}, true}, // Core 服务管理必需
	}

	results := make([]DependencyCheck, 0, len(deps))
	for _, dep := range deps {
		check := DependencyCheck{
			Name:     dep.name,
			Required: dep.required,
		}

		cmd := exec.Command(dep.cmd, dep.args...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			check.Installed = true
			check.Version = c.extractVersion(string(output))
		} else {
			check.Installed = false
			c.logger.Debug("Dependency not found",
				zap.String("name", dep.name),
				zap.Error(err),
			)
		}

		results = append(results, check)
	}

	return results
}

// extractVersion 从命令输出中提取版本号
func (c *Checker) extractVersion(output string) string {
	// 尝试匹配常见的版本号格式
	patterns := []string{
		`(\d+\.\d+\.\d+)`,  // 1.2.3
		`(\d+\.\d+)`,       // 1.2
		`version\s+(\S+)`,  // version xxx
		`Version:\s+(\S+)`, // Version: xxx
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	// 返回第一行的前50个字符
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		if len(line) > 50 {
			return line[:50] + "..."
		}
		return line
	}

	return "unknown"
}

// checkDocker 检查 Docker 状态
func (c *Checker) checkDocker() (running bool, version string) {
	// 检查 Docker 版本
	cmd := exec.Command("docker", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, ""
	}
	version = c.extractVersion(string(output))

	// 检查 Docker daemon 是否运行
	cmd = exec.Command("docker", "info")
	err = cmd.Run()
	running = err == nil

	return running, version
}

// checkPorts 检查端口占用
func (c *Checker) checkPorts() []PortCheck {
	ports := []int{80, 443, 8080}
	results := make([]PortCheck, 0, len(ports))

	for _, port := range ports {
		check := PortCheck{
			Port:  port,
			InUse: false,
		}

		// 尝试监听端口
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			check.InUse = true
			check.InUseProcess = c.getProcessOnPort(port)
		} else {
			listener.Close()
		}

		results = append(results, check)
	}

	return results
}

// getProcessOnPort 获取占用端口的服务名
func (c *Checker) getProcessOnPort(port int) string {
	// 使用 lsof 或 ss 获取占用端口的进程
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-t")
	output, err := cmd.Output()
	if err != nil {
		// 尝试使用 ss
		cmd = exec.Command("ss", "-tlnp", fmt.Sprintf("sport = :%d", port))
		output, _ = cmd.Output()
	}

	if len(output) > 0 {
		// 获取进程名
		pid := strings.TrimSpace(string(output))
		if pid != "" {
			cmdline, _ := os.ReadFile(fmt.Sprintf("/proc/%s/comm", pid))
			return strings.TrimSpace(string(cmdline))
		}
	}

	return "unknown"
}

// checkDiskSpace 检查磁盘空间
func (c *Checker) checkDiskSpace(path string) DiskSpaceCheck {
	check := DiskSpaceCheck{
		Path:        path,
		MinRequired: 10 * 1024 * 1024 * 1024, // 10GB
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		c.logger.Error("Failed to check disk space", zap.Error(err))
		return check
	}

	check.TotalBytes = int64(stat.Blocks) * int64(stat.Bsize)
	check.AvailBytes = int64(stat.Bavail) * int64(stat.Bsize)
	check.Sufficient = check.AvailBytes >= check.MinRequired

	return check
}

// DetectDrives 检测系统中的硬盘
func (c *Checker) DetectDrives() ([]DetectedDrive, error) {
	drives := make([]DetectedDrive, 0)

	// 读取 /proc/partitions
	file, err := os.Open("/proc/partitions")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// 跳过头两行
	scanner.Scan()
	scanner.Scan()

	diskRegex := regexp.MustCompile(`^\s*\d+\s+\d+\s+(\d+)\s+(sd[a-z]|nvme\d+n\d+|vd[a-z])$`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := diskRegex.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}

		size := matches[1]
		name := matches[2]
		devicePath := filepath.Join("/dev", name)

		drive := DetectedDrive{
			DevicePath: devicePath,
		}

		// 解析大小（KB -> Bytes）
		fmt.Sscanf(size, "%d", &drive.Size)
		drive.Size *= 1024

		// 获取硬盘型号
		modelPath := fmt.Sprintf("/sys/block/%s/device/model", name)
		if model, err := os.ReadFile(modelPath); err == nil {
			drive.Model = strings.TrimSpace(string(model))
		}

		// 获取序列号
		serialPath := fmt.Sprintf("/sys/block/%s/device/serial", name)
		if serial, err := os.ReadFile(serialPath); err == nil {
			drive.Serial = strings.TrimSpace(string(serial))
		}

		// 获取分区
		drive.Partitions = c.getPartitions(name)

		drives = append(drives, drive)
	}

	return drives, nil
}

// getPartitions 获取硬盘分区
func (c *Checker) getPartitions(diskName string) []Partition {
	partitions := make([]Partition, 0)

	// 读取 /proc/partitions 获取分区
	file, err := os.Open("/proc/partitions")
	if err != nil {
		return partitions
	}
	defer file.Close()

	partRegex := regexp.MustCompile(fmt.Sprintf(`^\s*\d+\s+\d+\s+(\d+)\s+(%s\d+|%sp\d+)$`, diskName, diskName))

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		matches := partRegex.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}

		size := matches[1]
		name := matches[2]

		part := Partition{
			DevicePath: filepath.Join("/dev", name),
		}

		fmt.Sscanf(size, "%d", &part.Size)
		part.Size *= 1024

		// 获取文件系统类型
		cmd := exec.Command("blkid", "-s", "TYPE", "-o", "value", part.DevicePath)
		if output, err := cmd.Output(); err == nil {
			part.Filesystem = strings.TrimSpace(string(output))
		}

		// 获取挂载点
		part.MountPoint = c.getMountPoint(part.DevicePath)

		// 获取标签
		cmd = exec.Command("blkid", "-s", "LABEL", "-o", "value", part.DevicePath)
		if output, err := cmd.Output(); err == nil {
			part.Label = strings.TrimSpace(string(output))
		}

		partitions = append(partitions, part)
	}

	return partitions
}

// getMountPoint 获取分区挂载点
func (c *Checker) getMountPoint(devicePath string) string {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == devicePath {
			return fields[1]
		}
	}

	return ""
}
