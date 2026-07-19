package translate

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ruizi-store/rde/backend/pkg/offline"
	"go.uber.org/zap"
)

// 与 fulliso rde-offline-bootstrap 保持一致，避免双容器抢 5000 端口
const (
	DefaultImage         = "libretranslate/libretranslate:latest"
	DefaultContainerName = "rde-libretranslate"
	LegacyContainerName  = "libretranslate"
	DefaultHostPort      = "5000"
	DefaultLoadOnly      = "en,zh"
	DefaultVolumeName    = "rde-libretranslate-data"
)

// knownContainerNames 优先使用标准名，其次兼容旧安装
var knownContainerNames = []string{DefaultContainerName, LegacyContainerName}

func dockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

func dockerImageExists(image string) bool {
	return exec.Command("docker", "image", "inspect", image).Run() == nil
}

func dockerContainerExists(name string) bool {
	out, err := exec.Command("docker", "ps", "-aq", "-f", "name=^/"+name+"$").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func dockerContainerRunning(name string) bool {
	out, err := exec.Command("docker", "ps", "-q", "-f", "name=^/"+name+"$").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func findExistingContainer() (name string, running bool) {
	// 优先复用已在运行的容器，避免 Created 的标准名抢占端口导致 ensure 失败
	for _, n := range knownContainerNames {
		if dockerContainerRunning(n) {
			return n, true
		}
	}
	for _, n := range knownContainerNames {
		if dockerContainerExists(n) {
			return n, false
		}
	}
	return "", false
}

func hostPortOpen(port string) bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (s *Service) httpReady() bool {
	return s.probeHTTP().Available
}

func (s *Service) probeHTTP() *ServiceStatus {
	s.mu.RLock()
	serviceURL := s.serviceURL
	s.mu.RUnlock()

	status := &ServiceStatus{
		Available: false,
		URL:       serviceURL,
		Phase:     PhaseMissing,
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(serviceURL + "/languages")
	if err != nil {
		status.Message = fmt.Sprintf("无法连接到翻译服务: %v", err)
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		status.Message = fmt.Sprintf("翻译服务返回错误: %d", resp.StatusCode)
		return status
	}

	status.Available = true
	status.Phase = PhaseReady
	status.Message = "翻译服务运行正常"
	return status
}

// enrichStatus 在 HTTP 探测之外补充 Docker/离线包信息
func (s *Service) enrichStatus(status *ServiceStatus) *ServiceStatus {
	if status == nil {
		status = &ServiceStatus{}
	}
	status.Image = DefaultImage
	status.Container = DefaultContainerName
	status.ImageReady = dockerAvailable() && dockerImageExists(DefaultImage)
	status.OfflineReady = offline.Global().HasImage(DefaultImage) ||
		offline.Global().HasModels(offline.LibreTranslateModelsKey)

	name, running := findExistingContainer()
	if name != "" {
		status.Container = name
		status.ContainerRunning = running
	}

	if status.Available {
		status.Phase = PhaseReady
		if status.Message == "" {
			status.Message = "翻译服务运行正常"
		}
		return status
	}

	if name != "" {
		// 容器在跑或可启动：首启加载模型时常出现 HTTP 未就绪
		status.Phase = PhaseStarting
		if running {
			status.Message = "翻译服务启动中，正在加载语言模型，请稍候…"
		} else {
			status.Message = "翻译容器已安装但未运行，可一键启动"
		}
		return status
	}

	if status.ImageReady || status.OfflineReady {
		status.Phase = PhaseReadyToStart
		status.Message = "已检测到本地/离线翻译镜像，可启动服务（无需联网）"
		return status
	}

	status.Phase = PhaseMissing
	if status.Message == "" {
		status.Message = "未找到翻译服务，需要安装或拉取镜像"
	}
	return status
}

// EnsureService 确保 LibreTranslate 可用：复用已有容器 → 离线 load → 必要时 pull → 等待就绪
func (s *Service) EnsureService(wait time.Duration) *EnsureResult {
	// wait==0：只拉起容器，不等待 HTTP 就绪（由前端轮询 /status）
	if wait < 0 {
		wait = 20 * time.Second
	}

	result := &EnsureResult{Phase: PhaseStarting}

	if s.httpReady() {
		result.Status = "success"
		result.Available = true
		result.Phase = PhaseReady
		result.Message = "翻译服务已就绪"
		return result
	}

	if !dockerAvailable() {
		result.Status = "failed"
		result.Phase = PhaseError
		result.Message = "Docker 未安装或不在 PATH 中"
		return result
	}

	// 镜像 + 离线 en/zh 模型（避免 Booting 阶段外网下载卡住）
	if err := s.ensureImage(result); err != nil {
		result.Status = "failed"
		result.Phase = PhaseError
		result.Message = err.Error()
		return result
	}
	if err := s.seedOfflineModels(); err != nil {
		s.logger.Warn("seed libretranslate models failed", zap.Error(err))
	}

	// 双容器清理：只保留一个可用实例
	s.reconcileContainers()

	// 1) 复用已有容器（含旧名 libretranslate）
	if name, running := findExistingContainer(); name != "" {
		s.logger.Info("reusing libretranslate container", zap.String("name", name), zap.Bool("running", running))
		if !running {
			if out, err := exec.Command("docker", "start", name).CombinedOutput(); err != nil {
				result.Status = "failed"
				result.Phase = PhaseError
				result.Message = fmt.Sprintf("启动已有容器失败: %v (%s)", err, strings.TrimSpace(string(out)))
				return result
			}
		}
		if s.waitUntilReady(wait, result) {
			return result
		}
		// 长时间 Booting（模型下载卡住）时：再灌一次离线模型并重启
		_ = s.seedOfflineModels()
		s.logger.Warn("libretranslate still not ready, restarting container", zap.String("name", name))
		_ = exec.Command("docker", "restart", name).Run()
		if s.waitUntilReady(wait, result) {
			return result
		}
		result.Status = "starting"
		result.Phase = PhaseStarting
		result.Message = "容器已启动，语言模型仍在加载，请稍候…"
		return result
	}

	// 2) 端口被非本服务占用
	if hostPortOpen(DefaultHostPort) && !s.httpReady() {
		result.Status = "failed"
		result.Phase = PhaseError
		result.Message = fmt.Sprintf("端口 %s 已被占用且不是可用的翻译服务，请释放端口后重试", DefaultHostPort)
		return result
	}

	// 3) 创建标准容器（与 ISO bootstrap 同名）
	_ = exec.Command("docker", "volume", "create", DefaultVolumeName).Run()
	_ = s.seedOfflineModels()
	args := []string{
		"run", "-d",
		"--name", DefaultContainerName,
		"--restart=always",
		"-p", DefaultHostPort + ":5000",
		"-v", DefaultVolumeName + ":/home/libretranslate/.local",
		"-e", "LT_LOAD_ONLY=" + DefaultLoadOnly,
		DefaultImage,
	}
	s.logger.Info("creating libretranslate container", zap.Strings("args", args))
	if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
		result.Status = "failed"
		result.Phase = PhaseError
		result.Message = fmt.Sprintf("创建翻译容器失败: %v (%s)", err, strings.TrimSpace(string(out)))
		return result
	}

	if s.waitUntilReady(wait, result) {
		return result
	}
	result.Status = "starting"
	result.Phase = PhaseStarting
	result.Message = "容器已创建，语言模型仍在加载，请稍候…"
	return result
}

func (s *Service) ensureImage(result *EnsureResult) error {
	if dockerImageExists(DefaultImage) {
		result.Message = "已使用本地 Docker 镜像"
		return nil
	}
	if loaded, err := offline.Global().LoadImage(DefaultImage); err != nil {
		s.logger.Warn("offline load libretranslate failed", zap.Error(err))
	} else if loaded {
		result.Message = "已从离线包加载镜像"
		if dockerImageExists(DefaultImage) {
			return nil
		}
	}
	s.logger.Info("pulling libretranslate image", zap.String("image", DefaultImage))
	out, err := exec.Command("docker", "pull", DefaultImage).CombinedOutput()
	if err != nil {
		return fmt.Errorf("镜像准备失败（本地 / 离线包 / 网络均不可用）: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	result.Message = "已从网络拉取镜像"
	return nil
}

// reconcileContainers 避免 rde-libretranslate(Created) 与 libretranslate(Running) 抢端口
func (s *Service) reconcileContainers() {
	stdExists := dockerContainerExists(DefaultContainerName)
	stdRun := dockerContainerRunning(DefaultContainerName)
	legExists := dockerContainerExists(LegacyContainerName)
	legRun := dockerContainerRunning(LegacyContainerName)

	switch {
	case stdRun && legExists:
		s.logger.Info("removing conflicting legacy libretranslate container")
		_ = exec.Command("docker", "rm", "-f", LegacyContainerName).Run()
	case legRun && stdExists && !stdRun:
		s.logger.Info("removing unused standard container that blocks port reuse")
		_ = exec.Command("docker", "rm", "-f", DefaultContainerName).Run()
	case !stdRun && !legRun && stdExists && legExists:
		// 都未运行：保留标准名，删旧名
		_ = exec.Command("docker", "rm", "-f", LegacyContainerName).Run()
	}
}

// seedOfflineModels 将离线 Argos en/zh 包解压进 docker volume（幂等）
func (s *Service) seedOfflineModels() error {
	tarPath, ok := offline.Global().FindModelTar(offline.LibreTranslateModelsKey)
	if !ok {
		return nil
	}
	_ = exec.Command("docker", "volume", "create", DefaultVolumeName).Run()

	mpOut, err := exec.Command("docker", "volume", "inspect", "-f", "{{.Mountpoint}}", DefaultVolumeName).Output()
	if err != nil {
		return fmt.Errorf("inspect volume: %w", err)
	}
	mountpoint := strings.TrimSpace(string(mpOut))
	if mountpoint == "" {
		return fmt.Errorf("empty volume mountpoint")
	}

	pkgMarker := filepath.Join(mountpoint, "share", "argos-translate", "packages", "translate-en_zh-1_9", "metadata.json")
	already := false
	if st, err := os.Stat(pkgMarker); err == nil && !st.IsDir() {
		already = true
	}

	if !already {
		s.logger.Info("seeding libretranslate offline models", zap.String("tar", tarPath), zap.String("volume", DefaultVolumeName))
		cmd := exec.Command("tar", "xf", tarPath, "-C", mountpoint)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("extract models: %w (%s)", err, strings.TrimSpace(string(out)))
		}
	}

	// 镜像内用户 libretranslate=1032:nogroup；root 解压会导致无法创建 cache 而 Restarting
	_ = os.MkdirAll(filepath.Join(mountpoint, "cache"), 0o755)
	if out, err := exec.Command("chown", "-R", "1032:65534", mountpoint).CombinedOutput(); err != nil {
		s.logger.Warn("chown libretranslate volume failed",
			zap.Error(err), zap.String("output", strings.TrimSpace(string(out))))
	}
	return nil
}

func (s *Service) waitUntilReady(wait time.Duration, result *EnsureResult) bool {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if s.httpReady() {
			result.Status = "success"
			result.Available = true
			result.Phase = PhaseReady
			result.Message = "翻译服务已就绪"
			return true
		}
		result.Status = "starting"
		result.Phase = PhaseStarting
		result.Message = "翻译服务启动中，正在加载语言模型…"
		time.Sleep(2 * time.Second)
	}
	return false
}
