// Package adb 提供 Android Debug Bridge 客户端功能
// 基于 gadb 库封装，提供设备发现、连接、文件传输等功能
package adb

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/electricbubble/gadb"
)

// Client ADB 客户端
type Client struct {
	client gadb.Client
}

// Device 设备信息
type Device struct {
	Serial         string `json:"serial"`
	Model          string `json:"model"`
	Brand          string `json:"brand"`
	AndroidVersion string `json:"android_version"`
	Connected      bool   `json:"connected"`
	State          string `json:"state"`
}

// NewClient 创建 ADB 客户端
func NewClient() (*Client, error) {
	client, err := gadb.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to adb server: %w", err)
	}

	return &Client{client: client}, nil
}

// ServerVersion 获取 ADB 服务版本
func (c *Client) ServerVersion() (int, error) {
	return c.client.ServerVersion()
}

// ListDevices 列出所有设备
func (c *Client) ListDevices() ([]Device, error) {
	devices, err := c.client.DeviceList()
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	var result []Device
	for _, d := range devices {
		dev := Device{
			Serial:    d.Serial(),
			Connected: true,
			State:     "device",
		}

		if props, err := getDeviceProps(d); err == nil {
			dev.Model = props["ro.product.model"]
			dev.Brand = props["ro.product.brand"]
			dev.AndroidVersion = props["ro.build.version.release"]
		}

		result = append(result, dev)
	}

	return result, nil
}

// getDeviceProps 获取设备属性
func getDeviceProps(device gadb.Device) (map[string]string, error) {
	props := make(map[string]string)

	keys := []string{
		"ro.product.model",
		"ro.product.brand",
		"ro.build.version.release",
		"ro.product.name",
	}

	for _, key := range keys {
		output, err := device.RunShellCommand("getprop", key)
		if err == nil {
			props[key] = strings.TrimSpace(output)
		}
	}

	return props, nil
}

// GetDevice 获取指定设备
func (c *Client) GetDevice(serial string) (*DeviceClient, error) {
	devices, err := c.client.DeviceList()
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	for _, d := range devices {
		if d.Serial() == serial {
			return &DeviceClient{device: d, serial: serial}, nil
		}
	}

	return nil, fmt.Errorf("device not found: %s", serial)
}

// Connect 连接到无线设备
func (c *Client) Connect(address string) error {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		host = address
		portStr = "5555"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port: %s", portStr)
	}

	err = c.client.Connect(host, port)
	if err != nil {
		return fmt.Errorf("failed to connect to %s:%d: %w", host, port, err)
	}

	return nil
}

// Disconnect 断开无线设备
func (c *Client) Disconnect(address string) error {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		host = address
		portStr = "5555"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port: %s", portStr)
	}

	err = c.client.Disconnect(host, port)
	if err != nil {
		return fmt.Errorf("failed to disconnect from %s:%d: %w", host, port, err)
	}

	return nil
}

// DisconnectAll 断开所有无线设备
func (c *Client) DisconnectAll() error {
	return c.client.DisconnectAll()
}

// DeviceClient 设备客户端
type DeviceClient struct {
	device gadb.Device
	serial string
}

// Serial 获取设备序列号
func (d *DeviceClient) Serial() string {
	return d.serial
}

// State 获取设备状态
func (d *DeviceClient) State() (string, error) {
	state, err := d.device.State()
	if err != nil {
		return "", err
	}
	return string(state), nil
}

// RunShellCommand 执行 Shell 命令
func (d *DeviceClient) RunShellCommand(cmd string, args ...string) (string, error) {
	return d.device.RunShellCommand(cmd, args...)
}

// Push 推送文件到设备
func (d *DeviceClient) Push(localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return d.device.PushFile(file, remotePath, time.Now())
}

// PushBytes 推送字节数据到设备
func (d *DeviceClient) PushBytes(data []byte, remotePath string) error {
	reader := bytes.NewReader(data)
	return d.device.Push(reader, remotePath, time.Now())
}

// Pull 从设备拉取文件
func (d *DeviceClient) Pull(remotePath string) ([]byte, error) {
	var buf bytes.Buffer
	err := d.device.Pull(remotePath, &buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Forward 设置端口转发
func (d *DeviceClient) Forward(localPort, remotePort int) error {
	return d.device.Forward(localPort, remotePort)
}

// ForwardKill 移除端口转发
func (d *DeviceClient) ForwardKill(localPort int) error {
	return d.device.ForwardKill(localPort)
}

// ScreenSize 获取屏幕尺寸
func (d *DeviceClient) ScreenSize() (width, height int, err error) {
	output, err := d.device.RunShellCommand("wm", "size")
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Physical size:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				size := strings.TrimSpace(parts[1])
				dims := strings.Split(size, "x")
				if len(dims) == 2 {
					width, _ = strconv.Atoi(dims[0])
					height, _ = strconv.Atoi(dims[1])
					return width, height, nil
				}
			}
		}
	}

	return 0, 0, fmt.Errorf("failed to parse screen size")
}

// InputTap 模拟点击
func (d *DeviceClient) InputTap(x, y int) error {
	_, err := d.device.RunShellCommand("input", "tap", strconv.Itoa(x), strconv.Itoa(y))
	return err
}

// InputSwipe 模拟滑动
func (d *DeviceClient) InputSwipe(x1, y1, x2, y2, durationMs int) error {
	_, err := d.device.RunShellCommand("input", "swipe",
		strconv.Itoa(x1), strconv.Itoa(y1),
		strconv.Itoa(x2), strconv.Itoa(y2),
		strconv.Itoa(durationMs))
	return err
}

// InputKeyEvent 发送按键事件
func (d *DeviceClient) InputKeyEvent(keycode int) error {
	_, err := d.device.RunShellCommand("input", "keyevent", strconv.Itoa(keycode))
	return err
}

// InputText 输入文本
func (d *DeviceClient) InputText(text string) error {
	text = strings.ReplaceAll(text, " ", "%s")
	_, err := d.device.RunShellCommand("input", "text", text)
	return err
}

// IsScreenOn 检查屏幕是否点亮
func (d *DeviceClient) IsScreenOn() (bool, error) {
	output, err := d.device.RunShellCommand("dumpsys", "power")
	if err != nil {
		return false, err
	}

	return strings.Contains(output, "mHoldingDisplaySuspendBlocker=true") ||
		strings.Contains(output, "Display Power: state=ON"), nil
}

// WakeUp 唤醒屏幕
func (d *DeviceClient) WakeUp() error {
	return d.InputKeyEvent(224)
}

// Sleep 关闭屏幕
func (d *DeviceClient) Sleep() error {
	return d.InputKeyEvent(223)
}

// GetRawDevice 获取底层 gadb.Device
func (d *DeviceClient) GetRawDevice() gadb.Device {
	return d.device
}
