// 配置文件认证模式读写测试（none→local Web 升级入口）
package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadModeFromConfigFile(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "rde.conf")

	// 不存在 → 空
	if m := ReadModeFromConfigFile(conf); m != "" {
		t.Fatalf("expected empty mode for missing file, got %q", m)
	}

	// [auth] 段（裸值与带引号）
	content := `[app]
DBPath = /var/lib/rde

[server]
HttpPort = 3080

[auth]
mode = local
`
	if err := os.WriteFile(conf, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if m := ReadModeFromConfigFile(conf); m != AuthModeLocal {
		t.Fatalf("expected local, got %q", m)
	}

	content2 := `[auth]
mode = "none"
`
	if err := os.WriteFile(conf, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}
	if m := ReadModeFromConfigFile(conf); m != AuthModeNone {
		t.Fatalf("expected none, got %q", m)
	}

	// 无 [auth] 段
	content3 := `[server]
HttpPort = 3080
`
	if err := os.WriteFile(conf, []byte(content3), 0644); err != nil {
		t.Fatal(err)
	}
	if m := ReadModeFromConfigFile(conf); m != "" {
		t.Fatalf("expected empty, got %q", m)
	}
}

func TestWriteModeToConfigFile(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "rde.conf")

	// 文件不存在 → 创建
	if err := WriteModeToConfigFile(conf, AuthModeLocal); err != nil {
		t.Fatal(err)
	}
	if m := ReadModeFromConfigFile(conf); m != AuthModeLocal {
		t.Fatalf("expected local, got %q", m)
	}

	// 已有其他内容：追加 [auth] 段且保留原内容
	orig := "[server]\nHttpPort = 3080\nHttpHost = 0.0.0.0\n"
	if err := os.WriteFile(conf, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteModeToConfigFile(conf, AuthModeLocal); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(conf)
	s := string(data)
	if m := ReadModeFromConfigFile(conf); m != AuthModeLocal {
		t.Fatalf("expected local after append, got %q", m)
	}
	for _, want := range []string{"[server]", "HttpPort = 3080", "HttpHost = 0.0.0.0", "[auth]"} {
		if !contains(s, want) {
			t.Fatalf("missing %q in written config:\n%s", want, s)
		}
	}

	// 已有 [auth] 段：替换 mode 行
	if err := WriteModeToConfigFile(conf, AuthModeNone); err != nil {
		t.Fatal(err)
	}
	if m := ReadModeFromConfigFile(conf); m != AuthModeNone {
		t.Fatalf("expected none after replace, got %q", m)
	}
	data, _ = os.ReadFile(conf)
	if contains(string(data), "mode = local") {
		t.Fatalf("old mode line still present:\n%s", data)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestSetConfigModeAndResolve(t *testing.T) {
	old := ConfigMode()
	t.Cleanup(func() { SetConfigMode(old) })

	// 环境变量优先（清理再设置）
	t.Setenv("RDE_AUTH_MODE", "")
	SetConfigMode(AuthModeLocal)
	if got := ResolveAuthMode(); got != AuthModeLocal {
		t.Fatalf("expected local from config, got %q", got)
	}

	t.Setenv("RDE_AUTH_MODE", "none")
	if got := ResolveAuthMode(); got != AuthModeNone {
		t.Fatalf("expected env none to override config local, got %q", got)
	}

	t.Setenv("RDE_AUTH_MODE", "")
	SetConfigMode("invalid")
	if got := ConfigMode(); got != AuthModeLocal {
		t.Fatalf("invalid SetConfigMode must be ignored, got %q", got)
	}
}
