package module

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockModule 用于测试的模拟模块
type mockModule struct {
	id           string
	name         string
	version      string
	dependencies []string
	initCalled   bool
	startCalled  bool
	stopCalled   bool
	initOrder    *[]string
}

func newMockModule(id string, deps ...string) *mockModule {
	return &mockModule{
		id:           id,
		name:         id + " Module",
		version:      "1.0.0",
		dependencies: deps,
	}
}

func (m *mockModule) ID() string                             { return m.id }
func (m *mockModule) Name() string                           { return m.name }
func (m *mockModule) Version() string                        { return m.version }
func (m *mockModule) Dependencies() []string                 { return m.dependencies }
func (m *mockModule) RegisterRoutes(router *gin.RouterGroup) {}

func (m *mockModule) Init(ctx *Context) error {
	m.initCalled = true
	if m.initOrder != nil {
		*m.initOrder = append(*m.initOrder, m.id)
	}
	return nil
}

func (m *mockModule) Start() error {
	m.startCalled = true
	return nil
}

func (m *mockModule) Stop() error {
	m.stopCalled = true
	return nil
}

func TestRegistry_Register(t *testing.T) {
	logger := zap.NewNop()
	r := NewRegistry(nil, nil, nil, logger, nil)

	m := newMockModule("test")
	err := r.Register(m)
	require.NoError(t, err)

	// 重复注册应该失败
	err = r.Register(m)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Get(t *testing.T) {
	logger := zap.NewNop()
	r := NewRegistry(nil, nil, nil, logger, nil)

	m := newMockModule("test")
	_ = r.Register(m)

	got := r.Get("test")
	assert.Equal(t, m, got)

	// 不存在的模块
	got = r.Get("nonexistent")
	assert.Nil(t, got)
}

func TestRegistry_TopologicalSort(t *testing.T) {
	logger := zap.NewNop()
	r := NewRegistry(nil, nil, nil, logger, nil)

	// 模块依赖关系:
	// files -> users
	// docker-apps -> docker
	// ai -> files, users
	_ = r.Register(newMockModule("users"))
	_ = r.Register(newMockModule("docker"))
	_ = r.Register(newMockModule("files", "users"))
	_ = r.Register(newMockModule("docker-apps", "docker"))
	_ = r.Register(newMockModule("ai", "files", "users"))

	order, err := r.topologicalSort()
	require.NoError(t, err)

	// 验证顺序
	indexOf := func(id string) int {
		for i, v := range order {
			if v == id {
				return i
			}
		}
		return -1
	}

	// users 必须在 files 之前
	assert.Less(t, indexOf("users"), indexOf("files"))
	// users 必须在 ai 之前
	assert.Less(t, indexOf("users"), indexOf("ai"))
	// files 必须在 ai 之前
	assert.Less(t, indexOf("files"), indexOf("ai"))
	// docker 必须在 docker-apps 之前
	assert.Less(t, indexOf("docker"), indexOf("docker-apps"))
}

func TestRegistry_CircularDependency(t *testing.T) {
	logger := zap.NewNop()
	r := NewRegistry(nil, nil, nil, logger, nil)

	// 循环依赖: a -> b -> c -> a
	_ = r.Register(newMockModule("a", "c"))
	_ = r.Register(newMockModule("b", "a"))
	_ = r.Register(newMockModule("c", "b"))

	_, err := r.topologicalSort()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestRegistry_MissingDependency(t *testing.T) {
	logger := zap.NewNop()
	r := NewRegistry(nil, nil, nil, logger, nil)

	// 依赖不存在的模块
	_ = r.Register(newMockModule("a", "nonexistent"))

	_, err := r.topologicalSort()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unregistered module")
}

func TestRegistry_StartStop(t *testing.T) {
	logger := zap.NewNop()
	r := NewRegistry(nil, nil, nil, logger, nil)

	initOrder := make([]string, 0)

	m1 := newMockModule("users")
	m1.initOrder = &initOrder
	m2 := newMockModule("files", "users")
	m2.initOrder = &initOrder

	_ = r.Register(m1)
	_ = r.Register(m2)

	// 启动
	err := r.Start()
	require.NoError(t, err)

	// 验证初始化顺序
	assert.Equal(t, []string{"users", "files"}, initOrder)

	// 验证所有模块都被初始化和启动
	assert.True(t, m1.initCalled)
	assert.True(t, m1.startCalled)
	assert.True(t, m2.initCalled)
	assert.True(t, m2.startCalled)

	// 停止
	err = r.Stop()
	require.NoError(t, err)

	assert.True(t, m1.stopCalled)
	assert.True(t, m2.stopCalled)
}

func TestRegistry_CannotRegisterAfterStart(t *testing.T) {
	logger := zap.NewNop()
	r := NewRegistry(nil, nil, nil, logger, nil)

	_ = r.Register(newMockModule("a"))
	_ = r.Start()

	err := r.Register(newMockModule("b"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot register")
}
