// Package translate 翻译模块类型定义
package translate

// Language 语言
type Language struct {
	Code string `json:"code"` // 语言代码，如 en, zh
	Name string `json:"name"` // 语言名称，如 English, Chinese
}

// TranslateRequest 翻译请求
type TranslateRequest struct {
	Text   string `json:"text" binding:"required"`   // 要翻译的文本
	Source string `json:"source"`                    // 源语言代码，空则自动检测
	Target string `json:"target" binding:"required"` // 目标语言代码
}

// TranslateResponse 翻译响应
type TranslateResponse struct {
	TranslatedText string `json:"translatedText"`     // 翻译结果
	DetectedLang   string `json:"detectedLang,omitempty"` // 检测到的源语言
}

// DetectRequest 语言检测请求
type DetectRequest struct {
	Text string `json:"text" binding:"required"` // 要检测的文本
}

// DetectResponse 语言检测响应
type DetectResponse struct {
	Language   string  `json:"language"`   // 检测到的语言代码
	Confidence float64 `json:"confidence"` // 置信度
}

// 服务生命周期阶段（供前端区分「启动中」与「安装失败」）
const (
	PhaseReady        = "ready"          // HTTP 可用
	PhaseStarting     = "starting"       // 容器已在跑/刚启动，模型加载中
	PhaseReadyToStart = "ready_to_start" // 本地或离线镜像已就绪，可一键启动
	PhaseMissing      = "missing"        // 无镜像，需要安装/拉取
	PhaseError        = "error"          // 明确错误
)

// ServiceStatus 服务状态
type ServiceStatus struct {
	Available        bool   `json:"available"`                   // 服务是否可用
	URL              string `json:"url"`                         // 服务地址
	Message          string `json:"message,omitempty"`           // 状态消息
	Phase            string `json:"phase,omitempty"`             // ready|starting|ready_to_start|missing|error
	Image            string `json:"image,omitempty"`             // 期望镜像
	Container        string `json:"container,omitempty"`         // 容器名
	ContainerRunning bool   `json:"containerRunning,omitempty"`  // 容器是否在跑
	ImageReady       bool   `json:"imageReady,omitempty"`        // 本地 Docker 已有镜像
	OfflineReady     bool   `json:"offlineReady,omitempty"`      // 离线 tar 可用
}

// EnsureResult 确保服务启动的结果
type EnsureResult struct {
	Status    string `json:"status"`              // success|starting|failed
	Available bool   `json:"available"`           // 此刻是否已可翻译
	Phase     string `json:"phase,omitempty"`     // 与 ServiceStatus.phase 相同语义
	Message   string `json:"message,omitempty"`
}

// TranslateConfig 翻译配置
type TranslateConfig struct {
	DefaultSource string `json:"defaultSource"` // 默认源语言
	DefaultTarget string `json:"defaultTarget"` // 默认目标语言
	ServiceURL    string `json:"serviceUrl"`    // LibreTranslate 服务地址
}

// LibreTranslate API 请求/响应结构

// libreTranslateRequest LibreTranslate API 请求
type libreTranslateRequest struct {
	Q      string `json:"q"`                 // 要翻译的文本
	Source string `json:"source"`            // 源语言
	Target string `json:"target"`            // 目标语言
	Format string `json:"format,omitempty"`  // 格式：text 或 html
}

// libreTranslateResponse LibreTranslate API 响应
type libreTranslateResponse struct {
	TranslatedText string `json:"translatedText"`
	DetectedLanguage *struct {
		Language   string  `json:"language"`
		Confidence float64 `json:"confidence"`
	} `json:"detectedLanguage,omitempty"`
}

// libreDetectResponse LibreTranslate 检测响应
type libreDetectResponse []struct {
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
}

// libreLanguage LibreTranslate 语言
type libreLanguage struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
