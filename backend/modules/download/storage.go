// Package download 数据持久化
package download

import (
	"database/sql"
	"time"
)

// Storage 下载数据存储
type Storage struct {
	db *sql.DB
}

// DownloadHistory 下载历史记录
type DownloadHistory struct {
	ID           int64      `json:"id"`
	GID          string     `json:"gid"`
	Name         string     `json:"name"`
	URL          string     `json:"url,omitempty"`
	Size         int64      `json:"size"`
	SavePath     string     `json:"save_path"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Duration     int64      `json:"duration"`  // 下载耗时(秒)
	AvgSpeed     int64      `json:"avg_speed"` // 平均速度 bytes/s
}

// UserSettings 用户设置
type UserSettings struct {
	DownloadDir         string  `json:"download_dir"`
	MaxConcurrent       int     `json:"max_concurrent"`
	MaxConnPerServer    int     `json:"max_conn_per_server"`
	Split               int     `json:"split"`
	GlobalDownloadLimit int64   `json:"global_download_limit"` // bytes/s, 0=无限制
	GlobalUploadLimit   int64   `json:"global_upload_limit"`
	SeedRatio           float64 `json:"seed_ratio"`
	SeedTime            int     `json:"seed_time"` // 分钟
	EnableDHT           bool    `json:"enable_dht"`
	NotifyOnComplete    bool    `json:"notify_on_complete"`
	AutoStart           bool    `json:"auto_start"`
}

// DefaultSettings 默认设置（DownloadDir 为空表示使用系统默认）
var DefaultSettings = UserSettings{
	DownloadDir:         "",
	MaxConcurrent:       5,
	MaxConnPerServer:    16,
	Split:               16,
	GlobalDownloadLimit: 0,
	GlobalUploadLimit:   0,
	SeedRatio:           1.0,
	SeedTime:            0,
	EnableDHT:           true,
	NotifyOnComplete:    true,
	AutoStart:           true,
}

// DownloadStatistics 下载统计
type DownloadStatistics struct {
	TotalDownloads    int   `json:"total_downloads"`
	CompletedCount    int   `json:"completed_count"`
	FailedCount       int   `json:"failed_count"`
	TotalSize         int64 `json:"total_size"`
	TodayDownloads    int   `json:"today_downloads"`
	TodaySize         int64 `json:"today_size"`
	WeekDownloads     int   `json:"week_downloads"`
	WeekSize          int64 `json:"week_size"`
	AverageSpeed      int64 `json:"average_speed"`
	FastestDownload   int64 `json:"fastest_download"`
}

// NewStorage 创建存储实例
func NewStorage(db *sql.DB) (*Storage, error) {
	s := &Storage{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// migrate 执行数据库迁移
func (s *Storage) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS download_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		gid TEXT NOT NULL,
		name TEXT NOT NULL,
		url TEXT,
		size INTEGER DEFAULT 0,
		save_path TEXT,
		status TEXT NOT NULL,
		error_message TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		duration INTEGER DEFAULT 0,
		avg_speed INTEGER DEFAULT 0
	);
	
	CREATE INDEX IF NOT EXISTS idx_history_created ON download_history(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_history_status ON download_history(status);
	CREATE INDEX IF NOT EXISTS idx_history_gid ON download_history(gid);
	
	CREATE TABLE IF NOT EXISTS download_settings (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		download_dir TEXT DEFAULT '',
		max_concurrent INTEGER DEFAULT 5,
		max_conn_per_server INTEGER DEFAULT 16,
		split INTEGER DEFAULT 16,
		global_download_limit INTEGER DEFAULT 0,
		global_upload_limit INTEGER DEFAULT 0,
		seed_ratio REAL DEFAULT 1.0,
		seed_time INTEGER DEFAULT 0,
		enable_dht INTEGER DEFAULT 1,
		notify_on_complete INTEGER DEFAULT 1,
		auto_start INTEGER DEFAULT 1
	);
	
	INSERT OR IGNORE INTO download_settings (id) VALUES (1);
	`
	_, err := s.db.Exec(schema)
	return err
}

// SaveHistory 保存下载历史
func (s *Storage) SaveHistory(task *Task) error {
	url := ""
	if len(task.Files) > 0 && len(task.Files[0].URIs) > 0 {
		url = task.Files[0].URIs[0].URI
	}

	duration := int64(0)
	avgSpeed := int64(0)
	if !task.CreatedAt.IsZero() {
		duration = int64(time.Since(task.CreatedAt).Seconds())
		if duration > 0 {
			avgSpeed = task.TotalLength / duration
		}
	}

	_, err := s.db.Exec(`
		INSERT INTO download_history 
		(gid, name, url, size, save_path, status, error_message, created_at, completed_at, duration, avg_speed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.GID, task.Name, url, task.TotalLength, task.Dir, task.Status,
		task.ErrorMessage, task.CreatedAt, time.Now(), duration, avgSpeed)
	return err
}

// UpdateHistoryStatus 更新历史记录状态
func (s *Storage) UpdateHistoryStatus(gid, status, errorMsg string) error {
	_, err := s.db.Exec(`
		UPDATE download_history 
		SET status = ?, error_message = ?, completed_at = ?
		WHERE gid = ? AND completed_at IS NULL
	`, status, errorMsg, time.Now(), gid)
	return err
}

// GetHistory 获取下载历史
func (s *Storage) GetHistory(limit, offset int, status string) ([]DownloadHistory, int, error) {
	var total int
	var countQuery string
	var args []interface{}

	if status != "" && status != "all" {
		countQuery = "SELECT COUNT(*) FROM download_history WHERE status = ?"
		args = append(args, status)
	} else {
		countQuery = "SELECT COUNT(*) FROM download_history"
	}
	s.db.QueryRow(countQuery, args...).Scan(&total)

	var query string
	var queryArgs []interface{}
	if status != "" && status != "all" {
		query = `
			SELECT id, gid, name, url, size, save_path, status, 
				   error_message, created_at, completed_at, duration, avg_speed
			FROM download_history 
			WHERE status = ?
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		queryArgs = []interface{}{status, limit, offset}
	} else {
		query = `
			SELECT id, gid, name, url, size, save_path, status, 
				   error_message, created_at, completed_at, duration, avg_speed
			FROM download_history 
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		queryArgs = []interface{}{limit, offset}
	}

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var history []DownloadHistory
	for rows.Next() {
		var h DownloadHistory
		var completedAt sql.NullTime
		var errorMsg, url, savePath sql.NullString
		err := rows.Scan(&h.ID, &h.GID, &h.Name, &url, &h.Size, &savePath,
			&h.Status, &errorMsg, &h.CreatedAt, &completedAt,
			&h.Duration, &h.AvgSpeed)
		if err != nil {
			continue
		}
		if completedAt.Valid {
			h.CompletedAt = &completedAt.Time
		}
		if errorMsg.Valid {
			h.ErrorMessage = errorMsg.String
		}
		if url.Valid {
			h.URL = url.String
		}
		if savePath.Valid {
			h.SavePath = savePath.String
		}
		history = append(history, h)
	}
	return history, total, nil
}

// DeleteHistory 删除单条历史记录
func (s *Storage) DeleteHistory(id int64) error {
	_, err := s.db.Exec("DELETE FROM download_history WHERE id = ?", id)
	return err
}

// ClearHistory 清空历史记录
func (s *Storage) ClearHistory() error {
	_, err := s.db.Exec("DELETE FROM download_history")
	return err
}

// SearchHistory 搜索历史记录
func (s *Storage) SearchHistory(keyword string, limit int) ([]DownloadHistory, error) {
	query := `
		SELECT id, gid, name, url, size, save_path, status, 
			   error_message, created_at, completed_at, duration, avg_speed
		FROM download_history 
		WHERE name LIKE ? OR url LIKE ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	pattern := "%" + keyword + "%"
	rows, err := s.db.Query(query, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []DownloadHistory
	for rows.Next() {
		var h DownloadHistory
		var completedAt sql.NullTime
		var errorMsg, url, savePath sql.NullString
		err := rows.Scan(&h.ID, &h.GID, &h.Name, &url, &h.Size, &savePath,
			&h.Status, &errorMsg, &h.CreatedAt, &completedAt,
			&h.Duration, &h.AvgSpeed)
		if err != nil {
			continue
		}
		if completedAt.Valid {
			h.CompletedAt = &completedAt.Time
		}
		if errorMsg.Valid {
			h.ErrorMessage = errorMsg.String
		}
		if url.Valid {
			h.URL = url.String
		}
		if savePath.Valid {
			h.SavePath = savePath.String
		}
		history = append(history, h)
	}
	return history, nil
}

// GetSettings 获取设置
func (s *Storage) GetSettings() (*UserSettings, error) {
	var settings UserSettings
	var enableDHT, notifyOnComplete, autoStart int

	err := s.db.QueryRow(`
		SELECT download_dir, max_concurrent, max_conn_per_server, split,
			   global_download_limit, global_upload_limit, seed_ratio, seed_time,
			   enable_dht, notify_on_complete, auto_start
		FROM download_settings WHERE id = 1
	`).Scan(
		&settings.DownloadDir, &settings.MaxConcurrent,
		&settings.MaxConnPerServer, &settings.Split,
		&settings.GlobalDownloadLimit, &settings.GlobalUploadLimit,
		&settings.SeedRatio, &settings.SeedTime,
		&enableDHT, &notifyOnComplete, &autoStart,
	)
	if err != nil {
		return &DefaultSettings, nil
	}

	settings.EnableDHT = enableDHT == 1
	settings.NotifyOnComplete = notifyOnComplete == 1
	settings.AutoStart = autoStart == 1

	return &settings, nil
}

// UpdateSettings 更新设置
func (s *Storage) UpdateSettings(settings *UserSettings) error {
	enableDHT := 0
	if settings.EnableDHT {
		enableDHT = 1
	}
	notifyOnComplete := 0
	if settings.NotifyOnComplete {
		notifyOnComplete = 1
	}
	autoStart := 0
	if settings.AutoStart {
		autoStart = 1
	}

	_, err := s.db.Exec(`
		UPDATE download_settings SET
			download_dir = ?,
			max_concurrent = ?,
			max_conn_per_server = ?,
			split = ?,
			global_download_limit = ?,
			global_upload_limit = ?,
			seed_ratio = ?,
			seed_time = ?,
			enable_dht = ?,
			notify_on_complete = ?,
			auto_start = ?
		WHERE id = 1
	`, settings.DownloadDir, settings.MaxConcurrent, settings.MaxConnPerServer,
		settings.Split, settings.GlobalDownloadLimit, settings.GlobalUploadLimit,
		settings.SeedRatio, settings.SeedTime,
		enableDHT, notifyOnComplete, autoStart)
	return err
}

// GetStatistics 获取下载统计
func (s *Storage) GetStatistics() (*DownloadStatistics, error) {
	stats := &DownloadStatistics{}

	// 总下载数和完成数
	s.db.QueryRow("SELECT COUNT(*) FROM download_history").Scan(&stats.TotalDownloads)
	s.db.QueryRow("SELECT COUNT(*) FROM download_history WHERE status = 'complete'").Scan(&stats.CompletedCount)
	s.db.QueryRow("SELECT COUNT(*) FROM download_history WHERE status = 'error'").Scan(&stats.FailedCount)

	// 总下载大小
	s.db.QueryRow("SELECT COALESCE(SUM(size), 0) FROM download_history WHERE status = 'complete'").Scan(&stats.TotalSize)

	// 今日统计
	today := time.Now().Format("2006-01-02")
	s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(size), 0) 
		FROM download_history 
		WHERE DATE(created_at) = ? AND status = 'complete'
	`, today).Scan(&stats.TodayDownloads, &stats.TodaySize)

	// 本周统计
	weekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(size), 0) 
		FROM download_history 
		WHERE DATE(created_at) >= ? AND status = 'complete'
	`, weekAgo).Scan(&stats.WeekDownloads, &stats.WeekSize)

	// 平均速度和最快下载
	s.db.QueryRow(`
		SELECT COALESCE(AVG(avg_speed), 0), COALESCE(MAX(avg_speed), 0) 
		FROM download_history 
		WHERE status = 'complete' AND avg_speed > 0
	`).Scan(&stats.AverageSpeed, &stats.FastestDownload)

	return stats, nil
}
