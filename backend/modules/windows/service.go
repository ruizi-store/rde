// Package windows Windows 服务
package windows

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service Windows 服务
type Service struct {
	logger      *zap.Logger
	dataDir     string
	prefixes    map[string]*WinePrefix
	apps        map[string]*App
	sessions    map[string]*Session
	processes   map[string]*exec.Cmd
	baseDisplay int
	basePort    int
	mu          sync.RWMutex
}

// NewService 创建服务实例
func NewService(logger *zap.Logger, dataDir string) *Service {
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(filepath.Join(dataDir, "prefixes"), 0755)

	s := &Service{
		logger:      logger,
		dataDir:     dataDir,
		prefixes:    make(map[string]*WinePrefix),
		apps:        make(map[string]*App),
		sessions:    make(map[string]*Session),
		processes:   make(map[string]*exec.Cmd),
		baseDisplay: 200,
		basePort:    11000,
	}

	s.loadData()
	return s
}

// Start 启动服务
func (s *Service) Start() error {
	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, cmd := range s.processes {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		delete(s.processes, id)
	}

	for id := range s.sessions {
		delete(s.sessions, id)
	}
}

// GetWineInfo 获取 Wine 信息
func (s *Service) GetWineInfo() *WineInfo {
	info := &WineInfo{}

	if out, err := exec.Command("wine", "--version").Output(); err == nil {
		info.Version = strings.TrimSpace(string(out))
	}

	if path, err := exec.LookPath("wine"); err == nil {
		info.Path = path
	}

	// 检测架构
	info.Arch = "win64"
	if out, err := exec.Command("wine64", "--version").CombinedOutput(); err != nil {
		info.Arch = "win32"
		_ = out
	}

	if out, err := exec.Command("winetricks", "--version").Output(); err == nil {
		info.WinetricksVer = strings.TrimSpace(string(out))
	}

	return info
}

// GetPrefixes 获取前缀列表
func (s *Service) GetPrefixes() []*WinePrefix {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefixes := make([]*WinePrefix, 0, len(s.prefixes))
	for _, p := range s.prefixes {
		prefixes = append(prefixes, p)
	}
	return prefixes
}

// GetPrefix 获取前缀
func (s *Service) GetPrefix(id string) (*WinePrefix, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.prefixes[id]
	if !ok {
		return nil, fmt.Errorf("prefix not found: %s", id)
	}
	return p, nil
}

// CreatePrefix 创建前缀
func (s *Service) CreatePrefix(req CreatePrefixRequest) (*WinePrefix, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()[:8]
	prefixPath := filepath.Join(s.dataDir, "prefixes", id)

	arch := req.Arch
	if arch == "" {
		arch = "win64"
	}

	prefix := &WinePrefix{
		ID:         id,
		Name:       req.Name,
		Path:       prefixPath,
		Arch:       arch,
		WindowsVer: req.WindowsVer,
		CreatedAt:  time.Now(),
	}

	// 创建 Wine 前缀
	env := s.buildWineEnv(prefix, nil)
	cmd := exec.Command("wineboot", "--init")
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("create prefix: %w", err)
	}

	// 设置 Windows 版本
	if req.WindowsVer != "" {
		s.setWindowsVersion(prefix, req.WindowsVer)
	}

	s.prefixes[id] = prefix
	s.saveData()

	s.logger.Info("prefix created", zap.String("id", id), zap.String("name", req.Name))
	return prefix, nil
}

// DeletePrefix 删除前缀
func (s *Service) DeletePrefix(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.prefixes[id]
	if !ok {
		return fmt.Errorf("prefix not found: %s", id)
	}

	// 检查是否有运行中的应用
	for _, sess := range s.sessions {
		if sess.PrefixID == id {
			return fmt.Errorf("prefix has running sessions")
		}
	}

	// 删除关联的应用
	for appID, app := range s.apps {
		if app.PrefixID == id {
			delete(s.apps, appID)
		}
	}

	// 删除目录
	os.RemoveAll(p.Path)

	delete(s.prefixes, id)
	s.saveData()

	s.logger.Info("prefix deleted", zap.String("id", id))
	return nil
}

// GetApps 获取应用列表
func (s *Service) GetApps(prefixID string) []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apps := make([]*App, 0)
	for _, app := range s.apps {
		if prefixID == "" || app.PrefixID == prefixID {
			apps = append(apps, app)
		}
	}
	return apps
}

// GetApp 获取应用
func (s *Service) GetApp(id string) (*App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.apps[id]
	if !ok {
		return nil, fmt.Errorf("app not found: %s", id)
	}
	return app, nil
}

// AddApp 添加应用
func (s *Service) AddApp(req AddAppRequest) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.prefixes[req.PrefixID]; !ok {
		return nil, fmt.Errorf("prefix not found: %s", req.PrefixID)
	}

	id := uuid.New().String()[:8]
	app := &App{
		ID:          id,
		Name:        req.Name,
		PrefixID:    req.PrefixID,
		ExePath:     req.ExePath,
		WorkDir:     req.WorkDir,
		Args:        req.Args,
		Env:         req.Env,
		Status:      AppStatusStopped,
		InstalledAt: time.Now(),
	}

	s.apps[id] = app
	s.saveData()

	s.logger.Info("app added", zap.String("id", id), zap.String("name", req.Name))
	return app, nil
}

// UpdateApp 更新应用
func (s *Service) UpdateApp(id string, req UpdateAppRequest) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.apps[id]
	if !ok {
		return nil, fmt.Errorf("app not found: %s", id)
	}

	if req.Name != "" {
		app.Name = req.Name
	}
	if req.ExePath != "" {
		app.ExePath = req.ExePath
	}
	if req.WorkDir != "" {
		app.WorkDir = req.WorkDir
	}
	if req.Args != nil {
		app.Args = req.Args
	}
	if req.Env != nil {
		app.Env = req.Env
	}

	s.saveData()
	return app, nil
}

// DeleteApp 删除应用
func (s *Service) DeleteApp(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.apps[id]; !ok {
		return fmt.Errorf("app not found: %s", id)
	}

	delete(s.apps, id)
	s.saveData()

	s.logger.Info("app deleted", zap.String("id", id))
	return nil
}

// InstallApp 安装应用
func (s *Service) InstallApp(req InstallAppRequest) (*App, error) {
	s.mu.Lock()
	prefix, ok := s.prefixes[req.PrefixID]
	s.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("prefix not found: %s", req.PrefixID)
	}

	env := s.buildWineEnv(prefix, nil)
	args := []string{req.InstallerPath}
	if req.Silent {
		args = append(args, "/S", "/silent", "/quiet")
	}

	cmd := exec.Command("wine", args...)
	cmd.Env = env
	cmd.Dir = filepath.Dir(req.InstallerPath)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("install failed: %w", err)
	}

	// 添加应用记录
	name := req.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(req.InstallerPath), filepath.Ext(req.InstallerPath))
	}

	return s.AddApp(AddAppRequest{
		PrefixID: req.PrefixID,
		Name:     name,
		ExePath:  req.InstallerPath,
	})
}

// GetSessions 获取会话列表
func (s *Service) GetSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	return sessions
}

// LaunchApp 启动应用
func (s *Service) LaunchApp(req LaunchRequest) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.apps[req.AppID]
	if !ok {
		return nil, fmt.Errorf("app not found: %s", req.AppID)
	}

	prefix, ok := s.prefixes[app.PrefixID]
	if !ok {
		return nil, fmt.Errorf("prefix not found: %s", app.PrefixID)
	}

	return s.launchWithXpra(app, prefix, req.Args, req.Env)
}

// RunExe 运行 EXE
func (s *Service) RunExe(req RunExeRequest) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefix, ok := s.prefixes[req.PrefixID]
	if !ok {
		return nil, fmt.Errorf("prefix not found: %s", req.PrefixID)
	}

	// 创建临时应用
	app := &App{
		ID:       uuid.New().String()[:8],
		Name:     filepath.Base(req.ExePath),
		PrefixID: req.PrefixID,
		ExePath:  req.ExePath,
		Env:      req.Env,
	}

	return s.launchWithXpra(app, prefix, req.Args, req.Env)
}

// StopSession 停止会话
func (s *Service) StopSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil
	}

	// 停止 xpra
	exec.Command("xpra", "stop", fmt.Sprintf(":%d", sess.Display)).Run()

	if cmd, ok := s.processes[id]; ok {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		delete(s.processes, id)
	}

	delete(s.sessions, id)
	s.logger.Info("session stopped", zap.String("id", id))
	return nil
}

// RunWinetricks 运行 Winetricks
func (s *Service) RunWinetricks(req WinetricksRequest) error {
	s.mu.RLock()
	prefix, ok := s.prefixes[req.PrefixID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("prefix not found: %s", req.PrefixID)
	}

	env := s.buildWineEnv(prefix, nil)
	args := append([]string{"-q"}, req.Verbs...)

	cmd := exec.Command("winetricks", args...)
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("winetricks failed: %s", string(output))
	}

	return nil
}

// GetStoreApps 获取商店应用
func (s *Service) GetStoreApps() []StoreApp {
	return []StoreApp{
		{ID: "steam", Name: "Steam", Category: "gaming", Verbs: []string{"steam"}},
		{ID: "office", Name: "Microsoft Office", Category: "productivity"},
		{ID: "photoshop", Name: "Adobe Photoshop", Category: "graphics"},
		{ID: "notepadpp", Name: "Notepad++", Category: "utility", Verbs: []string{"notepadpp"}},
		{ID: "7zip", Name: "7-Zip", Category: "utility", Verbs: []string{"7zip"}},
		{ID: "vlc", Name: "VLC Media Player", Category: "multimedia", Verbs: []string{"vlc"}},
		{ID: "firefox", Name: "Firefox", Category: "internet", Verbs: []string{"firefox"}},
		{ID: "chrome", Name: "Google Chrome", Category: "internet"},
	}
}

func (s *Service) launchWithXpra(app *App, prefix *WinePrefix, args []string, extraEnv map[string]string) (*Session, error) {
	id := uuid.New().String()[:8]
	display := s.allocateDisplay()
	port := s.basePort + display - s.baseDisplay

	// 合并环境变量
	env := make(map[string]string)
	for k, v := range app.Env {
		env[k] = v
	}
	for k, v := range extraEnv {
		env[k] = v
	}

	wineEnv := s.buildWineEnv(prefix, env)

	// 构建 Wine 命令
	wineArgs := []string{app.ExePath}
	wineArgs = append(wineArgs, app.Args...)
	wineArgs = append(wineArgs, args...)

	wineCmd := "wine " + strings.Join(wineArgs, " ")

	// 启动 xpra
	xpraArgs := []string{
		"start",
		fmt.Sprintf(":%d", display),
		"--start=" + wineCmd,
		"--bind-tcp=0.0.0.0:" + strconv.Itoa(port),
		"--html=on",
		"--daemon=no",
		"--systemd-run=no",
		"--exit-with-children=yes",
	}

	cmd := exec.Command("xpra", xpraArgs...)
	cmd.Env = wineEnv
	if app.WorkDir != "" {
		cmd.Dir = app.WorkDir
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start xpra: %w", err)
	}

	session := &Session{
		ID:        id,
		AppID:     app.ID,
		AppName:   app.Name,
		PrefixID:  prefix.ID,
		PID:       cmd.Process.Pid,
		Display:   display,
		Port:      port,
		Status:    AppStatusRunning,
		StartedAt: time.Now(),
	}

	s.sessions[id] = session
	s.processes[id] = cmd

	// 更新应用状态
	if a, ok := s.apps[app.ID]; ok {
		a.Status = AppStatusRunning
	}

	// 监控进程
	go func() {
		cmd.Wait()
		s.mu.Lock()
		if sess, ok := s.sessions[id]; ok {
			sess.Status = AppStatusStopped
		}
		if a, ok := s.apps[app.ID]; ok {
			a.Status = AppStatusStopped
		}
		delete(s.processes, id)
		s.mu.Unlock()
	}()

	s.logger.Info("session started",
		zap.String("id", id),
		zap.String("app", app.Name),
		zap.Int("display", display),
		zap.Int("port", port))

	return session, nil
}

func (s *Service) buildWineEnv(prefix *WinePrefix, extra map[string]string) []string {
	env := os.Environ()
	env = append(env, "WINEPREFIX="+prefix.Path)

	if prefix.Arch == "win32" {
		env = append(env, "WINEARCH=win32")
	} else {
		env = append(env, "WINEARCH=win64")
	}

	for k, v := range extra {
		env = append(env, k+"="+v)
	}

	return env
}

func (s *Service) setWindowsVersion(prefix *WinePrefix, version string) {
	env := s.buildWineEnv(prefix, nil)
	cmd := exec.Command("winetricks", "-q", version)
	cmd.Env = env
	cmd.Run()
}

func (s *Service) allocateDisplay() int {
	display := s.baseDisplay
	for _, sess := range s.sessions {
		if sess.Display >= display {
			display = sess.Display + 1
		}
	}
	return display
}

func (s *Service) loadData() {
	// 加载前缀
	prefixFile := filepath.Join(s.dataDir, "prefixes.json")
	if data, err := os.ReadFile(prefixFile); err == nil {
		json.Unmarshal(data, &s.prefixes)
	}

	// 加载应用
	appsFile := filepath.Join(s.dataDir, "apps.json")
	if data, err := os.ReadFile(appsFile); err == nil {
		json.Unmarshal(data, &s.apps)
	}
}

func (s *Service) saveData() {
	// 保存前缀
	prefixFile := filepath.Join(s.dataDir, "prefixes.json")
	if data, err := json.MarshalIndent(s.prefixes, "", "  "); err == nil {
		os.WriteFile(prefixFile, data, 0644)
	}

	// 保存应用
	appsFile := filepath.Join(s.dataDir, "apps.json")
	if data, err := json.MarshalIndent(s.apps, "", "  "); err == nil {
		os.WriteFile(appsFile, data, 0644)
	}
}
