package photos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// AIService AI 服务集成
type AIService struct {
	baseURL    string
	httpClient *http.Client
	enabled    bool
}

// NewAIService 创建 AI 服务实例
func NewAIService(enabled bool, baseURL string) *AIService {
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}
	
	return &AIService{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		enabled: enabled,
	}
}

// IsEnabled 检查 AI 服务是否启用
func (s *AIService) IsEnabled() bool {
	return s.enabled
}

// AISearchResult AI 搜索结果
type AISearchResult struct {
	Path      string            `json:"path"`
	Type      string            `json:"type"`
	Score     float64           `json:"score"`
	Thumbnail string            `json:"thumbnail,omitempty"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
}

// AISearchResponse AI 搜索响应
type AISearchResponse struct {
	Total int               `json:"total"`
	Items []AISearchResult  `json:"items"`
	User  string            `json:"user,omitempty"`
}

// SemanticSearch 语义搜索
func (s *AIService) SemanticSearch(userID, query string, limit int) (*AISearchResponse, error) {
	if !s.enabled {
		return nil, fmt.Errorf("AI service not enabled")
	}
	
	params := url.Values{}
	params.Set("q", query)
	params.Set("type", "image")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("user", userID)
	
	resp, err := s.httpClient.Get(fmt.Sprintf("%s/api/search?%s", s.baseURL, params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to call indexer: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("indexer returned error: %s", string(body))
	}
	
	var result AISearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &result, nil
}

// FaceSearchParams 人脸搜索参数
type FaceSearchParams struct {
	AgeMin *int    `json:"age_min,omitempty"`
	AgeMax *int    `json:"age_max,omitempty"`
	Gender string  `json:"gender,omitempty"` // male, female
	Limit  int     `json:"limit"`
}

// SearchByFace 人脸属性搜索
func (s *AIService) SearchByFace(userID string, params FaceSearchParams) (*AISearchResponse, error) {
	if !s.enabled {
		return nil, fmt.Errorf("AI service not enabled")
	}
	
	queryParams := url.Values{}
	queryParams.Set("user", userID)
	
	if params.AgeMin != nil {
		queryParams.Set("age_min", strconv.Itoa(*params.AgeMin))
	}
	if params.AgeMax != nil {
		queryParams.Set("age_max", strconv.Itoa(*params.AgeMax))
	}
	if params.Gender != "" {
		queryParams.Set("gender", params.Gender)
	}
	if params.Limit > 0 {
		queryParams.Set("limit", strconv.Itoa(params.Limit))
	} else {
		queryParams.Set("limit", "50")
	}
	
	resp, err := s.httpClient.Get(fmt.Sprintf("%s/api/search/face?%s", s.baseURL, queryParams.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to call indexer: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("indexer returned error: %s", string(body))
	}
	
	var result AISearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &result, nil
}

// SearchByText OCR 文字搜索
func (s *AIService) SearchByText(userID, text string, limit int) (*AISearchResponse, error) {
	if !s.enabled {
		return nil, fmt.Errorf("AI service not enabled")
	}
	
	params := url.Values{}
	params.Set("q", text)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("user", userID)
	
	resp, err := s.httpClient.Get(fmt.Sprintf("%s/api/search/text?%s", s.baseURL, params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to call indexer: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("indexer returned error: %s", string(body))
	}
	
	var result AISearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &result, nil
}

// GetIndexerStatus 获取索引服务状态
func (s *AIService) GetIndexerStatus(userID string) (map[string]any, error) {
	if !s.enabled {
		return nil, fmt.Errorf("AI service not enabled")
	}
	
	params := url.Values{}
	params.Set("user", userID)
	
	resp, err := s.httpClient.Get(fmt.Sprintf("%s/api/status?%s", s.baseURL, params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to call indexer: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("indexer returned error: %s", string(body))
	}
	
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// TriggerIndexing 触发索引任务
func (s *AIService) TriggerIndexing(userID, directory string) error {
	if !s.enabled {
		return fmt.Errorf("AI service not enabled")
	}
	
	params := url.Values{}
	params.Set("user", userID)
	if directory != "" {
		params.Set("dir", directory)
	}
	
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/scan?%s", s.baseURL, params.Encode()), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call indexer: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("indexer returned error: %s", string(body))
	}
	
	return nil
}

// ConvertToPhotos 将 AI 搜索结果转换为 Photo 对象
func (s *AIService) ConvertToPhotos(results *AISearchResponse, service *Service) ([]*Photo, error) {
	if results == nil {
		return nil, nil
	}
	
	photos := make([]*Photo, 0, len(results.Items))
	
	for _, item := range results.Items {
		// 通过路径查找照片
		photo, err := service.FindPhotoByPath(item.Path)
		if err != nil || photo == nil {
			// 如果照片不在数据库中，尝试用搜索结果构建临时对象
			photo = &Photo{
				Path:      item.Path,
				Filename:  filepath.Base(item.Path),
				MimeType:  getMimeTypeFromPath(item.Path),
			}
		}
		
		// 添加 AI 相关元数据
		if item.Metadata != nil {
			if age, ok := item.Metadata["age"].(float64); ok {
				ageInt := int(age)
				photo.AIAge = &ageInt
			}
			if gender, ok := item.Metadata["gender"].(string); ok {
				photo.AIGender = &gender
			}
		}
		photo.AIScore = item.Score
		
		photos = append(photos, photo)
	}
	
	return photos, nil
}

// getMimeTypeFromPath 从路径推断 MIME 类型
func getMimeTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".heic", ".heif":
		return "image/heic"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	default:
		return "image/jpeg"
	}
}
