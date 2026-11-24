// Package auth — 认证模式（免登录 / 本地账号）
//
// 容器唯一交付形态下默认免登录（none）：起容器即用，面向内网/反代后的
// 自托管场景。需要账号体系时显式设置 RDE_AUTH_MODE=local（登录页与
// 安装向导恢复），或通过设置页「启用账号登录」（写配置文件 [auth] 段，
// 重启生效）。IM 网关外部回调（webhook/telegram/wecom/feishu）不受模式
// 影响，始终使用各自的 API Key / secret_token / 签名鉴权。
package auth

import (
	"bufio"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// 认证模式
const (
	AuthModeNone  = "none"  // 免登录单用户模式（默认）
	AuthModeLocal = "local" // 本地账号 + JWT 登录
)

// 免登录模式的固定身份（审计字段、RequireAdmin 等照常工作）
const (
	NoneUserID   = "local-admin"
	NoneUsername = "admin"
)

// 配置文件指定的模式（main/bootstrap 启动时从 rde.conf [auth] 段读取）
var (
	configModeMu sync.RWMutex
	configMode   string
)

// SetConfigMode 设置配置文件指定的认证模式（空值清除；非法非空值忽略）
func SetConfigMode(mode string) {
	m := strings.ToLower(strings.TrimSpace(mode))
	configModeMu.Lock()
	defer configModeMu.Unlock()
	switch m {
	case AuthModeNone, AuthModeLocal:
		configMode = m
	case "":
		configMode = ""
	}
}

// ConfigMode 配置文件指定的模式（未设置返回空）
func ConfigMode() string {
	configModeMu.RLock()
	defer configModeMu.RUnlock()
	return configMode
}

// ReadModeFromConfigFile 从 rde.conf（INI 风格）读取 [auth] 段的 mode
// 文件不存在/无配置返回空（不视为错误）
func ReadModeFromConfigFile(path string) string {
	if path == "" {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	inAuth := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inAuth = strings.EqualFold(strings.TrimSpace(line[1:len(line)-1]), "auth")
			continue
		}
		if !inAuth {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "mode") {
			continue
		}
		return strings.Trim(strings.TrimSpace(parts[1]), `"'`)
	}
	return ""
}

// WriteModeToConfigFile 把 mode 写进 rde.conf 的 [auth] 段（保留其余内容；
// 不存在 [auth] 段则追加）
func WriteModeToConfigFile(path, mode string) error {
	if path == "" {
		return os.ErrInvalid
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 无配置文件：创建只含 [auth] 段的
			content := "[auth]\nmode = " + mode + "\n"
			return os.WriteFile(path, []byte(content), 0644)
		}
		return err
	}

	lines := []string{}
	if err := scannerLines(string(data), &lines); err != nil {
		return err
	}

	// 定位/改写 [auth] 段
	authStart := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") &&
			strings.EqualFold(strings.TrimSpace(t[1:len(t)-1]), "auth") {
			authStart = i
			break
		}
	}
	newLine := "mode = " + mode
	if authStart >= 0 {
		// 段内已有 mode 行则替换，否则插在段首
		replaced := false
		out := []string{}
		inAuth := false
		for _, l := range lines {
			t := strings.TrimSpace(l)
			if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
				inAuth = strings.EqualFold(strings.TrimSpace(t[1:len(t)-1]), "auth")
				out = append(out, l)
				continue
			}
			if inAuth && len(strings.SplitN(t, "=", 2)) == 2 &&
				strings.EqualFold(strings.TrimSpace(strings.SplitN(t, "=", 2)[0]), "mode") {
				out = append(out, newLine)
				replaced = true
				continue
			}
			out = append(out, l)
		}
		if !replaced {
			out = append(out[:authStart+1], append([]string{newLine}, out[authStart+1:]...)...)
		}
		lines = out
	} else {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "[auth]", newLine)
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}

func scannerLines(data string, out *[]string) error {
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		*out = append(*out, scanner.Text())
	}
	return scanner.Err()
}

// ResolveAuthMode 解析认证模式：环境变量 RDE_AUTH_MODE（显式覆盖）
// 优先，其次配置文件 [auth] 段，均未设置时默认 none。
func ResolveAuthMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("RDE_AUTH_MODE")))
	switch mode {
	case AuthModeLocal, AuthModeNone:
		return mode
	}
	if m := ConfigMode(); m != "" {
		return m
	}
	return AuthModeNone
}

// IsAuthNone 当前是否免登录模式
func IsAuthNone() bool {
	return ResolveAuthMode() == AuthModeNone
}

// NoneMiddleware 免登录模式中间件：注入固定管理员身份，不做 token 校验。
// 应用在 /api/v1 组（bootstrap 在 none 模式下用它替换 JWT 中间件）。
func NoneMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ContextKeyUserID, NoneUserID)
		c.Set(ContextKeyUsername, NoneUsername)
		c.Set(ContextKeyRole, "admin")
		// 兼容旧代码的简短键名
		c.Set("user_id", NoneUserID)
		c.Set("username", NoneUsername)
		c.Set("role", "admin")
		c.Next()
	}
}
