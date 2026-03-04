// Package ai 初始化向导服务
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SetupService 初始化向导服务
type SetupService struct {
	logger  *zap.Logger
	dataDir string
	state   *SetupState
	coreAPI *CoreAPI
	mu      sync.RWMutex
}

// NewSetupService 创建向导服务
func NewSetupService(logger *zap.Logger, dataDir string, coreAPI *CoreAPI) *SetupService {
	s := &SetupService{
		logger:  logger,
		dataDir: dataDir,
		coreAPI: coreAPI,
		state: &SetupState{
			Status:        SetupPending,
			CurrentStep:   StepEnvironment,
			StartedAt:     time.Now(),
			SkillsEnabled: []string{},
		},
	}
	s.loadState()
	return s
}

func (s *SetupService) loadState() {
	stateFile := filepath.Join(s.dataDir, "setup_state.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, s.state); err != nil {
		s.logger.Warn("failed to parse setup state", zap.Error(err))
	}
}

func (s *SetupService) saveState() {
	stateFile := filepath.Join(s.dataDir, "setup_state.json")
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		s.logger.Error("failed to marshal setup state", zap.Error(err))
		return
	}
	tmpFile := stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		s.logger.Error("failed to write setup state", zap.Error(err))
		return
	}
	os.Rename(tmpFile, stateFile)
}

// GetState 获取状态
func (s *SetupService) GetState() *SetupState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stateCopy := *s.state
	return &stateCopy
}

// GetStep 获取当前步骤
func (s *SetupService) GetStep() SetupStep {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.CurrentStep
}

// SetStep 设置当前步骤
func (s *SetupService) SetStep(step SetupStep) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.CurrentStep = step
	s.saveState()
}

// CheckEnvironment 检查环境
func (s *SetupService) CheckEnvironment() *EnvironmentCheck {
	check := &EnvironmentCheck{}

	// Docker 检查
	check.Docker = s.checkDocker()

	// 磁盘空间
	check.DiskSpace = s.checkDiskSpace()

	// 网络
	check.Network = s.checkNetwork()

	// GPU
	check.GPU = s.checkGPU()

	// 系统信息
	check.OS = runtime.GOOS
	check.Arch = runtime.GOARCH

	return check
}

func (s *SetupService) checkDocker() ComponentStatus {
	output, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return ComponentStatus{
			Name: "Docker", Status: "not_installed",
			Message: "Docker 未安装或未运行",
		}
	}
	version := strings.TrimSpace(string(output))

	// 检查 docker compose
	composeVersion := ""
	if out, err := exec.Command("docker", "compose", "version", "--short").Output(); err == nil {
		composeVersion = strings.TrimSpace(string(out))
	}

	return ComponentStatus{
		Name:    "Docker",
		Status:  "ready",
		Version: version,
		Message: fmt.Sprintf("Docker %s, Compose %s", version, composeVersion),
	}
}

func (s *SetupService) checkDiskSpace() DiskSpaceStatus {
	output, err := exec.Command("df", "-BG", "--output=avail,size", s.dataDir).Output()
	if err != nil {
		return DiskSpaceStatus{Available: "unknown", Required: "20GB", Sufficient: false}
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return DiskSpaceStatus{Available: "unknown", Required: "20GB", Sufficient: false}
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return DiskSpaceStatus{Available: "unknown", Required: "20GB", Sufficient: false}
	}
	return DiskSpaceStatus{
		Available:  fields[0],
		Total:      fields[1],
		Required:   "20GB",
		Sufficient: true,
	}
}

func (s *SetupService) checkNetwork() NetworkStatus {
	status := NetworkStatus{Internet: false}

	// 检查互联网
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
		"--connect-timeout", "3", "https://registry-1.docker.io/v2/").Output(); err == nil {
		status.Internet = strings.TrimSpace(string(output)) != "000"
	}
	status.DockerHub = status.Internet

	// Ollama
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	if output, err := exec.CommandContext(ctx2, "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
		"--connect-timeout", "2", "http://localhost:11434/api/version").Output(); err == nil {
		status.OllamaReachable = strings.TrimSpace(string(output)) == "200"
	}

	return status
}

func (s *SetupService) checkGPU() ComponentStatus {
	// NVIDIA
	if output, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total",
		"--format=csv,noheader,nounits").Output(); err == nil {
		return ComponentStatus{
			Name: "GPU", Status: "ready",
			Message: "NVIDIA: " + strings.TrimSpace(string(output)),
		}
	}

	// 检查 /dev/dri
	if entries, err := os.ReadDir("/dev/dri"); err == nil && len(entries) > 0 {
		return ComponentStatus{
			Name: "GPU", Status: "ready",
			Message: fmt.Sprintf("检测到 %d 个 DRI 设备", len(entries)),
		}
	}

	return ComponentStatus{
		Name: "GPU", Status: "not_available",
		Message: "未检测到 GPU，将使用 CPU 推理",
	}
}

// GetAvailableModels 获取可选模型
func (s *SetupService) GetAvailableModels() []ModelInfo {
	return []ModelInfo{
		{ID: "qwen2.5:3b", Name: "Qwen 2.5 3B", Size: "2.0GB", Description: "通义千问 3B - 轻量快速", Category: "recommended", MinRAM: "4GB"},
		{ID: "qwen2.5:7b", Name: "Qwen 2.5 7B", Size: "4.4GB", Description: "通义千问 7B - 均衡之选", Category: "recommended", MinRAM: "8GB"},
		{ID: "llama3.2:3b", Name: "Llama 3.2 3B", Size: "2.0GB", Description: "Meta Llama 3.2 3B", Category: "general", MinRAM: "4GB"},
		{ID: "llama3.1:8b", Name: "Llama 3.1 8B", Size: "4.7GB", Description: "Meta Llama 3.1 8B", Category: "general", MinRAM: "8GB"},
		{ID: "deepseek-r1:8b", Name: "DeepSeek R1 8B", Size: "4.9GB", Description: "DeepSeek R1 推理模型", Category: "reasoning", MinRAM: "8GB"},
		{ID: "mistral:7b", Name: "Mistral 7B", Size: "4.1GB", Description: "Mistral AI 7B", Category: "general", MinRAM: "8GB"},
		{ID: "phi3:3.8b", Name: "Phi-3 3.8B", Size: "2.3GB", Description: "Microsoft Phi-3 Mini", Category: "lightweight", MinRAM: "4GB"},
		{ID: "nomic-embed-text", Name: "Nomic Embed", Size: "274MB", Description: "文本嵌入模型", Category: "embedding", MinRAM: "2GB"},
	}
}

// GetDefaultSkills 获取默认技能列表
func (s *SetupService) GetDefaultSkills() []Skill {
	return []Skill{
		{ID: "storage_analyzer", Name: "存储分析", Description: "分析磁盘使用、查找大文件", Enabled: true, Category: "storage"},
		{ID: "file_manager", Name: "文件管理", Description: "搜索和管理文件", Enabled: true, Category: "storage"},
		{ID: "system_diagnosis", Name: "系统诊断", Description: "系统信息、硬件状态、网络诊断", Enabled: true, Category: "system"},
		{ID: "docker_manager", Name: "Docker 应用", Description: "查看 Docker 容器和镜像状态", Enabled: true, Category: "service"},
		{ID: "share_manager", Name: "共享管理", Description: "查看 SMB/NFS 共享状态", Enabled: true, Category: "service"},
		{ID: "scheduled_tasks", Name: "定时任务", Description: "查看系统定时任务", Enabled: true, Category: "automation"},
		{ID: "script_executor", Name: "脚本执行", Description: "执行自定义脚本（需确认）", Enabled: false, Category: "automation"},
	}
}

// DownloadModel 下载模型（使用 Ollama）
func (s *SetupService) DownloadModel(ctx context.Context, modelName string, progressCh chan<- DownloadProgress) error {
	s.logger.Info("Starting model download", zap.String("model", modelName))

	progressCh <- DownloadProgress{
		Model:  modelName,
		Status: "downloading",
	}

	cmd := exec.CommandContext(ctx, "docker", "exec", "rde-ollama", "ollama", "pull", modelName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		progressCh <- DownloadProgress{
			Model:  modelName,
			Status: "error",
			Error:  fmt.Sprintf("下载失败: %s\n%s", err, string(output)),
		}
		return err
	}

	progressCh <- DownloadProgress{
		Model:      modelName,
		Status:     "complete",
		Percentage: 100,
	}
	return nil
}

// CancelDownload 取消下载
func (s *SetupService) CancelDownload() {
	s.logger.Info("Cancelling model download")
}

// Complete 完成向导
func (s *SetupService) Complete(req SetupCompleteRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Status = SetupComplete
	s.state.CompletedAt = timePtr(time.Now())
	s.state.SelectedModel = req.SelectedModel
	s.state.SkillsEnabled = req.SkillsEnabled

	s.saveState()
	s.logger.Info("Setup completed",
		zap.String("model", req.SelectedModel),
		zap.Strings("skills", req.SkillsEnabled))
	return nil
}

// Reset 重置向导
func (s *SetupService) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = &SetupState{
		Status:        SetupPending,
		CurrentStep:   StepEnvironment,
		StartedAt:     time.Now(),
		SkillsEnabled: []string{},
	}
	s.saveState()
}

// IsCompleted 是否完成
func (s *SetupService) IsCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Status == SetupComplete
}

// SaveSkills 保存用户选择的技能列表
func (s *SetupService) SaveSkills(enabledSkills []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.SkillsEnabled = enabledSkills
	s.saveState()
	return nil
}

// isImageInstalled 检查 Docker 镜像是否已拉取
func (s *SetupService) isImageInstalled(image string) bool {
	output, err := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}", image).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

// checkDockerService 检查 Docker daemon 是否运行
func (s *SetupService) checkDockerService() bool {
	err := exec.Command("docker", "info").Run()
	return err == nil
}

// parseDockerError 将 Docker 错误转为用户友好消息
func (s *SetupService) parseDockerError(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "permission denied") {
		return "没有 Docker 权限，请将用户添加到 docker 组"
	}
	if strings.Contains(msg, "Cannot connect") || strings.Contains(msg, "connection refused") {
		return "Docker 服务未运行，请启动 Docker"
	}
	if strings.Contains(msg, "not found") {
		return "Docker 未安装"
	}
	return msg
}

// checkDockerCompose 检查 Docker Compose 是否安装
func (s *SetupService) checkDockerCompose() ComponentStatus {
	// 先尝试 docker compose (v2)
	if output, err := exec.Command("docker", "compose", "version").Output(); err == nil {
		re := regexp.MustCompile(`v?([0-9.]+)`)
		if matches := re.FindStringSubmatch(string(output)); len(matches) > 1 {
			return ComponentStatus{Name: "Docker Compose", Status: "ready", Version: matches[1]}
		}
		return ComponentStatus{Name: "Docker Compose", Status: "ready", Version: "v2"}
	}

	// 尝试 docker-compose (v1)
	if output, err := exec.Command("docker-compose", "--version").Output(); err == nil {
		re := regexp.MustCompile(`([0-9.]+)`)
		if matches := re.FindStringSubmatch(string(output)); len(matches) > 1 {
			return ComponentStatus{Name: "Docker Compose", Status: "ready", Version: matches[1]}
		}
		return ComponentStatus{Name: "Docker Compose", Status: "ready", Version: "v1"}
	}

	return ComponentStatus{Name: "Docker Compose", Status: "not_installed", Message: "Docker Compose 未安装"}
}

// parseDockerEvent 解析 Docker 事件为下载进度
func (s *SetupService) parseDockerEvent(event map[string]interface{}, layerProgress, layerTotal map[string]int64) *DownloadProgress {
	status, _ := event["status"].(string)
	id, _ := event["id"].(string)

	progress := &DownloadProgress{Status: "downloading"}

	if id != "" {
		progress.Log = fmt.Sprintf("%s: %s", id, status)
	} else {
		progress.Log = status
	}

	// 解析进度详情
	if detail, ok := event["progressDetail"].(map[string]interface{}); ok {
		if current, ok := detail["current"].(float64); ok && id != "" {
			layerProgress[id] = int64(current)
		}
		if total, ok := detail["total"].(float64); ok && id != "" {
			layerTotal[id] = int64(total)
		}
	}

	// 计算总进度
	var totalCurrent, totalSize int64
	for layerID, current := range layerProgress {
		totalCurrent += current
		if total, ok := layerTotal[layerID]; ok {
			totalSize += total
		}
	}

	if totalSize > 0 {
		progress.Percentage = float64(totalCurrent) / float64(totalSize) * 100
	}

	return progress
}

// parseSize 解析大小字符串 (如 "50MB", "1.2GB")
func parseSize(s string) int64 {
	s = strings.ToUpper(strings.TrimSpace(s))

	var multiplier int64 = 1
	if strings.HasSuffix(s, "KB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	} else if strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	} else if strings.HasSuffix(s, "TB") {
		multiplier = 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "TB")
	} else if strings.HasSuffix(s, "B") {
		s = strings.TrimSuffix(s, "B")
	}

	val, _ := strconv.ParseFloat(s, 64)
	return int64(val * float64(multiplier))
}

func timePtr(t time.Time) *time.Time { return &t }
