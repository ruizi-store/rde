package video

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ServiceConfig 服务配置
type ServiceConfig struct {
	CacheDir           string
	ThumbnailDir       string
	HLSSegmentDuration int
	MaxConcurrentJobs  int
}

// Service 视频服务
type Service struct {
	logger *zap.Logger
	config ServiceConfig

	// 转码会话管理
	sessions     map[string]*TranscodeSession
	sessionMutex sync.RWMutex

	// 并发控制
	jobSemaphore chan struct{}
}

// NewService 创建视频服务
func NewService(logger *zap.Logger, config ServiceConfig) *Service {
	// 确保目录存在
	os.MkdirAll(config.CacheDir, 0755)
	os.MkdirAll(config.ThumbnailDir, 0755)

	return &Service{
		logger:       logger,
		config:       config,
		sessions:     make(map[string]*TranscodeSession),
		jobSemaphore: make(chan struct{}, config.MaxConcurrentJobs),
	}
}

// GetVideoInfo 获取视频信息
func (s *Service) GetVideoInfo(videoPath string) (*VideoInfo, error) {
	// 使用 ffprobe 获取信息
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoPath)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe FFProbeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	info := &VideoInfo{
		Path: videoPath,
		Name: filepath.Base(videoPath),
	}

	// 解析格式信息
	if probe.Format.Duration != "" {
		info.Duration, _ = strconv.ParseFloat(probe.Format.Duration, 64)
	}
	if probe.Format.Size != "" {
		info.Size, _ = strconv.ParseInt(probe.Format.Size, 10, 64)
	}
	if probe.Format.BitRate != "" {
		info.Bitrate, _ = strconv.ParseInt(probe.Format.BitRate, 10, 64)
	}

	// 解析流信息
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			if info.Codec == "" {
				info.Codec = stream.CodecName
				info.Width = stream.Width
				info.Height = stream.Height
				// 解析帧率
				if stream.RFrameRate != "" {
					parts := strings.Split(stream.RFrameRate, "/")
					if len(parts) == 2 {
						num, _ := strconv.ParseFloat(parts[0], 64)
						den, _ := strconv.ParseFloat(parts[1], 64)
						if den > 0 {
							info.FPS = num / den
						}
					}
				}
			}
		case "audio":
			info.AudioTracks = append(info.AudioTracks, AudioTrack{
				Index:    stream.Index,
				Language: stream.Tags.Language,
				Codec:    stream.CodecName,
				Channels: stream.Channels,
				Title:    stream.Tags.Title,
			})
		case "subtitle":
			info.SubtitleTracks = append(info.SubtitleTracks, SubTrack{
				Index:    stream.Index,
				Language: stream.Tags.Language,
				Title:    stream.Tags.Title,
				Embedded: true,
			})
		}
	}

	// 判断是否需要转码
	ext := strings.ToLower(filepath.Ext(videoPath))
	info.NeedsTranscode = !NativeSupportedFormats[ext]
	// H.265/HEVC 也需要转码
	if strings.Contains(strings.ToLower(info.Codec), "hevc") ||
		strings.Contains(strings.ToLower(info.Codec), "h265") ||
		strings.Contains(strings.ToLower(info.Codec), "265") {
		info.NeedsTranscode = true
	}

	return info, nil
}

// GetSubtitles 检测可用字幕
func (s *Service) GetSubtitles(videoPath string) (*SubtitlesResponse, error) {
	result := &SubtitlesResponse{
		Subtitles: []SubtitleFile{},
	}

	// 1. 获取视频目录和基础文件名
	dir := filepath.Dir(videoPath)
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	// 2. 扫描同目录下的字幕文件
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result, nil // 目录读取失败不影响返回
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !SubtitleExtensions[ext] {
			continue
		}

		// 检查是否是该视频的字幕（文件名前缀匹配）
		nameWithoutExt := strings.TrimSuffix(name, ext)
		if !strings.HasPrefix(nameWithoutExt, baseName) {
			continue
		}

		// 尝试解析语言标签
		suffix := strings.TrimPrefix(nameWithoutExt, baseName)
		suffix = strings.TrimPrefix(suffix, ".")
		language := detectLanguage(suffix)
		label := formatSubtitleLabel(name, language)

		result.Subtitles = append(result.Subtitles, SubtitleFile{
			Path:     filepath.Join(dir, name),
			Language: language,
			Label:    label,
			Embedded: false,
		})
	}

	// 3. 获取内嵌字幕
	info, err := s.GetVideoInfo(videoPath)
	if err == nil {
		for _, sub := range info.SubtitleTracks {
			label := "内嵌"
			if sub.Title != "" {
				label = sub.Title
			} else if sub.Language != "" {
				label = formatLanguageName(sub.Language)
			}
			result.Subtitles = append(result.Subtitles, SubtitleFile{
				Path:     videoPath,
				Language: sub.Language,
				Label:    label,
				Embedded: true,
				Index:    sub.Index,
			})
		}
	}

	return result, nil
}

// GenerateThumbnail 生成视频缩略图
func (s *Service) GenerateThumbnail(videoPath string, timestamp float64) (string, error) {
	// 生成缓存文件路径
	hash := hashPath(videoPath)
	thumbPath := filepath.Join(s.config.ThumbnailDir, hash+".jpg")

	// 检查是否已存在
	if _, err := os.Stat(thumbPath); err == nil {
		return thumbPath, nil
	}

	// 如果没有指定时间戳，取视频 10% 位置
	if timestamp <= 0 {
		info, err := s.GetVideoInfo(videoPath)
		if err == nil && info.Duration > 0 {
			timestamp = info.Duration * 0.1
		} else {
			timestamp = 10 // 默认 10 秒
		}
	}

	// 使用 ffmpeg 生成缩略图
	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", videoPath,
		"-vframes", "1",
		"-vf", "scale=320:-1",
		"-y",
		thumbPath)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("generate thumbnail: %w", err)
	}

	return thumbPath, nil
}

// StartHLSTranscode 启动 HLS 转码
func (s *Service) StartHLSTranscode(videoPath string, startTime float64) (string, error) {
	// 检查是否已有该视频的转码会话
	sessionID := hashPath(videoPath)

	s.sessionMutex.Lock()
	if session, ok := s.sessions[sessionID]; ok && session.Active {
		s.sessionMutex.Unlock()
		return sessionID, nil
	}

	// 创建输出目录
	outputDir := filepath.Join(s.config.CacheDir, sessionID)
	os.MkdirAll(outputDir, 0755)

	session := &TranscodeSession{
		ID:        sessionID,
		VideoPath: videoPath,
		OutputDir: outputDir,
		StartTime: time.Now().Unix(),
		Active:    true,
	}
	s.sessions[sessionID] = session
	s.sessionMutex.Unlock()

	// 异步启动转码
	go func() {
		// 获取并发槽位
		s.jobSemaphore <- struct{}{}
		defer func() { <-s.jobSemaphore }()

		s.runTranscode(session, startTime)
	}()

	return sessionID, nil
}

// runTranscode 执行转码
func (s *Service) runTranscode(session *TranscodeSession, startTime float64) {
	playlistPath := filepath.Join(session.OutputDir, "playlist.m3u8")
	segmentPattern := filepath.Join(session.OutputDir, "segment_%05d.ts")

	args := []string{
		"-i", session.VideoPath,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-f", "hls",
		"-hls_time", strconv.Itoa(s.config.HLSSegmentDuration),
		"-hls_list_size", "0",
		"-hls_flags", "delete_segments+append_list",
		"-hls_segment_filename", segmentPattern,
		"-y",
		playlistPath,
	}

	// 如果有起始时间，添加 seek
	if startTime > 0 {
		args = append([]string{"-ss", fmt.Sprintf("%.2f", startTime)}, args...)
	}

	cmd := exec.Command("ffmpeg", args...)

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()
	cmd = exec.CommandContext(ctx, "ffmpeg", args...)

	s.logger.Info("starting HLS transcode",
		zap.String("session_id", session.ID),
		zap.String("video", session.VideoPath))

	if err := cmd.Run(); err != nil {
		s.logger.Error("HLS transcode failed",
			zap.String("session_id", session.ID),
			zap.Error(err))
	}

	s.sessionMutex.Lock()
	if sess, ok := s.sessions[session.ID]; ok {
		sess.Active = false
	}
	s.sessionMutex.Unlock()
}

// GetHLSPlaylist 获取 HLS 播放列表路径
func (s *Service) GetHLSPlaylist(sessionID string) (string, error) {
	s.sessionMutex.RLock()
	session, ok := s.sessions[sessionID]
	s.sessionMutex.RUnlock()

	if !ok {
		return "", fmt.Errorf("session not found")
	}

	playlistPath := filepath.Join(session.OutputDir, "playlist.m3u8")

	// 等待播放列表生成（最多等 30 秒）
	for i := 0; i < 60; i++ {
		if _, err := os.Stat(playlistPath); err == nil {
			return playlistPath, nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return "", fmt.Errorf("playlist not ready")
}

// GetHLSSegment 获取 HLS 分片路径
func (s *Service) GetHLSSegment(sessionID, segment string) (string, error) {
	s.sessionMutex.RLock()
	session, ok := s.sessions[sessionID]
	s.sessionMutex.RUnlock()

	if !ok {
		return "", fmt.Errorf("session not found")
	}

	segmentPath := filepath.Join(session.OutputDir, segment)
	if _, err := os.Stat(segmentPath); err != nil {
		return "", fmt.Errorf("segment not found")
	}

	return segmentPath, nil
}

// StopTranscode 停止转码会话
func (s *Service) StopTranscode(sessionID string) {
	s.sessionMutex.Lock()
	if session, ok := s.sessions[sessionID]; ok {
		session.Active = false
		// 清理输出目录
		os.RemoveAll(session.OutputDir)
		delete(s.sessions, sessionID)
	}
	s.sessionMutex.Unlock()
}

// StopAllTranscodes 停止所有转码
func (s *Service) StopAllTranscodes() {
	s.sessionMutex.Lock()
	for id, session := range s.sessions {
		session.Active = false
		os.RemoveAll(session.OutputDir)
		delete(s.sessions, id)
	}
	s.sessionMutex.Unlock()
}

// CleanupCache 清理过期缓存
func (s *Service) CleanupCache() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupOldCaches()
	}
}

func (s *Service) cleanupOldCaches() {
	maxAge := time.Hour * 2 // 2 小时过期

	entries, err := os.ReadDir(s.config.CacheDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(s.config.CacheDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 检查是否有活跃会话
		s.sessionMutex.RLock()
		_, active := s.sessions[entry.Name()]
		s.sessionMutex.RUnlock()

		if !active && time.Since(info.ModTime()) > maxAge {
			os.RemoveAll(path)
			s.logger.Debug("cleaned up cache", zap.String("path", path))
		}
	}
}

// ConvertSubtitleToVTT 将字幕转换为 VTT 格式
func (s *Service) ConvertSubtitleToVTT(subtitlePath string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(subtitlePath))

	// 如果已经是 VTT，直接返回
	if ext == ".vtt" {
		return os.ReadFile(subtitlePath)
	}

	// 使用 ffmpeg 转换
	cmd := exec.Command("ffmpeg",
		"-i", subtitlePath,
		"-f", "webvtt",
		"-")

	return cmd.Output()
}

// ExtractEmbeddedSubtitle 提取内嵌字幕
func (s *Service) ExtractEmbeddedSubtitle(videoPath string, index int) ([]byte, error) {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-map", fmt.Sprintf("0:%d", index),
		"-f", "webvtt",
		"-")

	return cmd.Output()
}

// 辅助函数

func hashPath(path string) string {
	// 简单的路径哈希
	h := uint64(0)
	for _, c := range path {
		h = h*31 + uint64(c)
	}
	return fmt.Sprintf("%x", h)
}

func detectLanguage(suffix string) string {
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	languageMap := map[string]string{
		"zh":      "zh",
		"chs":     "zh",
		"cht":     "zh",
		"chinese": "zh",
		"中文":      "zh",
		"简体":      "zh",
		"繁体":      "zh",
		"en":      "en",
		"eng":     "en",
		"english": "en",
		"英文":      "en",
		"jp":      "ja",
		"jpn":     "ja",
		"日文":      "ja",
		"kr":      "ko",
		"kor":     "ko",
		"韩文":      "ko",
	}
	if lang, ok := languageMap[suffix]; ok {
		return lang
	}
	return suffix
}

func formatLanguageName(code string) string {
	names := map[string]string{
		"zh":  "中文",
		"en":  "English",
		"ja":  "日本語",
		"ko":  "한국어",
		"chi": "中文",
		"eng": "English",
		"jpn": "日本語",
		"kor": "한국어",
	}
	if name, ok := names[strings.ToLower(code)]; ok {
		return name
	}
	return code
}

func formatSubtitleLabel(filename, language string) string {
	if language != "" {
		return formatLanguageName(language) + " (" + filename + ")"
	}
	return filename
}
