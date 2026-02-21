package utils

import (
	"net"
	"time"
)

// IsNetworkAvailable 检查网络是否可用
func IsNetworkAvailable() bool {
	timeout := 3 * time.Second
	conn, err := net.DialTimeout("tcp", "8.8.8.8:53", timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// GetLocalIP 获取本机IP
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
