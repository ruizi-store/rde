// Package download 下载类型定义
package download

import "time"

// Task 下载任务
type Task struct {
	GID           string    `json:"gid"`
	Status        string    `json:"status"` // active, waiting, paused, error, complete, removed
	TotalLength   int64     `json:"total_length"`
	CompletedLen  int64     `json:"completed_length"`
	DownloadSpeed int64     `json:"download_speed"`
	UploadSpeed   int64     `json:"upload_speed,omitempty"`
	Connections   int       `json:"connections"`
	Dir           string    `json:"dir"`
	Files         []File    `json:"files,omitempty"`
	BitTorrent    *BTInfo   `json:"bittorrent,omitempty"`
	ErrorCode     string    `json:"error_code,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	Progress      float64   `json:"progress"`
	Name          string    `json:"name"`
	CreatedAt     time.Time `json:"created_at"`
	FinishedAt    time.Time `json:"finished_at,omitempty"`
}

// File 下载文件
type File struct {
	Index           string `json:"index"`
	Path            string `json:"path"`
	Length          int64  `json:"length"`
	CompletedLength int64  `json:"completed_length"`
	Selected        bool   `json:"selected"`
	URIs            []URI  `json:"uris,omitempty"`
}

// URI 文件 URI
type URI struct {
	URI    string `json:"uri"`
	Status string `json:"status"`
}

// BTInfo BT 信息
type BTInfo struct {
	AnnounceList [][]string `json:"announce_list,omitempty"`
	Comment      string     `json:"comment,omitempty"`
	CreationDate int64      `json:"creation_date,omitempty"`
	Mode         string     `json:"mode,omitempty"`
	Name         string     `json:"name,omitempty"`
}

// GlobalStat 全局统计
type GlobalStat struct {
	DownloadSpeed   int64 `json:"download_speed"`
	UploadSpeed     int64 `json:"upload_speed"`
	NumActive       int   `json:"num_active"`
	NumWaiting      int   `json:"num_waiting"`
	NumStopped      int   `json:"num_stopped"`
	NumStoppedTotal int   `json:"num_stopped_total"`
}

// Options 下载选项
type Options struct {
	Dir               string `json:"dir,omitempty"`
	Out               string `json:"out,omitempty"`
	Split             int    `json:"split,omitempty"`
	MaxConnPerServer  int    `json:"max-connection-per-server,omitempty"`
	MinSplitSize      string `json:"min-split-size,omitempty"`
	MaxDownloadLimit  string `json:"max-download-limit,omitempty"`
	ContinueDownload  bool   `json:"continue,omitempty"`
	AllowOverwrite    bool   `json:"allow-overwrite,omitempty"`
	AutoFileRenaming  bool   `json:"auto-file-renaming,omitempty"`
	Header            string `json:"header,omitempty"`
	Referer           string `json:"referer,omitempty"`
	UserAgent         string `json:"user-agent,omitempty"`
	SeedRatio         string `json:"seed-ratio,omitempty"`
	SeedTime          string `json:"seed-time,omitempty"`
	SelectFile        string `json:"select-file,omitempty"`
	Proxy             string `json:"all-proxy,omitempty"`
	ProxyUser         string `json:"all-proxy-user,omitempty"`
	ProxyPassword     string `json:"all-proxy-passwd,omitempty"`
	CheckCertificate  bool   `json:"check-certificate,omitempty"`
	MaxRetries        int    `json:"max-tries,omitempty"`
	RetryWait         int    `json:"retry-wait,omitempty"`
	Timeout           int    `json:"timeout,omitempty"`
	ConnectTimeout    int    `json:"connect-timeout,omitempty"`
}

// AddURIRequest 添加 URI 请求
type AddURIRequest struct {
	URIs    []string `json:"uris" binding:"required"`
	Dir     string   `json:"dir,omitempty"`
	Out     string   `json:"out,omitempty"`
	Options *Options `json:"options,omitempty"`
}

// AddTorrentRequest 添加种子请求
type AddTorrentRequest struct {
	Torrent string   `json:"torrent" binding:"required"` // base64 encoded
	Dir     string   `json:"dir,omitempty"`
	Options *Options `json:"options,omitempty"`
}

// AddMetalinkRequest 添加 Metalink 请求
type AddMetalinkRequest struct {
	Metalink string   `json:"metalink" binding:"required"` // base64 encoded
	Dir      string   `json:"dir,omitempty"`
	Options  *Options `json:"options,omitempty"`
}

// BatchActionRequest 批量操作请求
type BatchActionRequest struct {
	GIDs []string `json:"gids" binding:"required"`
}

// UpdateOptionsRequest 更新选项请求
type UpdateOptionsRequest struct {
	MaxConcurrentDownloads int    `json:"max_concurrent_downloads,omitempty"`
	MaxDownloadSpeed       string `json:"max_download_speed,omitempty"`
	MaxUploadSpeed         string `json:"max_upload_speed,omitempty"`
	DefaultDir             string `json:"default_dir,omitempty"`
}

// TaskListResponse 任务列表响应
type TaskListResponse struct {
	Active   []Task `json:"active"`
	Waiting  []Task `json:"waiting"`
	Stopped  []Task `json:"stopped"`
	Stats    *GlobalStat `json:"stats"`
}

// Aria2Response aria2 RPC 响应
type Aria2Response struct {
	ID      string      `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Aria2Error `json:"error,omitempty"`
}

// Aria2Error aria2 错误
type Aria2Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Aria2TaskStatus aria2 任务状态原始数据
type Aria2TaskStatus struct {
	GID             string     `json:"gid"`
	Status          string     `json:"status"`
	TotalLength     string     `json:"totalLength"`
	CompletedLength string     `json:"completedLength"`
	DownloadSpeed   string     `json:"downloadSpeed"`
	UploadSpeed     string     `json:"uploadSpeed"`
	Connections     string     `json:"connections"`
	Dir             string     `json:"dir"`
	Files           []Aria2File `json:"files"`
	BitTorrent      *Aria2BT    `json:"bittorrent"`
	ErrorCode       string     `json:"errorCode"`
	ErrorMessage    string     `json:"errorMessage"`
}

// Aria2File aria2 文件信息
type Aria2File struct {
	Index           string     `json:"index"`
	Path            string     `json:"path"`
	Length          string     `json:"length"`
	CompletedLength string     `json:"completedLength"`
	Selected        string     `json:"selected"`
	URIs            []Aria2URI `json:"uris"`
}

// Aria2URI aria2 URI 信息
type Aria2URI struct {
	URI    string `json:"uri"`
	Status string `json:"status"`
}

// Aria2BT aria2 BT 信息
type Aria2BT struct {
	AnnounceList [][]string     `json:"announceList"`
	Comment      string         `json:"comment"`
	CreationDate int64          `json:"creationDate"`
	Mode         string         `json:"mode"`
	Info         *Aria2BTInfo   `json:"info"`
}

// Aria2BTInfo aria2 BT 信息详情
type Aria2BTInfo struct {
	Name string `json:"name"`
}

// Aria2GlobalStat aria2 全局统计
type Aria2GlobalStat struct {
	DownloadSpeed   string `json:"downloadSpeed"`
	UploadSpeed     string `json:"uploadSpeed"`
	NumActive       string `json:"numActive"`
	NumWaiting      string `json:"numWaiting"`
	NumStopped      string `json:"numStopped"`
	NumStoppedTotal string `json:"numStoppedTotal"`
}
