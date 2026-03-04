// Package ai 语音服务 (STT/TTS)
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// VoiceService 语音服务
type VoiceService struct {
	logger  *zap.Logger
	dataDir string
	config  *VoiceConfig
	client  *http.Client
}

// NewVoiceService 创建语音服务
func NewVoiceService(logger *zap.Logger, dataDir string) *VoiceService {
	vs := &VoiceService{
		logger:  logger,
		dataDir: dataDir,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
	vs.loadConfig()
	return vs
}

// loadConfig 加载配置
func (vs *VoiceService) loadConfig() {
	vs.config = &VoiceConfig{
		STTProvider: "whisper",
		TTSProvider: "edge",
		STTModel:    "whisper-1",
		TTSModel:    "tts-1",
		TTSVoice:    "zh-CN-XiaoxiaoNeural",
	}

	configFile := filepath.Join(vs.dataDir, "voice.json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, vs.config)
}

// saveConfig 保存配置
func (vs *VoiceService) saveConfig() error {
	configFile := filepath.Join(vs.dataDir, "voice.json")
	data, err := json.MarshalIndent(vs.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}

// GetConfig 获取配置
func (vs *VoiceService) GetConfig() *VoiceConfig {
	return vs.config
}

// UpdateConfig 更新配置
func (vs *VoiceService) UpdateConfig(config *VoiceConfig) error {
	vs.config = config
	return vs.saveConfig()
}

// TranscribeAudio 转录语音文件
func (vs *VoiceService) TranscribeAudio(ctx context.Context, audioPath string) (string, error) {
	switch vs.config.STTProvider {
	case "whisper":
		return vs.transcribeWithOpenAI(ctx, audioPath)
	case "whisper_local":
		return vs.transcribeWithLocalWhisper(ctx, audioPath)
	case "azure":
		return vs.transcribeWithAzure(ctx, audioPath)
	default:
		return "", fmt.Errorf("未知的 STT 提供者: %s", vs.config.STTProvider)
	}
}

// TranscribeURL 从 URL 下载并转录
func (vs *VoiceService) TranscribeURL(ctx context.Context, audioURL string) (string, error) {
	resp, err := vs.client.Get(audioURL)
	if err != nil {
		return "", fmt.Errorf("下载音频失败: %w", err)
	}
	defer resp.Body.Close()

	tmpFile := filepath.Join(vs.dataDir, fmt.Sprintf("temp_audio_%d", time.Now().UnixNano()))

	contentType := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(contentType, "ogg"):
		tmpFile += ".ogg"
	case strings.Contains(contentType, "mp3"):
		tmpFile += ".mp3"
	case strings.Contains(contentType, "wav"):
		tmpFile += ".wav"
	case strings.Contains(contentType, "m4a"):
		tmpFile += ".m4a"
	default:
		tmpFile += ".ogg"
	}

	out, err := os.Create(tmpFile)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile)

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return "", err
	}
	out.Close()

	return vs.TranscribeAudio(ctx, tmpFile)
}

// TextToSpeech 文字转语音 (TTS)
func (vs *VoiceService) TextToSpeech(ctx context.Context, text string) (string, error) {
	switch vs.config.TTSProvider {
	case "edge":
		return vs.ttsWithEdge(ctx, text)
	case "openai":
		return vs.ttsWithOpenAI(ctx, text)
	case "azure":
		return vs.ttsWithAzure(ctx, text)
	default:
		return "", fmt.Errorf("未知的 TTS 提供者: %s", vs.config.TTSProvider)
	}
}

// ==================== OpenAI Whisper ====================

func (vs *VoiceService) transcribeWithOpenAI(ctx context.Context, audioPath string) (string, error) {
	if vs.config.OpenAIKey == "" {
		return "", fmt.Errorf("OpenAI API Key 未配置")
	}

	file, err := os.Open(audioPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}

	model := vs.config.STTModel
	if model == "" {
		model = "whisper-1"
	}
	writer.WriteField("model", model)
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+vs.config.OpenAIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := vs.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Whisper API 错误: %s", string(body))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	vs.logger.Info("Audio transcribed with OpenAI",
		zap.String("file", audioPath),
		zap.Int("textLen", len(result.Text)))

	return result.Text, nil
}

// ==================== 本地 Whisper ====================

func (vs *VoiceService) transcribeWithLocalWhisper(ctx context.Context, audioPath string) (string, error) {
	if _, err := exec.LookPath("whisper"); err != nil {
		return "", fmt.Errorf("whisper 未安装，请运行: pip install openai-whisper")
	}

	model := vs.config.STTModel
	if model == "" {
		model = "base"
	}

	args := []string{
		audioPath,
		"--model", model,
		"--output_format", "txt",
		"--output_dir", vs.dataDir,
	}

	cmd := exec.CommandContext(ctx, "whisper", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("whisper 执行失败: %s, %w", string(output), err)
	}

	baseName := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	txtFile := filepath.Join(vs.dataDir, baseName+".txt")

	text, err := os.ReadFile(txtFile)
	if err != nil {
		return "", err
	}
	os.Remove(txtFile)

	vs.logger.Info("Audio transcribed with local Whisper",
		zap.String("file", audioPath),
		zap.Int("textLen", len(text)))

	return strings.TrimSpace(string(text)), nil
}

// ==================== Azure Speech ====================

func (vs *VoiceService) transcribeWithAzure(ctx context.Context, audioPath string) (string, error) {
	if vs.config.AzureKey == "" || vs.config.AzureRegion == "" {
		return "", fmt.Errorf("Azure Speech 配置不完整")
	}

	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://%s.stt.speech.microsoft.com/speech/recognition/conversation/cognitiveservices/v1?language=zh-CN",
		vs.config.AzureRegion)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(audioData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", vs.config.AzureKey)
	req.Header.Set("Content-Type", "audio/wav")

	resp, err := vs.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Azure Speech 错误: %s", string(body))
	}

	var result struct {
		RecognitionStatus string `json:"RecognitionStatus"`
		DisplayText       string `json:"DisplayText"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.RecognitionStatus != "Success" {
		return "", fmt.Errorf("语音识别失败: %s", result.RecognitionStatus)
	}

	vs.logger.Info("Audio transcribed with Azure",
		zap.String("file", audioPath),
		zap.Int("textLen", len(result.DisplayText)))

	return result.DisplayText, nil
}

// ==================== Edge TTS ====================

func (vs *VoiceService) ttsWithEdge(ctx context.Context, text string) (string, error) {
	voice := vs.config.TTSVoice
	if voice == "" {
		voice = "zh-CN-XiaoxiaoNeural"
	}

	outputPath := filepath.Join(vs.dataDir, fmt.Sprintf("tts_%d.mp3", time.Now().UnixNano()))

	// 调用 edge-tts 命令行工具
	cmd := exec.CommandContext(ctx, "edge-tts", "--text", text, "--voice", voice, "--write-media", outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		vs.logger.Error("edge-tts failed",
			zap.Error(err),
			zap.String("output", string(output)))
		return "", fmt.Errorf("edge-tts 执行失败: %w", err)
	}

	// 检查文件是否生成
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("edge-tts 未生成音频文件")
	}

	vs.logger.Info("TTS generated with edge-tts",
		zap.String("voice", voice),
		zap.Int("textLen", len(text)))

	return outputPath, nil
}

// ==================== OpenAI TTS ====================

func (vs *VoiceService) ttsWithOpenAI(ctx context.Context, text string) (string, error) {
	if vs.config.OpenAIKey == "" {
		return "", fmt.Errorf("OpenAI API Key 未配置")
	}

	model := vs.config.TTSModel
	if model == "" {
		model = "tts-1"
	}
	voice := vs.config.TTSVoice
	if voice == "" {
		voice = "alloy"
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"voice": voice,
		"input": text,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/speech", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+vs.config.OpenAIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := vs.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("TTS API 错误: %s", string(body))
	}

	outputPath := filepath.Join(vs.dataDir, fmt.Sprintf("tts_%d.mp3", time.Now().UnixNano()))
	out, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}

	return outputPath, nil
}

// ==================== Azure TTS ====================

func (vs *VoiceService) ttsWithAzure(ctx context.Context, text string) (string, error) {
	if vs.config.AzureKey == "" || vs.config.AzureRegion == "" {
		return "", fmt.Errorf("Azure Speech 配置不完整")
	}

	voiceName := "zh-CN-XiaoxiaoNeural"

	ssml := fmt.Sprintf(`<speak version='1.0' xml:lang='zh-CN'>
		<voice name='%s'>%s</voice>
	</speak>`, voiceName, text)

	url := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", vs.config.AzureRegion)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(ssml))
	if err != nil {
		return "", err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", vs.config.AzureKey)
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", "audio-16khz-128kbitrate-mono-mp3")

	resp, err := vs.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Azure TTS 错误: %s", string(body))
	}

	outputPath := filepath.Join(vs.dataDir, fmt.Sprintf("tts_%d.mp3", time.Now().UnixNano()))
	out, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}

	return outputPath, nil
}

// ConvertAudioFormat 转换音频格式 (需要 ffmpeg)
func (vs *VoiceService) ConvertAudioFormat(inputPath, outputFormat string) (string, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return "", fmt.Errorf("ffmpeg 未安装")
	}

	outputPath := strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + "." + outputFormat
	cmd := exec.Command("ffmpeg", "-i", inputPath, "-y", outputPath)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return outputPath, nil
}

// getLangCode 根据配置语言返回对应的语言代码
func (vs *VoiceService) getLangCode() string {
	switch vs.config.Language {
	case "zh", "zh-CN":
		return "zh-CN"
	case "zh-TW":
		return "zh-TW"
	case "en", "en-US":
		return "en-US"
	case "en-GB":
		return "en-GB"
	case "ja":
		return "ja-JP"
	case "ko":
		return "ko-KR"
	case "de":
		return "de-DE"
	case "fr":
		return "fr-FR"
	case "es":
		return "es-ES"
	default:
		if vs.config.Language != "" {
			return vs.config.Language
		}
		return "zh-CN"
	}
}
