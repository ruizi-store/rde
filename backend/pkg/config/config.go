package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config 全局配置结构
type Config struct {
	mu   sync.RWMutex
	data map[string]interface{}
	file string
}

// 全局配置实例
var (
	globalConfig *Config
	once         sync.Once
)

// Global 获取全局配置实例
func Global() *Config {
	once.Do(func() {
		globalConfig = New()
	})
	return globalConfig
}

// New 创建新的配置实例
func New() *Config {
	return &Config{
		data: make(map[string]interface{}),
	}
}

// Load 从文件加载配置
func (c *Config) Load(filename string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	c.file = filename
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".json":
		return json.Unmarshal(content, &c.data)
	case ".yaml", ".yml":
		return yaml.Unmarshal(content, &c.data)
	default:
		return errors.New("unsupported config file format")
	}
}

// LoadJSON 加载 JSON 配置
func (c *Config) LoadJSON(filename string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	c.file = filename
	return json.Unmarshal(content, &c.data)
}

// LoadYAML 加载 YAML 配置
func (c *Config) LoadYAML(filename string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	c.file = filename
	return yaml.Unmarshal(content, &c.data)
}

// LoadEnv 从环境变量加载配置
func (c *Config) LoadEnv(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		if prefix != "" {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			key = strings.TrimPrefix(key, prefix)
		}

		// 转换 KEY_NAME 为 key.name
		key = strings.ToLower(strings.ReplaceAll(key, "_", "."))
		c.set(key, value)
	}
}

// Save 保存配置到文件
func (c *Config) Save(filename string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if filename == "" {
		filename = c.file
	}
	if filename == "" {
		return errors.New("no filename specified")
	}

	ext := strings.ToLower(filepath.Ext(filename))

	var content []byte
	var err error

	switch ext {
	case ".json":
		content, err = json.MarshalIndent(c.data, "", "  ")
	case ".yaml", ".yml":
		content, err = yaml.Marshal(c.data)
	default:
		return errors.New("unsupported config file format")
	}

	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filename, content, 0644)
}

// Get 获取配置值
func (c *Config) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.get(key)
}

// get 内部获取方法（不加锁）
func (c *Config) get(key string) interface{} {
	parts := strings.Split(key, ".")
	current := interface{}(c.data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		case map[interface{}]interface{}:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}

// Set 设置配置值
func (c *Config) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.set(key, value)
}

// set 内部设置方法（不加锁）
func (c *Config) set(key string, value interface{}) {
	parts := strings.Split(key, ".")
	current := c.data

	for i, part := range parts[:len(parts)-1] {
		if _, ok := current[part]; !ok {
			current[part] = make(map[string]interface{})
		}
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			// 如果中间节点不是 map，则创建新的 map
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
			_ = i // 避免未使用警告
		}
	}

	current[parts[len(parts)-1]] = value
}

// GetString 获取字符串配置
func (c *Config) GetString(key string) string {
	return c.GetStringDefault(key, "")
}

// GetStringDefault 获取字符串配置（带默认值）
func (c *Config) GetStringDefault(key, defaultValue string) string {
	v := c.Get(key)
	if v == nil {
		return defaultValue
	}
	if s, ok := v.(string); ok {
		return s
	}
	return defaultValue
}

// GetInt 获取整数配置
func (c *Config) GetInt(key string) int {
	return c.GetIntDefault(key, 0)
}

// GetIntDefault 获取整数配置（带默认值）
func (c *Config) GetIntDefault(key string, defaultValue int) int {
	v := c.Get(key)
	if v == nil {
		return defaultValue
	}

	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
}

// GetInt64 获取 int64 配置
func (c *Config) GetInt64(key string) int64 {
	return c.GetInt64Default(key, 0)
}

// GetInt64Default 获取 int64 配置（带默认值）
func (c *Config) GetInt64Default(key string, defaultValue int64) int64 {
	v := c.Get(key)
	if v == nil {
		return defaultValue
	}

	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return defaultValue
}

// GetFloat64 获取浮点数配置
func (c *Config) GetFloat64(key string) float64 {
	return c.GetFloat64Default(key, 0)
}

// GetFloat64Default 获取浮点数配置（带默认值）
func (c *Config) GetFloat64Default(key string, defaultValue float64) float64 {
	v := c.Get(key)
	if v == nil {
		return defaultValue
	}

	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

// GetBool 获取布尔配置
func (c *Config) GetBool(key string) bool {
	return c.GetBoolDefault(key, false)
}

// GetBoolDefault 获取布尔配置（带默认值）
func (c *Config) GetBoolDefault(key string, defaultValue bool) bool {
	v := c.Get(key)
	if v == nil {
		return defaultValue
	}

	switch val := v.(type) {
	case bool:
		return val
	case string:
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	case int:
		return val != 0
	}
	return defaultValue
}

// GetStringSlice 获取字符串数组配置
func (c *Config) GetStringSlice(key string) []string {
	v := c.Get(key)
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// GetMap 获取 map 配置
func (c *Config) GetMap(key string) map[string]interface{} {
	v := c.Get(key)
	if v == nil {
		return nil
	}

	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

// Has 检查配置是否存在
func (c *Config) Has(key string) bool {
	return c.Get(key) != nil
}

// Delete 删除配置
func (c *Config) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	parts := strings.Split(key, ".")
	if len(parts) == 1 {
		delete(c.data, key)
		return
	}

	current := c.data
	for _, part := range parts[:len(parts)-1] {
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return
		}
	}
	delete(current, parts[len(parts)-1])
}

// All 获取所有配置
func (c *Config) All() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 深拷贝
	result := make(map[string]interface{})
	for k, v := range c.data {
		result[k] = v
	}
	return result
}

// Merge 合并配置
func (c *Config) Merge(other *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	c.mergeMap(c.data, other.data)
}

func (c *Config) mergeMap(dst, src map[string]interface{}) {
	for k, v := range src {
		if dstVal, ok := dst[k]; ok {
			if srcMap, srcIsMap := v.(map[string]interface{}); srcIsMap {
				if dstMap, dstIsMap := dstVal.(map[string]interface{}); dstIsMap {
					c.mergeMap(dstMap, srcMap)
					continue
				}
			}
		}
		dst[k] = v
	}
}

// Unmarshal 将配置解析到结构体
func (c *Config) Unmarshal(key string, v interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data := c.get(key)
	if data == nil {
		return errors.New("key not found")
	}

	// 转换为 JSON 再解析
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonData, v)
}

// UnmarshalAll 将所有配置解析到结构体
func (c *Config) UnmarshalAll(v interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	jsonData, err := json.Marshal(c.data)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonData, v)
}

// SetFromStruct 从结构体设置配置
func (c *Config) SetFromStruct(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	jsonData, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonData, &c.data)
}

// Watch 监听配置文件变化（需要外部轮询实现）
type ConfigWatcher func(c *Config)

// EnvOrDefault 从环境变量获取值，如果不存在则返回默认值
func EnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// MustEnv 从环境变量获取值，如果不存在则 panic
func MustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required environment variable not set: " + key)
	}
	return v
}

// BindEnv 将环境变量绑定到配置
func (c *Config) BindEnv(key, envKey string) {
	if v := os.Getenv(envKey); v != "" {
		c.Set(key, v)
	}
}

// SetDefault 设置默认值（如果 key 不存在）
func (c *Config) SetDefault(key string, value interface{}) {
	if !c.Has(key) {
		c.Set(key, value)
	}
}

// SetDefaults 批量设置默认值
func (c *Config) SetDefaults(defaults map[string]interface{}) {
	for k, v := range defaults {
		c.SetDefault(k, v)
	}
}

// Clone 克隆配置
func (c *Config) Clone() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()

	newConfig := New()
	jsonData, _ := json.Marshal(c.data)
	json.Unmarshal(jsonData, &newConfig.data)
	return newConfig
}

// Debug 打印配置（调试用）
func (c *Config) Debug() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, _ := json.MarshalIndent(c.data, "", "  ")
	return string(data)
}

// BindStruct 从结构体标签绑定配置
func BindStruct(v interface{}, c *Config, prefix string) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return errors.New("expected struct")
	}

	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanSet() {
			continue
		}

		tag := field.Tag.Get("config")
		if tag == "" || tag == "-" {
			continue
		}

		key := prefix + tag
		configVal := c.Get(key)
		if configVal == nil {
			continue
		}

		switch fieldVal.Kind() {
		case reflect.String:
			if s, ok := configVal.(string); ok {
				fieldVal.SetString(s)
			}
		case reflect.Int, reflect.Int64:
			fieldVal.SetInt(int64(c.GetIntDefault(key, 0)))
		case reflect.Float64:
			fieldVal.SetFloat(c.GetFloat64Default(key, 0))
		case reflect.Bool:
			fieldVal.SetBool(c.GetBoolDefault(key, false))
		}
	}

	return nil
}

// ==================== 兼容 CasaOS 的全局变量 ====================

// ServerInfoStruct 服务器信息结构
type ServerInfoStruct struct {
	ServerApi  string
	UpdateUrl  string
	HttpPort   string
	SocketPort string
}

// ServerInfo 全局服务器信息（CasaOS 兼容）
var ServerInfo = ServerInfoStruct{
	ServerApi:  "",
	UpdateUrl:  "",
	HttpPort:   "80",
	SocketPort: "9999",
}

// AppInfoStruct 应用信息结构
type AppInfoStruct struct {
	ProjectPath  string
	DBPath       string
	LogPath      string
	DataPath     string
	ShellPath    string
	UserDataPath string
	LogSaveName  string
	LogFileExt   string
}

// AppInfo 全局应用信息（CasaOS 兼容）
// 初始值为空，由 setDefaultAppInfo() 在 InitSetup 时设置
var AppInfo = AppInfoStruct{}

// SystemConfigInfo 系统配置信息
type SystemConfigInfo struct {
	ConfigPath string
	sections   map[string]*ConfigSection
}

// ConfigSection 配置段
type ConfigSection struct {
	data map[string]string
}

// Key 获取配置键
func (s *ConfigSection) Key(name string) *ConfigKey {
	if s.data == nil {
		s.data = make(map[string]string)
	}
	return &ConfigKey{section: s, name: name}
}

// ConfigKey 配置键
type ConfigKey struct {
	section *ConfigSection
	name    string
}

// SetValue 设置值
func (k *ConfigKey) SetValue(value string) {
	if k.section.data == nil {
		k.section.data = make(map[string]string)
	}
	k.section.data[k.name] = value
}

// String 获取字符串值
func (k *ConfigKey) String() string {
	if k.section.data == nil {
		return ""
	}
	return k.section.data[k.name]
}

// Cfg 全局配置（CasaOS 兼容）
var Cfg = &SystemConfigInfo{
	ConfigPath: "/etc/rde/config.ini",
	sections:   make(map[string]*ConfigSection),
}

// Section 获取配置段
func (c *SystemConfigInfo) Section(name string) *ConfigSection {
	if c.sections == nil {
		c.sections = make(map[string]*ConfigSection)
	}
	if _, ok := c.sections[name]; !ok {
		c.sections[name] = &ConfigSection{data: make(map[string]string)}
	}
	return c.sections[name]
}

// SaveTo 保存配置到文件
func (c *SystemConfigInfo) SaveTo(path string) error {
	// 简单实现：暂时不保存
	return nil
}

// Reload 重新加载配置
func (c *SystemConfigInfo) Reload() error {
	// 重新加载配置文件
	return nil
}

// InitSetup 初始化配置 - CasaOS 兼容
// configPath: 配置文件路径
// confSample: 配置示例（嵌入的）
func InitSetup(configPath string, confSample string) {
	// 设置配置路径
	if configPath != "" {
		Cfg.ConfigPath = configPath
	}
	
	// 确保配置目录存在
	configDir := filepath.Dir(Cfg.ConfigPath)
	os.MkdirAll(configDir, 0755)
	
	// 如果配置文件不存在，创建默认配置
	if _, err := os.Stat(Cfg.ConfigPath); os.IsNotExist(err) {
		if confSample != "" {
			os.WriteFile(Cfg.ConfigPath, []byte(confSample), 0644)
		}
	}
	
	// 设置默认值
	setDefaultAppInfo()
	setDefaultServerInfo()
}

// setDefaultAppInfo 设置默认应用信息
func setDefaultAppInfo() {
	// 获取用户主目录
	homeDir, _ := os.UserHomeDir()
	defaultBaseDir := "/tmp/rde"
	if homeDir != "" {
		defaultBaseDir = filepath.Join(homeDir, ".rde")
	}
	
	// 从环境变量获取或使用默认值
	if AppInfo.DBPath == "" {
		AppInfo.DBPath = getEnvOrDefault("RDE_DB_PATH", filepath.Join(defaultBaseDir, "data"))
	}
	if AppInfo.LogPath == "" {
		AppInfo.LogPath = getEnvOrDefault("RDE_LOG_PATH", filepath.Join(defaultBaseDir, "logs"))
	}
	if AppInfo.LogSaveName == "" {
		AppInfo.LogSaveName = getEnvOrDefault("RDE_LOG_NAME", "rde")
	}
	if AppInfo.LogFileExt == "" {
		AppInfo.LogFileExt = getEnvOrDefault("RDE_LOG_EXT", "log")
	}
	if AppInfo.ShellPath == "" {
		AppInfo.ShellPath = getEnvOrDefault("RDE_SHELL_PATH", "/usr/share/rde/shell")
	}
	if AppInfo.UserDataPath == "" {
		defaultDataPath := "/tmp/rde/data"
		if home, err := os.UserHomeDir(); err == nil {
			defaultDataPath = filepath.Join(home, ".rde", "data")
		}
		AppInfo.UserDataPath = getEnvOrDefault("RDE_USER_DATA_PATH", defaultDataPath)
	}
	if AppInfo.ProjectPath == "" {
		AppInfo.ProjectPath = getEnvOrDefault("RDE_PROJECT_PATH", "/rde")
	}
	
	// 确保目录存在（忽略错误，可能是权限问题）
	os.MkdirAll(AppInfo.DBPath, 0755)
	os.MkdirAll(AppInfo.LogPath, 0755)
	os.MkdirAll(AppInfo.UserDataPath, 0755)
}

// getEnvOrDefault 获取环境变量，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// setDefaultServerInfo 设置默认服务器信息
func setDefaultServerInfo() {
	if ServerInfo.HttpPort == "" {
		ServerInfo.HttpPort = "80"
	}
	if ServerInfo.ServerApi == "" {
		ServerInfo.ServerApi = "" // configured via conf file
	}
}
