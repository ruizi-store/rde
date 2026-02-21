package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfig(t *testing.T) {
	cfg := New()
	if cfg == nil {
		t.Fatal("New() 返回 nil")
	}
	
	if cfg.data == nil {
		t.Error("data map 不应为 nil")
	}
}

func TestConfigSetGet(t *testing.T) {
	cfg := New()
	
	cfg.Set("key1", "value1")
	cfg.Set("key2", 123)
	cfg.Set("key3", true)
	
	if v := cfg.GetString("key1"); v != "value1" {
		t.Errorf("GetString('key1') = %s, 期望 'value1'", v)
	}
	
	if v := cfg.GetInt("key2"); v != 123 {
		t.Errorf("GetInt('key2') = %d, 期望 123", v)
	}
	
	if v := cfg.GetBool("key3"); !v {
		t.Error("GetBool('key3') 应返回 true")
	}
}

func TestConfigSetDefault(t *testing.T) {
	cfg := New()
	
	// 设置默认值
	cfg.SetDefault("key1", "default1")
	if v := cfg.GetString("key1"); v != "default1" {
		t.Errorf("默认值应为 'default1', 实际为 %s", v)
	}
	
	// 设置新值覆盖默认值
	cfg.Set("key1", "new_value")
	if v := cfg.GetString("key1"); v != "new_value" {
		t.Errorf("值应为 'new_value', 实际为 %s", v)
	}
	
	// 设置默认值不应覆盖已有值
	cfg.SetDefault("key1", "default_again")
	if v := cfg.GetString("key1"); v != "new_value" {
		t.Errorf("SetDefault 不应覆盖已有值, 实际为 %s", v)
	}
}

func TestConfigNestedKeys(t *testing.T) {
	cfg := New()
	
	cfg.Set("server.host", "localhost")
	cfg.Set("server.port", 8080)
	cfg.Set("database.user", "admin")
	
	if v := cfg.GetString("server.host"); v != "localhost" {
		t.Errorf("GetString('server.host') = %s, 期望 'localhost'", v)
	}
	
	if v := cfg.GetInt("server.port"); v != 8080 {
		t.Errorf("GetInt('server.port') = %d, 期望 8080", v)
	}
}

func TestConfigExists(t *testing.T) {
	cfg := New()
	
	cfg.Set("existing_key", "value")
	
	if !cfg.Has("existing_key") {
		t.Error("Has 应返回 true 对于已存在的键")
	}
	
	if cfg.Has("non_existing_key") {
		t.Error("Has 应返回 false 对于不存在的键")
	}
}

func TestConfigDelete(t *testing.T) {
	cfg := New()
	
	cfg.Set("key_to_delete", "value")
	
	if !cfg.Has("key_to_delete") {
		t.Error("键应该存在")
	}
	
	cfg.Delete("key_to_delete")
	
	if cfg.Has("key_to_delete") {
		t.Error("删除后键不应存在")
	}
}

func TestConfigLoadYAML(t *testing.T) {
	// 创建临时 YAML 文件
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	
	yamlContent := `
server:
  host: localhost
  port: 8080
database:
  user: admin
  password: secret
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("无法写入测试文件: %v", err)
	}
	
	cfg := New()
	if err := cfg.Load(yamlFile); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	
	if v := cfg.GetString("server.host"); v != "localhost" {
		t.Errorf("YAML 加载后 server.host = %s, 期望 'localhost'", v)
	}
}

func TestConfigLoadJSON(t *testing.T) {
	// 创建临时 JSON 文件
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "config.json")
	
	jsonContent := `{
  "server": {
    "host": "0.0.0.0",
    "port": 3000
  },
  "debug": true
}`
	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("无法写入测试文件: %v", err)
	}
	
	cfg := New()
	if err := cfg.Load(jsonFile); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	
	if v := cfg.GetString("server.host"); v != "0.0.0.0" {
		t.Errorf("JSON 加载后 server.host = %s, 期望 '0.0.0.0'", v)
	}
	
	if !cfg.GetBool("debug") {
		t.Error("debug 应为 true")
	}
}

func TestConfigSave(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "save_test.yaml")
	
	cfg := New()
	cfg.Set("key1", "value1")
	cfg.Set("key2", 123)
	
	if err := cfg.Save(configFile); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	
	// 重新加载验证
	cfg2 := New()
	if err := cfg2.Load(configFile); err != nil {
		t.Fatalf("重新加载失败: %v", err)
	}
	
	if v := cfg2.GetString("key1"); v != "value1" {
		t.Errorf("重新加载后 key1 = %s, 期望 'value1'", v)
	}
}

func TestGlobalConfig(t *testing.T) {
	cfg := Global()
	if cfg == nil {
		t.Fatal("Global() 返回 nil")
	}
	
	// 多次调用应返回同一实例
	cfg2 := Global()
	if cfg != cfg2 {
		t.Error("Global() 应返回相同实例")
	}
}

func TestConfigUnmarshal(t *testing.T) {
	type ServerConfig struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	
	cfg := New()
	cfg.Set("host", "localhost")
	cfg.Set("port", 8080)
	
	var server ServerConfig
	if err := cfg.UnmarshalAll(&server); err != nil {
		t.Fatalf("UnmarshalAll 失败: %v", err)
	}
	
	if server.Host != "localhost" {
		t.Errorf("Host = %s, 期望 'localhost'", server.Host)
	}
	
	if server.Port != 8080 {
		t.Errorf("Port = %d, 期望 8080", server.Port)
	}
}

func TestConfigLoadEnv(t *testing.T) {
	// 设置环境变量
	os.Setenv("TEST_HOST", "envhost")
	os.Setenv("TEST_PORT", "9000")
	defer os.Unsetenv("TEST_HOST")
	defer os.Unsetenv("TEST_PORT")
	
	cfg := New()
	cfg.LoadEnv("TEST_")
	
	if v := cfg.GetString("host"); v != "envhost" {
		t.Errorf("从环境变量加载后 host = %s, 期望 'envhost'", v)
	}
}

func TestAppInfo(t *testing.T) {
	// 测试 AppInfo 全局变量
	if AppInfo.DBPath == "" && AppInfo.LogPath == "" {
		// 可能未初始化，这是正常的
		t.Log("AppInfo 未初始化，跳过测试")
		return
	}
}

func TestServerInfo(t *testing.T) {
	// 测试 ServerInfo 全局变量
	if ServerInfo.HttpPort != "" {
		if len(ServerInfo.HttpPort) < 1 || len(ServerInfo.HttpPort) > 5 {
			t.Error("HttpPort 格式不正确")
		}
	}
}

func TestInitSetup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	
	sampleConfig := `
[app]
name = rde
version = 1.0.0
`
	
	InitSetup(configPath, sampleConfig)
	
	// 检查配置文件是否被创建
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("InitSetup 应该创建配置文件")
	}
}

func TestSystemConfigInfo(t *testing.T) {
	// 测试 Section 方法
	section := Cfg.Section("test")
	if section == nil {
		t.Fatal("Section 返回 nil")
	}
	
	// 测试 Key 方法
	key := section.Key("key1")
	key.SetValue("value1")
	
	if key.String() != "value1" {
		t.Errorf("Key.String() = %s, 期望 'value1'", key.String())
	}
}

// 基准测试
func BenchmarkConfigSet(b *testing.B) {
	cfg := New()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		cfg.Set("key", "value")
	}
}

func BenchmarkConfigGet(b *testing.B) {
	cfg := New()
	cfg.Set("key", "value")
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		cfg.GetString("key")
	}
}

func BenchmarkConfigLoadYAML(b *testing.B) {
	tmpDir := b.TempDir()
	yamlFile := filepath.Join(tmpDir, "bench.yaml")
	os.WriteFile(yamlFile, []byte("key: value"), 0644)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := New()
		cfg.Load(yamlFile)
	}
}
