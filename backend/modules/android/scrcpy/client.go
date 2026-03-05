// Package scrcpy 提供 scrcpy 协议客户端
// 直接与 scrcpy-server 通信，无需依赖 scrcpy 命令行工具
// 使用 reverse 模式：客户端监听，server 连接过来
package scrcpy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/ruizi-store/rde/backend/modules/android/adb"
)

// 常量定义
const (
	// ServerVersion scrcpy-server 版本
	ServerVersion = "3.3.4"

	// DefaultLocalPort 默认本地监听端口
	DefaultLocalPort = 27183

	// 消息类型
	MsgTypeInjectKeycode           = 0
	MsgTypeInjectText              = 1
	MsgTypeInjectTouchEvent        = 2
	MsgTypeInjectScrollEvent       = 3
	MsgTypeBackOrScreenOn          = 4
	MsgTypeExpandNotificationPanel = 5
	MsgTypeExpandSettingsPanel     = 6
	MsgTypeCollapsePanels          = 7
	MsgTypeGetClipboard            = 8
	MsgTypeSetClipboard            = 9
	MsgTypeSetScreenPowerMode      = 10
	MsgTypeRotateDevice            = 11

	// 触摸动作
	ActionDown = 0
	ActionUp   = 1
	ActionMove = 2

	// 按键动作
	KeyActionDown = 0
	KeyActionUp   = 1

	// 屏幕电源模式
	ScreenPowerModeOff = 0
	ScreenPowerModeOn  = 2
)

// Config scrcpy 配置
type Config struct {
	MaxSize            int    `json:"max_size"`
	Bitrate            int    `json:"bitrate"`
	MaxFps             int    `json:"max_fps"`
	VideoCodec         string `json:"video_codec"`
	AudioCodec         string `json:"audio_codec"`
	AudioEnabled       bool   `json:"audio_enabled"`
	TurnScreenOff      bool   `json:"turn_screen_off"`
	StayAwake          bool   `json:"stay_awake"`
	ShowTouches        bool   `json:"show_touches"`
	ControlEnabled     bool   `json:"control_enabled"`
	CaptureOrientation string `json:"capture_orientation"`
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		MaxSize:        1920,
		Bitrate:        8000000,
		MaxFps:         60,
		VideoCodec:     "h264",
		AudioCodec:     "aac",
		AudioEnabled:   false,
		TurnScreenOff:  false,
		StayAwake:      true,
		ShowTouches:    false,
		ControlEnabled: true,
	}
}

// Client scrcpy 客户端
type Client struct {
	adbClient *adb.Client
	device    *adb.DeviceClient
	config    Config

	mu       sync.RWMutex
	running  bool
	cancelFn context.CancelFunc

	videoConn   net.Conn
	audioConn   net.Conn
	controlConn net.Conn

	// 视频信息
	deviceName   string
	screenWidth  int
	screenHeight int

	// 回调
	onVideo       func(data []byte, pts int64, isKeyFrame bool)
	onAudio       func(data []byte)
	onAudioConfig func(data []byte)
	onError       func(err error)
	onDisconnect  func()
}

// NewClient 创建 scrcpy 客户端
func NewClient(adbClient *adb.Client, serial string, config Config) (*Client, error) {
	device, err := adbClient.GetDevice(serial)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	return &Client{
		adbClient: adbClient,
		device:    device,
		config:    config,
	}, nil
}

// SetVideoCallback 设置视频回调
func (c *Client) SetVideoCallback(fn func(data []byte, pts int64, isKeyFrame bool)) {
	c.onVideo = fn
}

// SetAudioCallback 设置音频回调
func (c *Client) SetAudioCallback(fn func(data []byte)) {
	c.onAudio = fn
}

// SetAudioConfigCallback 设置音频配置回调
func (c *Client) SetAudioConfigCallback(fn func(data []byte)) {
	c.onAudioConfig = fn
}

// SetErrorCallback 设置错误回调
func (c *Client) SetErrorCallback(fn func(err error)) {
	c.onError = fn
}

// SetDisconnectCallback 设置断开回调
func (c *Client) SetDisconnectCallback(fn func()) {
	c.onDisconnect = fn
}

// Start 启动 scrcpy (使用 reverse 模式)
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("already running")
	}
	c.running = true
	c.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	c.cancelFn = cancel

	// 1. 部署 scrcpy-server
	if err := c.deployServer(); err != nil {
		c.Stop()
		return fmt.Errorf("failed to deploy server: %w", err)
	}

	// 2. 使用固定的 scid
	localPort := DefaultLocalPort
	scid := uint32(localPort)
	socketName := fmt.Sprintf("scrcpy_%08x", scid)

	// 3. 设置 reverse 端口转发
	if err := c.setupReverse(socketName, localPort); err != nil {
		c.Stop()
		return fmt.Errorf("failed to setup reverse: %w", err)
	}

	// 4. 本地监听端口
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		c.Stop()
		return fmt.Errorf("failed to listen on port %d: %w", localPort, err)
	}

	// 5. 启动 server
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- c.startServer(ctx, scid)
	}()

	// 6. 接受 video socket 连接
	acceptTimeout := 30 * time.Second
	videoConn, err := c.acceptWithServerCheck(listener, acceptTimeout, serverErrCh)
	if err != nil {
		listener.Close()
		c.Stop()
		return fmt.Errorf("failed to accept video socket: %w", err)
	}
	c.videoConn = videoConn

	// 7. 读取设备信息
	if err := c.readDeviceInfo(); err != nil {
		listener.Close()
		c.Stop()
		return fmt.Errorf("failed to read device info: %w", err)
	}

	// 8. 接受音频 socket 连接
	if c.config.AudioEnabled {
		audioConn, err := c.acceptWithServerCheck(listener, 10*time.Second, serverErrCh)
		if err != nil {
			listener.Close()
			c.Stop()
			return fmt.Errorf("failed to accept audio socket: %w", err)
		}
		c.audioConn = audioConn
	}

	// 9. 接受控制 socket 连接
	if c.config.ControlEnabled {
		controlConn, err := c.acceptWithServerCheck(listener, 10*time.Second, serverErrCh)
		if err != nil {
			listener.Close()
			c.Stop()
			return fmt.Errorf("failed to accept control socket: %w", err)
		}
		c.controlConn = controlConn
	}

	listener.Close()

	// 10. 开始读取视频流
	go c.readVideoLoop(ctx)

	// 11. 开始读取音频流
	if c.config.AudioEnabled && c.audioConn != nil {
		go c.readAudioLoop(ctx)
	}

	return nil
}

// setupReverse 设置 adb reverse 端口转发
func (c *Client) setupReverse(socketName string, localPort int) error {
	serial := c.device.Serial()

	// 先移除可能存在的旧转发
	removeCmd := exec.Command("adb", "-s", serial, "reverse", "--remove",
		fmt.Sprintf("localabstract:%s", socketName))
	removeCmd.Run()

	// 设置 reverse
	cmd := exec.Command("adb", "-s", serial, "reverse",
		fmt.Sprintf("localabstract:%s", socketName),
		fmt.Sprintf("tcp:%d", localPort))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb reverse failed: %s - %w", string(output), err)
	}
	return nil
}

// cleanupReverse 清理 adb reverse
func (c *Client) cleanupReverse(socketName string) {
	cmd := exec.Command("adb", "-s", c.device.Serial(), "reverse", "--remove",
		fmt.Sprintf("localabstract:%s", socketName))
	cmd.Run()
}

// deployServer 部署 scrcpy-server 到设备
func (c *Client) deployServer() error {
	serverData := GetServerJar()
	if len(serverData) == 0 {
		return fmt.Errorf("scrcpy-server.jar not embedded")
	}

	remotePath := "/data/local/tmp/scrcpy-server.jar"
	return c.device.PushBytes(serverData, remotePath)
}

// acceptWithServerCheck 接受连接，同时监控 server 是否提前退出
func (c *Client) acceptWithServerCheck(listener net.Listener, timeout time.Duration, serverErrCh <-chan error) (net.Conn, error) {
	type acceptResult struct {
		conn net.Conn
		err  error
	}

	resultCh := make(chan acceptResult, 1)
	go func() {
		listener.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))
		conn, err := listener.Accept()
		resultCh <- acceptResult{conn, err}
	}()

	select {
	case result := <-resultCh:
		return result.conn, result.err
	case err := <-serverErrCh:
		if err != nil {
			return nil, fmt.Errorf("scrcpy server exited: %w", err)
		}
		result := <-resultCh
		return result.conn, result.err
	}
}

// startServer 启动 scrcpy-server (reverse 模式)
func (c *Client) startServer(ctx context.Context, scid uint32) error {
	shellCmd := fmt.Sprintf(
		"CLASSPATH=/data/local/tmp/scrcpy-server.jar app_process / com.genymobile.scrcpy.Server %s "+
			"scid=%08x "+
			"audio=%t "+
			"audio_codec=%s "+
			"control=%t "+
			"cleanup=true "+
			"max_size=%d "+
			"max_fps=%d "+
			"video_bit_rate=%d "+
			"video_codec=%s "+
			"send_frame_meta=true "+
			"log_level=info",
		ServerVersion,
		scid,
		c.config.AudioEnabled,
		c.config.AudioCodec,
		c.config.ControlEnabled,
		c.config.MaxSize,
		c.config.MaxFps,
		c.config.Bitrate,
		c.config.VideoCodec,
	)

	if c.config.CaptureOrientation != "" {
		shellCmd += fmt.Sprintf(" capture_orientation=%s", c.config.CaptureOrientation)
	}

	log.Printf("[SCRCPY] Starting server")

	cmd := exec.CommandContext(ctx, "adb", "-s", c.device.Serial(), "shell", shellCmd)

	stdoutPipe, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			log.Printf("[SCRCPY-SERVER] %s", scanner.Text())
		}
	}()

	err := cmd.Wait()
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("[SCRCPY] Server exited with error: %v", err)
			if c.onError != nil {
				c.onError(fmt.Errorf("server error: %w", err))
			}
			return fmt.Errorf("server exited: %w", err)
		}
	}
	log.Printf("[SCRCPY] Server exited normally")
	return nil
}

// readDeviceInfo 读取设备信息 (scrcpy v3.x 协议)
func (c *Client) readDeviceInfo() error {
	// 读取设备名称 (64 bytes)
	nameBuf := make([]byte, 64)
	if _, err := io.ReadFull(c.videoConn, nameBuf); err != nil {
		return fmt.Errorf("failed to read device name: %w", err)
	}
	c.deviceName = string(bytes.TrimRight(nameBuf, "\x00"))

	// 读取 codec metadata (12 bytes: codec_id + width + height)
	codecMeta := make([]byte, 12)
	if _, err := io.ReadFull(c.videoConn, codecMeta); err != nil {
		return fmt.Errorf("failed to read codec metadata: %w", err)
	}
	c.screenWidth = int(binary.BigEndian.Uint32(codecMeta[4:8]))
	c.screenHeight = int(binary.BigEndian.Uint32(codecMeta[8:12]))

	return nil
}

// readVideoLoop 读取视频流循环
func (c *Client) readVideoLoop(ctx context.Context) {
	defer func() {
		if c.onDisconnect != nil {
			c.onDisconnect()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 读取帧元数据 (12 bytes: PTS 8 + Size 4)
		metaBuf := make([]byte, 12)
		if _, err := io.ReadFull(c.videoConn, metaBuf); err != nil {
			if err != io.EOF {
				if c.onError != nil {
					c.onError(fmt.Errorf("failed to read frame meta: %w", err))
				}
			}
			return
		}

		pts := int64(binary.BigEndian.Uint64(metaBuf[0:8]))
		size := binary.BigEndian.Uint32(metaBuf[8:12])

		// 检查是否是配置包
		isConfig := pts == -1 || pts == int64(^uint64(0)>>1)

		// 读取帧数据
		frameBuf := make([]byte, size)
		if _, err := io.ReadFull(c.videoConn, frameBuf); err != nil {
			if c.onError != nil {
				c.onError(fmt.Errorf("failed to read frame data: %w", err))
			}
			return
		}

		isKeyFrame := isConfig || isIDRFrame(frameBuf)

		if c.onVideo != nil {
			c.onVideo(frameBuf, pts, isKeyFrame)
		}
	}
}

// readAudioLoop 读取音频流循环
func (c *Client) readAudioLoop(ctx context.Context) {
	// 先读取音频 codec metadata (4 bytes codec_id)
	codecMeta := make([]byte, 4)
	if _, err := io.ReadFull(c.audioConn, codecMeta); err != nil {
		if c.onError != nil {
			c.onError(fmt.Errorf("failed to read audio codec metadata: %w", err))
		}
		return
	}

	codecId := binary.BigEndian.Uint32(codecMeta)
	if codecId == 0 {
		if c.onError != nil {
			c.onError(fmt.Errorf("audio stream disabled by device"))
		}
		return
	}
	if codecId == 1 {
		if c.onError != nil {
			c.onError(fmt.Errorf("audio stream configuration error on device"))
		}
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 读取音频帧元数据 (12 bytes: PTS 8 + Size 4)
		metaBuf := make([]byte, 12)
		if _, err := io.ReadFull(c.audioConn, metaBuf); err != nil {
			if err != io.EOF {
				if c.onError != nil {
					c.onError(fmt.Errorf("failed to read audio frame meta: %w", err))
				}
			}
			return
		}

		pts := binary.BigEndian.Uint64(metaBuf[0:8])
		size := binary.BigEndian.Uint32(metaBuf[8:12])

		if size == 0 || size > 1024*1024 {
			continue
		}

		audioBuf := make([]byte, size)
		if _, err := io.ReadFull(c.audioConn, audioBuf); err != nil {
			if c.onError != nil {
				c.onError(fmt.Errorf("failed to read audio data: %w", err))
			}
			return
		}

		// 配置包 (PTS bit 63 = PACKET_FLAG_CONFIG)
		if pts&(1<<63) != 0 {
			if c.onAudioConfig != nil {
				c.onAudioConfig(audioBuf)
			}
			continue
		}

		if c.onAudio != nil {
			c.onAudio(audioBuf)
		}
	}
}

// isIDRFrame 检测是否是 IDR 帧
func isIDRFrame(data []byte) bool {
	if len(data) < 5 {
		return false
	}

	i := 0
	for i < len(data)-4 {
		if data[i] == 0 && data[i+1] == 0 {
			var nalStart int
			if data[i+2] == 0 && data[i+3] == 1 {
				nalStart = i + 4
			} else if data[i+2] == 1 {
				nalStart = i + 3
			} else {
				i++
				continue
			}

			if nalStart < len(data) {
				nalType := data[nalStart] & 0x1F
				if nalType == 5 || nalType == 7 || nalType == 8 {
					return true
				}
			}
		}
		i++
	}

	return false
}

// Stop 停止 scrcpy
func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	c.running = false

	if c.cancelFn != nil {
		c.cancelFn()
	}

	if c.videoConn != nil {
		c.videoConn.Close()
		c.videoConn = nil
	}

	if c.audioConn != nil {
		c.audioConn.Close()
		c.audioConn = nil
	}

	if c.controlConn != nil {
		c.controlConn.Close()
		c.controlConn = nil
	}

	// 清理 reverse 转发
	scid := uint32(DefaultLocalPort)
	socketName := fmt.Sprintf("scrcpy_%08x", scid)
	c.cleanupReverse(socketName)
}

// IsRunning 是否运行中
func (c *Client) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// DeviceName 获取设备名称
func (c *Client) DeviceName() string {
	return c.deviceName
}

// ScreenSize 获取屏幕尺寸
func (c *Client) ScreenSize() (width, height int) {
	return c.screenWidth, c.screenHeight
}

// === 控制方法 ===

// InjectTouch 注入触摸事件
func (c *Client) InjectTouch(action int, x, y float32, pressure float32) error {
	if c.controlConn == nil {
		return fmt.Errorf("control not enabled")
	}

	posX := int(x * float32(c.screenWidth))
	posY := int(y * float32(c.screenHeight))

	// scrcpy v3.x 格式: 32 bytes
	msg := make([]byte, 32)
	msg[0] = MsgTypeInjectTouchEvent
	msg[1] = byte(action)
	binary.BigEndian.PutUint64(msg[2:10], 0xFFFFFFFFFFFFFFFF)
	binary.BigEndian.PutUint32(msg[10:14], uint32(posX))
	binary.BigEndian.PutUint32(msg[14:18], uint32(posY))
	binary.BigEndian.PutUint16(msg[18:20], uint16(c.screenWidth))
	binary.BigEndian.PutUint16(msg[20:22], uint16(c.screenHeight))
	binary.BigEndian.PutUint16(msg[22:24], uint16(pressure*0xFFFF))
	actionButton := uint32(0)
	if action == ActionDown || action == ActionUp {
		actionButton = 1
	}
	binary.BigEndian.PutUint32(msg[24:28], actionButton)
	buttons := uint32(0)
	if action == ActionDown || action == ActionMove {
		buttons = 1
	}
	binary.BigEndian.PutUint32(msg[28:32], buttons)

	_, err := c.controlConn.Write(msg)
	return err
}

// InjectKeycode 注入按键事件
func (c *Client) InjectKeycode(action, keycode, repeat, metaState int) error {
	if c.controlConn == nil {
		return fmt.Errorf("control not enabled")
	}

	msg := make([]byte, 14)
	msg[0] = MsgTypeInjectKeycode
	msg[1] = byte(action)
	binary.BigEndian.PutUint32(msg[2:6], uint32(keycode))
	binary.BigEndian.PutUint32(msg[6:10], uint32(repeat))
	binary.BigEndian.PutUint32(msg[10:14], uint32(metaState))

	_, err := c.controlConn.Write(msg)
	return err
}

// InjectScroll 注入滚动事件
func (c *Client) InjectScroll(x, y float32, hScroll, vScroll int) error {
	if c.controlConn == nil {
		return fmt.Errorf("control not enabled")
	}

	posX := int(x * float32(c.screenWidth))
	posY := int(y * float32(c.screenHeight))

	msg := make([]byte, 21)
	msg[0] = MsgTypeInjectScrollEvent
	binary.BigEndian.PutUint32(msg[1:5], uint32(posX))
	binary.BigEndian.PutUint32(msg[5:9], uint32(posY))
	binary.BigEndian.PutUint16(msg[9:11], uint16(c.screenWidth))
	binary.BigEndian.PutUint16(msg[11:13], uint16(c.screenHeight))
	binary.BigEndian.PutUint32(msg[13:17], uint32(hScroll))
	binary.BigEndian.PutUint32(msg[17:21], uint32(vScroll))

	_, err := c.controlConn.Write(msg)
	return err
}

// BackOrScreenOn 返回键或点亮屏幕
func (c *Client) BackOrScreenOn(action int) error {
	if c.controlConn == nil {
		return fmt.Errorf("control not enabled")
	}

	msg := []byte{MsgTypeBackOrScreenOn, byte(action)}
	_, err := c.controlConn.Write(msg)
	return err
}

// SetScreenPowerMode 设置屏幕电源模式
func (c *Client) SetScreenPowerMode(mode int) error {
	if c.controlConn == nil {
		return fmt.Errorf("control not enabled")
	}

	msg := []byte{MsgTypeSetScreenPowerMode, byte(mode)}
	_, err := c.controlConn.Write(msg)
	return err
}

// ExpandNotificationPanel 展开通知面板
func (c *Client) ExpandNotificationPanel() error {
	if c.controlConn == nil {
		return fmt.Errorf("control not enabled")
	}

	msg := []byte{MsgTypeExpandNotificationPanel}
	_, err := c.controlConn.Write(msg)
	return err
}

// CollapsePanels 收起面板
func (c *Client) CollapsePanels() error {
	if c.controlConn == nil {
		return fmt.Errorf("control not enabled")
	}

	msg := []byte{MsgTypeCollapsePanels}
	_, err := c.controlConn.Write(msg)
	return err
}

// RotateDevice 旋转设备
func (c *Client) RotateDevice() error {
	if c.controlConn == nil {
		return fmt.Errorf("control not enabled")
	}

	msg := []byte{MsgTypeRotateDevice}
	_, err := c.controlConn.Write(msg)
	return err
}
