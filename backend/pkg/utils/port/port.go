package port

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// 端口范围
	MinPort = 1
	MaxPort = 65535

	// 常用端口范围
	WellKnownPortMax = 1023
	RegisteredPortMax = 49151
	DynamicPortMin   = 49152
	DynamicPortMax   = 65535
)

// IsAvailable 检查端口是否可用（TCP）
func IsAvailable(port int) bool {
	return IsAvailableTCP(port)
}

// IsAvailableTCP 检查 TCP 端口是否可用
func IsAvailableTCP(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// IsAvailableUDP 检查 UDP 端口是否可用
func IsAvailableUDP(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// IsAvailableBoth 检查 TCP 和 UDP 端口是否都可用
func IsAvailableBoth(port int) bool {
	return IsAvailableTCP(port) && IsAvailableUDP(port)
}

// IsInUse 检查端口是否被占用
func IsInUse(port int) bool {
	return !IsAvailable(port)
}

// GetAvailable 获取一个可用端口（系统分配）
func GetAvailable() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// GetAvailableInRange 在指定范围内获取可用端口
func GetAvailableInRange(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		if IsAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port in range %d-%d", start, end)
}

// GetAvailablePorts 获取多个可用端口
func GetAvailablePorts(count int) ([]int, error) {
	ports := make([]int, 0, count)
	for i := 0; i < count; i++ {
		port, err := GetAvailable()
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}
	return ports, nil
}

// GetAvailablePortsInRange 在指定范围内获取多个可用端口
func GetAvailablePortsInRange(start, end, count int) ([]int, error) {
	ports := make([]int, 0, count)
	for port := start; port <= end && len(ports) < count; port++ {
		if IsAvailable(port) {
			ports = append(ports, port)
		}
	}
	if len(ports) < count {
		return nil, fmt.Errorf("only found %d available ports in range %d-%d", len(ports), start, end)
	}
	return ports, nil
}

// IsValid 检查端口号是否有效
func IsValid(port int) bool {
	return port >= MinPort && port <= MaxPort
}

// IsWellKnown 检查是否为知名端口 (0-1023)
func IsWellKnown(port int) bool {
	return port >= 0 && port <= WellKnownPortMax
}

// IsRegistered 检查是否为注册端口 (1024-49151)
func IsRegistered(port int) bool {
	return port > WellKnownPortMax && port <= RegisteredPortMax
}

// IsDynamic 检查是否为动态端口 (49152-65535)
func IsDynamic(port int) bool {
	return port >= DynamicPortMin && port <= DynamicPortMax
}

// CanConnect 检查是否可以连接到指定端口
func CanConnect(host string, port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// WaitForPort 等待端口可连接
func WaitForPort(host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if CanConnect(host, port, time.Second) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %s:%d", host, port)
}

// WaitForPortAvailable 等待端口可用（释放）
func WaitForPortAvailable(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if IsAvailable(port) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d to become available", port)
}

// PortInfo 端口占用信息
type PortInfo struct {
	Port     int
	Protocol string
	PID      int
	Process  string
	State    string
}

// GetProcessByPort 获取占用端口的进程信息（Linux）
func GetProcessByPort(port int) (*PortInfo, error) {
	// 读取 /proc/net/tcp 和 /proc/net/tcp6
	info, err := getPortInfoFromProc(port, "/proc/net/tcp", "tcp")
	if err == nil && info != nil {
		return info, nil
	}

	info, err = getPortInfoFromProc(port, "/proc/net/tcp6", "tcp6")
	if err == nil && info != nil {
		return info, nil
	}

	return nil, fmt.Errorf("no process found using port %d", port)
}

// getPortInfoFromProc 从 /proc/net/* 读取端口信息
func getPortInfoFromProc(targetPort int, procPath, protocol string) (*PortInfo, error) {
	file, err := os.Open(procPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// 跳过标题行
	scanner.Scan()

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}

		// 解析本地地址和端口
		localAddr := fields[1]
		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 {
			continue
		}

		portHex := parts[1]
		port, err := strconv.ParseInt(portHex, 16, 32)
		if err != nil {
			continue
		}

		if int(port) == targetPort {
			// 获取 inode
			inode := fields[9]
			
			// 查找进程
			pid, processName := findProcessByInode(inode)

			return &PortInfo{
				Port:     targetPort,
				Protocol: protocol,
				PID:      pid,
				Process:  processName,
				State:    getStateString(fields[3]),
			}, nil
		}
	}

	return nil, nil
}

// findProcessByInode 通过 inode 查找进程
func findProcessByInode(inode string) (int, string) {
	// 遍历 /proc 目录
	procDir, err := os.Open("/proc")
	if err != nil {
		return 0, ""
	}
	defer procDir.Close()

	entries, err := procDir.Readdirnames(-1)
	if err != nil {
		return 0, ""
	}

	for _, entry := range entries {
		// 检查是否为 PID 目录
		pid, err := strconv.Atoi(entry)
		if err != nil {
			continue
		}

		// 检查 fd 目录
		fdPath := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdPath)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(fmt.Sprintf("%s/%s", fdPath, fd.Name()))
			if err != nil {
				continue
			}

			if strings.Contains(link, "socket:["+inode+"]") {
				// 获取进程名
				cmdline, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
				return pid, strings.TrimSpace(string(cmdline))
			}
		}
	}

	return 0, ""
}

// getStateString 获取连接状态字符串
func getStateString(stateHex string) string {
	states := map[string]string{
		"01": "ESTABLISHED",
		"02": "SYN_SENT",
		"03": "SYN_RECV",
		"04": "FIN_WAIT1",
		"05": "FIN_WAIT2",
		"06": "TIME_WAIT",
		"07": "CLOSE",
		"08": "CLOSE_WAIT",
		"09": "LAST_ACK",
		"0A": "LISTEN",
		"0B": "CLOSING",
	}

	if state, ok := states[strings.ToUpper(stateHex)]; ok {
		return state
	}
	return "UNKNOWN"
}

// GetListeningPorts 获取所有监听端口
func GetListeningPorts() ([]PortInfo, error) {
	var ports []PortInfo

	// 读取 TCP 端口
	tcpPorts, err := getListeningPortsFromProc("/proc/net/tcp", "tcp")
	if err == nil {
		ports = append(ports, tcpPorts...)
	}

	// 读取 TCP6 端口
	tcp6Ports, err := getListeningPortsFromProc("/proc/net/tcp6", "tcp6")
	if err == nil {
		ports = append(ports, tcp6Ports...)
	}

	return ports, nil
}

// getListeningPortsFromProc 从 proc 文件获取监听端口
func getListeningPortsFromProc(procPath, protocol string) ([]PortInfo, error) {
	file, err := os.Open(procPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ports []PortInfo
	scanner := bufio.NewScanner(file)
	scanner.Scan() // 跳过标题

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}

		// 检查是否为 LISTEN 状态 (0A)
		if fields[3] != "0A" {
			continue
		}

		// 解析端口
		localAddr := fields[1]
		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 {
			continue
		}

		portHex := parts[1]
		port, err := strconv.ParseInt(portHex, 16, 32)
		if err != nil {
			continue
		}

		inode := fields[9]
		pid, processName := findProcessByInode(inode)

		ports = append(ports, PortInfo{
			Port:     int(port),
			Protocol: protocol,
			PID:      pid,
			Process:  processName,
			State:    "LISTEN",
		})
	}

	return ports, nil
}

// ==================== CasaOS 兼容别名 ====================

// IsPortAvailable 检查端口是否可用 - CasaOS 兼容别名
// port: 可以是 int 或 string 类型的端口号
// protocol: "tcp" 或 "udp"
func IsPortAvailable(portNum int, protocol string) bool {
	if protocol == "udp" {
		return IsAvailableUDP(portNum)
	}
	return IsAvailableTCP(portNum)
}

// IsPortAvailableAddr 通过地址检查端口是否可用
func IsPortAvailableAddr(address string, protocol string) bool {
	// 解析地址中的端口
	_, portStr, err := net.SplitHostPort(address)
	if err != nil {
		// 尝试直接解析为端口号
		port, err := strconv.Atoi(address)
		if err != nil {
			return false
		}
		return IsPortAvailable(port, protocol)
	}
	
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	
	return IsPortAvailable(port, protocol)
}

// GetAvailablePort 获取可用端口 - CasaOS 兼容别名
func GetAvailablePort(protocol string) (int, error) {
	if protocol == "udp" {
		// UDP 端口获取
		conn, err := net.ListenPacket("udp", ":0")
		if err != nil {
			return 0, err
		}
		defer conn.Close()
		return conn.LocalAddr().(*net.UDPAddr).Port, nil
	}
	// TCP 端口获取
	return GetAvailable()
}
