// Package samba 提供 Samba 服务管理
package samba

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// Service Samba 服务
type Service struct {
	logger       *zap.Logger
	configParser *ConfigParser
}

// NewService 创建服务实例
func NewService(logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		logger:       logger,
		configParser: NewConfigParser(""),
	}
}

// ==================== 服务管理 ====================

// GetServiceStatus 获取服务状态
func (s *Service) GetServiceStatus(ctx context.Context) (*ServiceStatus, error) {
	status := &ServiceStatus{}

	// 检查是否安装
	if _, err := exec.LookPath("smbd"); err != nil {
		status.Installed = false
		return status, nil
	}
	status.Installed = true

	// 获取版本
	if output, err := exec.Command("smbd", "--version").Output(); err == nil {
		status.Version = strings.TrimSpace(string(output))
	}

	// 检查运行状态
	if err := exec.Command("systemctl", "is-active", "--quiet", "smbd").Run(); err == nil {
		status.Running = true
	}

	// 检查开机启动
	if err := exec.Command("systemctl", "is-enabled", "--quiet", "smbd").Run(); err == nil {
		status.Enabled = true
	}

	return status, nil
}

// StartService 启动服务
func (s *Service) StartService(ctx context.Context) error {
	if output, err := exec.Command("systemctl", "start", "smbd").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start smbd: %s", string(output))
	}
	s.logger.Info("smbd service started")
	exec.Command("systemctl", "start", "nmbd").Run()
	return nil
}

// StopService 停止服务
func (s *Service) StopService(ctx context.Context) error {
	if output, err := exec.Command("systemctl", "stop", "smbd").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop smbd: %s", string(output))
	}
	s.logger.Info("smbd service stopped")
	exec.Command("systemctl", "stop", "nmbd").Run()
	return nil
}

// RestartService 重启服务
func (s *Service) RestartService(ctx context.Context) error {
	if output, err := exec.Command("systemctl", "restart", "smbd").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart smbd: %s", string(output))
	}
	s.logger.Info("smbd service restarted")
	exec.Command("systemctl", "restart", "nmbd").Run()
	return nil
}

// ReloadService 重载配置
func (s *Service) ReloadService(ctx context.Context) error {
	if output, err := exec.Command("smbcontrol", "smbd", "reload-config").CombinedOutput(); err == nil {
		s.logger.Info("smbd config reloaded")
		return nil
	} else {
		s.logger.Debug("smbcontrol failed, trying systemctl reload", zap.String("output", string(output)))
	}

	if output, err := exec.Command("systemctl", "reload", "smbd").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload smbd: %s", string(output))
	}
	s.logger.Info("smbd config reloaded via systemctl")
	return nil
}

// ==================== 共享管理 ====================

// ListShares 获取所有共享
func (s *Service) ListShares(ctx context.Context) ([]SambaShare, error) {
	config, err := s.configParser.Parse()
	if err != nil {
		return nil, err
	}
	return config.Shares, nil
}

// GetShare 获取单个共享
func (s *Service) GetShare(ctx context.Context, name string) (*SambaShare, error) {
	return s.configParser.GetShare(name)
}

// CreateShare 创建共享
func (s *Service) CreateShare(ctx context.Context, req CreateShareRequest) (*SambaShare, error) {
	if err := ValidateShareName(req.Name); err != nil {
		return nil, err
	}
	if err := ValidateSharePath(req.Path); err != nil {
		return nil, err
	}

	browseable := true
	if req.Browseable != nil {
		browseable = *req.Browseable
	}
	writable := true
	if req.Writable != nil {
		writable = *req.Writable
	}

	share := SambaShare{
		Name:       req.Name,
		Path:       req.Path,
		Comment:    req.Comment,
		Browseable: browseable,
		Writable:   writable,
		ReadOnly:   !writable,
		GuestOK:    req.GuestOK,
		ValidUsers: req.ValidUsers,
		CreateMask: "0644",
		DirMask:    "0755",
	}

	if err := s.configParser.AddShare(share); err != nil {
		return nil, err
	}

	if err := s.ReloadService(ctx); err != nil {
		s.logger.Warn("failed to reload after create share", zap.Error(err))
	}

	return &share, nil
}

// UpdateShare 更新共享
func (s *Service) UpdateShare(ctx context.Context, name string, req UpdateShareRequest) (*SambaShare, error) {
	share, err := s.configParser.GetShare(name)
	if err != nil {
		return nil, err
	}

	if req.Path != "" {
		if err := ValidateSharePath(req.Path); err != nil {
			return nil, err
		}
		share.Path = req.Path
	}
	if req.Comment != "" {
		share.Comment = req.Comment
	}
	if req.Browseable != nil {
		share.Browseable = *req.Browseable
	}
	if req.Writable != nil {
		share.Writable = *req.Writable
		share.ReadOnly = !*req.Writable
	}
	if req.ReadOnly != nil {
		share.ReadOnly = *req.ReadOnly
		share.Writable = !*req.ReadOnly
	}
	if req.GuestOK != nil {
		share.GuestOK = *req.GuestOK
	}
	if req.ValidUsers != nil {
		share.ValidUsers = req.ValidUsers
	}
	if req.InvalidUsers != nil {
		share.InvalidUsers = req.InvalidUsers
	}
	if req.CreateMask != "" {
		share.CreateMask = req.CreateMask
	}
	if req.DirMask != "" {
		share.DirMask = req.DirMask
	}

	if err := s.configParser.UpdateShare(*share); err != nil {
		return nil, err
	}

	if err := s.ReloadService(ctx); err != nil {
		s.logger.Warn("failed to reload after update share", zap.Error(err))
	}

	return share, nil
}

// DeleteShare 删除共享
func (s *Service) DeleteShare(ctx context.Context, name string) error {
	if err := s.configParser.DeleteShare(name); err != nil {
		return err
	}

	if err := s.ReloadService(ctx); err != nil {
		s.logger.Warn("failed to reload after delete share", zap.Error(err))
	}
	return nil
}

// ==================== 全局配置 ====================

// GetGlobalConfig 获取全局配置
func (s *Service) GetGlobalConfig(ctx context.Context) (*SambaGlobalConfig, error) {
	config, err := s.configParser.Parse()
	if err != nil {
		return nil, err
	}
	return &config.Global, nil
}

// UpdateGlobalConfig 更新全局配置
func (s *Service) UpdateGlobalConfig(ctx context.Context, req UpdateGlobalConfigRequest) error {
	config, err := s.configParser.Parse()
	if err != nil {
		return err
	}

	if req.Workgroup != "" {
		config.Global.Workgroup = req.Workgroup
	}
	if req.ServerString != "" {
		config.Global.ServerString = req.ServerString
	}
	if req.NetbiosName != "" {
		config.Global.NetbiosName = req.NetbiosName
	}

	if err := s.configParser.Write(config); err != nil {
		return err
	}

	return s.ReloadService(ctx)
}

// ==================== 用户管理 ====================

// ListUsers 获取 Samba 用户列表
func (s *Service) ListUsers(ctx context.Context) ([]SambaUser, error) {
	// 先检查 pdbedit 是否可用
	if _, err := exec.LookPath("pdbedit"); err != nil {
		s.logger.Debug("pdbedit not found, returning empty user list")
		return []SambaUser{}, nil
	}

	output, err := exec.Command("pdbedit", "-L", "-v").Output()
	if err != nil {
		s.logger.Warn("failed to list samba users", zap.Error(err))
		return []SambaUser{}, nil
	}

	users := make([]SambaUser, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	var currentUser *SambaUser
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Unix username:") {
			if currentUser != nil {
				users = append(users, *currentUser)
			}
			currentUser = &SambaUser{
				Username: strings.TrimSpace(strings.TrimPrefix(line, "Unix username:")),
				Enabled:  true,
			}
		} else if currentUser != nil && strings.HasPrefix(line, "Unix user ID:") {
			uid, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Unix user ID:")))
			currentUser.UID = uid
		} else if currentUser != nil && strings.Contains(line, "Account Flags:") {
			if strings.Contains(line, "D") {
				currentUser.Enabled = false
			}
		}
	}
	if currentUser != nil {
		users = append(users, *currentUser)
	}

	return users, nil
}

// AddUser 添加 Samba 用户
// smbpasswd -a 需要对应的 Linux 系统用户存在，因此先自动创建
func (s *Service) AddUser(ctx context.Context, username, password string) error {
	// 检查系统用户是否存在，不存在则创建（无登录 shell）
	if _, err := exec.Command("id", username).Output(); err != nil {
		if output, err := exec.Command("useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", username).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create system user: %s", string(output))
		}
		s.logger.Info("created system user for samba", zap.String("username", username))
	}

	cmd := exec.Command("smbpasswd", "-a", "-s", username)
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add samba user: %s", string(output))
	}
	return nil
}

// DeleteUser 删除 Samba 用户
func (s *Service) DeleteUser(ctx context.Context, username string) error {
	if output, err := exec.Command("smbpasswd", "-x", username).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete samba user: %s", string(output))
	}
	return nil
}

// SetUserPassword 设置用户密码
func (s *Service) SetUserPassword(ctx context.Context, username, password string) error {
	cmd := exec.Command("smbpasswd", "-s", username)
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set password: %s", string(output))
	}
	return nil
}

// GetSystemUsers 获取可用的系统用户
func (s *Service) GetSystemUsers(ctx context.Context) ([]string, error) {
	output, err := exec.Command("getent", "passwd").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get system users: %w", err)
	}

	users := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ":")
		if len(parts) >= 3 {
			uid, _ := strconv.Atoi(parts[2])
			if uid >= 1000 && uid < 65534 {
				users = append(users, parts[0])
			}
		}
	}
	return users, nil
}

// ==================== 会话管理 ====================

// GetSessions 获取活动会话
func (s *Service) GetSessions(ctx context.Context) (*SessionsInfo, error) {
	info := &SessionsInfo{
		Sessions:  make([]SambaSession, 0),
		Shares:    make([]SambaShareConnection, 0),
		OpenFiles: make([]SambaOpenFile, 0),
	}

	// 获取会话信息
	if output, err := exec.Command("smbstatus", "-p").Output(); err == nil {
		info.Sessions = parseSessionsText(string(output))
	}

	// 获取共享连接
	if output, err := exec.Command("smbstatus", "-S").Output(); err == nil {
		info.Shares = parseSharesText(string(output))
	}

	// 获取打开的文件
	if output, err := exec.Command("smbstatus", "-L").Output(); err == nil {
		info.OpenFiles = parseFilesText(string(output))
	}

	info.TotalCount = len(info.Sessions)
	userSet := make(map[string]bool)
	for _, sess := range info.Sessions {
		userSet[sess.Username] = true
	}
	info.UniqueUsers = len(userSet)

	return info, nil
}

// KillSession 终止指定会话
func (s *Service) KillSession(ctx context.Context, pid int) error {
	if output, err := exec.Command("kill", "-TERM", fmt.Sprintf("%d", pid)).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to kill session: %s", string(output))
	}
	return nil
}

// KillUserSessions 终止用户的所有会话
func (s *Service) KillUserSessions(ctx context.Context, username string) error {
	sessions, err := s.GetSessions(ctx)
	if err != nil {
		return err
	}

	var lastErr error
	for _, sess := range sessions.Sessions {
		if sess.Username == username {
			if err := s.KillSession(ctx, sess.PID); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// TestConfig 测试配置有效性
func (s *Service) TestConfig(ctx context.Context) (string, error) {
	output, err := exec.Command("testparm", "-s").CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("config test failed: %w", err)
	}
	return string(output), nil
}

// ==================== 辅助函数 ====================

func parseSessionsText(output string) []SambaSession {
	sessions := make([]SambaSession, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	inData := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "---") {
			inData = true
			continue
		}
		if !inData {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			pid, _ := strconv.Atoi(fields[0])
			sess := SambaSession{
				PID:      pid,
				Username: fields[1],
				Group:    fields[2],
				Machine:  fields[3],
			}
			if len(fields) >= 5 {
				sess.ProtocolVer = fields[4]
			}
			sessions = append(sessions, sess)
		}
	}
	return sessions
}

func parseSharesText(output string) []SambaShareConnection {
	shares := make([]SambaShareConnection, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	inData := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "---") {
			inData = true
			continue
		}
		if !inData {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 {
			pid, _ := strconv.Atoi(fields[1])
			share := SambaShareConnection{
				Service: fields[0],
				PID:     pid,
				Machine: fields[2],
			}
			shares = append(shares, share)
		}
	}
	return shares
}

func parseFilesText(output string) []SambaOpenFile {
	files := make([]SambaOpenFile, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	inData := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "---") {
			inData = true
			continue
		}
		if !inData {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 5 {
			pid, _ := strconv.Atoi(fields[0])
			file := SambaOpenFile{
				PID:      pid,
				Username: fields[1],
				DenyMode: fields[2],
				Access:   fields[3],
			}
			if len(fields) > 4 {
				file.FileName = strings.Join(fields[4:], " ")
			}
			files = append(files, file)
		}
	}
	return files
}
