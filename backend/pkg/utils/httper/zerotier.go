package httper

import (
	"fmt"
	"os"
	"time"
)

// ZeroTierClient ZeroTier API 客户端
type ZeroTierClient struct {
	client    *Client
	baseURL   string
	authToken string
}

// ZeroTierNetwork 网络信息
type ZeroTierNetwork struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Status             string   `json:"status"`
	MAC                string   `json:"mac"`
	AssignedAddresses  []string `json:"assignedAddresses"`
	AllowManaged       bool     `json:"allowManaged"`
	AllowGlobal        bool     `json:"allowGlobal"`
	AllowDefault       bool     `json:"allowDefault"`
	AllowDNS           bool     `json:"allowDNS"`
	Bridge             bool     `json:"bridge"`
	BroadcastEnabled   bool     `json:"broadcastEnabled"`
	PortDeviceName     string   `json:"portDeviceName"`
	NetconfRevision    int      `json:"netconfRevision"`
}

// ZeroTierPeer 节点信息
type ZeroTierPeer struct {
	Address      string         `json:"address"`
	Latency      int            `json:"latency"`
	Role         string         `json:"role"`
	Version      string         `json:"version"`
	Paths        []ZeroTierPath `json:"paths"`
}

// ZeroTierPath 路径信息
type ZeroTierPath struct {
	Active        bool   `json:"active"`
	Address       string `json:"address"`
	LastReceive   int64  `json:"lastReceive"`
	LastSend      int64  `json:"lastSend"`
	Preferred     bool   `json:"preferred"`
	TrustedPathID int64  `json:"trustedPathId"`
}

// ZeroTierStatus 服务状态
type ZeroTierStatus struct {
	Address           string `json:"address"`
	Clock             int64  `json:"clock"`
	ClusterNode       string `json:"clusterNode,omitempty"`
	Online            bool   `json:"online"`
	PublicIdentity    string `json:"publicIdentity"`
	TCPFallbackActive bool   `json:"tcpFallbackActive"`
	Version           string `json:"version"`
	VersionBuild      int    `json:"versionBuild"`
	VersionMajor      int    `json:"versionMajor"`
	VersionMinor      int    `json:"versionMinor"`
	VersionRev        int    `json:"versionRev"`
}

// NewZeroTierClient 创建 ZeroTier 客户端
func NewZeroTierClient(authToken string) *ZeroTierClient {
	return &ZeroTierClient{
		client:    NewClient().WithTimeout(10 * time.Second),
		baseURL:   "http://localhost:9993",
		authToken: authToken,
	}
}

// NewZeroTierClientWithURL 创建指定 URL 的 ZeroTier 客户端
func NewZeroTierClientWithURL(baseURL, authToken string) *ZeroTierClient {
	return &ZeroTierClient{
		client:    NewClient().WithTimeout(10 * time.Second),
		baseURL:   baseURL,
		authToken: authToken,
	}
}

// request 发送请求
func (z *ZeroTierClient) request(method, path string, body interface{}) (*Response, error) {
	url := z.baseURL + path

	z.client.WithHeader("X-ZT1-Auth", z.authToken)
	z.client.WithHeader("Content-Type", "application/json")

	switch method {
	case "GET":
		return z.client.Get(url)
	case "POST":
		return z.client.Post(url, body)
	case "DELETE":
		return z.client.Delete(url)
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

// GetStatus 获取服务状态
func (z *ZeroTierClient) GetStatus() (*ZeroTierStatus, error) {
	resp, err := z.request("GET", "/status", nil)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to get status: status %d", resp.StatusCode)
	}

	var status ZeroTierStatus
	if err := resp.JSON(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

// ListNetworks 列出已加入的网络
func (z *ZeroTierClient) ListNetworks() ([]ZeroTierNetwork, error) {
	resp, err := z.request("GET", "/network", nil)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to list networks: status %d", resp.StatusCode)
	}

	var networks []ZeroTierNetwork
	if err := resp.JSON(&networks); err != nil {
		return nil, err
	}

	return networks, nil
}

// GetNetwork 获取网络详情
func (z *ZeroTierClient) GetNetwork(networkID string) (*ZeroTierNetwork, error) {
	resp, err := z.request("GET", "/network/"+networkID, nil)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to get network: status %d", resp.StatusCode)
	}

	var network ZeroTierNetwork
	if err := resp.JSON(&network); err != nil {
		return nil, err
	}

	return &network, nil
}

// JoinNetwork 加入网络
func (z *ZeroTierClient) JoinNetwork(networkID string) (*ZeroTierNetwork, error) {
	resp, err := z.request("POST", "/network/"+networkID, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to join network: status %d", resp.StatusCode)
	}

	var network ZeroTierNetwork
	if err := resp.JSON(&network); err != nil {
		return nil, err
	}

	return &network, nil
}

// LeaveNetwork 离开网络
func (z *ZeroTierClient) LeaveNetwork(networkID string) error {
	resp, err := z.request("DELETE", "/network/"+networkID, nil)
	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("failed to leave network: status %d", resp.StatusCode)
	}

	return nil
}

// ListPeers 列出节点
func (z *ZeroTierClient) ListPeers() ([]ZeroTierPeer, error) {
	resp, err := z.request("GET", "/peer", nil)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to list peers: status %d", resp.StatusCode)
	}

	var peers []ZeroTierPeer
	if err := resp.JSON(&peers); err != nil {
		return nil, err
	}

	return peers, nil
}

// GetPeer 获取节点详情
func (z *ZeroTierClient) GetPeer(address string) (*ZeroTierPeer, error) {
	resp, err := z.request("GET", "/peer/"+address, nil)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to get peer: status %d", resp.StatusCode)
	}

	var peer ZeroTierPeer
	if err := resp.JSON(&peer); err != nil {
		return nil, err
	}

	return &peer, nil
}

// IsOnline 检查是否在线
func (z *ZeroTierClient) IsOnline() bool {
	status, err := z.GetStatus()
	if err != nil {
		return false
	}
	return status.Online
}

// GetAddress 获取本机 ZeroTier 地址
func (z *ZeroTierClient) GetAddress() (string, error) {
	status, err := z.GetStatus()
	if err != nil {
		return "", err
	}
	return status.Address, nil
}

// GetVersion 获取版本号
func (z *ZeroTierClient) GetVersion() (string, error) {
	status, err := z.GetStatus()
	if err != nil {
		return "", err
	}
	return status.Version, nil
}

// GetNetworkIPs 获取网络分配的 IP 地址
func (z *ZeroTierClient) GetNetworkIPs(networkID string) ([]string, error) {
	network, err := z.GetNetwork(networkID)
	if err != nil {
		return nil, err
	}
	return network.AssignedAddresses, nil
}

// WaitForNetwork 等待网络就绪
func (z *ZeroTierClient) WaitForNetwork(networkID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		network, err := z.GetNetwork(networkID)
		if err == nil && network.Status == "OK" && len(network.AssignedAddresses) > 0 {
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("timeout waiting for network %s", networkID)
}

// ReadZeroTierAuthToken 从文件读取认证令牌
func ReadZeroTierAuthToken() (string, error) {
	// 默认路径
	paths := []string{
		"/var/lib/zerotier-one/authtoken.secret",
		"/Library/Application Support/ZeroTier/One/authtoken.secret",
		"C:\\ProgramData\\ZeroTier\\One\\authtoken.secret",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("zerotier authtoken not found")
}

