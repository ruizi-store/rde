package setup

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/ruizi-store/rde/backend/core/auth"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrSetupAlreadyCompleted = errors.New("setup already completed")
	ErrSetupNotStarted       = errors.New("setup not started")
	ErrInvalidStep           = errors.New("invalid step")
	ErrStepNotCompleted      = errors.New("previous step not completed")
	ErrUserExists            = errors.New("user already exists")
	ErrReservedUsername      = errors.New("reserved username")
	ErrInvalidPassword       = errors.New("invalid password")
	ErrInvalidConfirmText    = errors.New("invalid confirm text, must be 'RESET'")
)

// 保留用户名列表
var reservedUsernames = []string{"admin", "root", "system", "administrator", "guest"}

// Service 初始化向导服务
type Service struct {
	db           *gorm.DB
	logger       *zap.Logger
	checker      *Checker
	dataDir      string
	config       ConfigProvider
	tokenManager *auth.TokenManager // 由 bootstrap 注入的共享 TokenManager
}

// ConfigProvider 配置提供者接口
type ConfigProvider interface {
	GetString(key string) string
}

// NewService 创建服务实例
func NewService(db *gorm.DB, logger *zap.Logger, dataDir string, config ConfigProvider) *Service {
	return &Service{
		db:      db,
		logger:  logger,
		checker: NewChecker(logger),
		dataDir: dataDir,
		config:  config,
	}
}

// SetTokenManager 设置由 bootstrap 注入的共享 TokenManager
func (s *Service) SetTokenManager(tm *auth.TokenManager) {
	s.tokenManager = tm
}

// ----- 状态管理 -----

// GetStatus 获取初始化状态
func (s *Service) GetStatus() (*SetupStatus, error) {
	settings, err := s.getOrCreateSettings()
	if err != nil {
		return nil, err
	}

	status := &SetupStatus{
		Completed:      settings.SetupCompleted,
		CurrentStep:    settings.CurrentStep,
		CompletedSteps: s.parseCompletedSteps(settings.CompletedSteps),
		CanSkipSetup:   false,
	}

	return status, nil
}

// IsCompleted 检查初始化是否完成
func (s *Service) IsCompleted() bool {
	settings, err := s.getOrCreateSettings()
	if err != nil {
		return false
	}
	return settings.SetupCompleted
}

// NeedsSetup 检查是否需要初始化
func (s *Service) NeedsSetup() bool {
	return !s.IsCompleted()
}

// getOrCreateSettings 获取或创建设置
func (s *Service) getOrCreateSettings() (*SetupSettings, error) {
	var settings SetupSettings
	err := s.db.First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建默认设置
		now := time.Now()
		settings = SetupSettings{
			SetupCompleted: false,
			CurrentStep:    1,
			CompletedSteps: "[]",
			Language:       "zh-CN",
			Timezone:       "Asia/Shanghai",
			DataPath:       s.dataDir,
			NetworkMode:    "dhcp",
			HTTPPort:       80,
			HTTPSPort:      443,
			StartedAt:      &now,
		}
		if err := s.db.Create(&settings).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return &settings, nil
}

// updateSettings 更新设置
func (s *Service) updateSettings(updates map[string]interface{}) error {
	// 先获取实际记录（id 可能不为 1），再更新
	settings, err := s.getOrCreateSettings()
	if err != nil {
		return err
	}
	return s.db.Model(settings).Updates(updates).Error
}

// markStepCompleted 标记步骤完成
func (s *Service) markStepCompleted(step int) error {
	settings, err := s.getOrCreateSettings()
	if err != nil {
		return err
	}

	completedSteps := s.parseCompletedSteps(settings.CompletedSteps)

	// 检查是否已完成
	for _, cs := range completedSteps {
		if cs == step {
			return nil
		}
	}

	completedSteps = append(completedSteps, step)
	completedJSON, _ := json.Marshal(completedSteps)

	// 更新当前步骤 (简化为 3 步)
	nextStep := step + 1
	if nextStep > 3 {
		nextStep = 3
	}

	return s.updateSettings(map[string]interface{}{
		"completed_steps": string(completedJSON),
		"current_step":    nextStep,
	})
}

// parseCompletedSteps 解析已完成步骤
func (s *Service) parseCompletedSteps(data string) []int {
	var steps []int
	if data == "" {
		return steps
	}
	json.Unmarshal([]byte(data), &steps)
	return steps
}

// ----- Step 1: 系统检查 -----

// CheckSystem 执行系统检查
func (s *Service) CheckSystem() (*SystemCheckResult, error) {
	return s.checker.CheckSystem()
}

// InstallDependencies 安装缺失的依赖
func (s *Service) InstallDependencies(packages []string, progressChan chan<- map[string]interface{}) error {
	defer close(progressChan)

	// 白名单：只允许安装特定的包
	allowedPackages := map[string]bool{
		"docker.io":  true,
		"docker-ce":  true,
		"xpra":       true,
		"xpra-html5": true,
		"pulseaudio": true,
		"ffmpeg":     true,
	}

	for _, pkg := range packages {
		if !allowedPackages[pkg] {
			s.logger.Warn("Package not in whitelist", zap.String("package", pkg))
			continue
		}

		progressChan <- map[string]interface{}{
			"package":  pkg,
			"status":   "installing",
			"progress": 0,
		}

		// 执行 apt install
		cmd := exec.Command("apt-get", "install", "-y", pkg)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

		if err := cmd.Run(); err != nil {
			s.logger.Error("Failed to install package",
				zap.String("package", pkg),
				zap.Error(err),
			)
			progressChan <- map[string]interface{}{
				"package":  pkg,
				"status":   "failed",
				"progress": 100,
				"error":    err.Error(),
			}
			continue
		}

		progressChan <- map[string]interface{}{
			"package":  pkg,
			"status":   "completed",
			"progress": 100,
		}
	}

	return nil
}

// CompleteStep1 完成系统检查步骤
func (s *Service) CompleteStep1() error {
	result, err := s.CheckSystem()
	if err != nil {
		return err
	}

	if !result.AllPassed {
		return errors.New("system check not passed")
	}

	return s.markStepCompleted(1)
}

// ----- Step 1: 语言/时区 -----

// SetLocale 设置语言和时区
func (s *Service) SetLocale(req *LocaleSettings) error {
	// 验证语言
	validLanguages := map[string]bool{
		"zh-CN": true,
		"en-US": true,
	}
	if !validLanguages[req.Language] {
		req.Language = "zh-CN"
	}

	// 设置系统时区
	if err := s.setSystemTimezone(req.Timezone); err != nil {
		s.logger.Warn("Failed to set system timezone", zap.Error(err))
		// 不阻止继续
	}

	// 保存设置
	if err := s.updateSettings(map[string]interface{}{
		"language":    req.Language,
		"timezone":    req.Timezone,
		"time_format": req.TimeFormat,
		"date_format": req.DateFormat,
	}); err != nil {
		return err
	}

	return s.markStepCompleted(1)
}

// setSystemTimezone 设置系统时区
func (s *Service) setSystemTimezone(timezone string) error {
	// 检查时区是否有效
	tzFile := filepath.Join("/usr/share/zoneinfo", timezone)
	if _, err := os.Stat(tzFile); os.IsNotExist(err) {
		return fmt.Errorf("invalid timezone: %s", timezone)
	}

	// 使用 timedatectl 设置时区
	cmd := exec.Command("timedatectl", "set-timezone", timezone)
	return cmd.Run()
}

// ----- Step 2: 创建管理员 -----

// CreateAdmin 创建管理员用户
func (s *Service) CreateAdmin(req *SetupUserRequest) (*TwoFactorSetup, error) {
	// 检查是否已有管理员
	var count int64
	s.db.Table("users_accounts").Where("role = ?", "admin").Count(&count)
	if count > 0 {
		// 用户已存在时，确保 step2 被标记完成（修复中断导致的状态不一致）
		s.markStepCompleted(2)
		return nil, ErrUserExists
	}

	// 检查保留用户名
	for _, reserved := range reservedUsernames {
		if strings.EqualFold(req.Username, reserved) {
			return nil, ErrReservedUsername
		}
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 创建用户
	userID := uuid.New().String()
	user := map[string]interface{}{
		"id":         userID,
		"username":   req.Username,
		"password":   string(hashedPassword),
		"nickname":   req.Username,
		"role":       "admin",
		"status":     "active",
		"avatar":     req.Avatar,
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}

	if err := s.db.Table("users_accounts").Create(user).Error; err != nil {
		return nil, err
	}

	// 同时创建 Linux 系统用户（用于 Samba 等服务）
	if err := s.createSystemUser(req.Username, req.Password); err != nil {
		s.logger.Warn("Failed to create system user", zap.Error(err))
		// 不阻止流程，系统用户可以后续手动创建
	}

	var twoFactorSetup *TwoFactorSetup

	// 如果启用 2FA
	if req.Enable2FA {
		twoFactorSetup, err = s.setup2FA(userID, req.Username)
		if err != nil {
			s.logger.Error("Failed to setup 2FA", zap.Error(err))
			// 不阻止用户创建
		}
	}

	// 不再锁定 root 账户和修改 SSH 配置，保持 SSH 原样
	// if err := s.lockRootAccount(req.Username); err != nil {
	// 	s.logger.Warn("Failed to lock root account", zap.Error(err))
	// }

	if err := s.markStepCompleted(2); err != nil {
		return twoFactorSetup, err
	}

	return twoFactorSetup, nil
}

// ConfigureSecurity 配置安全选项（Setup 中调用）
// 可选将端口从默认 3080 切换为随机端口
func (s *Service) ConfigureSecurity(req *SecurityConfig) (*SecurityConfigResponse, error) {
	resp := &SecurityConfigResponse{}

	if req.CustomPort > 0 {
		// 使用用户指定的自定义端口
		if req.CustomPort < 1024 || req.CustomPort > 65535 {
			return nil, fmt.Errorf("port must be between 1024 and 65535")
		}

		configPath := "/etc/rde/rde.conf"
		if err := s.updateConfigPort(configPath, req.CustomPort); err != nil {
			s.logger.Error("Failed to update port in config", zap.Error(err))
			return nil, fmt.Errorf("failed to update port: %w", err)
		}

		resp.PortChanged = true
		resp.NewPort = req.CustomPort
		resp.Message = fmt.Sprintf("端口已更改为 %d，服务将在完成设置后重启", req.CustomPort)

		s.logger.Info("Port changed via custom port", zap.Int("new_port", req.CustomPort))
	} else if req.UseRandomPort {
		// 生成随机端口 (10000-65535)
		b := make([]byte, 2)
		rand.Read(b)
		newPort := int(b[0])<<8 + int(b[1])
		newPort = newPort%55535 + 10000

		// 更新配置文件
		configPath := "/etc/rde/rde.conf"
		if err := s.updateConfigPort(configPath, newPort); err != nil {
			s.logger.Error("Failed to update port in config", zap.Error(err))
			return nil, fmt.Errorf("failed to update port: %w", err)
		}

		resp.PortChanged = true
		resp.NewPort = newPort
		resp.Message = fmt.Sprintf("端口已更改为 %d，服务将在完成设置后重启", newPort)

		s.logger.Info("Port changed via security config", zap.Int("new_port", newPort))
	} else {
		resp.Message = "保持默认端口 3080"
	}

	return resp, nil
}

// updateConfigPort 更新配置文件中的端口
func (s *Service) updateConfigPort(configPath string, port int) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "HttpPort") {
			lines[i] = fmt.Sprintf("HttpPort = %d", port)
			break
		}
	}

	return os.WriteFile(configPath, []byte(strings.Join(lines, "\n")), 0644)
}

// createSystemUser 创建 Linux 系统用户
// 用于 Samba 等需要系统用户的服务
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
		if err := s.createUserHomeDirectories(username); err != nil {
			s.logger.Warn("Failed to create user home directories", zap.Error(err))
		}

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

	// 添加到 sudo 组（管理员权限）
	sudoCmd := exec.Command("usermod", "-aG", "sudo", username)
	if output, err := sudoCmd.CombinedOutput(); err != nil {
		s.logger.Warn("Failed to add user to sudo group",
			zap.Error(err),
			zap.String("output", string(output)))
	}

	s.logger.Info("System user created successfully", zap.String("username", username))

	// 创建标准用户目录（Downloads, Documents 等）
	if err := s.createUserHomeDirectories(username); err != nil {
		s.logger.Warn("Failed to create user home directories", zap.Error(err))
	}

	return nil
}

// createUserHomeDirectories 在用户 home 目录下创建标准子目录
func (s *Service) createUserHomeDirectories(username string) error {
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

		// 设置正确的所有者（useradd -m 创建的 home 目录属于该用户）
		chownCmd := exec.Command("chown", fmt.Sprintf("%s:%s", username, username), dirPath)
		if output, err := chownCmd.CombinedOutput(); err != nil {
			s.logger.Warn("Failed to chown directory",
				zap.String("path", dirPath),
				zap.Error(err),
				zap.String("output", string(output)))
		}
	}

	s.logger.Info("User home directories created", zap.String("username", username))
	return nil
}

// lockRootAccount 锁定 root 账户并关闭 SSH
// 在创建管理员后调用，用于安全加固
// 1. 修改 sshd_config 禁止 root 登录，只允许指定用户
// 2. 禁止 root 通过 TTY 登录
// 3. 关闭 SSH 服务
func (s *Service) lockRootAccount(username string) error {
	s.logger.Info("Starting security hardening", zap.String("allowedUser", username))

	// 1. 配置 SSH：禁止 root 登录，只允许指定用户
	if err := s.configureSSHSecurity(username); err != nil {
		s.logger.Warn("Failed to configure SSH security", zap.Error(err))
		// 继续执行其他安全措施
	}

	// 2. 禁止 root 通过 TTY 登录
	if err := s.disableRootTTY(); err != nil {
		s.logger.Warn("Failed to disable root TTY login", zap.Error(err))
		// 继续执行其他安全措施
	}

	// 3. 关闭 SSH 服务
	if err := s.stopSSHService(); err != nil {
		s.logger.Warn("Failed to stop SSH service", zap.Error(err))
	}

	s.logger.Info("Security hardening completed", zap.String("allowedUser", username))
	return nil
}

// configureSSHSecurity 配置 SSH 安全设置
func (s *Service) configureSSHSecurity(allowedUser string) error {
	sshdConfig := "/etc/ssh/sshd_config"

	// 读取当前配置
	content, err := os.ReadFile(sshdConfig)
	if err != nil {
		return fmt.Errorf("failed to read sshd_config: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	newLines := make([]string, 0, len(lines)+10)

	// 要设置的选项
	settings := map[string]string{
		"PermitRootLogin":        "no",
		"AllowUsers":             allowedUser,
		"PasswordAuthentication": "yes",
		"PubkeyAuthentication":   "yes",
	}

	foundSettings := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		modified := false

		for key, value := range settings {
			// 匹配配置项（包括被注释掉的）
			if strings.HasPrefix(trimmed, key) || strings.HasPrefix(trimmed, "#"+key) {
				newLines = append(newLines, fmt.Sprintf("%s %s", key, value))
				foundSettings[key] = true
				modified = true
				break
			}
		}

		if !modified {
			newLines = append(newLines, line)
		}
	}

	// 添加未找到的设置
	for key, value := range settings {
		if !foundSettings[key] {
			newLines = append(newLines, fmt.Sprintf("%s %s", key, value))
		}
	}

	// 写回配置文件
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(sshdConfig, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write sshd_config: %w", err)
	}

	s.logger.Info("SSH security configured",
		zap.String("PermitRootLogin", "no"),
		zap.String("AllowUsers", allowedUser))

	return nil
}

// disableRootTTY 禁止 root 通过 TTY 登录
func (s *Service) disableRootTTY() error {
	securettyPath := "/etc/securetty"

	// 创建空的 securetty 文件（禁止所有 TTY root 登录）
	// 或者也可以通过 PAM 配置
	if err := os.WriteFile(securettyPath, []byte("# Root login disabled by RDE\n"), 0600); err != nil {
		// 如果写入失败，尝试通过 PAM 配置
		return s.disableRootViaPAM()
	}

	s.logger.Info("Root TTY login disabled via securetty")
	return nil
}

// disableRootViaPAM 通过 PAM 配置禁止 root 登录
func (s *Service) disableRootViaPAM() error {
	pamLoginPath := "/etc/pam.d/login"

	content, err := os.ReadFile(pamLoginPath)
	if err != nil {
		return fmt.Errorf("failed to read PAM login config: %w", err)
	}

	// 检查是否已添加 pam_succeed_if 规则
	pamRule := "auth required pam_succeed_if.so user != root"
	if strings.Contains(string(content), pamRule) {
		return nil // 已配置
	}

	// 在文件开头添加规则
	newContent := pamRule + "\n" + string(content)
	if err := os.WriteFile(pamLoginPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write PAM login config: %w", err)
	}

	s.logger.Info("Root login disabled via PAM")
	return nil
}

// stopSSHService 停止 SSH 服务
func (s *Service) stopSSHService() error {
	// 先停止服务
	stopCmd := exec.Command("systemctl", "stop", "ssh")
	if output, err := stopCmd.CombinedOutput(); err != nil {
		// 尝试 sshd 服务名
		stopCmd = exec.Command("systemctl", "stop", "sshd")
		if output2, err2 := stopCmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("failed to stop SSH: %s / %s", string(output), string(output2))
		}
	}

	// 禁用服务自启动
	disableCmd := exec.Command("systemctl", "disable", "ssh")
	disableCmd.Run() // 忽略错误

	disableCmd = exec.Command("systemctl", "disable", "sshd")
	disableCmd.Run() // 忽略错误

	s.logger.Info("SSH service stopped and disabled")
	return nil
}

// setup2FA 设置双因素认证
func (s *Service) setup2FA(userID, username string) (*TwoFactorSetup, error) {
	// 生成 TOTP 密钥
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "RDE",
		AccountName: username,
	})
	if err != nil {
		return nil, err
	}

	// 生成备用恢复码
	backupCodes := make([]string, 8)
	for i := 0; i < 8; i++ {
		code := make([]byte, 5)
		rand.Read(code)
		backupCodes[i] = strings.ToUpper(base32.StdEncoding.EncodeToString(code)[:8])
	}

	// 存储 2FA 设置
	backupCodesJSON, _ := json.Marshal(backupCodes)
	twoFAData := map[string]interface{}{
		"totp_secret":  key.Secret(),
		"totp_enabled": false, // 需要验证后才启用
		"backup_codes": string(backupCodesJSON),
	}

	// 更新用户设置（假设 settings 字段存储 JSON）
	settingsJSON, _ := json.Marshal(twoFAData)
	s.db.Table("users_accounts").Where("id = ?", userID).Update("settings", string(settingsJSON))

	return &TwoFactorSetup{
		Secret:      key.Secret(),
		QRCodeURL:   key.URL(),
		BackupCodes: backupCodes,
	}, nil
}

// Verify2FA 验证 2FA 码并启用
func (s *Service) Verify2FA(userID, code string) error {
	var settings string
	if err := s.db.Table("users_accounts").Where("id = ?", userID).Pluck("settings", &settings).Error; err != nil {
		return err
	}

	var twoFAData map[string]interface{}
	if err := json.Unmarshal([]byte(settings), &twoFAData); err != nil {
		return err
	}

	secret, ok := twoFAData["totp_secret"].(string)
	if !ok {
		return errors.New("2FA not configured")
	}

	// 验证 TOTP 码
	valid := totp.Validate(code, secret)
	if !valid {
		return errors.New("invalid 2FA code")
	}

	// 启用 2FA
	twoFAData["totp_enabled"] = true
	settingsJSON, _ := json.Marshal(twoFAData)
	return s.db.Table("users_accounts").Where("id = ?", userID).Update("settings", string(settingsJSON)).Error
}

// ----- Step 4: 存储配置 -----

// GetDrives 获取检测到的硬盘
func (s *Service) GetDrives() ([]DetectedDrive, error) {
	return s.checker.DetectDrives()
}

// ConfigureStorage 配置存储
func (s *Service) ConfigureStorage(req *StorageConfig) error {
	// 验证数据路径
	if req.DataPath == "" {
		req.DataPath = s.dataDir
	}

	// 创建数据目录结构
	dirs := []string{
		"files",
		"database",
		"backups",
		"thumbnails",
		"temp",
	}

	for _, dir := range dirs {
		path := filepath.Join(req.DataPath, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	// 处理外部硬盘挂载
	for _, drive := range req.ExternalDrives {
		if err := s.mountDrive(drive); err != nil {
			s.logger.Error("Failed to mount drive",
				zap.String("device", drive.DevicePath),
				zap.Error(err),
			)
			// 不阻止继续
		}
	}

	// 保存设置
	if err := s.updateSettings(map[string]interface{}{
		"data_path": req.DataPath,
	}); err != nil {
		return err
	}

	return s.markStepCompleted(4)
}

// SkipStorageConfig 跳过存储配置，使用默认路径
func (s *Service) SkipStorageConfig() error {
	// 设置默认数据路径
	defaultPath := "/var/lib/rde"

	// 确保目录存在
	if err := os.MkdirAll(defaultPath, 0755); err != nil {
		s.logger.Warn("Failed to create default data path", zap.Error(err))
	}

	// 保存默认设置
	if err := s.updateSettings(map[string]interface{}{
		"data_path": defaultPath,
	}); err != nil {
		return err
	}

	return s.markStepCompleted(4)
}

// GetAvailableDisks 获取可用于创建存储池的硬盘
func (s *Service) GetAvailableDisks() ([]AvailableDisk, error) {
	output, err := exec.Command("lsblk", "-J", "-b", "-o",
		"NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,MODEL,SERIAL,TRAN,ROTA").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list block devices: %w", err)
	}

	var result struct {
		Blockdevices []struct {
			Name       string `json:"name"`
			Size       uint64 `json:"size"`
			Type       string `json:"type"`
			Mountpoint string `json:"mountpoint"`
			Fstype     string `json:"fstype"`
			Model      string `json:"model"`
			Serial     string `json:"serial"`
			Tran       string `json:"tran"`
			Rota       bool   `json:"rota"`
			Children   []struct {
				Name       string `json:"name"`
				Size       uint64 `json:"size"`
				Type       string `json:"type"`
				Mountpoint string `json:"mountpoint"`
				Fstype     string `json:"fstype"`
			} `json:"children"`
		} `json:"blockdevices"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse lsblk output: %w", err)
	}

	var disks []AvailableDisk
	for _, dev := range result.Blockdevices {
		if dev.Type != "disk" {
			continue
		}

		// 检查是否为系统盘
		isSystem := s.isSystemDisk(&dev)

		// 检查是否已被使用（有文件系统或已挂载）
		inUse := dev.Fstype != "" || dev.Mountpoint != ""
		if !inUse {
			for _, child := range dev.Children {
				if child.Mountpoint != "" || child.Fstype != "" {
					inUse = true
					break
				}
			}
		}

		// 确定磁盘类型
		diskType := "hdd"
		if !dev.Rota {
			if dev.Tran == "nvme" {
				diskType = "nvme"
			} else {
				diskType = "ssd"
			}
		}

		disks = append(disks, AvailableDisk{
			Path:      "/dev/" + dev.Name,
			Name:      dev.Name,
			Model:     dev.Model,
			Serial:    dev.Serial,
			Size:      dev.Size,
			Type:      diskType,
			Transport: dev.Tran,
			IsSystem:  isSystem,
			InUse:     inUse,
		})
	}

	return disks, nil
}

// isSystemDisk 检查是否为系统盘
func (s *Service) isSystemDisk(dev *struct {
	Name       string `json:"name"`
	Size       uint64 `json:"size"`
	Type       string `json:"type"`
	Mountpoint string `json:"mountpoint"`
	Fstype     string `json:"fstype"`
	Model      string `json:"model"`
	Serial     string `json:"serial"`
	Tran       string `json:"tran"`
	Rota       bool   `json:"rota"`
	Children   []struct {
		Name       string `json:"name"`
		Size       uint64 `json:"size"`
		Type       string `json:"type"`
		Mountpoint string `json:"mountpoint"`
		Fstype     string `json:"fstype"`
	} `json:"children"`
}) bool {
	// 检查是否有系统分区
	systemMounts := []string{"/", "/boot", "/boot/efi", "/var"}

	if dev.Mountpoint != "" {
		for _, mount := range systemMounts {
			if dev.Mountpoint == mount {
				return true
			}
		}
	}

	for _, child := range dev.Children {
		for _, mount := range systemMounts {
			if child.Mountpoint == mount {
				return true
			}
		}
	}

	return false
}

// mountDrive 挂载硬盘
func (s *Service) mountDrive(drive DriveMount) error {
	// 创建挂载点
	if err := os.MkdirAll(drive.MountPoint, 0755); err != nil {
		return err
	}

	// 执行挂载
	cmd := exec.Command("mount", drive.DevicePath, drive.MountPoint)
	if err := cmd.Run(); err != nil {
		return err
	}

	// 如果需要自动挂载，添加到 fstab
	if drive.AutoMount {
		if err := s.addToFstab(drive); err != nil {
			s.logger.Warn("Failed to add to fstab", zap.Error(err))
		}
	}

	return nil
}

// addToFstab 添加到 fstab
func (s *Service) addToFstab(drive DriveMount) error {
	fstabEntry := fmt.Sprintf("%s %s %s defaults 0 2\n",
		drive.DevicePath,
		drive.MountPoint,
		drive.Filesystem,
	)

	f, err := os.OpenFile("/etc/fstab", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(fstabEntry)
	return err
}

// ----- Step 5: 网络设置 -----

// ConfigureNetwork 配置网络
func (s *Service) ConfigureNetwork(req *NetworkConfig) error {
	// 如果未指定 mode，默认使用 dhcp
	if req.Mode == "" {
		req.Mode = "dhcp"
	}
	// 设置默认端口
	if req.HTTPPort == 0 {
		req.HTTPPort = 80
	}
	if req.HTTPSPort == 0 {
		req.HTTPSPort = 443
	}

	// 如果是静态 IP，配置网络
	if req.Mode == "static" {
		if err := s.configureStaticIP(req); err != nil {
			s.logger.Error("Failed to configure static IP", zap.Error(err))
			// 不阻止继续，用户可以手动配置
		}
	}

	// 保存设置（暂时只保存 network_mode，不保存端口配置）
	if err := s.updateSettings(map[string]interface{}{
		"network_mode": req.Mode,
	}); err != nil {
		return err
	}

	return s.markStepCompleted(5)
}

// configureStaticIP 配置静态 IP
func (s *Service) configureStaticIP(req *NetworkConfig) error {
	// 使用 netplan 配置（Ubuntu）
	netplanConfig := fmt.Sprintf(`network:
  version: 2
  ethernets:
    eth0:
      dhcp4: no
      addresses:
        - %s/%s
      gateway4: %s
      nameservers:
        addresses: [%s]
`, req.IPAddress, req.Netmask, req.Gateway, strings.Join(req.DNS, ", "))

	configPath := "/etc/netplan/99-rde.yaml"
	if err := os.WriteFile(configPath, []byte(netplanConfig), 0600); err != nil {
		return err
	}

	// 应用配置
	cmd := exec.Command("netplan", "apply")
	return cmd.Run()
}

// SkipNetworkConfig 跳过网络配置
func (s *Service) SkipNetworkConfig() error {
	return s.markStepCompleted(5)
}

// ----- Step 6: 功能选择 -----

// GetFeatureOptions 获取可选功能列表
func (s *Service) GetFeatureOptions() (*FeatureOptionsResponse, error) {
	// 检测硬件支持
	hwSupport := s.detectHardwareSupport()

	// 定义功能分类
	categories := []FeatureCategory{
		{ID: "container", Name: "容器服务", Description: "Docker 容器和应用商店", Icon: "mdi-docker"},
		{ID: "virtualization", Name: "虚拟化", Description: "运行其他操作系统", Icon: "mdi-desktop-classic"},
		{ID: "tools", Name: "工具", Description: "备份和系统工具", Icon: "mdi-tools"},
	}

	// 定义可选功能
	features := []FeatureOption{
		{
			ID:           "docker",
			Name:         "Docker 应用",
			Description:  "Docker 应用商店",
			Icon:         "mdi-docker",
			Category:     "container",
			Dependencies: []string{},
			Recommended:  true,
			RequiresHW:   []string{"docker"},
		},
		{
			ID:           "windows",
			Name:         "Windows 应用",
			Description:  "通过 Wine/Proton 运行 Windows 应用",
			Icon:         "mdi-microsoft-windows",
			Category:     "virtualization",
			Dependencies: []string{},
			Recommended:  false,
			RequiresHW:   []string{},
		},
	}

	return &FeatureOptionsResponse{
		Categories: categories,
		Features:   features,
		HWSupport:  hwSupport,
	}, nil
}

// detectHardwareSupport 检测硬件支持
func (s *Service) detectHardwareSupport() HardwareSupport {
	support := HardwareSupport{}

	// 检测 KVM
	if _, err := os.Stat("/dev/kvm"); err == nil {
		support.KVMAvailable = true
	}

	// 检测 Docker
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err == nil {
		support.DockerAvailable = true
	}

	// 检测 GPU (简单检测)
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		support.GPUAvailable = true
		support.GPUType = "nvidia"
	} else if _, err := os.Stat("/sys/class/drm/card0"); err == nil {
		support.GPUAvailable = true
		// 尝试检测 GPU 类型
		data, _ := os.ReadFile("/sys/class/drm/card0/device/vendor")
		vendor := strings.TrimSpace(string(data))
		switch vendor {
		case "0x1002":
			support.GPUType = "amd"
		case "0x8086":
			support.GPUType = "intel"
		default:
			support.GPUType = "unknown"
		}
	}

	return support
}

// SaveFeatureSelection 保存功能选择
func (s *Service) SaveFeatureSelection(req *FeatureSelection) error {
	// 验证模块依赖
	enabledMap := make(map[string]bool)
	for _, id := range req.EnabledModules {
		enabledMap[id] = true
	}

	// 检查依赖
	featureOptions, _ := s.GetFeatureOptions()
	for _, feature := range featureOptions.Features {
		if enabledMap[feature.ID] {
			for _, dep := range feature.Dependencies {
				if !enabledMap[dep] {
					return fmt.Errorf("模块 %s 依赖 %s，请先启用 %s", feature.Name, dep, dep)
				}
			}
		}
	}

	// 保存模块设置到数据库
	for _, feature := range featureOptions.Features {
		setting := ModuleSetting{
			ModuleID: feature.ID,
			Enabled:  enabledMap[feature.ID],
			Config:   make(map[string]interface{}),
		}

		// 使用 upsert
		if err := s.db.Where("module_id = ?", feature.ID).
			Assign(map[string]interface{}{
				"enabled":    setting.Enabled,
				"updated_at": time.Now(),
			}).
			FirstOrCreate(&setting).Error; err != nil {
			s.logger.Error("Failed to save module setting", zap.String("module", feature.ID), zap.Error(err))
			return err
		}
	}

	s.logger.Info("Feature selection saved", zap.Strings("enabled", req.EnabledModules))
	return s.markStepCompleted(6)
}

// SkipFeatureSelection 跳过功能选择（使用默认配置）
func (s *Service) SkipFeatureSelection() error {
	// 启用推荐的模块
	featureOptions, _ := s.GetFeatureOptions()
	recommended := make([]string, 0)
	for _, feature := range featureOptions.Features {
		if feature.Recommended {
			recommended = append(recommended, feature.ID)
		}
	}

	return s.SaveFeatureSelection(&FeatureSelection{
		EnabledModules: recommended,
	})
}

// ----- Step 3: 完成 -----

// Complete 完成初始化
func (s *Service) Complete() (*CompleteResponse, error) {
	settings, err := s.getOrCreateSettings()
	if err != nil {
		return nil, err
	}

	// 检查必要步骤是否完成
	completedSteps := s.parseCompletedSteps(settings.CompletedSteps)
	requiredSteps := []int{1, 2} // 只需要 语言时区(1) 和 创建管理员(2)

	for _, required := range requiredSteps {
		found := false
		for _, completed := range completedSteps {
			if completed == required {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("step %d not completed", required)
		}
	}

	// 标记完成
	now := time.Now()
	if err := s.updateSettings(map[string]interface{}{
		"setup_completed": true,
		"completed_at":    &now,
		"current_step":    3,
	}); err != nil {
		return nil, err
	}

	s.markStepCompleted(3)

	// 生成自动登录 token 给刚创建的管理员
	var adminUser UserAccount
	if err := s.db.Where("role = ?", "admin").First(&adminUser).Error; err == nil {
		if s.tokenManager == nil {
			s.logger.Warn("TokenManager not set, cannot generate auto-login token")
		} else {
			// 使用 bootstrap 注入的共享 TokenManager 生成 token pair
			tokenPair, err := s.tokenManager.GenerateTokenPair(adminUser.ID, adminUser.Username, adminUser.Role)
			if err == nil {
				s.logger.Info("Auto-login token pair generated for admin", zap.String("username", adminUser.Username))
				return &CompleteResponse{
					Success:        true,
					RedirectURL:    "/",
					AutoLoginToken: tokenPair.AccessToken,
					RefreshToken:   tokenPair.RefreshToken,
					TokenExpiresAt: tokenPair.ExpiresAt,
				}, nil
			}
			s.logger.Warn("Failed to generate auto-login token", zap.Error(err))
		}
	}

	// 如果生成 token 失败，返回默认的登录页跳转
	return &CompleteResponse{
		Success:     true,
		RedirectURL: "/login",
	}, nil
}

// MarkSetupCompleted 直接标记初始化完成（用于 CLI 安装）
func (s *Service) MarkSetupCompleted() error {
	now := time.Now()
	if err := s.updateSettings(map[string]interface{}{
		"setup_completed": true,
		"completed_at":    &now,
		"current_step":    3,
		"completed_steps": "1,2,3",
	}); err != nil {
		return err
	}
	s.logger.Info("Setup marked as completed via CLI")
	return nil
}

// ----- 恢复出厂设置 -----

// UserAccount 用户账户结构（用于密码验证）
type UserAccount struct {
	ID       string `gorm:"primaryKey"`
	Username string
	Password string
	Role     string
}

func (UserAccount) TableName() string {
	return "users_accounts"
}

// ValidateUserPassword 验证用户密码
func (s *Service) ValidateUserPassword(userID string, password string) error {
	var user UserAccount
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		s.logger.Error("Failed to find user", zap.String("user_id", userID), zap.Error(err))
		return ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return ErrInvalidPassword
	}

	return nil
}

// FactoryReset 恢复出厂设置
// 用户认证已在 handler 层通过 JWT 验证
func (s *Service) FactoryReset(req *FactoryResetRequest) (*FactoryResetResponse, error) {
	// 验证确认文本
	if req.ConfirmText != "RESET" {
		return nil, ErrInvalidConfirmText
	}

	s.logger.Warn("Factory reset initiated")

	// 1. 重置 setup_settings 表
	if err := s.resetSetupSettings(); err != nil {
		s.logger.Error("Failed to reset setup settings", zap.Error(err))
		return nil, fmt.Errorf("failed to reset setup settings: %w", err)
	}

	// 2. 删除用户数据（如果不保留）
	if !req.KeepUserFiles {
		if err := s.clearUserFiles(); err != nil {
			s.logger.Error("Failed to clear user files", zap.Error(err))
			// 继续执行，不中断
		}
	}

	// 3. 清理 Docker 应用（如果不保留）
	if !req.KeepDockerApps {
		if err := s.clearDockerApps(); err != nil {
			s.logger.Error("Failed to clear docker apps", zap.Error(err))
			// 继续执行，不中断
		}
	}

	// 4. 清理数据库中的用户相关数据
	if err := s.clearDatabaseTables(); err != nil {
		s.logger.Error("Failed to clear database tables", zap.Error(err))
		// 继续执行，不中断
	}

	s.logger.Info("Factory reset completed")

	return &FactoryResetResponse{
		Success:     true,
		Message:     "恢复出厂设置完成，系统将重新初始化",
		RedirectURL: "/setup",
	}, nil
}

// resetSetupSettings 重置初始化设置
func (s *Service) resetSetupSettings() error {
	// 删除所有设置记录
	if err := s.db.Exec("DELETE FROM setup_settings").Error; err != nil {
		return err
	}
	return nil
}

// clearUserFiles 清理用户文件
func (s *Service) clearUserFiles() error {
	if s.dataDir == "" {
		return nil
	}

	// 需要清理的目录
	dirsToClean := []string{
		filepath.Join(s.dataDir, "files"),
		filepath.Join(s.dataDir, "uploads"),
		filepath.Join(s.dataDir, "thumbnails"),
		filepath.Join(s.dataDir, "cache"),
		filepath.Join(s.dataDir, "backups"),
	}

	for _, dir := range dirsToClean {
		if _, err := os.Stat(dir); err == nil {
			if err := os.RemoveAll(dir); err != nil {
				s.logger.Warn("Failed to remove directory", zap.String("dir", dir), zap.Error(err))
			}
		}
	}

	return nil
}

// clearDockerApps 清理 Docker 应用
func (s *Service) clearDockerApps() error {
	// 停止并删除所有由 rde 管理的容器
	// 使用 label 过滤: rde.managed=true
	cmd := exec.Command("docker", "ps", "-aq", "--filter", "label=rde.managed=true")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	containers := strings.Fields(string(output))
	if len(containers) == 0 {
		return nil
	}

	// 停止容器
	stopArgs := append([]string{"stop"}, containers...)
	if err := exec.Command("docker", stopArgs...).Run(); err != nil {
		s.logger.Warn("Failed to stop containers", zap.Error(err))
	}

	// 删除容器
	rmArgs := append([]string{"rm", "-v"}, containers...)
	if err := exec.Command("docker", rmArgs...).Run(); err != nil {
		s.logger.Warn("Failed to remove containers", zap.Error(err))
	}

	return nil
}

// clearDatabaseTables 清理数据库表
func (s *Service) clearDatabaseTables() error {
	// 需要清理的表（保留 setup_settings 已经单独处理）
	tablesToClear := []string{
		"users_accounts",
		"user_sessions",
		"files",
		"shares",
		"notifications",
		"audit_logs",
		"backups",
		"tasks",
	}

	for _, table := range tablesToClear {
		// 使用 TRUNCATE 或 DELETE 清空表
		if err := s.db.Exec(fmt.Sprintf("DELETE FROM %s", table)).Error; err != nil {
			// 表可能不存在，忽略错误
			s.logger.Debug("Failed to clear table", zap.String("table", table), zap.Error(err))
		}
	}

	return nil
}

// ==================== 模块设置管理 ====================

// GetAllModuleSettings 获取所有模块的设置和元信息
func (s *Service) GetAllModuleSettings() ([]ModuleSettingWithMeta, error) {
	// 获取数据库中的设置
	var settings []ModuleSetting
	if err := s.db.Find(&settings).Error; err != nil {
		return nil, err
	}

	// 创建 map 便于查找
	settingsMap := make(map[string]*ModuleSetting)
	for i := range settings {
		settingsMap[settings[i].ModuleID] = &settings[i]
	}

	// 定义所有可选模块的元信息
	optionalModulesMeta := []struct {
		ID           string
		Name         string
		Description  string
		Dependencies []string
		ConfigSchema []ConfigField
	}{
		{
			ID:           "docker",
			Name:         "Docker 应用",
			Description:  "Docker 应用商店",
			Dependencies: []string{},
			ConfigSchema: []ConfigField{
				{Key: "socket_path", Label: "Docker Socket 路径", Type: "string", Default: "/var/run/docker.sock", Description: "Docker daemon socket 路径"},
			},
		},
		{
			ID:           "windows",
			Name:         "Windows 应用",
			Description:  "Windows 应用商店 (Wine/Proton)",
			Dependencies: []string{},
			ConfigSchema: []ConfigField{
				{Key: "wine_prefix", Label: "Wine 前缀路径", Type: "string", Default: "~/.wine", Description: "Wine 前缀目录"},
				{Key: "use_proton", Label: "使用 Proton", Type: "bool", Default: false, Description: "使用 Steam Proton 代替 Wine"},
				{Key: "dxvk_enabled", Label: "启用 DXVK", Type: "bool", Default: true, Description: "使用 DXVK 加速 DirectX"},
				{Key: "vkd3d_enabled", Label: "启用 VKD3D", Type: "bool", Default: true, Description: "使用 VKD3D 支持 DirectX 12"},
			},
		},
	}

	// 构建结果
	result := make([]ModuleSettingWithMeta, 0, len(optionalModulesMeta))
	for _, meta := range optionalModulesMeta {
		setting := ModuleSettingWithMeta{
			ModuleID:     meta.ID,
			Name:         meta.Name,
			Description:  meta.Description,
			Category:     "optional",
			Dependencies: meta.Dependencies,
			ConfigSchema: meta.ConfigSchema,
			Enabled:      false,
			Config:       make(map[string]interface{}),
		}

		// 如果数据库中有设置，覆盖默认值
		if dbSetting, exists := settingsMap[meta.ID]; exists {
			setting.Enabled = dbSetting.Enabled
			if dbSetting.Config != nil {
				setting.Config = dbSetting.Config
			}
			setting.UpdatedAt = dbSetting.UpdatedAt
		}

		// 检查依赖是否满足
		setting.DepsSatisfied = s.checkDependenciesSatisfied(meta.Dependencies, settingsMap)

		result = append(result, setting)
	}

	return result, nil
}

// checkDependenciesSatisfied 检查依赖是否满足
func (s *Service) checkDependenciesSatisfied(deps []string, settingsMap map[string]*ModuleSetting) bool {
	for _, dep := range deps {
		setting, exists := settingsMap[dep]
		if !exists || !setting.Enabled {
			return false
		}
	}
	return true
}

// GetModuleSetting 获取单个模块的设置
func (s *Service) GetModuleSetting(moduleID string) (*ModuleSetting, error) {
	var setting ModuleSetting
	err := s.db.Where("module_id = ?", moduleID).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 返回默认设置
		return &ModuleSetting{
			ModuleID: moduleID,
			Enabled:  false,
			Config:   make(map[string]interface{}),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// UpdateModuleSetting 更新模块设置
func (s *Service) UpdateModuleSetting(moduleID string, enabled bool, config map[string]interface{}) (*ModuleSetting, error) {
	setting := &ModuleSetting{
		ModuleID:  moduleID,
		Enabled:   enabled,
		Config:    config,
		UpdatedAt: time.Now(),
	}

	// 使用 Upsert
	err := s.db.Save(setting).Error
	if err != nil {
		return nil, err
	}

	s.logger.Info("Module setting updated",
		zap.String("module_id", moduleID),
		zap.Bool("enabled", enabled),
	)

	return setting, nil
}

// ModuleSetting 模块设置（与 model 包中的定义对应）
type ModuleSetting struct {
	ModuleID  string                 `json:"module_id" gorm:"primaryKey;size:64"`
	Enabled   bool                   `json:"enabled" gorm:"default:false"`
	Config    map[string]interface{} `json:"config" gorm:"serializer:json;type:text"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// TableName 返回表名
func (ModuleSetting) TableName() string {
	return "module_settings"
}

// ModuleSettingWithMeta 带元信息的模块设置
type ModuleSettingWithMeta struct {
	ModuleID      string                 `json:"module_id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Category      string                 `json:"category"`
	Enabled       bool                   `json:"enabled"`
	Dependencies  []string               `json:"dependencies"`
	DepsSatisfied bool                   `json:"deps_satisfied"`
	Config        map[string]interface{} `json:"config"`
	ConfigSchema  []ConfigField          `json:"config_schema"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// ConfigField 配置字段定义
type ConfigField struct {
	Key         string      `json:"key"`
	Label       string      `json:"label"`
	Type        string      `json:"type"`
	Default     interface{} `json:"default"`
	Options     []string    `json:"options,omitempty"`
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required,omitempty"`
}
