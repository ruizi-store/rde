package ip_helper

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// GetLocalIP 获取本机局域网 IP
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

// GetAllIPs 获取所有本机 IP 地址
func GetAllIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ip := ipnet.IP.To4(); ip != nil {
				ips = append(ips, ip.String())
			}
		}
	}
	return ips
}

// GetPublicIP 获取公网 IP
func GetPublicIP() (string, error) {
	// 尝试多个服务
	services := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			buf := make([]byte, 64)
			n, _ := resp.Body.Read(buf)
			ip := strings.TrimSpace(string(buf[:n]))
			if IsValidIP(ip) {
				return ip, nil
			}
		}
	}

	return "", fmt.Errorf("failed to get public IP")
}

// GetPublicIPInfo 获取公网 IP 详细信息
type IPInfo struct {
	IP       string `json:"ip"`
	Country  string `json:"country"`
	Region   string `json:"region"`
	City     string `json:"city"`
	ISP      string `json:"isp"`
	Timezone string `json:"timezone"`
}

func GetPublicIPInfo() (*IPInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Query    string `json:"query"`
		Country  string `json:"country"`
		Region   string `json:"regionName"`
		City     string `json:"city"`
		ISP      string `json:"isp"`
		Timezone string `json:"timezone"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &IPInfo{
		IP:       result.Query,
		Country:  result.Country,
		Region:   result.Region,
		City:     result.City,
		ISP:      result.ISP,
		Timezone: result.Timezone,
	}, nil
}

// IsValidIP 验证 IP 地址格式
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// IsIPv4 检查是否为 IPv4 地址
func IsIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() != nil
}

// IsIPv6 检查是否为 IPv6 地址
func IsIPv6(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() == nil
}

// IsPrivateIP 检查是否为私有 IP
func IsPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	// 私有地址范围
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(parsed) {
			return true
		}
	}

	return false
}

// IsLoopback 检查是否为回环地址
func IsLoopback(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsLoopback()
}

// IsInSubnet 检查 IP 是否在指定网段内
func IsInSubnet(ip, cidr string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}

	return subnet.Contains(parsed)
}

// ParseCIDR 解析 CIDR 格式
func ParseCIDR(cidr string) (ip string, mask string, err error) {
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", err
	}
	return subnet.IP.String(), net.IP(subnet.Mask).String(), nil
}

// GetNetworkInfo 获取网络接口信息
type NetworkInterface struct {
	Name       string   `json:"name"`
	HardwareAddr string `json:"hardware_addr"`
	IPs        []string `json:"ips"`
	MTU        int      `json:"mtu"`
	Flags      string   `json:"flags"`
}

func GetNetworkInterfaces() ([]NetworkInterface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []NetworkInterface
	for _, iface := range interfaces {
		ni := NetworkInterface{
			Name:         iface.Name,
			HardwareAddr: iface.HardwareAddr.String(),
			MTU:          iface.MTU,
			Flags:        iface.Flags.String(),
		}

		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				ni.IPs = append(ni.IPs, addr.String())
			}
		}

		result = append(result, ni)
	}

	return result, nil
}

// GetDefaultGateway 获取默认网关
func GetDefaultGateway() string {
	// 这个需要根据系统实现，这里提供一个简单版本
	// 可以通过解析 /proc/net/route 或执行 ip route 命令获取
	return ""
}

// IPToInt 将 IP 地址转换为整数
func IPToInt(ip string) uint32 {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return 0
	}
	ip4 := parsed.To4()
	if ip4 == nil {
		return 0
	}
	return uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
}

// IntToIP 将整数转换为 IP 地址
func IntToIP(n uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

// ValidateIPRange 验证 IP 范围格式 (如: 192.168.1.1-192.168.1.100)
func ValidateIPRange(ipRange string) bool {
	parts := strings.Split(ipRange, "-")
	if len(parts) != 2 {
		return false
	}
	return IsValidIP(strings.TrimSpace(parts[0])) && IsValidIP(strings.TrimSpace(parts[1]))
}

// ValidateCIDR 验证 CIDR 格式
func ValidateCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

// ValidateHostname 验证主机名格式
func ValidateHostname(hostname string) bool {
	if len(hostname) > 253 {
		return false
	}
	pattern := `^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))*$`
	matched, _ := regexp.MatchString(pattern, hostname)
	return matched
}

// ResolveHostname 解析主机名到 IP
func ResolveHostname(hostname string) ([]string, error) {
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, ip := range ips {
		result = append(result, ip.String())
	}
	return result, nil
}

// ReverseLookup 反向 DNS 查询
func ReverseLookup(ip string) ([]string, error) {
	return net.LookupAddr(ip)
}
