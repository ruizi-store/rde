package model

// ProxyMode 代理模式
type ProxyMode string

const (
	ProxyModeOff    ProxyMode = "off"    // 关闭
	ProxyModeManual ProxyMode = "manual" // 手动配置
	ProxyModePAC    ProxyMode = "pac"    // PAC 自动配置
)

// ProxyAuth 代理认证信息
type ProxyAuth struct {
	Username string `json:"username"`
	Password string `json:"password"` // 存储时加密
}

// SystemProxyConfig 全局代理配置
type SystemProxyConfig struct {
	Mode               ProxyMode  `json:"mode"`
	HttpProxy          string     `json:"http_proxy"`
	HttpsProxy         string     `json:"https_proxy"`
	Socks5             string     `json:"socks5"`
	NoProxy            string     `json:"no_proxy"`
	PacUrl             string     `json:"pac_url"`
	Auth               *ProxyAuth `json:"auth,omitempty"`
	DockerMirror       string     `json:"docker_mirror,omitempty"`        // Docker Hub Registry Mirror URL
	DockerProxyEnabled bool       `json:"docker_proxy_enabled,omitempty"` // 让 Docker daemon 也使用系统代理
}

// ProxyTestRequest 代理测试请求
type ProxyTestRequest struct {
	ProxyUrl string `json:"proxy_url"`
	TestUrl  string `json:"test_url"`
}

// ProxyTestResponse 代理测试响应
type ProxyTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency"` // 延迟毫秒
}
