package files

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/gin-gonic/gin"
)

// AudioMetadataResponse 音频元数据响应
type AudioMetadataResponse struct {
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"album_artist,omitempty"`
	Composer    string `json:"composer,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Year        int    `json:"year,omitempty"`
	Track       int    `json:"track,omitempty"`
	TrackTotal  int    `json:"track_total,omitempty"`
	Disc        int    `json:"disc,omitempty"`
	DiscTotal   int    `json:"disc_total,omitempty"`
	Duration    int    `json:"duration,omitempty"` // 秒
	Lyrics      string `json:"lyrics,omitempty"`   // 内嵌歌词
	Format      string `json:"format,omitempty"`   // MP3, FLAC, etc.
	FileType    string `json:"file_type,omitempty"`
	HasPicture  bool   `json:"has_picture"`
}

// GetAudioMetadata 获取音频文件元数据
// @Summary 获取音频元数据
// @Tags files
// @Accept json
// @Produce json
// @Param path query string true "文件路径"
// @Success 200 {object} AudioMetadataResponse
// @Router /files/audio/metadata [get]
func (h *Handler) GetAudioMetadata(c *gin.Context) {
	virtualPath := c.Query("path")
	if virtualPath == "" {
		fail(c, 400, "path is required")
		return
	}

	// 解析虚拟路径
	realPath := resolvePathWithContext(c, virtualPath)
	if realPath == "" {
		fail(c, 400, "invalid path")
		return
	}

	// 权限检查
	if !h.checkReadPermission(c, realPath) {
		return
	}

	// 检查文件是否存在
	info, err := os.Stat(realPath)
	if err != nil {
		fail(c, 404, "file not found")
		return
	}
	if info.IsDir() {
		fail(c, 400, "path is a directory")
		return
	}

	// 检查是否为音频文件
	ext := strings.ToLower(filepath.Ext(realPath))
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".m4a": true, ".aac": true,
		".ogg": true, ".wav": true, ".wma": true, ".opus": true,
	}
	if !audioExts[ext] {
		fail(c, 400, "not an audio file")
		return
	}

	// 打开文件
	file, err := os.Open(realPath)
	if err != nil {
		fail(c, 500, "failed to open file")
		return
	}
	defer file.Close()

	// 读取元数据
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		// 即使元数据读取失败，也返回基本信息
		ok(c, AudioMetadataResponse{
			Title:    strings.TrimSuffix(filepath.Base(realPath), ext),
			FileType: ext[1:],
		})
		return
	}

	// 构建响应
	resp := AudioMetadataResponse{
		Title:       metadata.Title(),
		Artist:      metadata.Artist(),
		Album:       metadata.Album(),
		AlbumArtist: metadata.AlbumArtist(),
		Composer:    metadata.Composer(),
		Genre:       metadata.Genre(),
		Year:        metadata.Year(),
		Format:      string(metadata.Format()),
		FileType:    string(metadata.FileType()),
		HasPicture:  metadata.Picture() != nil,
	}

	// 获取音轨信息
	track, trackTotal := metadata.Track()
	resp.Track = track
	resp.TrackTotal = trackTotal

	disc, discTotal := metadata.Disc()
	resp.Disc = disc
	resp.DiscTotal = discTotal

	// 尝试获取歌词（从 Raw 标签中）
	resp.Lyrics = extractLyrics(metadata)

	// 如果标题为空，使用文件名
	if resp.Title == "" {
		resp.Title = strings.TrimSuffix(filepath.Base(realPath), ext)
	}

	ok(c, resp)
}

// extractLyrics 从元数据中提取歌词
func extractLyrics(metadata tag.Metadata) string {
	raw := metadata.Raw()
	if raw == nil {
		return ""
	}

	// ID3v2 标签中的歌词字段
	// USLT: Unsynchronized lyrics
	// SYLT: Synchronized lyrics
	lyricsKeys := []string{
		"USLT", "SYLT", "lyrics", "LYRICS",
		"unsyncedlyrics", "UNSYNCEDLYRICS",
		"©lyr", // iTunes
	}

	for _, key := range lyricsKeys {
		if val, ok := raw[key]; ok {
			switch v := val.(type) {
			case string:
				if v != "" {
					return v
				}
			case []byte:
				if len(v) > 0 {
					return string(v)
				}
			case *tag.Comm:
				if v != nil && v.Text != "" {
					return v.Text
				}
			}
		}
	}

	return ""
}

// GetAudioLyrics 获取音频歌词
// @Summary 获取音频歌词
// @Tags files
// @Accept json
// @Produce json
// @Param path query string true "文件路径"
// @Success 200 {object} map[string]interface{}
// @Router /files/audio/lyrics [get]
func (h *Handler) GetAudioLyrics(c *gin.Context) {
	virtualPath := c.Query("path")
	if virtualPath == "" {
		fail(c, 400, "path is required")
		return
	}

	// 解析虚拟路径
	realPath := resolvePathWithContext(c, virtualPath)
	if realPath == "" {
		fail(c, 400, "invalid path")
		return
	}

	// 权限检查
	if !h.checkReadPermission(c, realPath) {
		return
	}

	// 检查文件是否存在
	info, err := os.Stat(realPath)
	if err != nil {
		fail(c, 404, "file not found")
		return
	}
	if info.IsDir() {
		fail(c, 400, "path is a directory")
		return
	}

	// 1. 尝试从音频文件读取内嵌歌词
	embeddedLyrics := ""
	file, err := os.Open(realPath)
	if err == nil {
		defer file.Close()
		if metadata, err := tag.ReadFrom(file); err == nil {
			embeddedLyrics = extractLyrics(metadata)
		}
	}

	if embeddedLyrics != "" {
		ok(c, gin.H{
			"source": "embedded",
			"lyrics": embeddedLyrics,
			"format": detectLyricsFormat(embeddedLyrics),
		})
		return
	}

	// 2. 尝试读取同名 .lrc 文件
	ext := filepath.Ext(realPath)
	lrcPath := strings.TrimSuffix(realPath, ext) + ".lrc"
	if lrcContent, err := os.ReadFile(lrcPath); err == nil && len(lrcContent) > 0 {
		ok(c, gin.H{
			"source": "lrc_file",
			"lyrics": string(lrcContent),
			"format": "lrc",
		})
		return
	}

	// 3. 尝试在线搜索歌词
	// 从文件名提取歌曲信息
	baseName := strings.TrimSuffix(filepath.Base(realPath), ext)
	onlineLyrics := searchOnlineLyrics(baseName)
	if onlineLyrics != "" {
		ok(c, gin.H{
			"source": "online",
			"lyrics": onlineLyrics,
			"format": detectLyricsFormat(onlineLyrics),
		})
		return
	}

	// 4. 没有找到歌词
	ok(c, gin.H{
		"source": "none",
		"lyrics": "",
		"format": "",
	})
}

// searchOnlineLyrics 在线搜索歌词
func searchOnlineLyrics(query string) string {
	// 使用 lrclib.net API（免费公开的歌词API）
	client := &http.Client{Timeout: 5 * time.Second}
	
	// 搜索歌曲
	searchURL := fmt.Sprintf("https://lrclib.net/api/search?q=%s", url.QueryEscape(query))
	resp, err := client.Get(searchURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return ""
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	
	// 解析JSON
	var results []struct {
		SyncedLyrics string `json:"syncedLyrics"`
		PlainLyrics  string `json:"plainLyrics"`
	}
	
	if err := json.Unmarshal(body, &results); err != nil {
		return ""
	}
	
	if len(results) > 0 {
		// 优先返回同步歌词
		if results[0].SyncedLyrics != "" {
			return results[0].SyncedLyrics
		}
		return results[0].PlainLyrics
	}
	
	return ""
}

// detectLyricsFormat 检测歌词格式
func detectLyricsFormat(lyrics string) string {
	// LRC 格式通常以时间戳开头 [00:00.00]
	if strings.Contains(lyrics, "[") && strings.Contains(lyrics, "]") {
		// 检查是否包含时间戳格式
		lines := strings.Split(lyrics, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) > 0 && line[0] == '[' {
				// 检查是否匹配时间戳模式
				if strings.Contains(line, ":") && strings.Contains(line, ".") {
					return "lrc"
				}
			}
		}
	}
	return "plain"
}
