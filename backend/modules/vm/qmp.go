// Package vm QMP (QEMU Machine Protocol) 客户端
package vm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// QMPClient QEMU Machine Protocol 客户端
type QMPClient struct {
	socketPath string
	conn       net.Conn
	reader     *bufio.Reader
	mu         sync.Mutex
	connected  bool
}

// QMPResponse QMP 响应
type QMPResponse struct {
	Return json.RawMessage `json:"return,omitempty"`
	Error  *QMPError       `json:"error,omitempty"`
	Event  string          `json:"event,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// QMPError QMP 错误
type QMPError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

// QMPCommand QMP 命令
type QMPCommand struct {
	Execute   string      `json:"execute"`
	Arguments interface{} `json:"arguments,omitempty"`
}

// QMPGreeting QMP 握手消息
type QMPGreeting struct {
	QMP struct {
		Version struct {
			Qemu struct {
				Micro int `json:"micro"`
				Minor int `json:"minor"`
				Major int `json:"major"`
			} `json:"qemu"`
		} `json:"version"`
		Capabilities []string `json:"capabilities"`
	} `json:"QMP"`
}

// QMPStatusInfo 虚拟机状态信息
type QMPStatusInfo struct {
	Running    bool   `json:"running"`
	Singlestep bool   `json:"singlestep"`
	Status     string `json:"status"`
}

// NewQMPClient 创建 QMP 客户端
func NewQMPClient(socketPath string) *QMPClient {
	return &QMPClient{
		socketPath: socketPath,
	}
}

// Connect 连接到 QMP socket
func (c *QMPClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	conn, err := net.DialTimeout("unix", c.socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect to QMP socket: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.connected = true

	// 读取 greeting 消息
	if _, err := c.readResponse(); err != nil {
		c.Close()
		return fmt.Errorf("read greeting: %w", err)
	}

	// 发送 qmp_capabilities 命令进入命令模式
	if err := c.sendCommand("qmp_capabilities", nil); err != nil {
		c.Close()
		return fmt.Errorf("negotiate capabilities: %w", err)
	}

	return nil
}

// Close 关闭连接
func (c *QMPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected 检查是否已连接
func (c *QMPClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// sendCommand 发送命令 (内部使用，需要持有锁)
func (c *QMPClient) sendCommand(execute string, args interface{}) error {
	cmd := QMPCommand{
		Execute:   execute,
		Arguments: args,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write command: %w", err)
	}

	// 读取响应
	resp, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("QMP error: %s - %s", resp.Error.Class, resp.Error.Desc)
	}

	return nil
}

// Execute 执行 QMP 命令
func (c *QMPClient) Execute(execute string, args interface{}) (*QMPResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	cmd := QMPCommand{
		Execute:   execute,
		Arguments: args,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("marshal command: %w", err)
	}

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		c.connected = false
		return nil, fmt.Errorf("write command: %w", err)
	}

	return c.readResponse()
}

// readResponse 读取响应
func (c *QMPClient) readResponse() (*QMPResponse, error) {
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// 可能会收到事件消息，跳过它们
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			c.connected = false
			return nil, fmt.Errorf("read response: %w", err)
		}

		var resp QMPResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}

		// 跳过事件消息
		if resp.Event != "" {
			continue
		}

		return &resp, nil
	}
}

// Stop 暂停虚拟机
func (c *QMPClient) Stop() error {
	_, err := c.Execute("stop", nil)
	return err
}

// Cont 恢复虚拟机
func (c *QMPClient) Cont() error {
	_, err := c.Execute("cont", nil)
	return err
}

// SystemPowerdown 发送 ACPI 关机信号 (优雅关机)
func (c *QMPClient) SystemPowerdown() error {
	_, err := c.Execute("system_powerdown", nil)
	return err
}

// SystemReset 重置虚拟机
func (c *QMPClient) SystemReset() error {
	_, err := c.Execute("system_reset", nil)
	return err
}

// Quit 立即退出 QEMU
func (c *QMPClient) Quit() error {
	_, err := c.Execute("quit", nil)
	return err
}

// QueryStatus 查询虚拟机状态
func (c *QMPClient) QueryStatus() (*QMPStatusInfo, error) {
	resp, err := c.Execute("query-status", nil)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("QMP error: %s - %s", resp.Error.Class, resp.Error.Desc)
	}

	var status QMPStatusInfo
	if err := json.Unmarshal(resp.Return, &status); err != nil {
		return nil, fmt.Errorf("unmarshal status: %w", err)
	}

	return &status, nil
}

// SendKey 发送按键
func (c *QMPClient) SendKey(keys []string) error {
	type keyValue struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}
	type args struct {
		Keys []keyValue `json:"keys"`
	}

	keyValues := make([]keyValue, len(keys))
	for i, key := range keys {
		keyValues[i] = keyValue{
			Type: "qcode",
			Data: key,
		}
	}

	_, err := c.Execute("send-key", args{Keys: keyValues})
	return err
}

// SendCtrlAltDel 发送 Ctrl+Alt+Del
func (c *QMPClient) SendCtrlAltDel() error {
	return c.SendKey([]string{"ctrl", "alt", "delete"})
}

// Screendump 截取屏幕
func (c *QMPClient) Screendump(filename string) error {
	type args struct {
		Filename string `json:"filename"`
	}
	_, err := c.Execute("screendump", args{Filename: filename})
	return err
}

// HumanMonitorCommand 执行 Human Monitor 命令
func (c *QMPClient) HumanMonitorCommand(cmd string) (string, error) {
	type args struct {
		CommandLine string `json:"command-line"`
	}
	resp, err := c.Execute("human-monitor-command", args{CommandLine: cmd})
	if err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("QMP error: %s - %s", resp.Error.Class, resp.Error.Desc)
	}

	var result string
	json.Unmarshal(resp.Return, &result)
	return result, nil
}

// QMPManager 管理多个 VM 的 QMP 连接
type QMPManager struct {
	clients map[string]*QMPClient // vmID -> client
	mu      sync.RWMutex
}

// NewQMPManager 创建 QMP 管理器
func NewQMPManager() *QMPManager {
	return &QMPManager{
		clients: make(map[string]*QMPClient),
	}
}

// GetOrCreate 获取或创建 QMP 客户端
func (m *QMPManager) GetOrCreate(vmID, socketPath string) (*QMPClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[vmID]; ok && client.IsConnected() {
		return client, nil
	}

	client := NewQMPClient(socketPath)
	if err := client.Connect(); err != nil {
		return nil, err
	}

	m.clients[vmID] = client
	return client, nil
}

// Remove 移除 QMP 客户端
func (m *QMPManager) Remove(vmID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[vmID]; ok {
		client.Close()
		delete(m.clients, vmID)
	}
}

// Close 关闭所有连接
func (m *QMPManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.Close()
	}
	m.clients = make(map[string]*QMPClient)
}
