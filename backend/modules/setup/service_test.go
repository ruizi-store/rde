package setup

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// mockConfigProvider 模拟配置提供者
type mockConfigProvider struct {
	values map[string]string
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{
		values: map[string]string{
			"auth.jwt_secret": "test-secret-key-for-testing",
		},
	}
}

func (m *mockConfigProvider) GetString(key string) string {
	if v, ok := m.values[key]; ok {
		return v
	}
	return ""
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// 迁移 setup 相关表
	err = db.AutoMigrate(&SetupSettings{}, &ModuleSetting{})
	require.NoError(t, err)

	return db
}

func setupTestService(t *testing.T) *Service {
	t.Helper()
	db := setupTestDB(t)
	dataDir := t.TempDir()
	config := newMockConfigProvider()
	return NewService(db, zap.NewNop(), dataDir, config)
}

// ----- 状态管理测试 -----

func TestService_GetStatus_Initial(t *testing.T) {
	svc := setupTestService(t)

	status, err := svc.GetStatus()
	require.NoError(t, err)

	assert.False(t, status.Completed)
	assert.Equal(t, 1, status.CurrentStep)
	assert.Empty(t, status.CompletedSteps)
	assert.False(t, status.CanSkipSetup)
}

func TestService_IsCompleted_False(t *testing.T) {
	svc := setupTestService(t)

	assert.False(t, svc.IsCompleted())
	assert.True(t, svc.NeedsSetup())
}

func TestService_MarkSetupCompleted(t *testing.T) {
	svc := setupTestService(t)

	// 初始状态未完成
	assert.False(t, svc.IsCompleted())

	// 标记完成
	err := svc.MarkSetupCompleted()
	require.NoError(t, err)

	// 验证已完成
	assert.True(t, svc.IsCompleted())
	assert.False(t, svc.NeedsSetup())

	status, err := svc.GetStatus()
	require.NoError(t, err)
	assert.True(t, status.Completed)
}

// ----- 步骤完成标记测试 -----

func TestService_MarkStepCompleted(t *testing.T) {
	svc := setupTestService(t)

	// 标记 step 1 完成
	err := svc.markStepCompleted(1)
	require.NoError(t, err)

	status, err := svc.GetStatus()
	require.NoError(t, err)
	assert.Contains(t, status.CompletedSteps, 1)
	assert.Equal(t, 2, status.CurrentStep)

	// 标记 step 2 完成
	err = svc.markStepCompleted(2)
	require.NoError(t, err)

	status, err = svc.GetStatus()
	require.NoError(t, err)
	assert.Contains(t, status.CompletedSteps, 1)
	assert.Contains(t, status.CompletedSteps, 2)
	assert.Equal(t, 3, status.CurrentStep)
}

func TestService_MarkStepCompleted_Idempotent(t *testing.T) {
	svc := setupTestService(t)

	// 重复标记同一步骤
	err := svc.markStepCompleted(1)
	require.NoError(t, err)

	err = svc.markStepCompleted(1)
	require.NoError(t, err)

	status, err := svc.GetStatus()
	require.NoError(t, err)

	// 步骤只出现一次
	count := 0
	for _, s := range status.CompletedSteps {
		if s == 1 {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

// ----- 语言时区设置测试 -----

func TestService_SetLocale(t *testing.T) {
	svc := setupTestService(t)

	// 先确保记录存在
	_, err := svc.getOrCreateSettings()
	require.NoError(t, err)

	req := &LocaleSettings{
		Language:   "en-US",
		Timezone:   "America/New_York",
		TimeFormat: "12h",
		DateFormat: "MM/DD/YYYY",
	}

	err = svc.SetLocale(req)
	require.NoError(t, err)

	// 验证设置已保存到数据库
	var settings SetupSettings
	err = svc.db.First(&settings).Error
	require.NoError(t, err)

	assert.Equal(t, "en-US", settings.Language)
	assert.Equal(t, "America/New_York", settings.Timezone)
	assert.Equal(t, "12h", settings.TimeFormat)
	assert.Equal(t, "MM/DD/YYYY", settings.DateFormat)
}

func TestService_SetLocale_InvalidTimezone(t *testing.T) {
	svc := setupTestService(t)

	req := &LocaleSettings{
		Language:   "zh-CN",
		Timezone:   "Invalid/Timezone",
		TimeFormat: "24h",
		DateFormat: "YYYY-MM-DD",
	}

	// 应该仍然保存（只是 timedatectl 可能失败）
	err := svc.SetLocale(req)
	require.NoError(t, err)
}

// ----- 完成初始化测试 -----

func TestService_Complete_AllStepsRequired(t *testing.T) {
	svc := setupTestService(t)

	// 没有完成任何步骤时尝试完成
	_, err := svc.Complete()
	assert.Error(t, err)
}

func TestService_Complete_Success(t *testing.T) {
	svc := setupTestService(t)

	// 模拟完成所有必要步骤
	for i := 1; i <= 6; i++ {
		err := svc.markStepCompleted(i)
		require.NoError(t, err)
	}

	// 完成初始化
	resp, err := svc.Complete()
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.Equal(t, "/login", resp.RedirectURL)

	// 验证状态
	assert.True(t, svc.IsCompleted())
}

// ----- 功能选择测试 -----

func TestService_SaveFeatureSelection(t *testing.T) {
	svc := setupTestService(t)

	req := &FeatureSelection{
		EnabledModules: []string{"docker", "terminal"},
	}

	err := svc.SaveFeatureSelection(req)
	require.NoError(t, err)

	// 验证步骤已完成
	status, err := svc.GetStatus()
	require.NoError(t, err)
	assert.Contains(t, status.CompletedSteps, 6)
}

func TestService_SaveFeatureSelection_Empty(t *testing.T) {
	svc := setupTestService(t)

	req := &FeatureSelection{
		EnabledModules: []string{},
	}

	err := svc.SaveFeatureSelection(req)
	require.NoError(t, err)
}

func TestService_SkipFeatureSelection(t *testing.T) {
	svc := setupTestService(t)

	err := svc.SkipFeatureSelection()
	require.NoError(t, err)

	status, err := svc.GetStatus()
	require.NoError(t, err)
	assert.Contains(t, status.CompletedSteps, 6)
}

// ----- 网络配置跳过测试 -----

func TestService_SkipNetworkConfig(t *testing.T) {
	svc := setupTestService(t)

	err := svc.SkipNetworkConfig()
	require.NoError(t, err)

	status, err := svc.GetStatus()
	require.NoError(t, err)
	assert.Contains(t, status.CompletedSteps, 5)
}

// ----- 存储配置跳过测试 -----

func TestService_SkipStorageConfig(t *testing.T) {
	svc := setupTestService(t)

	err := svc.SkipStorageConfig()
	require.NoError(t, err)

	status, err := svc.GetStatus()
	require.NoError(t, err)
	assert.Contains(t, status.CompletedSteps, 4)
}

// ----- 模块设置测试 -----

func TestService_GetAllModuleSettings(t *testing.T) {
	svc := setupTestService(t)

	settings, err := svc.GetAllModuleSettings()
	require.NoError(t, err)

	// 应该返回所有预定义模块
	assert.NotEmpty(t, settings)

	// 查找 docker 模块
	var found bool
	for _, s := range settings {
		if s.ModuleID == "docker" {
			found = true
			assert.Equal(t, "Docker 应用", s.Name)
			break
		}
	}
	assert.True(t, found, "docker module should be in settings")
}

func TestService_UpdateModuleSetting(t *testing.T) {
	svc := setupTestService(t)

	// 更新 docker 模块设置
	setting, err := svc.UpdateModuleSetting("docker", true, map[string]interface{}{
		"auto_start": true,
	})
	require.NoError(t, err)

	assert.Equal(t, "docker", setting.ModuleID)
	assert.True(t, setting.Enabled)

	// 获取并验证
	retrieved, err := svc.GetModuleSetting("docker")
	require.NoError(t, err)
	assert.True(t, retrieved.Enabled)
}

func TestService_UpdateModuleSetting_Disable(t *testing.T) {
	svc := setupTestService(t)

	// 先启用
	_, err := svc.UpdateModuleSetting("docker", true, nil)
	require.NoError(t, err)

	// 再禁用
	setting, err := svc.UpdateModuleSetting("docker", false, nil)
	require.NoError(t, err)

	assert.False(t, setting.Enabled)
}

// ----- 密码验证测试 -----

func TestService_ValidateUserPassword(t *testing.T) {
	svc := setupTestService(t)

	// 创建测试用户表
	type UserAccount struct {
		ID       string `gorm:"primaryKey"`
		Password string
	}
	err := svc.db.Table("users_accounts").AutoMigrate(&UserAccount{})
	require.NoError(t, err)

	// 创建测试用户（使用 bcrypt 加密密码）
	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("testpass123"), bcrypt.DefaultCost)
	svc.db.Table("users_accounts").Create(map[string]interface{}{
		"id":       "user-123",
		"password": string(hashedPwd),
	})

	// 验证正确密码
	err = svc.ValidateUserPassword("user-123", "testpass123")
	assert.NoError(t, err)

	// 验证错误密码
	err = svc.ValidateUserPassword("user-123", "wrongpassword")
	assert.Error(t, err)

	// 验证不存在的用户
	err = svc.ValidateUserPassword("nonexistent", "anypassword")
	assert.Error(t, err)
}

// ----- 恢复出厂设置测试 -----

func TestService_FactoryReset_InvalidConfirmText(t *testing.T) {
	svc := setupTestService(t)

	req := &FactoryResetRequest{
		Password:    "anypassword",
		ConfirmText: "wrong",
	}

	_, err := svc.FactoryReset(req)
	assert.ErrorIs(t, err, ErrInvalidConfirmText)
}

func TestService_ValidateUserPassword_Invalid(t *testing.T) {
	svc := setupTestService(t)

	// 创建测试用户
	type UserAccount struct {
		ID       string `gorm:"primaryKey"`
		Password string
	}
	svc.db.Table("users_accounts").AutoMigrate(&UserAccount{})
	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("correctpass"), bcrypt.DefaultCost)
	svc.db.Table("users_accounts").Create(map[string]interface{}{
		"id":       "user-123",
		"password": string(hashedPwd),
	})

	err := svc.ValidateUserPassword("user-123", "wrongpassword")
	assert.ErrorIs(t, err, ErrInvalidPassword)
}

// ----- 2FA 验证测试 -----

func TestService_Verify2FA(t *testing.T) {
	svc := setupTestService(t)

	// 创建用户表并设置 2FA
	type UserAccount struct {
		ID       string `gorm:"primaryKey"`
		Settings string `gorm:"type:text"`
	}
	svc.db.Table("users_accounts").AutoMigrate(&UserAccount{})

	// 生成 TOTP secret
	secret := "JBSWY3DPEHPK3PXP" // 测试用固定 secret

	settings := map[string]interface{}{
		"totp_secret":     secret,
		"totp_verified":   false,
		"totp_enabled":    false,
		"totp_enabled_at": nil,
	}
	settingsJSON, _ := json.Marshal(settings)

	svc.db.Table("users_accounts").Create(map[string]interface{}{
		"id":       "user-2fa",
		"settings": string(settingsJSON),
	})

	// 生成有效的 TOTP 码
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	// 验证 2FA
	err = svc.Verify2FA("user-2fa", code)
	assert.NoError(t, err)

	// 验证已启用
	var user struct {
		Settings string
	}
	svc.db.Table("users_accounts").Where("id = ?", "user-2fa").First(&user)

	var updatedSettings map[string]interface{}
	json.Unmarshal([]byte(user.Settings), &updatedSettings)
	// Verify2FA 只设置 totp_enabled，不设置 totp_verified
	assert.True(t, updatedSettings["totp_enabled"].(bool))
}

func TestService_Verify2FA_InvalidCode(t *testing.T) {
	svc := setupTestService(t)

	type UserAccount struct {
		ID       string `gorm:"primaryKey"`
		Settings string `gorm:"type:text"`
	}
	svc.db.Table("users_accounts").AutoMigrate(&UserAccount{})

	secret := "JBSWY3DPEHPK3PXP"
	settings := map[string]interface{}{
		"totp_secret":   secret,
		"totp_verified": false,
	}
	settingsJSON, _ := json.Marshal(settings)

	svc.db.Table("users_accounts").Create(map[string]interface{}{
		"id":       "user-2fa-invalid",
		"settings": string(settingsJSON),
	})

	// 使用无效码
	err := svc.Verify2FA("user-2fa-invalid", "000000")
	assert.Error(t, err)
}

// ----- parseCompletedSteps 测试 -----

func TestService_ParseCompletedSteps(t *testing.T) {
	svc := setupTestService(t)

	tests := []struct {
		input    string
		expected []int
	}{
		{"[]", []int{}},
		{"[1]", []int{1}},
		{"[1,2,3]", []int{1, 2, 3}},
		{"[1, 2, 3]", []int{1, 2, 3}},
		{"invalid", nil},
		{"", nil},
	}

	for _, tt := range tests {
		result := svc.parseCompletedSteps(tt.input)
		assert.Equal(t, tt.expected, result, "input: %s", tt.input)
	}
}

// ----- getOrCreateSettings 测试 -----

func TestService_GetOrCreateSettings_CreatesDefault(t *testing.T) {
	svc := setupTestService(t)

	settings, err := svc.getOrCreateSettings()
	require.NoError(t, err)

	assert.False(t, settings.SetupCompleted)
	assert.Equal(t, 1, settings.CurrentStep)
	assert.Equal(t, "zh-CN", settings.Language)
	assert.Equal(t, "Asia/Shanghai", settings.Timezone)
	assert.NotNil(t, settings.StartedAt)
}

func TestService_GetOrCreateSettings_ReturnExisting(t *testing.T) {
	svc := setupTestService(t)

	// 第一次调用创建
	settings1, err := svc.getOrCreateSettings()
	require.NoError(t, err)

	// 修改
	svc.db.Model(&SetupSettings{}).Where("id = ?", settings1.ID).Update("language", "en-US")

	// 第二次调用应返回已存在的
	settings2, err := svc.getOrCreateSettings()
	require.NoError(t, err)

	assert.Equal(t, settings1.ID, settings2.ID)
	assert.Equal(t, "en-US", settings2.Language)
}
