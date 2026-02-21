package users

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("user already exists")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrGroupNotFound     = errors.New("group not found")
	ErrGroupExists       = errors.New("group already exists")
	ErrCannotDeleteAdmin = errors.New("cannot delete admin user")
)

// Service 用户服务
type Service struct {
	db           *gorm.DB
	logger       *zap.Logger
	eventBus     module.EventBus
	activeUsers  map[string]time.Time // user_id -> last active time
	activeMu     sync.RWMutex
	avatarsDir   string // 头像存储目录
}

// NewService 创建用户服务
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{
		db:          db,
		logger:      logger,
		activeUsers: make(map[string]time.Time),
	}
}

// SetAvatarsDir 设置头像存储目录
func (s *Service) SetAvatarsDir(dir string) {
	s.avatarsDir = dir
	os.MkdirAll(dir, 0755)
}

// RecordActivity 记录用户活跃（由 auth 中间件调用）
func (s *Service) RecordActivity(userID string) {
	s.activeMu.Lock()
	s.activeUsers[userID] = time.Now()
	s.activeMu.Unlock()
}

// IsOnline 检查用户是否在线（5 分钟内有活动）
func (s *Service) IsOnline(userID string) bool {
	s.activeMu.RLock()
	lastActive, ok := s.activeUsers[userID]
	s.activeMu.RUnlock()
	if !ok {
		return false
	}
	return time.Since(lastActive) < 5*time.Minute
}

// SetEventBus 设置事件总线
func (s *Service) SetEventBus(eventBus module.EventBus) {
	s.eventBus = eventBus
}

// publishEvent 发布事件
func (s *Service) publishEvent(eventType string, data interface{}) {
	if s.eventBus != nil {
		s.eventBus.Publish(eventType, data)
	}
}

// EnsureAdminExists 检查是否存在管理员账户
// 注意：不再自动创建默认 admin 账户，管理员应通过 Setup 流程创建
func (s *Service) EnsureAdminExists() error {
	var count int64
	s.db.Model(&User{}).Where("role = ?", RoleAdmin).Count(&count)
	if count == 0 {
		s.logger.Warn("No admin account found. Please complete the setup process to create an admin account.")
	}
	return nil
}

// ----- 用户操作 -----

// CreateUser 创建用户
func (s *Service) CreateUser(req *CreateUserRequest) error {
	// 检查用户名是否存在
	var exists int64
	s.db.Model(&User{}).Where("username = ?", req.Username).Count(&exists)
	if exists > 0 {
		return ErrUserExists
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &User{
		ID:       uuid.New().String(),
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Nickname: req.Nickname,
		Role:     req.Role,
		GroupID:  req.GroupID,
		Status:   StatusActive,
	}

	if user.Role == "" {
		user.Role = RoleAdmin
	}
	if user.Nickname == "" {
		user.Nickname = user.Username
	}

	if err := s.db.Create(user).Error; err != nil {
		return err
	}

	// 创建 Linux 系统用户和标准 home 目录
	if err := s.createSystemUser(req.Username, req.Password); err != nil {
		s.logger.Warn("Failed to create system user", zap.Error(err))
		// 不阻止流程，系统用户可以后续手动创建
	}

	return nil
}

// createSystemUser 创建 Linux 系统用户并初始化 home 目录
func (s *Service) createSystemUser(username, password string) error {
	// 检查用户是否已存在
	checkCmd := exec.Command("id", username)
	if err := checkCmd.Run(); err == nil {
		s.logger.Info("System user already exists, ensuring home directory and shell", zap.String("username", username))

		// 确保 home 目录存在
		homeDir := filepath.Join("/home", username)
		if err := os.MkdirAll(homeDir, 0755); err != nil {
			s.logger.Warn("Failed to create home directory", zap.Error(err))
		}

		// 修正 shell 和 home 目录配置（可能之前被 Samba 等创建为 nologin）
		usermodCmd := exec.Command("usermod", "-d", homeDir, "-s", "/bin/bash", username)
		if output, err := usermodCmd.CombinedOutput(); err != nil {
			s.logger.Warn("Failed to fix user shell/home",
				zap.Error(err),
				zap.String("output", string(output)))
		}

		// chown home 目录
		chownCmd := exec.Command("chown", "-R", fmt.Sprintf("%s:%s", username, username), homeDir)
		if output, err := chownCmd.CombinedOutput(); err != nil {
			s.logger.Warn("Failed to chown home directory",
				zap.Error(err),
				zap.String("output", string(output)))
		}

		// 确保标准子目录存在
		s.createUserHomeDirectories(username)

		return nil
	}

	// 创建用户（带 home 目录）
	createCmd := exec.Command("useradd", "-m", "-s", "/bin/bash", username)
	if output, err := createCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create user: %s", string(output))
	}

	// 设置密码
	passwdCmd := exec.Command("chpasswd")
	passwdCmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", username, password))
	if output, err := passwdCmd.CombinedOutput(); err != nil {
		s.logger.Warn("Failed to set system user password",
			zap.Error(err),
			zap.String("output", string(output)))
	}

	// 加入 sudo 组
	sudoCmd := exec.Command("usermod", "-aG", "sudo", username)
	if output, err := sudoCmd.CombinedOutput(); err != nil {
		s.logger.Warn("Failed to add user to sudo group",
			zap.Error(err),
			zap.String("output", string(output)))
	}

	// 创建标准用户目录
	s.createUserHomeDirectories(username)

	s.logger.Info("System user created successfully", zap.String("username", username))
	return nil
}

// createUserHomeDirectories 在用户 home 目录下创建标准子目录
func (s *Service) createUserHomeDirectories(username string) {
	homeDir := filepath.Join("/home", username)

	standardDirs := []string{
		"Desktop",
		"Downloads",
		"Documents",
		"Music",
		"Pictures",
		"Videos",
	}

	for _, dir := range standardDirs {
		dirPath := filepath.Join(homeDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			s.logger.Warn("Failed to create directory",
				zap.String("path", dirPath),
				zap.Error(err))
			continue
		}

		chownCmd := exec.Command("chown", fmt.Sprintf("%s:%s", username, username), dirPath)
		if output, err := chownCmd.CombinedOutput(); err != nil {
			s.logger.Warn("Failed to chown directory",
				zap.String("path", dirPath),
				zap.Error(err),
				zap.String("output", string(output)))
		}
	}

	s.logger.Info("User home directories created", zap.String("username", username))
}

// GetUserByID 根据 ID 获取用户
func (s *Service) GetUserByID(id string) (*User, error) {
	var user User
	err := s.db.Preload("Group").First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

// GetByID 根据 ID 获取用户（GetUserByID 的别名）
func (s *Service) GetByID(id string) (*User, error) {
	return s.GetUserByID(id)
}

// GetUserByUsername 根据用户名获取用户
func (s *Service) GetUserByUsername(username string) (*User, error) {
	var user User
	err := s.db.Preload("Group").First(&user, "username = ?", username).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

// UpdateLastLogin 更新用户最后登录时间
func (s *Service) UpdateLastLogin(userID string) error {
	now := time.Now()
	return s.db.Model(&User{}).Where("id = ?", userID).Update("last_login", &now).Error
}

// ListUsers 获取用户列表
func (s *Service) ListUsers(page, pageSize int) ([]User, int64, error) {
	var users []User
	var total int64

	query := s.db.Model(&User{})
	query.Count(&total)

	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err := query.Preload("Group").Order("created_at DESC").Find(&users).Error
	return users, total, err
}

// UpdateUser 更新用户
func (s *Service) UpdateUser(id string, req *UpdateUserRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
	}
	if req.Role != "" {
		updates["role"] = req.Role
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.GroupID != "" {
		updates["group_id"] = req.GroupID
	}
	if req.Settings != "" {
		updates["settings"] = req.Settings
	}

	return s.db.Model(user).Updates(updates).Error
}

// SaveAvatar 保存用户头像文件并更新数据库
func (s *Service) SaveAvatar(userID string, file *multipart.FileHeader, c *gin.Context) (string, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return "", err
	}

	// 删除旧头像文件（如果存在）
	if user.Avatar != "" {
		oldFile := filepath.Join(s.avatarsDir, filepath.Base(user.Avatar))
		os.Remove(oldFile)
	}

	// 生成文件名: {userId}{ext}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	filename := userID + ext
	savePath := filepath.Join(s.avatarsDir, filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		return "", fmt.Errorf("save avatar file: %w", err)
	}

	// 更新数据库中的头像 URL
	avatarURL := "/avatars/" + filename
	if err := s.db.Model(user).Update("avatar", avatarURL).Error; err != nil {
		os.Remove(savePath) // 回滚文件
		return "", err
	}

	return avatarURL, nil
}

// DeleteUser 删除用户
func (s *Service) DeleteUser(id string) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 不允许删除最后一个用户
	var userCount int64
	s.db.Model(&User{}).Count(&userCount)
	if userCount <= 1 {
		return ErrCannotDeleteAdmin
	}

	if err := s.db.Delete(user).Error; err != nil {
		return err
	}

	// 发布用户删除事件
	s.publishEvent("users.deleted", map[string]string{
		"user_id":  user.ID,
		"username": user.Username,
	})

	return nil
}

// ChangePassword 修改密码
func (s *Service) ChangePassword(id, oldPassword, newPassword string) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return ErrInvalidPassword
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := s.db.Model(user).Update("password", string(hashedPassword)).Error; err != nil {
		return err
	}

	// 发布密码修改事件
	s.publishEvent("users.password.changed", map[string]string{
		"user_id":  user.ID,
		"username": user.Username,
	})

	return nil
}

// ResetPassword 管理员重置密码（无需旧密码）
func (s *Service) ResetPassword(id, newPassword string) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := s.db.Model(user).Update("password", string(hashedPassword)).Error; err != nil {
		return err
	}

	s.publishEvent("users.password.reset", map[string]string{
		"user_id":  user.ID,
		"username": user.Username,
	})

	return nil
}

// ValidatePassword 验证密码
func (s *Service) ValidatePassword(username, password string) (*User, error) {
	user, err := s.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}

	if user.Status != StatusActive {
		return nil, errors.New("user is disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	// 更新最后登录时间
	now := time.Now()
	s.db.Model(user).Update("last_login", now)
	user.LastLogin = &now

	return user, nil
}

// ----- 两步验证（2FA） -----

// getUserSettings 从 User.Settings JSON 读取 2FA 设置
func (s *Service) getUserSettings(userID string) (*TwoFactorSettings, error) {
	var settingsJSON string
	if err := s.db.Model(&User{}).Where("id = ?", userID).Pluck("settings", &settingsJSON).Error; err != nil {
		return nil, err
	}
	if settingsJSON == "" {
		return &TwoFactorSettings{}, nil
	}
	var settings TwoFactorSettings
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return &TwoFactorSettings{}, nil // 无法解析当作未启用
	}
	return &settings, nil
}

// saveUserSettings 保存 2FA 设置到 User.Settings JSON
func (s *Service) saveUserSettings(userID string, settings *TwoFactorSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return s.db.Model(&User{}).Where("id = ?", userID).Update("settings", string(data)).Error
}

// IsTotpEnabled 检查用户是否启用了 2FA
func (s *Service) IsTotpEnabled(userID string) bool {
	settings, err := s.getUserSettings(userID)
	if err != nil {
		return false
	}
	return settings.TotpEnabled && settings.TotpSecret != ""
}

// Setup2FA 设置 2FA（生成密钥和恢复码，尚未启用）
func (s *Service) Setup2FA(userID string) (*TwoFactorSetupResponse, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	// 生成 TOTP 密钥
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "RDE",
		AccountName: user.Username,
	})
	if err != nil {
		return nil, fmt.Errorf("generate TOTP key: %w", err)
	}

	// 生成 8 个备用恢复码
	backupCodes := make([]string, 8)
	for i := 0; i < 8; i++ {
		code := make([]byte, 5)
		rand.Read(code)
		backupCodes[i] = strings.ToUpper(base32.StdEncoding.EncodeToString(code)[:8])
	}

	// 保存（totp_enabled = false，需要验证后才启用）
	settings := &TwoFactorSettings{
		TotpSecret:  key.Secret(),
		TotpEnabled: false,
		BackupCodes: backupCodes,
	}
	if err := s.saveUserSettings(userID, settings); err != nil {
		return nil, err
	}

	return &TwoFactorSetupResponse{
		Secret:      key.Secret(),
		QRCodeURL:   key.URL(),
		BackupCodes: backupCodes,
	}, nil
}

// Enable2FA 验证 TOTP 码并启用 2FA
func (s *Service) Enable2FA(userID, code string) error {
	settings, err := s.getUserSettings(userID)
	if err != nil {
		return err
	}
	if settings.TotpSecret == "" {
		return errors.New("2FA not configured, call setup first")
	}

	if !totp.Validate(code, settings.TotpSecret) {
		return errors.New("invalid verification code")
	}

	settings.TotpEnabled = true
	return s.saveUserSettings(userID, settings)
}

// Disable2FA 关闭 2FA
func (s *Service) Disable2FA(userID string) error {
	settings := &TwoFactorSettings{} // 清空所有 2FA 数据
	return s.saveUserSettings(userID, settings)
}

// Validate2FA 验证 TOTP 码或备用恢复码
func (s *Service) Validate2FA(userID, code string) (bool, error) {
	settings, err := s.getUserSettings(userID)
	if err != nil {
		return false, err
	}
	if !settings.TotpEnabled || settings.TotpSecret == "" {
		return false, errors.New("2FA not enabled")
	}

	// 先尝试 TOTP
	if totp.Validate(code, settings.TotpSecret) {
		return true, nil
	}

	// 再尝试恢复码（一次性）
	normalizedCode := strings.ToUpper(strings.TrimSpace(code))
	for i, bc := range settings.BackupCodes {
		if bc == normalizedCode {
			// 消费掉这个恢复码
			settings.BackupCodes = append(settings.BackupCodes[:i], settings.BackupCodes[i+1:]...)
			s.saveUserSettings(userID, settings)
			return true, nil
		}
	}

	return false, nil
}

// ----- 用户组操作 -----

// CreateGroup 创建用户组
func (s *Service) CreateGroup(req *CreateGroupRequest) (*UserGroup, error) {
	// 检查是否存在
	var exists int64
	s.db.Model(&UserGroup{}).Where("name = ?", req.Name).Count(&exists)
	if exists > 0 {
		return nil, ErrGroupExists
	}

	group := &UserGroup{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Permissions: req.Permissions,
	}

	if err := s.db.Create(group).Error; err != nil {
		return nil, err
	}
	return group, nil
}

// GetGroupByID 根据 ID 获取用户组
func (s *Service) GetGroupByID(id string) (*UserGroup, error) {
	var group UserGroup
	err := s.db.First(&group, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGroupNotFound
	}
	return &group, err
}

// ListGroups 获取用户组列表
func (s *Service) ListGroups() ([]UserGroup, error) {
	var groups []UserGroup
	err := s.db.Preload("Users").Order("created_at DESC").Find(&groups).Error
	return groups, err
}

// UpdateGroup 更新用户组
func (s *Service) UpdateGroup(id string, req *UpdateGroupRequest) error {
	group, err := s.GetGroupByID(id)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Permissions != "" {
		updates["permissions"] = req.Permissions
	}

	return s.db.Model(group).Updates(updates).Error
}

// DeleteGroup 删除用户组
func (s *Service) DeleteGroup(id string) error {
	// 先将该组下的用户移出
	s.db.Model(&User{}).Where("group_id = ?", id).Update("group_id", nil)
	return s.db.Delete(&UserGroup{}, "id = ?", id).Error
}

// ----- 设备信任管理 -----

const (
	TrustedDeviceDuration = 30 * 24 * time.Hour // 30 天
)

// AddTrustedDevice 添加受信任设备
func (s *Service) AddTrustedDevice(userID, userAgent, ip string) (string, error) {
	settings, err := s.getUserSettings(userID)
	if err != nil {
		settings = &TwoFactorSettings{}
	}

	// 生成设备令牌
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	deviceToken := base64.RawURLEncoding.EncodeToString(tokenBytes)

	// 生成设备 ID
	idBytes := make([]byte, 8)
	rand.Read(idBytes)
	deviceID := hex.EncodeToString(idBytes)

	// 清理过期设备
	now := time.Now()
	validDevices := make([]TrustedDevice, 0)
	for _, d := range settings.TrustedDevices {
		if d.ExpiresAt.After(now) {
			validDevices = append(validDevices, d)
		}
	}

	// 添加新设备
	device := TrustedDevice{
		ID:        deviceID,
		Token:     deviceToken,
		UserAgent: userAgent,
		IP:        ip,
		CreatedAt: now,
		ExpiresAt: now.Add(TrustedDeviceDuration),
	}
	validDevices = append(validDevices, device)

	// 限制最多 10 个受信任设备
	if len(validDevices) > 10 {
		validDevices = validDevices[len(validDevices)-10:]
	}

	settings.TrustedDevices = validDevices
	if err := s.saveUserSettings(userID, settings); err != nil {
		return "", err
	}

	return deviceToken, nil
}

// IsTrustedDevice 检查是否为受信任设备
func (s *Service) IsTrustedDevice(userID, deviceToken string) bool {
	if deviceToken == "" {
		return false
	}

	settings, err := s.getUserSettings(userID)
	if err != nil {
		return false
	}

	now := time.Now()
	for _, d := range settings.TrustedDevices {
		if d.Token == deviceToken && d.ExpiresAt.After(now) {
			return true
		}
	}
	return false
}

// RemoveTrustedDevice 移除受信任设备
func (s *Service) RemoveTrustedDevice(userID, deviceID string) error {
	settings, err := s.getUserSettings(userID)
	if err != nil {
		return err
	}

	validDevices := make([]TrustedDevice, 0)
	for _, d := range settings.TrustedDevices {
		if d.ID != deviceID {
			validDevices = append(validDevices, d)
		}
	}

	settings.TrustedDevices = validDevices
	return s.saveUserSettings(userID, settings)
}

// ListTrustedDevices 列出用户的受信任设备
func (s *Service) ListTrustedDevices(userID string) ([]TrustedDevice, error) {
	settings, err := s.getUserSettings(userID)
	if err != nil {
		return nil, err
	}

	// 过滤掉过期的设备
	now := time.Now()
	validDevices := make([]TrustedDevice, 0)
	for _, d := range settings.TrustedDevices {
		if d.ExpiresAt.After(now) {
			// 隐藏令牌
			d.Token = ""
			validDevices = append(validDevices, d)
		}
	}

	return validDevices, nil
}
