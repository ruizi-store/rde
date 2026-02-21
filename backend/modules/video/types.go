// Package video 提供视频流媒体服务
package video

// VideoInfo 视频信息
type VideoInfo struct {
	Path            string       `json:"path"`
	Name            string       `json:"name"`
	Size            int64        `json:"size"`
	Duration        float64      `json:"duration"`
	Width           int          `json:"width"`
	Height          int          `json:"height"`
	Codec           string       `json:"codec"`
	Bitrate         int64        `json:"bitrate"`
	FPS             float64      `json:"fps"`
	AudioTracks     []AudioTrack `json:"audio_tracks"`
	SubtitleTracks  []SubTrack   `json:"subtitle_tracks"`
	NeedsTranscode  bool         `json:"needs_transcode"`
	ThumbnailURL    string       `json:"thumbnail_url,omitempty"`
}

// AudioTrack 音轨信息
type AudioTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Codec    string `json:"codec"`
	Channels int    `json:"channels"`
	Title    string `json:"title,omitempty"`
}

// SubTrack 字幕轨道信息
type SubTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Title    string `json:"title,omitempty"`
	Embedded bool   `json:"embedded"`
	Path     string `json:"path,omitempty"` // 外部字幕文件路径
}

// SubtitleFile 字幕文件信息
type SubtitleFile struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Label    string `json:"label"`
	Embedded bool   `json:"embedded"`
	Index    int    `json:"index,omitempty"` // 内嵌字幕索引
}

// SubtitlesResponse 字幕检测响应
type SubtitlesResponse struct {
	Subtitles []SubtitleFile `json:"subtitles"`
}

// PlaybackHistory 播放历史
type PlaybackHistory struct {
	Path         string  `json:"path"`
	Position     float64 `json:"position"`
	Duration     float64 `json:"duration"`
	LastPlayed   string  `json:"last_played"`
	ThumbnailURL string  `json:"thumbnail_url,omitempty"`
}

// HistoryRequest 保存历史请求
type HistoryRequest struct {
	Path     string  `json:"path"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
}

// TranscodeSession HLS 转码会话
type TranscodeSession struct {
	ID        string `json:"id"`
	VideoPath string `json:"video_path"`
	OutputDir string `json:"output_dir"`
	StartTime int64  `json:"start_time"`
	Active    bool   `json:"active"`
}

// FFProbeOutput ffprobe JSON 输出结构
type FFProbeOutput struct {
	Streams []FFProbeStream `json:"streams"`
	Format  FFProbeFormat   `json:"format"`
}

// FFProbeStream 流信息
type FFProbeStream struct {
	Index         int    `json:"index"`
	CodecName     string `json:"codec_name"`
	CodecType     string `json:"codec_type"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	RFrameRate    string `json:"r_frame_rate,omitempty"`
	AvgFrameRate  string `json:"avg_frame_rate,omitempty"`
	Channels      int    `json:"channels,omitempty"`
	ChannelLayout string `json:"channel_layout,omitempty"`
	Tags          struct {
		Language string `json:"language,omitempty"`
		Title    string `json:"title,omitempty"`
	} `json:"tags,omitempty"`
}

// FFProbeFormat 格式信息
type FFProbeFormat struct {
	Filename       string `json:"filename"`
	FormatName     string `json:"format_name"`
	FormatLongName string `json:"format_long_name"`
	Duration       string `json:"duration"`
	Size           string `json:"size"`
	BitRate        string `json:"bit_rate"`
}

// 支持的视频格式
var VideoExtensions = map[string]bool{
	".mp4":  true,
	".webm": true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".m4v":  true,
	".ts":   true,
	".mts":  true,
	".ogv":  true,
	".3gp":  true,
}

// 浏览器原生支持的格式（不需要转码）
var NativeSupportedFormats = map[string]bool{
	".mp4":  true,
	".webm": true,
	".ogv":  true,
	".m4v":  true,
}

// 字幕文件扩展名
var SubtitleExtensions = map[string]bool{
	".srt": true,
	".vtt": true,
	".ass": true,
	".ssa": true,
	".sub": true,
}

// 视频 MIME 类型
var VideoMimeTypes = map[string]string{
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",
	".wmv":  "video/x-ms-wmv",
	".flv":  "video/x-flv",
	".m4v":  "video/x-m4v",
	".ts":   "video/mp2t",
	".mts":  "video/mp2t",
	".ogv":  "video/ogg",
	".3gp":  "video/3gpp",
}
