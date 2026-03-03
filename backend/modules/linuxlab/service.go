package linuxlab

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

const (
	// Docker 镜像与容器名
	DefaultImage      = "tinylab/linux-lab"
	ContainerName     = "rde-linux-lab"
	LabDirInContainer = "/labs/linux-lab"
)

// Service Linux Lab 容器服务
type Service struct {
	logger   *zap.Logger
	cli      *client.Client
	image    string
	mu       sync.Mutex
	building bool
	booting  bool
	setting  bool
}

// NewService 创建服务
func NewService(logger *zap.Logger) *Service {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		logger.Warn("Failed to create Docker client for linux-lab", zap.Error(err))
	}

	return &Service{
		logger: logger,
		cli:    cli,
		image:  DefaultImage,
	}
}

// --- Docker 基础 ---

// DockerOK 检查 Docker 是否可用
func (s *Service) DockerOK() bool {
	if s.cli == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := s.cli.Ping(ctx)
	return err == nil
}

// ImageExists 检查 linux-lab 镜像是否已拉取
func (s *Service) ImageExists() bool {
	if s.cli == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	images, err := s.cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return false
	}
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == s.image+":latest" || strings.HasPrefix(tag, s.image+":") {
				return true
			}
		}
	}
	return false
}

// findContainer 查找 linux-lab 容器
func (s *Service) findContainer(ctx context.Context) (string, bool) {
	if s.cli == nil {
		return "", false
	}
	containers, err := s.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return "", false
	}
	for _, c := range containers {
		for _, name := range c.Names {
			if strings.TrimPrefix(name, "/") == ContainerName {
				return c.ID, c.State == "running"
			}
		}
	}
	return "", false
}

// ContainerRunning 检查容器是否运行中
func (s *Service) ContainerRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, running := s.findContainer(ctx)
	return running
}

// ContainerExists 检查容器是否存在（无论状态）
func (s *Service) ContainerExists() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, _ := s.findContainer(ctx)
	return id != ""
}

// IsInstalled 环境就绪 = 容器存在并运行
func (s *Service) IsInstalled() bool {
	return s.ContainerRunning()
}

// --- 状态 ---

// GetStatus 获取环境状态
func (s *Service) GetStatus() *LabStatus {
	dockerOK := s.DockerOK()
	imageReady := s.ImageExists()
	containerRunning := s.ContainerRunning()
	containerExists := s.ContainerExists()

	var currentBoard string
	if containerRunning {
		currentBoard = s.getCurrentBoard()
	}

	return &LabStatus{
		DockerOK:         dockerOK,
		ImageReady:       imageReady,
		ContainerRunning: containerRunning,
		ContainerExists:  containerExists,
		CurrentBoard:     currentBoard,
		Building:         s.IsBuilding(),
		Booting:          s.IsRunning(),
		Image:            s.image,
	}
}

func (s *Service) getCurrentBoard() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := s.execInContainer(ctx, "cat .board_config 2>/dev/null")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// --- Setup: 拉取镜像 + 创建并启动容器 ---

// IsSetting 检查是否正在安装
func (s *Service) IsSetting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setting
}

// Setup 拉取 linux-lab 镜像并创建容器
func (s *Service) Setup(progressChan chan<- ProgressEvent) {
	defer close(progressChan)

	s.mu.Lock()
	if s.setting {
		s.mu.Unlock()
		progressChan <- ProgressEvent{Status: "failed", Message: "安装正在进行中，请稍后"}
		return
	}
	s.setting = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.setting = false
		s.mu.Unlock()
	}()

	if !s.DockerOK() {
		progressChan <- ProgressEvent{Status: "failed", Message: "Docker 未运行或不可用，请先安装并启动 Docker"}
		return
	}

	// 1. 拉取镜像（如果不存在）
	if !s.ImageExists() {
		progressChan <- ProgressEvent{Status: "running", Message: fmt.Sprintf("正在拉取 Docker 镜像 %s（首次可能需要 10-30 分钟）...", s.image)}
		if err := s.pullImage(progressChan); err != nil {
			progressChan <- ProgressEvent{Status: "failed", Message: fmt.Sprintf("拉取镜像失败: %v", err)}
			return
		}
		progressChan <- ProgressEvent{Status: "running", Message: "✓ 镜像拉取完成"}
	} else {
		progressChan <- ProgressEvent{Status: "running", Message: "✓ 镜像已就绪"}
	}

	// 2. 创建并启动容器
	ctx := context.Background()
	id, running := s.findContainer(ctx)

	if id == "" {
		progressChan <- ProgressEvent{Status: "running", Message: "正在创建 Linux Lab 容器..."}
		var err error
		id, err = s.createContainer(ctx)
		if err != nil {
			progressChan <- ProgressEvent{Status: "failed", Message: fmt.Sprintf("创建容器失败: %v", err)}
			return
		}
		progressChan <- ProgressEvent{Status: "running", Message: fmt.Sprintf("容器已创建: %s", id[:12])}
	}

	// 确保容器在运行
	if !running {
		progressChan <- ProgressEvent{Status: "running", Message: "正在启动容器..."}
		if err := s.cli.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
			progressChan <- ProgressEvent{Status: "failed", Message: fmt.Sprintf("启动容器失败: %v", err)}
			return
		}
	}

	progressChan <- ProgressEvent{Status: "completed", Message: "Linux Lab 容器已就绪"}
	s.logger.Info("Linux Lab container ready", zap.String("container", id[:12]))
}

func (s *Service) pullImage(progressChan chan<- ProgressEvent) error {
	ctx := context.Background()
	reader, err := s.cli.ImagePull(ctx, s.image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	for {
		var event map[string]interface{}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if status, ok := event["status"].(string); ok {
			line := status
			if id, ok := event["id"].(string); ok {
				line = id + ": " + line
			}
			if progress, ok := event["progress"].(string); ok {
				line += " " + progress
			}
			progressChan <- ProgressEvent{Status: "running", Line: line}
		}
	}
	return nil
}

func (s *Service) createContainer(ctx context.Context) (string, error) {
	containerConfig := &containertypes.Config{
		Image: s.image,
		// 保持容器常驻运行，按需 docker exec 进去执行
		Cmd: []string{"sleep", "infinity"},
		Labels: map[string]string{
			"rde.app": "linux-lab",
		},
	}

	hostConfig := &containertypes.HostConfig{
		// 特权模式，QEMU/KVM 需要
		Privileged: true,
		RestartPolicy: containertypes.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	resp, err := s.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, ContainerName)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// --- 容器内执行命令 ---

// execInContainer 在容器中执行命令，返回完整输出
func (s *Service) execInContainer(ctx context.Context, command string) (string, error) {
	if s.cli == nil {
		return "", fmt.Errorf("docker client not available")
	}

	id, running := s.findContainer(ctx)
	if !running {
		return "", fmt.Errorf("容器未运行")
	}

	execConfig := types.ExecConfig{
		Cmd:          []string{"/bin/bash", "-c", command},
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		WorkingDir:   LabDirInContainer,
	}

	execResp, err := s.cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return "", err
	}

	attachResp, err := s.cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", err
	}
	defer attachResp.Close()

	var output strings.Builder
	scanner := bufio.NewScanner(attachResp.Reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		// Docker multiplex stream header (8 bytes) for non-TTY
		if len(line) > 8 && (line[0] == 1 || line[0] == 2) {
			line = line[8:]
		}
		output.WriteString(line)
		output.WriteString("\n")
	}

	return output.String(), nil
}

// execInContainerStream 在容器中执行命令，逐行流式输出
func (s *Service) execInContainerStream(ctx context.Context, command string, progressChan chan<- ProgressEvent) error {
	if s.cli == nil {
		return fmt.Errorf("docker client not available")
	}

	id, running := s.findContainer(ctx)
	if !running {
		return fmt.Errorf("容器未运行")
	}

	execConfig := types.ExecConfig{
		Cmd:          []string{"/bin/bash", "-c", command},
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true, // TTY 让 make 输出实时刷新
		WorkingDir:   LabDirInContainer,
	}

	execResp, err := s.cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return err
	}

	attachResp, err := s.cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return err
	}
	defer attachResp.Close()

	scanner := bufio.NewScanner(attachResp.Reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Text()
		progressChan <- ProgressEvent{
			Status: "running",
			Line:   line,
		}
	}

	// 检查退出码
	inspectResp, err := s.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return err
	}
	if inspectResp.ExitCode != 0 {
		return fmt.Errorf("命令退出码: %d", inspectResp.ExitCode)
	}

	return nil
}

// --- 开发板 ---

// ListBoards 在容器内列出所有开发板
func (s *Service) ListBoards() ([]*Board, error) {
	if !s.ContainerRunning() {
		return nil, fmt.Errorf("容器未运行")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 在容器内用 find 列出 boards/{arch}/{mach}/Makefile
	out, err := s.execInContainer(ctx, `find boards -mindepth 3 -maxdepth 3 -name Makefile -type f | sort`)
	if err != nil {
		return nil, err
	}

	var boards []*Board
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// boards/arm/vexpress-a9/Makefile -> arm/vexpress-a9
		parts := strings.Split(line, "/")
		if len(parts) < 4 {
			continue
		}
		arch := parts[1]
		mach := parts[2]

		detail, err := s.getBoardFromContainer(ctx, arch, mach)
		if err != nil {
			boards = append(boards, &Board{
				Arch:     arch,
				Name:     mach,
				FullPath: arch + "/" + mach,
			})
			continue
		}
		boards = append(boards, detail)
	}

	return boards, nil
}

func (s *Service) getBoardFromContainer(ctx context.Context, arch, mach string) (*Board, error) {
	makefilePath := fmt.Sprintf("boards/%s/%s/Makefile", arch, mach)
	out, err := s.execInContainer(ctx, fmt.Sprintf("cat %s", makefilePath))
	if err != nil {
		return nil, err
	}
	return ParseBoardFromContent(out, arch, mach), nil
}

// GetBoardDetail 获取指定开发板详情
func (s *Service) GetBoardDetail(boardPath string) (*Board, error) {
	parts := strings.SplitN(boardPath, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid board path: %s", boardPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.getBoardFromContainer(ctx, parts[0], parts[1])
}

// SwitchBoard 切换开发板
func (s *Service) SwitchBoard(boardPath string) error {
	if !s.ContainerRunning() {
		return fmt.Errorf("容器未运行")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查板存在
	checkCmd := fmt.Sprintf("test -f boards/%s/Makefile && echo ok", boardPath)
	out, err := s.execInContainer(ctx, checkCmd)
	if err != nil || !strings.Contains(out, "ok") {
		return fmt.Errorf("board %s does not exist", boardPath)
	}

	_, err = s.execInContainer(ctx, fmt.Sprintf("echo '%s' > .board_config", boardPath))
	return err
}

// --- 构建与执行 ---

// IsBuilding 检查是否正在构建
func (s *Service) IsBuilding() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.building
}

// ExecMake 在容器内执行 make 命令
func (s *Service) ExecMake(ctx context.Context, target string, board string, progressChan chan<- ProgressEvent) error {
	defer close(progressChan)

	s.mu.Lock()
	if s.building {
		s.mu.Unlock()
		progressChan <- ProgressEvent{Status: "failed", Message: "已有构建任务正在运行"}
		return fmt.Errorf("build already in progress")
	}
	s.building = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.building = false
		s.mu.Unlock()
	}()

	if !s.ContainerRunning() {
		progressChan <- ProgressEvent{Status: "failed", Message: "容器未运行，请先初始化环境"}
		return fmt.Errorf("container not running")
	}

	cmd := "make"
	if board != "" {
		cmd += fmt.Sprintf(" BOARD=%s", board)
	}
	cmd += " " + target

	s.logger.Info("Executing make in container",
		zap.String("target", target),
		zap.String("board", board),
		zap.String("cmd", cmd),
	)

	progressChan <- ProgressEvent{
		Status:  "running",
		Message: fmt.Sprintf(">>> %s  (容器内执行)", cmd),
	}

	err := s.execInContainerStream(ctx, cmd, progressChan)
	if err != nil {
		progressChan <- ProgressEvent{
			Status:  "failed",
			Message: fmt.Sprintf("命令执行失败: %v", err),
		}
		return err
	}

	progressChan <- ProgressEvent{
		Status:  "completed",
		Message: "执行完成",
	}
	return nil
}

// Boot 在容器内启动虚拟板
func (s *Service) Boot(board string, progressChan chan<- ProgressEvent) error {
	defer close(progressChan)

	s.mu.Lock()
	if s.booting {
		s.mu.Unlock()
		progressChan <- ProgressEvent{Status: "failed", Message: "已有虚拟板正在运行"}
		return fmt.Errorf("already booting")
	}
	s.booting = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.booting = false
		s.mu.Unlock()
	}()

	if !s.ContainerRunning() {
		progressChan <- ProgressEvent{Status: "failed", Message: "容器未运行"}
		return fmt.Errorf("container not running")
	}

	cmd := "make"
	if board != "" {
		cmd += fmt.Sprintf(" BOARD=%s", board)
	}
	cmd += " boot"

	s.logger.Info("Booting board in container", zap.String("board", board))
	progressChan <- ProgressEvent{
		Status:  "running",
		Message: fmt.Sprintf("启动开发板: %s (容器内)", board),
	}

	ctx := context.Background()
	err := s.execInContainerStream(ctx, cmd, progressChan)

	if err != nil {
		progressChan <- ProgressEvent{Status: "failed", Message: fmt.Sprintf("启动结束: %v", err)}
		return err
	}

	progressChan <- ProgressEvent{Status: "completed", Message: "虚拟板已停止"}
	return nil
}

// IsRunning 检查是否有虚拟板在运行
func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.booting
}

// StopBoot 停止正在运行的虚拟板
func (s *Service) StopBoot() error {
	if !s.ContainerRunning() {
		return fmt.Errorf("容器未运行")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 在容器内 kill qemu 进程
	_, _ = s.execInContainer(ctx, "pkill -f qemu-system || true")
	return nil
}

// StopContainer 停止整个容器
func (s *Service) StopContainer() error {
	if s.cli == nil {
		return fmt.Errorf("docker client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, _ := s.findContainer(ctx)
	if id == "" {
		return nil
	}

	timeout := 15
	return s.cli.ContainerStop(ctx, id, containertypes.StopOptions{Timeout: &timeout})
}

// RemoveContainer 删除容器
func (s *Service) RemoveContainer() error {
	if s.cli == nil {
		return fmt.Errorf("docker client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	id, _ := s.findContainer(ctx)
	if id == "" {
		return nil
	}

	return s.cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
}
