// Package download 下载服务
package download

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service 下载服务
type Service struct {
	logger              *zap.Logger
	dataDir             string
	downloadDir         string
	client              *Aria2Client
	cmd                 *exec.Cmd
	secret              string
	rpcPort             int
	mu                  sync.RWMutex
	running             bool
	storage             *Storage
	hub                 *Hub
	eventListener       *EventListener
	progressBroadcaster *ProgressBroadcaster
}

// NewService 创建服务实例
func NewService(logger *zap.Logger, db *sql.DB, dataDir, downloadDir string) (*Service, error) {
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(downloadDir, 0755)

	// 初始化存储层
	storage, err := NewStorage(db)
	if err != nil {
		return nil, fmt.Errorf("init storage: %w", err)
	}

	// 从数据库加载用户设置
	settings, _ := storage.GetSettings()
	if settings.DownloadDir != "" {
		downloadDir = settings.DownloadDir
	}

	// 初始化 WebSocket Hub
	hub := NewHub(logger)
	go hub.Run()

	return &Service{
		logger:      logger,
		dataDir:     dataDir,
		downloadDir: downloadDir,
		secret:      uuid.New().String(),
		rpcPort:     6800,
		storage:     storage,
		hub:         hub,
	}, nil
}

// NewServiceSimple 简化的服务创建（无数据库）
func NewServiceSimple(logger *zap.Logger, dataDir, downloadDir string) *Service {
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(downloadDir, 0755)

	hub := NewHub(logger)
	go hub.Run()

	return &Service{
		logger:      logger,
		dataDir:     dataDir,
		downloadDir: downloadDir,
		secret:      uuid.New().String(),
		rpcPort:     6800,
		hub:         hub,
	}
}

// Start 启动 aria2c
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running && s.cmd != nil && s.cmd.Process != nil {
		return nil
	}

	// 检查 aria2c 是否存在
	if _, err := exec.LookPath("aria2c"); err != nil {
		s.logger.Warn("aria2c not found, download module will be disabled", zap.Error(err))
		return nil
	}

	sessionFile := filepath.Join(s.dataDir, "aria2.session")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		os.WriteFile(sessionFile, []byte{}, 0644)
	}

	args := []string{
		"--enable-rpc=true",
		fmt.Sprintf("--rpc-listen-port=%d", s.rpcPort),
		"--rpc-listen-all=false",
		fmt.Sprintf("--rpc-secret=%s", s.secret),
		fmt.Sprintf("--dir=%s", s.downloadDir),
		fmt.Sprintf("--input-file=%s", sessionFile),
		fmt.Sprintf("--save-session=%s", sessionFile),
		"--save-session-interval=30",
		"--continue=true",
		"--max-concurrent-downloads=5",
		"--max-connection-per-server=16",
		"--min-split-size=1M",
		"--split=16",
		"--max-overall-download-limit=0",
		"--max-overall-upload-limit=0",
		"--max-file-not-found=3",
		"--max-tries=5",
		"--retry-wait=3",
		"--timeout=60",
		"--connect-timeout=60",
		"--allow-overwrite=true",
		"--auto-file-renaming=true",
		"--file-allocation=falloc",
		"--disk-cache=64M",
		"--enable-dht=true",
		"--enable-dht6=true",
		"--dht-listen-port=6881-6999",
		"--listen-port=6881-6999",
		"--seed-ratio=1.0",
		fmt.Sprintf("--dht-file-path=%s", filepath.Join(s.dataDir, "dht.dat")),
		fmt.Sprintf("--dht-file-path6=%s", filepath.Join(s.dataDir, "dht6.dat")),
		"--bt-enable-lpd=true",
		"--bt-max-peers=55",
		"--bt-request-peer-speed-limit=100K",
		"--follow-torrent=mem",
		"--follow-metalink=mem",
		"--check-certificate=false",
		"--daemon=false",
	}

	s.cmd = exec.Command("aria2c", args...)
	s.cmd.Stdout = nil
	s.cmd.Stderr = nil

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("start aria2c: %w", err)
	}

	rpcURL := fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", s.rpcPort)
	s.client = NewAria2Client(rpcURL, s.secret)

	for i := 0; i < 50; i++ {
		time.Sleep(200 * time.Millisecond)
		if _, err := s.client.GetVersion(); err == nil {
			s.running = true
			s.logger.Info("aria2c started", zap.Int("rpc_port", s.rpcPort))

			// 启动事件监听器
			if s.storage != nil && s.hub != nil {
				s.eventListener = NewEventListener(s, s.hub, s.storage, s.logger, s.rpcPort)
				s.eventListener.Start()
			}

			// 启动进度广播器
			if s.hub != nil {
				s.progressBroadcaster = NewProgressBroadcaster(s, s.hub, s.logger)
				s.progressBroadcaster.Start()
			}

			// 广播服务状态
			if s.hub != nil {
				s.hub.BroadcastServiceStatus(true)
			}

			return nil
		}
	}

	// aria2c 启动超时，不阻塞主服务，仅禁用下载功能
	s.logger.Warn("aria2c not ready after 10 seconds, download module disabled (port may be occupied)")
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd = nil
	}
	return nil
}

// Stop 停止 aria2c
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 停止进度广播器
	if s.progressBroadcaster != nil {
		s.progressBroadcaster.Stop()
		s.progressBroadcaster = nil
	}

	// 停止事件监听器
	if s.eventListener != nil {
		s.eventListener.Stop()
		s.eventListener = nil
	}

	if s.client != nil {
		s.client.Shutdown()
	}

	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
		s.cmd = nil
	}
	s.running = false

	// 广播服务状态
	if s.hub != nil {
		s.hub.BroadcastServiceStatus(false)
	}

	s.logger.Info("aria2c stopped")
}

// IsRunning 检查是否运行中
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// EnsureRunning 确保 aria2c 已启动（懒启动）
func (s *Service) EnsureRunning() error {
	if s.IsRunning() {
		return nil
	}
	return s.Start()
}

// AddURI 添加 URI 下载
func (s *Service) AddURI(req AddURIRequest) (string, error) {
	if err := s.EnsureRunning(); err != nil {
		return "", fmt.Errorf("aria2 not running: %w", err)
	}

	options := make(map[string]interface{})
	if req.Dir != "" {
		options["dir"] = req.Dir
	} else {
		options["dir"] = s.downloadDir
	}
	if req.Out != "" {
		options["out"] = req.Out
	}
	if req.Options != nil {
		s.mergeOptions(options, req.Options)
	}

	return s.client.AddURI(req.URIs, options)
}

// AddTorrent 添加种子下载
func (s *Service) AddTorrent(req AddTorrentRequest) (string, error) {
	if err := s.EnsureRunning(); err != nil {
		return "", fmt.Errorf("aria2 not running: %w", err)
	}

	options := make(map[string]interface{})
	if req.Dir != "" {
		options["dir"] = req.Dir
	} else {
		options["dir"] = s.downloadDir
	}
	if req.Options != nil {
		s.mergeOptions(options, req.Options)
	}

	return s.client.AddTorrent(req.Torrent, options)
}

// AddMetalink 添加 Metalink 下载
func (s *Service) AddMetalink(req AddMetalinkRequest) ([]string, error) {
	if err := s.EnsureRunning(); err != nil {
		return nil, fmt.Errorf("aria2 not running: %w", err)
	}

	options := make(map[string]interface{})
	if req.Dir != "" {
		options["dir"] = req.Dir
	} else {
		options["dir"] = s.downloadDir
	}
	if req.Options != nil {
		s.mergeOptions(options, req.Options)
	}

	return s.client.AddMetalink(req.Metalink, options)
}

// Pause 暂停任务
func (s *Service) Pause(gid string) error {
	if err := s.EnsureRunning(); err != nil {
		return fmt.Errorf("aria2 not running: %w", err)
	}
	return s.client.Pause(gid)
}

// PauseAll 暂停所有任务
func (s *Service) PauseAll() error {
	if err := s.EnsureRunning(); err != nil {
		return fmt.Errorf("aria2 not running: %w", err)
	}
	return s.client.PauseAll()
}

// Resume 恢复任务
func (s *Service) Resume(gid string) error {
	if err := s.EnsureRunning(); err != nil {
		return fmt.Errorf("aria2 not running: %w", err)
	}
	return s.client.Unpause(gid)
}

// ResumeAll 恢复所有任务
func (s *Service) ResumeAll() error {
	if err := s.EnsureRunning(); err != nil {
		return fmt.Errorf("aria2 not running: %w", err)
	}
	return s.client.UnpauseAll()
}

// Remove 移除任务
func (s *Service) Remove(gid string, force bool) error {
	if err := s.EnsureRunning(); err != nil {
		return fmt.Errorf("aria2 not running: %w", err)
	}
	if force {
		return s.client.ForceRemove(gid)
	}
	return s.client.Remove(gid)
}

// RemoveResult 移除下载结果
func (s *Service) RemoveResult(gid string) error {
	if err := s.EnsureRunning(); err != nil {
		return fmt.Errorf("aria2 not running: %w", err)
	}
	return s.client.RemoveDownloadResult(gid)
}

// PurgeResults 清除所有下载结果
func (s *Service) PurgeResults() error {
	if err := s.EnsureRunning(); err != nil {
		return fmt.Errorf("aria2 not running: %w", err)
	}
	return s.client.PurgeDownloadResult()
}

// GetTask 获取任务
func (s *Service) GetTask(gid string) (*Task, error) {
	if err := s.EnsureRunning(); err != nil {
		return nil, fmt.Errorf("aria2 not running: %w", err)
	}

	status, err := s.client.TellStatus(gid)
	if err != nil {
		return nil, err
	}
	return s.convertTask(status), nil
}

// GetTasks 获取任务列表
func (s *Service) GetTasks() (*TaskListResponse, error) {
	resp := &TaskListResponse{
		Active:  make([]Task, 0),
		Waiting: make([]Task, 0),
		Stopped: make([]Task, 0),
	}

	// 从数据库读取已完成的任务（持久化，不会因重启丢失）
	if s.storage != nil {
		history, total, _ := s.storage.GetHistory(100, 0, "")
		for _, h := range history {
			var finishedAt time.Time
			if h.CompletedAt != nil {
				finishedAt = *h.CompletedAt
			}
			resp.Stopped = append(resp.Stopped, Task{
				GID:          h.GID,
				Name:         h.Name,
				Status:       h.Status,
				TotalLength:  h.Size,
				CompletedLen: h.Size, // 已完成的任务
				Dir:          h.SavePath,
				ErrorMessage: h.ErrorMessage,
				Progress:     100,
				CreatedAt:    h.CreatedAt,
				FinishedAt:   finishedAt,
			})
		}
		// 更新 NumStoppedTotal 为数据库中的总数
		if resp.Stats == nil {
			resp.Stats = &GlobalStat{}
		}
		resp.Stats.NumStoppedTotal = total
	}

	// Aria2 未运行时，仅返回历史记录
	if err := s.EnsureRunning(); err != nil {
		return resp, nil
	}

	// 从 Aria2 获取活动和等待中的任务（实时状态）
	active, _ := s.client.TellActive()
	waiting, _ := s.client.TellWaiting(0, 100)
	stats, _ := s.client.GetGlobalStat()

	for _, t := range active {
		resp.Active = append(resp.Active, *s.convertTask(t))
	}
	for _, t := range waiting {
		resp.Waiting = append(resp.Waiting, *s.convertTask(t))
	}

	if stats != nil {
		resp.Stats = &GlobalStat{
			DownloadSpeed:   ParseInt(stats.DownloadSpeed),
			UploadSpeed:     ParseInt(stats.UploadSpeed),
			NumActive:       ParseIntToInt(stats.NumActive),
			NumWaiting:      ParseIntToInt(stats.NumWaiting),
			NumStopped:      len(resp.Stopped),
			NumStoppedTotal: len(resp.Stopped),
		}
	}

	return resp, nil
}

// GetStats 获取统计
func (s *Service) GetStats() (*GlobalStat, error) {
	if err := s.EnsureRunning(); err != nil {
		return &GlobalStat{}, nil
	}

	stats, err := s.client.GetGlobalStat()
	if err != nil {
		return nil, err
	}

	return &GlobalStat{
		DownloadSpeed:   ParseInt(stats.DownloadSpeed),
		UploadSpeed:     ParseInt(stats.UploadSpeed),
		NumActive:       ParseIntToInt(stats.NumActive),
		NumWaiting:      ParseIntToInt(stats.NumWaiting),
		NumStopped:      ParseIntToInt(stats.NumStopped),
		NumStoppedTotal: ParseIntToInt(stats.NumStoppedTotal),
	}, nil
}

// UpdateOptions 更新全局选项
func (s *Service) UpdateOptions(req UpdateOptionsRequest) error {
	if err := s.EnsureRunning(); err != nil {
		return fmt.Errorf("aria2 not running: %w", err)
	}

	options := make(map[string]interface{})
	if req.MaxConcurrentDownloads > 0 {
		options["max-concurrent-downloads"] = fmt.Sprintf("%d", req.MaxConcurrentDownloads)
	}
	if req.MaxDownloadSpeed != "" {
		options["max-overall-download-limit"] = req.MaxDownloadSpeed
	}
	if req.MaxUploadSpeed != "" {
		options["max-overall-upload-limit"] = req.MaxUploadSpeed
	}

	if len(options) > 0 {
		return s.client.ChangeGlobalOption(options)
	}
	return nil
}

func (s *Service) convertTask(status *Aria2TaskStatus) *Task {
	task := &Task{
		GID:           status.GID,
		Status:        status.Status,
		TotalLength:   ParseInt(status.TotalLength),
		CompletedLen:  ParseInt(status.CompletedLength),
		DownloadSpeed: ParseInt(status.DownloadSpeed),
		UploadSpeed:   ParseInt(status.UploadSpeed),
		Connections:   ParseIntToInt(status.Connections),
		Dir:           status.Dir,
		ErrorCode:     status.ErrorCode,
		ErrorMessage:  status.ErrorMessage,
	}

	if task.TotalLength > 0 {
		task.Progress = float64(task.CompletedLen) / float64(task.TotalLength) * 100
	}

	for _, f := range status.Files {
		file := File{
			Index:           f.Index,
			Path:            f.Path,
			Length:          ParseInt(f.Length),
			CompletedLength: ParseInt(f.CompletedLength),
			Selected:        f.Selected == "true",
		}
		for _, u := range f.URIs {
			file.URIs = append(file.URIs, URI{URI: u.URI, Status: u.Status})
		}
		task.Files = append(task.Files, file)

		if task.Name == "" && f.Path != "" {
			task.Name = filepath.Base(f.Path)
		}
	}

	if status.BitTorrent != nil {
		task.BitTorrent = &BTInfo{
			AnnounceList: status.BitTorrent.AnnounceList,
			Comment:      status.BitTorrent.Comment,
			CreationDate: status.BitTorrent.CreationDate,
			Mode:         status.BitTorrent.Mode,
		}
		if status.BitTorrent.Info != nil {
			task.BitTorrent.Name = status.BitTorrent.Info.Name
			if task.BitTorrent.Name != "" {
				task.Name = task.BitTorrent.Name
			}
		}
	}

	return task
}

func (s *Service) mergeOptions(target map[string]interface{}, opts *Options) {
	if opts.Out != "" {
		target["out"] = opts.Out
	}
	if opts.Split > 0 {
		target["split"] = fmt.Sprintf("%d", opts.Split)
	}
	if opts.MaxConnPerServer > 0 {
		target["max-connection-per-server"] = fmt.Sprintf("%d", opts.MaxConnPerServer)
	}
	if opts.MaxDownloadLimit != "" {
		target["max-download-limit"] = opts.MaxDownloadLimit
	}
	if opts.Header != "" {
		target["header"] = opts.Header
	}
	if opts.Referer != "" {
		target["referer"] = opts.Referer
	}
	if opts.UserAgent != "" {
		target["user-agent"] = opts.UserAgent
	}
	if opts.Proxy != "" {
		target["all-proxy"] = opts.Proxy
	}
	if opts.SelectFile != "" {
		target["select-file"] = opts.SelectFile
	}
}

// GetHub 获取 WebSocket Hub
func (s *Service) GetHub() *Hub {
	return s.hub
}

// GetActiveTasks 获取活跃任务列表（供进度广播器使用）
func (s *Service) GetActiveTasks() ([]*Task, error) {
	if !s.IsRunning() {
		return nil, nil
	}

	active, err := s.client.TellActive()
	if err != nil {
		return nil, err
	}

	tasks := make([]*Task, 0, len(active))
	for _, t := range active {
		tasks = append(tasks, s.convertTask(t))
	}
	return tasks, nil
}

// GetHistory 获取下载历史
func (s *Service) GetHistory(limit, offset int, status string) ([]DownloadHistory, int, error) {
	if s.storage == nil {
		return nil, 0, fmt.Errorf("storage not initialized")
	}
	return s.storage.GetHistory(limit, offset, status)
}

// DeleteHistoryItem 删除单条历史记录
func (s *Service) DeleteHistoryItem(id int64) error {
	if s.storage == nil {
		return fmt.Errorf("storage not initialized")
	}
	return s.storage.DeleteHistory(id)
}

// ClearHistory 清空历史记录
func (s *Service) ClearHistory() error {
	if s.storage == nil {
		return fmt.Errorf("storage not initialized")
	}
	return s.storage.ClearHistory()
}

// SearchHistory 搜索历史记录
func (s *Service) SearchHistory(keyword string, limit int) ([]DownloadHistory, error) {
	if s.storage == nil {
		return nil, fmt.Errorf("storage not initialized")
	}
	return s.storage.SearchHistory(keyword, limit)
}

// GetSettings 获取用户设置
func (s *Service) GetSettings() (*UserSettings, error) {
	if s.storage == nil {
		return &DefaultSettings, nil
	}
	return s.storage.GetSettings()
}

// UpdateSettings 更新用户设置
func (s *Service) UpdateSettings(settings *UserSettings) error {
	if s.storage == nil {
		return fmt.Errorf("storage not initialized")
	}

	// 保存到数据库
	if err := s.storage.UpdateSettings(settings); err != nil {
		return err
	}

	// 更新本地下载目录
	if settings.DownloadDir != "" {
		s.downloadDir = settings.DownloadDir
		os.MkdirAll(s.downloadDir, 0755)
	}

	// 动态应用到 aria2
	if s.IsRunning() {
		return s.applySettingsToAria2(settings)
	}
	return nil
}

// applySettingsToAria2 将设置应用到运行中的 aria2
func (s *Service) applySettingsToAria2(settings *UserSettings) error {
	options := make(map[string]interface{})

	options["max-concurrent-downloads"] = fmt.Sprintf("%d", settings.MaxConcurrent)
	options["max-connection-per-server"] = fmt.Sprintf("%d", settings.MaxConnPerServer)
	options["split"] = fmt.Sprintf("%d", settings.Split)
	options["max-overall-download-limit"] = fmt.Sprintf("%d", settings.GlobalDownloadLimit)
	options["max-overall-upload-limit"] = fmt.Sprintf("%d", settings.GlobalUploadLimit)

	if settings.SeedRatio > 0 {
		options["seed-ratio"] = fmt.Sprintf("%.1f", settings.SeedRatio)
	}
	if settings.SeedTime > 0 {
		options["seed-time"] = fmt.Sprintf("%d", settings.SeedTime)
	}

	return s.client.ChangeGlobalOption(options)
}

// GetStatistics 获取下载统计
func (s *Service) GetStatistics() (*DownloadStatistics, error) {
	if s.storage == nil {
		return &DownloadStatistics{}, nil
	}
	return s.storage.GetStatistics()
}
