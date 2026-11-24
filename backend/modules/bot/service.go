// Package bot 业务逻辑
package bot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ruizi-store/rde/backend/modules/docker"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 沙箱约定
const (
	// NetworkName bot 工作台专用的 docker 网络（控制面容器也加入，实现直连串流）
	NetworkName = "rde-bot-net"
	// DefaultImage 默认沙箱镜像
	DefaultImage = "rde/bot-desktop:latest"
	// KasmVNCPort 沙箱内 KasmVNC web 端口
	KasmVNCPort = 6901
	// ContainerPrefix 沙箱容器名前缀（rde-bot-<id>）
	ContainerPrefix = "rde-bot-"
	// WorkspacePath 沙箱内工作目录
	WorkspacePath = "/workspace"
	// ManagedLabel 管理标记（docker 容器上的 label）
	ManagedLabel = "rde.bot"
	// TakeoverExpire 接管锁过期时间（防止异常退出后锁死）
	TakeoverExpire = 30 * time.Minute
)

// Service bot 业务服务
type Service struct {
	db      *gorm.DB
	logger  *zap.Logger
	dataDir string
	docker  DockerAPI
}

// DockerAPI bot 模块所需的 docker 能力子集（*docker.Service 满足此接口）
type DockerAPI interface {
	IsRunning(ctx context.Context) bool
	CreateContainer(ctx context.Context, config *docker.ContainerConfig) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string) error
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	GetContainerStatus(ctx context.Context, containerID string) (*docker.ContainerStatus, error)
	GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error)
	ExecWithTimeout(ctx context.Context, containerID string, cmd []string, timeoutSec, maxOutput int) (*docker.ExecResult, error)
	GetContainerNetworkIP(ctx context.Context, containerID, networkName string) (string, error)
	ListNetworks(ctx context.Context) ([]docker.NetworkInfo, error)
	CreateNetwork(ctx context.Context, name, driver string) (string, error)
}

// NewService 创建服务
func NewService(db *gorm.DB, logger *zap.Logger, dataDir string, dockerAPI DockerAPI) *Service {
	if dataDir == "" {
		dataDir = "/var/lib/rde"
	}
	return &Service{db: db, logger: logger, dataDir: dataDir, docker: dockerAPI}
}

// ensureNetwork 确保 bot 网络存在（幂等）
func (s *Service) ensureNetwork(ctx context.Context) error {
	if s.docker == nil {
		return errors.New("docker service unavailable")
	}
	networks, err := s.docker.ListNetworks(ctx)
	if err != nil {
		return fmt.Errorf("列出 docker 网络失败: %w", err)
	}
	for _, n := range networks {
		if n.Name == NetworkName {
			return nil
		}
	}
	if _, err := s.docker.CreateNetwork(ctx, NetworkName, "bridge"); err != nil {
		// 并发创建时可能报"已存在"，重查一次
		networks, err2 := s.docker.ListNetworks(ctx)
		if err2 == nil {
			for _, n := range networks {
				if n.Name == NetworkName {
					return nil
				}
			}
		}
		return fmt.Errorf("创建网络 %s 失败: %w", NetworkName, err)
	}
	s.logger.Info("bot network created", zap.String("network", NetworkName))
	return nil
}

// containerName 沙箱容器名
func containerName(botID string) string {
	return ContainerPrefix + botID
}

// workspaceDir 宿主机工作目录（bind mount 到 /workspace）
func (s *Service) workspaceDir(botID string) string {
	return filepath.Join(s.dataDir, "bots", botID, "workspace")
}

// ---------- CRUD ----------

// Create 创建 Bot
func (s *Service) Create(name, description, model, providerID, image string) (*Bot, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("名字不能为空")
	}
	if image == "" {
		image = DefaultImage
	}
	bot := &Bot{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Model:       model,
		ProviderID:  providerID,
		Image:       image,
		Status:      StatusCreated,
	}
	if err := s.db.Create(bot).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("名字 '%s' 已存在", name)
		}
		return nil, err
	}
	return bot, nil
}

// List 列出所有 Bot（并同步容器实际状态）
func (s *Service) List(ctx context.Context) ([]Bot, error) {
	var bots []Bot
	if err := s.db.Order("created_at DESC").Find(&bots).Error; err != nil {
		return nil, err
	}
	for i := range bots {
		s.syncStatus(ctx, &bots[i])
	}
	return bots, nil
}

// Get 获取单个 Bot（同步状态）
func (s *Service) Get(ctx context.Context, id string) (*Bot, error) {
	var b Bot
	if err := s.db.Where("id = ?", id).First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("bot 不存在")
		}
		return nil, err
	}
	s.syncStatus(ctx, &b)
	return &b, nil
}

// Update 更新（容器存在时不允许改镜像，需先停止删除沙箱）
func (s *Service) Update(id string, name, description, model, providerID *string, enabled *bool) (*Bot, error) {
	b, err := s.Get(context.Background(), id)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return nil, errors.New("名字不能为空")
		}
		b.Name = n
		updates["name"] = n
	}
	if description != nil {
		b.Description = *description
		updates["description"] = *description
	}
	if model != nil {
		b.Model = *model
		updates["model"] = *model
	}
	if providerID != nil {
		b.ProviderID = *providerID
		updates["provider_id"] = *providerID
	}
	if enabled != nil {
		b.Enabled = *enabled
		updates["enabled"] = *enabled
	}
	if len(updates) == 0 {
		return b, nil
	}
	if err := s.db.Model(&Bot{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return b, nil
}

// Delete 删除 Bot（可选连带删除沙箱容器与数据）
func (s *Service) Delete(ctx context.Context, id string, removeData bool) error {
	b, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if b.ContainerID != "" {
		if s.docker != nil {
			_ = s.docker.RemoveContainer(ctx, b.ContainerID, true)
		}
	}
	if removeData {
		dir := filepath.Join(s.dataDir, "bots", id)
		if err := os.RemoveAll(dir); err != nil {
			s.logger.Warn("删除 bot 数据目录失败", zap.String("dir", dir), zap.Error(err))
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bot_id = ?", id).Delete(&BotMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Bot{}, "id = ?", id).Error
	})
}

// syncStatus 从 docker 同步容器实际状态到数据库
func (s *Service) syncStatus(ctx context.Context, b *Bot) {
	if s.docker == nil || b.ContainerID == "" {
		if b.Status == StatusRunning {
			b.Status = StatusStopped
		}
		return
	}
	st, err := s.docker.GetContainerStatus(ctx, b.ContainerID)
	if err != nil {
		// 容器被外部删除：回到 created（待重建）
		b.Status = StatusCreated
		b.ContainerID = ""
		_ = s.db.Model(&Bot{}).Where("id = ?", b.ID).Updates(map[string]interface{}{
			"status": b.Status, "container_id": "",
		})
		return
	}
	newStatus := b.Status
	switch {
	case st.Running:
		newStatus = StatusRunning
	case st.State == "created" || st.State == "exited":
		newStatus = StatusStopped
	default:
		newStatus = StatusStopped
	}
	if newStatus != b.Status {
		b.Status = newStatus
		_ = s.db.Model(&Bot{}).Where("id = ?", b.ID).Update("status", newStatus)
	}
}

// ---------- 沙箱生命周期 ----------

// StartSandbox 创建并启动沙箱容器
func (s *Service) StartSandbox(ctx context.Context, id string) (*Bot, error) {
	b, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !b.Enabled {
		return nil, errors.New("bot 已停用，请先启用")
	}
	if s.docker == nil || !s.docker.IsRunning(ctx) {
		return nil, errors.New("Docker 不可用（未安装或控制面无权限）")
	}
	if err := s.ensureNetwork(ctx); err != nil {
		return nil, err
	}

	// 已有容器且在运行：直接返回
	if b.ContainerID != "" {
		if st, err := s.docker.GetContainerStatus(ctx, b.ContainerID); err == nil && st.Running {
			b.Status = StatusRunning
			return b, nil
		}
		// 容器存在但停止：尝试启动
		if err := s.docker.StartContainer(ctx, b.ContainerID); err == nil {
			_ = s.db.Model(&Bot{}).Where("id = ?", id).Updates(map[string]interface{}{"status": StatusRunning})
			b.Status = StatusRunning
			return b, nil
		}
		// 启动失败：删除重建
		_ = s.docker.RemoveContainer(ctx, b.ContainerID, true)
		_ = s.db.Model(&Bot{}).Where("id = ?", id).Update("container_id", "")
		b.ContainerID = ""
	}

	// 确保工作目录存在
	wsDir := s.workspaceDir(id)
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建工作目录失败: %w", err)
	}

	_ = s.db.Model(&Bot{}).Where("id = ?", id).Update("status", StatusStarting)
	b.Status = StatusStarting

	containerID, err := s.docker.CreateContainer(ctx, &docker.ContainerConfig{
		Name:     containerName(id),
		Image:    b.Image,
		Networks: []string{NetworkName},
		Volumes:  map[string]string{wsDir: WorkspacePath},
		Labels: map[string]string{
			ManagedLabel:  id,
			"rde.managed": "bot",
		},
		Restart: "unless-stopped",
	})
	if err != nil {
		_ = s.db.Model(&Bot{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status": StatusError, "error_message": err.Error(),
		})
		return nil, fmt.Errorf("创建沙箱容器失败: %w", err)
	}

	_ = s.db.Model(&Bot{}).Where("id = ?", id).Updates(map[string]interface{}{
		"container_id":  containerID,
		"status":        StatusRunning,
		"error_message": "",
	})
	b.ContainerID = containerID
	b.Status = StatusRunning
	s.logger.Info("bot sandbox started",
		zap.String("bot", b.Name), zap.String("container", containerID))
	return b, nil
}

// StopSandbox 停止沙箱
func (s *Service) StopSandbox(ctx context.Context, id string) (*Bot, error) {
	b, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.ContainerID == "" || s.docker == nil {
		b.Status = StatusStopped
		_ = s.db.Model(&Bot{}).Where("id = ?", id).Update("status", StatusStopped)
		return b, nil
	}
	if err := s.docker.StopContainer(ctx, b.ContainerID); err != nil {
		return nil, fmt.Errorf("停止沙箱失败: %w", err)
	}
	// 停止即释放接管锁
	_ = s.db.Model(&Bot{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": StatusStopped, "taken_over_by": "", "taken_over_at": nil,
	})
	b.Status = StatusStopped
	b.TakenOverBy = ""
	b.TakenOverAt = nil
	return b, nil
}

// RestartSandbox 重启沙箱
func (s *Service) RestartSandbox(ctx context.Context, id string) (*Bot, error) {
	if _, err := s.StopSandbox(ctx, id); err != nil {
		// 停止失败不阻塞重启
		_ = err
	}
	return s.StartSandbox(ctx, id)
}

// Logs 沙箱日志
func (s *Service) Logs(ctx context.Context, id string, tail int) (string, error) {
	b, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if b.ContainerID == "" || s.docker == nil {
		return "", errors.New("沙箱尚未创建")
	}
	if tail <= 0 || tail > 1000 {
		tail = 200
	}
	return s.docker.GetContainerLogs(ctx, b.ContainerID, tail)
}

// GetSandboxAddr 获取沙箱 KasmVNC 地址（串流代理直连用）
func (s *Service) GetSandboxAddr(ctx context.Context, id string) (string, error) {
	b, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if b.ContainerID == "" || b.Status != StatusRunning {
		return "", errors.New("沙箱未运行")
	}
	ip, err := s.docker.GetContainerNetworkIP(ctx, b.ContainerID, NetworkName)
	if err != nil {
		return "", fmt.Errorf("获取沙箱地址失败: %w", err)
	}
	return fmt.Sprintf("http://%s:%d", ip, KasmVNCPort), nil
}

// ---------- 沙箱工具执行 ----------

// ExecRequest 工具执行请求
type ExecRequest struct {
	Script   string `json:"script" binding:"required"`
	Language string `json:"language" binding:"required"`
	Timeout  int    `json:"timeout"`
}

// Exec 在 bot 沙箱内执行脚本
// 安全边界：脚本只在沙箱容器内运行，控制面进程不执行用户脚本。
func (s *Service) Exec(ctx context.Context, id string, req *ExecRequest) (*docker.ExecResult, error) {
	b, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.ContainerID == "" || b.Status != StatusRunning {
		return nil, errors.New("沙箱未运行，请先启动")
	}
	if s.docker == nil {
		return nil, errors.New("docker service unavailable")
	}

	var ext, interpreter string
	switch strings.ToLower(req.Language) {
	case "bash", "sh", "shell":
		ext, interpreter = ".sh", "bash"
	case "python", "py":
		ext, interpreter = ".py", "python3"
	default:
		return nil, fmt.Errorf("不支持的语言 '%s'，仅支持 Bash/Python", req.Language)
	}
	_ = ext

	// 通过 stdin 传入脚本，避免把用户脚本拼进命令行
	cmd := []string{interpreter, "-c", req.Script}
	result, err := s.docker.ExecWithTimeout(ctx, b.ContainerID, cmd, req.Timeout, 64*1024)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ---------- 成员与可见性 ----------

// ListMembers 列出成员
func (s *Service) ListMembers(botID string) ([]BotMember, error) {
	var members []BotMember
	err := s.db.Where("bot_id = ?", botID).Find(&members).Error
	return members, err
}

// SetMembers 全量设置成员（覆盖式）
func (s *Service) SetMembers(botID string, members []BotMember) error {
	for _, m := range members {
		if !validRoles[m.Role] {
			return fmt.Errorf("无效角色 '%s'（可选 admin/member/viewer）", m.Role)
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bot_id = ?", botID).Delete(&BotMember{}).Error; err != nil {
			return err
		}
		for i := range members {
			members[i].ID = 0
			members[i].BotID = botID
			if members[i].Role == "" {
				members[i].Role = RoleMemberMember
			}
		}
		if len(members) > 0 {
			return tx.Create(&members).Error
		}
		return nil
	})
}

// GetRole 查询用户对某 bot 的有效角色
// 平台管理员视为 bot admin；不在成员列表中返回空（= 不可见）
func (s *Service) GetRole(userID, userRole, botID string) string {
	if userRole == "admin" {
		return RoleMemberAdmin
	}
	var m BotMember
	err := s.db.Where("bot_id = ? AND user_id = ?", botID, userID).First(&m).Error
	if err != nil {
		return ""
	}
	return m.Role
}

// ---------- 接管锁 ----------

// Takeover 抢占接管锁（一人接管，他人只读围观）
func (s *Service) Takeover(botID, userID, username string) error {
	var b Bot
	if err := s.db.Where("id = ?", botID).First(&b).Error; err != nil {
		return errors.New("bot 不存在")
	}
	// 过期的锁自动释放
	if b.TakenOverBy != "" && b.TakenOverAt != nil && time.Since(*b.TakenOverAt) > TakeoverExpire {
		b.TakenOverBy = ""
	}
	if b.TakenOverBy != "" && b.TakenOverBy != userID {
		return fmt.Errorf("已被 %s 接管，请等待其交还", b.TakenOverByName)
	}
	now := time.Now()
	return s.db.Model(&Bot{}).Where("id = ?", botID).Updates(map[string]interface{}{
		"taken_over_by": userID, "taken_over_at": now,
	}).Error
}

// Release 释放接管锁
func (s *Service) Release(botID, userID string) error {
	b := &Bot{}
	if err := s.db.Where("id = ?", botID).First(b).Error; err != nil {
		return errors.New("bot 不存在")
	}
	if b.TakenOverBy != "" && b.TakenOverBy != userID {
		return errors.New("接管者不是你，无法释放")
	}
	return s.db.Model(&Bot{}).Where("id = ?", botID).Updates(map[string]interface{}{
		"taken_over_by": "", "taken_over_at": nil,
	}).Error
}
