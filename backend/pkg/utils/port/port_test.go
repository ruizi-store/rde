package port

import (
	"net"
	"testing"
	"time"
)

func TestIsValid(t *testing.T) {
	tests := []struct {
		port     int
		expected bool
	}{
		{0, false},
		{1, true},
		{80, true},
		{443, true},
		{8080, true},
		{65535, true},
		{65536, false},
		{-1, false},
	}

	for _, tt := range tests {
		result := IsValid(tt.port)
		if result != tt.expected {
			t.Errorf("IsValid(%d) = %v, 期望 %v", tt.port, result, tt.expected)
		}
	}
}

func TestIsWellKnown(t *testing.T) {
	tests := []struct {
		port     int
		expected bool
	}{
		{0, true},
		{22, true},
		{80, true},
		{443, true},
		{1023, true},
		{1024, false},
		{8080, false},
	}

	for _, tt := range tests {
		result := IsWellKnown(tt.port)
		if result != tt.expected {
			t.Errorf("IsWellKnown(%d) = %v, 期望 %v", tt.port, result, tt.expected)
		}
	}
}

func TestIsRegistered(t *testing.T) {
	tests := []struct {
		port     int
		expected bool
	}{
		{1023, false},
		{1024, true},
		{3306, true},
		{8080, true},
		{49151, true},
		{49152, false},
	}

	for _, tt := range tests {
		result := IsRegistered(tt.port)
		if result != tt.expected {
			t.Errorf("IsRegistered(%d) = %v, 期望 %v", tt.port, result, tt.expected)
		}
	}
}

func TestIsDynamic(t *testing.T) {
	tests := []struct {
		port     int
		expected bool
	}{
		{49151, false},
		{49152, true},
		{50000, true},
		{65535, true},
	}

	for _, tt := range tests {
		result := IsDynamic(tt.port)
		if result != tt.expected {
			t.Errorf("IsDynamic(%d) = %v, 期望 %v", tt.port, result, tt.expected)
		}
	}
}

func TestIsAvailableTCP(t *testing.T) {
	// 获取一个可用端口
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("无法创建监听器: %v", err)
	}
	
	port := listener.Addr().(*net.TCPAddr).Port
	
	// 端口被占用时应返回 false
	if IsAvailableTCP(port) {
		t.Errorf("端口 %d 被占用但 IsAvailableTCP 返回 true", port)
	}
	
	// 关闭监听器后应返回 true
	listener.Close()
	time.Sleep(10 * time.Millisecond) // 等待端口释放
	
	if !IsAvailableTCP(port) {
		t.Errorf("端口 %d 已释放但 IsAvailableTCP 返回 false", port)
	}
}

func TestIsAvailableUDP(t *testing.T) {
	// 获取一个可用 UDP 端口
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatalf("无法创建 UDP 监听器: %v", err)
	}
	
	port := conn.LocalAddr().(*net.UDPAddr).Port
	
	// 端口被占用时应返回 false
	if IsAvailableUDP(port) {
		t.Errorf("UDP 端口 %d 被占用但 IsAvailableUDP 返回 true", port)
	}
	
	// 关闭后应返回 true
	conn.Close()
	time.Sleep(10 * time.Millisecond)
	
	if !IsAvailableUDP(port) {
		t.Errorf("UDP 端口 %d 已释放但 IsAvailableUDP 返回 false", port)
	}
}

func TestGetAvailable(t *testing.T) {
	port, err := GetAvailable()
	if err != nil {
		t.Fatalf("GetAvailable 失败: %v", err)
	}
	
	if !IsValid(port) {
		t.Errorf("GetAvailable 返回的端口 %d 无效", port)
	}
	
	// 返回的端口应该是可用的
	if !IsAvailableTCP(port) {
		t.Errorf("GetAvailable 返回的端口 %d 不可用", port)
	}
}

func TestGetAvailableInRange(t *testing.T) {
	port, err := GetAvailableInRange(50000, 50100)
	if err != nil {
		t.Fatalf("GetAvailableInRange 失败: %v", err)
	}
	
	if port < 50000 || port > 50100 {
		t.Errorf("端口 %d 不在范围 [50000, 50100] 内", port)
	}
}

func TestGetAvailablePorts(t *testing.T) {
	ports, err := GetAvailablePorts(5)
	if err != nil {
		t.Fatalf("GetAvailablePorts 失败: %v", err)
	}
	
	if len(ports) != 5 {
		t.Errorf("应返回 5 个端口, 实际返回 %d 个", len(ports))
	}
	
	// 检查端口有效性
	for _, port := range ports {
		if !IsValid(port) {
			t.Errorf("端口 %d 无效", port)
		}
	}
}

func TestIsInUse(t *testing.T) {
	// 创建一个监听器占用端口
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("无法创建监听器: %v", err)
	}
	defer listener.Close()
	
	port := listener.Addr().(*net.TCPAddr).Port
	
	if !IsInUse(port) {
		t.Errorf("端口 %d 被占用但 IsInUse 返回 false", port)
	}
}

func TestCanConnect(t *testing.T) {
	// 创建一个 TCP 服务器
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("无法创建监听器: %v", err)
	}
	defer listener.Close()
	
	port := listener.Addr().(*net.TCPAddr).Port
	
	// 在后台接受连接
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			conn.Close()
		}
	}()
	
	if !CanConnect("127.0.0.1", port, time.Second) {
		t.Errorf("应该能连接到 127.0.0.1:%d", port)
	}
	
	// 测试无法连接的情况
	if CanConnect("127.0.0.1", 1, 100*time.Millisecond) {
		t.Error("不应该能连接到 127.0.0.1:1")
	}
}

func TestIsPortAvailable(t *testing.T) {
	// 获取一个可用端口
	port, _ := GetAvailable()
	
	// 测试可用端口
	if !IsPortAvailable(port, "tcp") {
		t.Errorf("端口 %d 应该可用", port)
	}
	
	// 占用端口
	listener, _ := net.Listen("tcp", ":"+string(rune(port)))
	if listener != nil {
		defer listener.Close()
		
		if IsPortAvailable(port, "tcp") {
			t.Errorf("端口 %d 被占用后应该不可用", port)
		}
	}
}

func TestGetAvailablePort(t *testing.T) {
	// TCP 端口
	port, err := GetAvailablePort("tcp")
	if err != nil {
		t.Fatalf("GetAvailablePort(tcp) 失败: %v", err)
	}
	
	if !IsValid(port) {
		t.Errorf("返回的 TCP 端口 %d 无效", port)
	}
	
	// UDP 端口
	port, err = GetAvailablePort("udp")
	if err != nil {
		t.Fatalf("GetAvailablePort(udp) 失败: %v", err)
	}
	
	if !IsValid(port) {
		t.Errorf("返回的 UDP 端口 %d 无效", port)
	}
}

// 基准测试
func BenchmarkIsAvailableTCP(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsAvailableTCP(8080)
	}
}

func BenchmarkGetAvailable(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetAvailable()
	}
}
