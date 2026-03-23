// Package translate 翻译服务
package translate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// DefaultLibreTranslateURL LibreTranslate 默认地址
	DefaultLibreTranslateURL = "http://localhost:5000"

	// 默认超时时间
	defaultTimeout = 30 * time.Second
)

// Service 翻译服务
type Service struct {
	logger     *zap.Logger
	client     *http.Client
	serviceURL string
	mu         sync.RWMutex

	// 缓存语言列表
	languages     []Language
	languagesTime time.Time
}

// NewService 创建翻译服务
func NewService(logger *zap.Logger) *Service {
	return &Service{
		logger:     logger,
		serviceURL: DefaultLibreTranslateURL,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// SetServiceURL 设置服务地址
func (s *Service) SetServiceURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serviceURL = url
}

// GetServiceURL 获取服务地址
func (s *Service) GetServiceURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serviceURL
}

// Translate 翻译文本
func (s *Service) Translate(req TranslateRequest) (*TranslateResponse, error) {
	s.mu.RLock()
	serviceURL := s.serviceURL
	s.mu.RUnlock()

	// 构建 LibreTranslate 请求
	libreReq := libreTranslateRequest{
		Q:      req.Text,
		Source: req.Source,
		Target: req.Target,
		Format: "text",
	}

	// 如果未指定源语言，使用自动检测
	if libreReq.Source == "" {
		libreReq.Source = "auto"
	}

	data, err := json.Marshal(libreReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求
	url := serviceURL + "/translate"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("translate failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var libreResp libreTranslateResponse
	if err := json.NewDecoder(resp.Body).Decode(&libreResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := &TranslateResponse{
		TranslatedText: libreResp.TranslatedText,
	}

	// 如果有检测到的语言信息
	if libreResp.DetectedLanguage != nil {
		result.DetectedLang = libreResp.DetectedLanguage.Language
	}

	return result, nil
}

// DetectLanguage 检测语言
func (s *Service) DetectLanguage(text string) (*DetectResponse, error) {
	s.mu.RLock()
	serviceURL := s.serviceURL
	s.mu.RUnlock()

	reqBody := map[string]string{"q": text}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := serviceURL + "/detect"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("detect failed with status %d: %s", resp.StatusCode, string(body))
	}

	var libreResp libreDetectResponse
	if err := json.NewDecoder(resp.Body).Decode(&libreResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(libreResp) == 0 {
		return nil, fmt.Errorf("no language detected")
	}

	return &DetectResponse{
		Language:   libreResp[0].Language,
		Confidence: libreResp[0].Confidence,
	}, nil
}

// GetLanguages 获取支持的语言列表
func (s *Service) GetLanguages() ([]Language, error) {
	s.mu.RLock()
	// 如果缓存有效（5分钟内），直接返回
	if len(s.languages) > 0 && time.Since(s.languagesTime) < 5*time.Minute {
		languages := s.languages
		s.mu.RUnlock()
		return languages, nil
	}
	serviceURL := s.serviceURL
	s.mu.RUnlock()

	url := serviceURL + "/languages"
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get languages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get languages failed with status %d: %s", resp.StatusCode, string(body))
	}

	var libreLanguages []libreLanguage
	if err := json.NewDecoder(resp.Body).Decode(&libreLanguages); err != nil {
		return nil, fmt.Errorf("failed to decode languages: %w", err)
	}

	languages := make([]Language, len(libreLanguages))
	for i, lang := range libreLanguages {
		languages[i] = Language{
			Code: lang.Code,
			Name: lang.Name,
		}
	}

	// 更新缓存
	s.mu.Lock()
	s.languages = languages
	s.languagesTime = time.Now()
	s.mu.Unlock()

	return languages, nil
}

// CheckStatus 检查服务状态
func (s *Service) CheckStatus() *ServiceStatus {
	s.mu.RLock()
	serviceURL := s.serviceURL
	s.mu.RUnlock()

	status := &ServiceStatus{
		Available: false,
		URL:       serviceURL,
	}

	// 尝试获取语言列表来验证服务
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(serviceURL + "/languages")
	if err != nil {
		status.Message = fmt.Sprintf("无法连接到翻译服务: %v", err)
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		status.Message = fmt.Sprintf("翻译服务返回错误: %d", resp.StatusCode)
		return status
	}

	status.Available = true
	status.Message = "翻译服务运行正常"
	return status
}

// GetConfig 获取翻译配置（根据系统语言设置默认值）
func (s *Service) GetConfig(systemLang string) *TranslateConfig {
	config := &TranslateConfig{
		ServiceURL: s.GetServiceURL(),
	}

	// 根据系统语言设置默认翻译方向
	// 中文系统：中文 -> 英文
	// 英文系统：英文 -> 中文
	if systemLang == "zh-CN" || systemLang == "zh" {
		config.DefaultSource = "zh"
		config.DefaultTarget = "en"
	} else {
		config.DefaultSource = "en"
		config.DefaultTarget = "zh"
	}

	return config
}
