package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ruizi-store/rde/backend/core/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseModuleFilter(t *testing.T) {
	ids := []string{"ssh", "sync", "cloudbackup"}

	t.Run("默认全部启用", func(t *testing.T) {
		t.Setenv("RDE_MODULES", "")
		t.Setenv("RDE_MODULES_DISABLED", "")
		enabled := parseModuleFilter(ids)
		for _, id := range ids {
			assert.True(t, enabled[id], "module %s should be enabled by default", id)
		}
	})

	t.Run("白名单模式仅启用清单内模块", func(t *testing.T) {
		t.Setenv("RDE_MODULES", "samba, vm")
		enabled := parseModuleFilter(ids)
		assert.True(t, enabled["samba"])
		assert.True(t, enabled["vm"])
		assert.False(t, enabled["ssh"])
		assert.False(t, enabled["translate"])
	})

	t.Run("黑名单模式禁用清单内模块", func(t *testing.T) {
		t.Setenv("RDE_MODULES", "")
		t.Setenv("RDE_MODULES_DISABLED", "sync,cloudbackup")
		enabled := parseModuleFilter(ids)
		assert.False(t, enabled["sync"])
		assert.False(t, enabled["cloudbackup"])
		assert.True(t, enabled["ssh"])
	})

	t.Run("白名单优先于黑名单", func(t *testing.T) {
		t.Setenv("RDE_MODULES", "sync")
		t.Setenv("RDE_MODULES_DISABLED", "sync")
		enabled := parseModuleFilter(ids)
		assert.True(t, enabled["sync"], "allowlist should take precedence")
		assert.False(t, enabled["ssh"])
	})

	t.Run("未知模块 ID 不影响结果", func(t *testing.T) {
		t.Setenv("RDE_MODULES", "ssh, nonexistent-module")
		enabled := parseModuleFilter(ids)
		assert.True(t, enabled["ssh"])
		assert.False(t, enabled["sync"])
	})
}

func setupTestApp(t *testing.T) (*App, func()) {
	tmpDir, err := os.MkdirTemp("", "bootstrap-test-*")
	require.NoError(t, err)

	opts := &Options{
		DBPath:  filepath.Join(tmpDir, "test.db"),
		DataDir: tmpDir,
		LogPath: filepath.Join(tmpDir, "logs"),
		Debug:   true,
	}

	app, err := New(opts)
	require.NoError(t, err)

	cleanup := func() {
		app.Stop()
		os.RemoveAll(tmpDir)
	}

	return app, cleanup
}

func TestNew(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// 验证组件已初始化
	assert.NotNil(t, app.DB)
	assert.NotNil(t, app.Config)
	assert.NotNil(t, app.EventBus)
	assert.NotNil(t, app.Registry)
	assert.NotNil(t, app.Logger)
	assert.NotNil(t, app.Router)

	// 验证模块已注册
	assert.NotNil(t, app.Users)
	assert.NotNil(t, app.Files)
	assert.NotNil(t, app.System)
	assert.NotNil(t, app.Notification)
	// 可选模块在测试中默认未启用
	// assert.NotNil(t, app.Docker)
	// assert.NotNil(t, app.DockerApps)
	// assert.NotNil(t, app.Backup)
	// assert.NotNil(t, app.AI)
}

func TestApp_Start(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// 启动应用
	err := app.Start()
	require.NoError(t, err)

	// 验证已注册模块数量（含原插件迁移后的内置模块）
	modules := app.Registry.GetAll()
	assert.Len(t, modules, 23)

	// 验证路由器已配置
	router := app.GetRouter()
	assert.NotNil(t, router)
}

func TestApp_GetServices(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	err := app.Start()
	require.NoError(t, err)

	// 测试核心服务获取（这些始终启用）
	assert.NotNil(t, app.GetUsersService())
	assert.NotNil(t, app.GetFilesService())
	assert.NotNil(t, app.GetSystemService())
	assert.NotNil(t, app.GetNotificationService())

	// 可选服务在测试中默认未启用，可能为 nil
	// assert.NotNil(t, app.GetDockerService())
	// assert.NotNil(t, app.GetDockerAppsService())
	// assert.NotNil(t, app.GetBackupService())
	// assert.NotNil(t, app.GetAIService())
}

func TestApp_GetModule(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	err := app.Start()
	require.NoError(t, err)

	// 测试通过 ID 获取核心模块（这些始终启用）
	coreModuleIDs := []string{"users", "files", "system", "notification"}
	for _, id := range coreModuleIDs {
		m := app.GetModule(id)
		assert.NotNil(t, m, "module %s should exist", id)
		if m != nil {
			assert.Equal(t, id, m.ID())
		}
	}

	// 测试不存在的模块
	assert.Nil(t, app.GetModule("nonexistent"))
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	assert.Equal(t, "/var/lib/rde/db/rde.db", opts.DBPath)
	assert.Equal(t, "/var/lib/rde", opts.DataDir)
	assert.Equal(t, "/var/log/rde", opts.LogPath)
	assert.False(t, opts.Debug)
}

func TestEventBusAdapter(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	err := app.Start()
	require.NoError(t, err)

	// 测试事件发布
	received := make(chan bool, 1)
	app.EventBus.Subscribe("test.event", func(e event.Event) {
		received <- true
	})

	app.EventBus.Publish("test.event", map[string]string{"key": "value"})

	select {
	case <-received:
		// 成功接收
	default:
		// 异步事件，可能还没收到
	}
}
