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
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/ruizi-store/rde/backend/pkg/offline"
	"go.uber.org/zap"
)

const (
	// Docker 镜像与容器名
	// 官方镜像 ENTRYPOINT 为 /tools/lab/run，依赖 cloud-lab 工具挂载；
	// RDE 无覆盖入口点 + 容器内克隆源码的方式独立运行。
	DefaultImage      = "tinylab/linux-lab"
	ContainerName     = "rde-linux-lab"
	LabDirInContainer = "/labs/linux-lab"
	LabVolumeName     = "rde-linux-lab-data"
	LabRepoURL        = "https://github.com/tinyclub/linux-lab.git"
	LabUnixUser       = "ubuntu"
	LabUnixUID        = "1000"
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
	// 当前构建元数据（供刷新后查询）
	buildBoard  string
	buildTarget string
	currentJob  *BuildJob // 最近一次构建（含环形日志，供重连）
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

	// 确保容器在运行（旧容器可能仍带损坏的 ENTRYPOINT，启动失败则重建）
	if !running {
		progressChan <- ProgressEvent{Status: "running", Message: "正在启动容器..."}
		if err := s.cli.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
			s.logger.Warn("start linux-lab failed, recreating", zap.Error(err))
			progressChan <- ProgressEvent{Status: "running", Message: "启动失败，正在重建容器（覆盖镜像 ENTRYPOINT）..."}
			_ = s.RemoveContainer()
			id, err = s.createContainer(ctx)
			if err != nil {
				progressChan <- ProgressEvent{Status: "failed", Message: fmt.Sprintf("重建容器失败: %v", err)}
				return
			}
			if err := s.cli.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
				progressChan <- ProgressEvent{Status: "failed", Message: fmt.Sprintf("启动容器失败: %v", err)}
				return
			}
		}
	}

	// 3. 准备 linux-lab 源码（镜像内 /labs 为空，开发板/构建依赖仓库）
	progressChan <- ProgressEvent{Status: "running", Message: "正在检查 linux-lab 源码..."}
	if err := s.ensureLabSource(ctx, progressChan); err != nil {
		progressChan <- ProgressEvent{Status: "failed", Message: fmt.Sprintf("准备源码失败: %v", err)}
		return
	}

	// 4. Cloud Lab 环境标记（Makefile 强制检查 /home/ubuntu/Desktop/lab.desktop）
	progressChan <- ProgressEvent{Status: "running", Message: "正在配置 Cloud Lab 运行环境..."}
	if err := s.ensureCloudLabEnv(ctx); err != nil {
		progressChan <- ProgressEvent{Status: "failed", Message: fmt.Sprintf("配置运行环境失败: %v", err)}
		return
	}
	progressChan <- ProgressEvent{Status: "running", Message: "✓ Cloud Lab 环境已就绪"}

	progressChan <- ProgressEvent{Status: "completed", Message: "Linux Lab 容器已就绪"}
	s.logger.Info("Linux Lab container ready", zap.String("container", id[:12]))
}

func (s *Service) pullImage(progressChan chan<- ProgressEvent) error {
	if loaded, err := offline.Global().LoadImage(s.image); err == nil && loaded {
		progressChan <- ProgressEvent{Status: "running", Message: "已从本地离线包加载镜像"}
		return nil
	} else if err != nil {
		s.logger.Warn("offline image load failed", zap.String("image", s.image), zap.Error(err))
	}

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
	// 必须覆盖 ENTRYPOINT：镜像默认 /tools/lab/run 不存在于镜像内
	// （官方用法会挂载 cloud-lab 的 tools/），否则 OCI 创建进程直接失败。
	containerConfig := &containertypes.Config{
		Image:      s.image,
		Entrypoint: []string{"sleep"},
		Cmd:        []string{"infinity"},
		Env: []string{
			"UNIX_USER=" + LabUnixUser,
			"UNIX_UID=" + LabUnixUID,
			"WARN_ON_USER=0",
		},
		Labels: map[string]string{
			"rde.app": "linux-lab",
		},
		WorkingDir: LabDirInContainer,
	}

	hostConfig := &containertypes.HostConfig{
		// 特权模式，QEMU/KVM 需要
		Privileged: true,
		RestartPolicy: containertypes.RestartPolicy{
			Name: "unless-stopped",
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: LabVolumeName,
				Target: "/labs",
			},
		},
	}

	resp, err := s.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, ContainerName)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// ensureLabSource 确保 /labs/linux-lab 存在（浅克隆，幂等）
func (s *Service) ensureLabSource(ctx context.Context, progressChan chan<- ProgressEvent) error {
	out, err := s.execInContainerAs(ctx, `test -f /labs/linux-lab/Makefile && echo ok`, "/", "0")
	if err == nil && strings.Contains(out, "ok") {
		progressChan <- ProgressEvent{Status: "running", Message: "✓ linux-lab 源码已就绪"}
		return nil
	}

	progressChan <- ProgressEvent{Status: "running", Message: "正在克隆 linux-lab 源码（首次可能需要几分钟）..."}
	// 清理半成品目录后浅克隆（workdir=/，因目标目录尚不存在；需 root）
	cmd := fmt.Sprintf(`rm -rf /labs/linux-lab && mkdir -p /labs && git clone --depth 1 %s /labs/linux-lab`, LabRepoURL)
	if _, err := s.execInContainerAs(ctx, cmd, "/", "0"); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	out, err = s.execInContainerAs(ctx, `test -f /labs/linux-lab/Makefile && test -d /labs/linux-lab/boards && echo ok`, "/", "0")
	if err != nil || !strings.Contains(out, "ok") {
		return fmt.Errorf("clone incomplete: missing Makefile/boards")
	}
	progressChan <- ProgressEvent{Status: "running", Message: "✓ linux-lab 源码克隆完成"}
	return nil
}

// ensureCloudLabEnv 满足 linux-lab Makefile 对 Cloud Lab 的检测
// 要求存在 /home/ubuntu/Desktop/lab.desktop，并以 ubuntu 用户运行 make。
func (s *Service) ensureCloudLabEnv(ctx context.Context) error {
	script := `
set -e
id -u ` + LabUnixUser + ` &>/dev/null || useradd --create-home --shell /bin/bash --user-group --groups adm,sudo ` + LabUnixUser + `
mkdir -p /home/` + LabUnixUser + `/Desktop
touch /home/` + LabUnixUser + `/Desktop/lab.desktop
# Makefile 会 stat /.git/description 检查属主（警告项）
mkdir -p /.git
touch /.git/description
chown -R ` + LabUnixUser + `:` + LabUnixUser + ` /home/` + LabUnixUser + ` /.git
[ -d /labs/linux-lab ] && chown -R ` + LabUnixUser + `:` + LabUnixUser + ` /labs/linux-lab || true
test -f /home/` + LabUnixUser + `/Desktop/lab.desktop
`
	// useradd/chown 需 root
	_, err := s.execInContainerAs(ctx, script, "/", "0")
	return err
}

// --- 容器内执行命令 ---

// execInContainer 在容器中执行命令，返回完整输出（工作目录为 linux-lab，用户 ubuntu）
func (s *Service) execInContainer(ctx context.Context, command string) (string, error) {
	return s.execInContainerAs(ctx, command, LabDirInContainer, LabUnixUser)
}

// execInContainerAt 在容器指定工作目录执行命令（默认 ubuntu）
func (s *Service) execInContainerAt(ctx context.Context, command, workDir string) (string, error) {
	return s.execInContainerAs(ctx, command, workDir, LabUnixUser)
}

// execInContainerAs 在容器指定用户与工作目录执行命令
func (s *Service) execInContainerAs(ctx context.Context, command, workDir, user string) (string, error) {
	if s.cli == nil {
		return "", fmt.Errorf("docker client not available")
	}

	id, running := s.findContainer(ctx)
	if !running {
		return "", fmt.Errorf("容器未运行")
	}

	if workDir == "" {
		workDir = "/"
	}

	execConfig := types.ExecConfig{
		User:         user,
		Cmd:          []string{"/bin/bash", "-c", command},
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		WorkingDir:   workDir,
		Env: []string{
			"UNIX_USER=" + LabUnixUser,
			"UNIX_UID=" + LabUnixUID,
			"WARN_ON_USER=0",
			"HOME=/home/" + LabUnixUser,
		},
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
		User:         LabUnixUser,
		Cmd:          []string{"/bin/bash", "-c", command},
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true, // TTY 让 make 输出实时刷新
		WorkingDir:   LabDirInContainer,
		Env: []string{
			"UNIX_USER=" + LabUnixUser,
			"UNIX_UID=" + LabUnixUID,
			"WARN_ON_USER=0",
			"HOME=/home/" + LabUnixUser,
		},
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
		sendProgress(progressChan, ProgressEvent{
			Status: "running",
			Line:   line,
		})
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

// IsBuilding 检查是否正在构建（含页面断开后仍在容器内跑的 make）
func (s *Service) IsBuilding() bool {
	s.mu.Lock()
	flag := s.building
	job := s.currentJob
	s.mu.Unlock()
	if flag {
		return true
	}
	if job != nil && !job.IsDone() {
		return true
	}
	return s.containerHasMake()
}

// CurrentJob 返回最近一次构建任务（可能已结束，仍可用于拉历史日志）
func (s *Service) CurrentJob() *BuildJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentJob
}

// BuildInfo 构建状态详情
type BuildInfo struct {
	Building bool
	Board    string
	Target   string
	LastSeq  int64
	Status   string // running|completed|failed|""
	JobID    string
}

// GetBuildInfo 返回构建状态详情
func (s *Service) GetBuildInfo() BuildInfo {
	s.mu.Lock()
	info := BuildInfo{
		Building: s.building,
		Board:    s.buildBoard,
		Target:   s.buildTarget,
	}
	job := s.currentJob
	s.mu.Unlock()

	if job != nil {
		info.Board = job.Board
		info.Target = job.Target
		info.LastSeq = job.LastSeq()
		info.Status = job.Status()
		info.JobID = job.ID
		if !job.IsDone() {
			info.Building = true
		}
	}
	if !info.Building && s.containerHasMake() {
		info.Building = true
		if info.Status == "" {
			info.Status = "running"
		}
	}
	return info
}

// StartBuild 启动后台构建并返回任务（日志写入环形缓冲，可重连）
func (s *Service) StartBuild(target, board string) (*BuildJob, error) {
	if s.containerHasMake() {
		return nil, fmt.Errorf("已有构建任务正在运行（可能在后台），请稍候再试")
	}

	s.mu.Lock()
	if s.building || (s.currentJob != nil && !s.currentJob.IsDone()) {
		s.mu.Unlock()
		return nil, fmt.Errorf("已有构建任务正在运行（可能在后台），请稍候再试")
	}
	job := newBuildJob(board, target, defaultLogCapacity)
	s.building = true
	s.buildBoard = board
	s.buildTarget = target
	s.currentJob = job
	s.mu.Unlock()

	go s.runMake(context.Background(), job, target, board)
	return job, nil
}

// containerHasMake 检测容器内是否仍有 make 进程（页面刷新后 building 标志可能已清）
func (s *Service) containerHasMake() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := s.execInContainerAs(ctx, `pgrep -af '^make |/usr/bin/make ' 2>/dev/null | grep -v pgrep | head -1`, "/", "0")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

// sendProgress 非阻塞推送，避免页面断开后无人消费导致构建卡住
func sendProgress(ch chan<- ProgressEvent, ev ProgressEvent) {
	select {
	case ch <- ev:
	default:
	}
}

// runMake 在容器内执行 make，日志写入 job（环形缓冲）
func (s *Service) runMake(ctx context.Context, job *BuildJob, target, board string) {
	defer func() {
		s.mu.Lock()
		s.building = false
		// 保留 board/target/job，便于结束后短时间内查询
		s.mu.Unlock()
		if !job.IsDone() {
			job.AppendEvent(ProgressEvent{Status: "failed", Message: "构建异常结束"})
		}
	}()

	if !s.ContainerRunning() {
		job.AppendEvent(ProgressEvent{Status: "failed", Message: "容器未运行，请先初始化环境"})
		return
	}

	if err := s.ensureCloudLabEnv(ctx); err != nil {
		job.AppendEvent(ProgressEvent{Status: "failed", Message: fmt.Sprintf("准备构建环境失败: %v", err)})
		return
	}

	cmd := "make"
	if board != "" {
		cmd += fmt.Sprintf(" BOARD=%s", board)
	}
	cmd += " " + target

	s.logger.Info("Executing make in container",
		zap.String("target", target),
		zap.String("board", board),
		zap.String("user", LabUnixUser),
		zap.String("cmd", cmd),
		zap.String("job", job.ID),
	)

	job.AppendEvent(ProgressEvent{
		Status:  "running",
		Message: fmt.Sprintf(">>> %s  (容器内执行)", cmd),
	})

	lineChan := make(chan ProgressEvent, 256)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for ev := range lineChan {
			job.AppendEvent(ev)
		}
	}()

	err := s.execInContainerStream(ctx, cmd, lineChan)
	close(lineChan)
	wg.Wait()

	if err != nil {
		job.AppendEvent(ProgressEvent{
			Status:  "failed",
			Message: fmt.Sprintf("命令执行失败: %v", err),
		})
		return
	}

	job.AppendEvent(ProgressEvent{
		Status:  "completed",
		Message: "执行完成",
	})
}

// ExecMake 在容器内执行 make（高级模式 /make）。
// 与 StartBuild 共用互斥；日志同样写入 currentJob，可重连。
func (s *Service) ExecMake(ctx context.Context, target string, board string, progressChan chan<- ProgressEvent) error {
	defer close(progressChan)

	job, err := s.StartBuild(target, board)
	if err != nil {
		sendProgress(progressChan, ProgressEvent{Status: "failed", Message: err.Error()})
		return err
	}

	// StartBuild 已在后台跑；此处订阅并转发到 progressChan（兼容旧 SSE）
	ch, cancel := job.Subscribe(0)
	defer cancel()

	for ev := range ch {
		if ev.Done {
			return nil
		}
		pe := ProgressEvent{Status: ev.Status, Message: ev.Message, Line: ev.Line}
		sendProgress(progressChan, pe)
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
