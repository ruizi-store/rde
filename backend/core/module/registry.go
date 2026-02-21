package module

import (
	"fmt"
	"sort"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Registry 是模块注册中心，负责管理所有模块的生命周期
type Registry struct {
	mu      sync.RWMutex
	modules map[string]Module
	order   []string // 按依赖顺序排列的模块 ID
	ctx     *Context
	logger  *zap.Logger
	started bool
}

// NewRegistry 创建新的模块注册中心
// extra 参数用于传递额外的组件（如 TokenManager）
func NewRegistry(db *gorm.DB, config ConfigProvider, eventBus EventBus, logger *zap.Logger, extra map[string]interface{}) *Registry {
	r := &Registry{
		modules: make(map[string]Module),
		order:   make([]string, 0),
		logger:  logger,
	}

	if extra == nil {
		extra = make(map[string]interface{})
	}

	r.ctx = &Context{
		DB:        db,
		Config:    config,
		EventBus:  eventBus,
		Logger:    logger,
		GetModule: r.Get,
		Extra:     extra,
	}

	return r
}

// Register 注册一个模块
// 注意：只能在 Start 调用前注册模块
func (r *Registry) Register(m Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return fmt.Errorf("cannot register module after registry started")
	}

	id := m.ID()
	if _, exists := r.modules[id]; exists {
		return fmt.Errorf("module %s already registered", id)
	}

	r.modules[id] = m
	r.logger.Info("Module registered", zap.String("id", id), zap.String("name", m.Name()))
	return nil
}

// Get 获取已注册的模块
func (r *Registry) Get(id string) Module {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.modules[id]
}

// getUnsafe 不加锁获取模块（仅内部使用）
func (r *Registry) getUnsafe(id string) Module {
	return r.modules[id]
}

// GetAll 获取所有已注册模块的信息
func (r *Registry) GetAll() []ModuleInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]ModuleInfo, 0, len(r.modules))
	for _, m := range r.modules {
		status := "stopped"
		if r.started {
			status = "running"
		}
		infos = append(infos, ModuleInfo{
			ID:           m.ID(),
			Name:         m.Name(),
			Version:      m.Version(),
			Dependencies: m.Dependencies(),
			Status:       status,
		})
	}
	return infos
}

// Start 按依赖顺序初始化并启动所有模块
func (r *Registry) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return fmt.Errorf("registry already started")
	}

	// 1. 拓扑排序，确定初始化顺序
	order, err := r.topologicalSort()
	if err != nil {
		return fmt.Errorf("failed to resolve module dependencies: %w", err)
	}
	r.order = order

	r.logger.Info("Module initialization order", zap.Strings("order", order))

	// 2. 按顺序初始化模块
	for _, id := range order {
		m := r.modules[id]
		moduleLogger := r.logger.With(zap.String("module", id))
		moduleCtx := &Context{
			DB:        r.ctx.DB,
			Config:    r.ctx.Config,
			EventBus:  r.ctx.EventBus,
			Logger:    moduleLogger,
			GetModule: r.getUnsafe, // 使用不加锁版本避免死锁
			Extra:     r.ctx.Extra, // 传递额外组件
		}

		r.logger.Info("Initializing module", zap.String("id", id))
		if err := m.Init(moduleCtx); err != nil {
			return fmt.Errorf("failed to initialize module %s: %w", id, err)
		}
	}

	// 3. 按顺序启动模块
	for _, id := range order {
		m := r.modules[id]
		r.logger.Info("Starting module", zap.String("id", id))
		if err := m.Start(); err != nil {
			return fmt.Errorf("failed to start module %s: %w", id, err)
		}
	}

	r.started = true
	r.logger.Info("All modules started", zap.Int("count", len(order)))
	return nil
}

// Stop 按依赖逆序停止所有模块
func (r *Registry) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return nil
	}

	// 逆序停止
	for i := len(r.order) - 1; i >= 0; i-- {
		id := r.order[i]
		m := r.modules[id]
		r.logger.Info("Stopping module", zap.String("id", id))
		if err := m.Stop(); err != nil {
			r.logger.Error("Failed to stop module", zap.String("id", id), zap.Error(err))
			// 继续停止其他模块
		}
	}

	r.started = false
	r.logger.Info("All modules stopped")
	return nil
}

// RegisterRoutes 注册所有模块的路由
func (r *Registry) RegisterRoutes(router *gin.RouterGroup) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, id := range r.order {
		m := r.modules[id]
		r.logger.Debug("Registering routes for module", zap.String("id", id))
		m.RegisterRoutes(router)
	}
}

// topologicalSort 对模块进行拓扑排序
// 确保依赖的模块排在前面
func (r *Registry) topologicalSort() ([]string, error) {
	// 构建依赖图
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for id := range r.modules {
		inDegree[id] = 0
	}

	for id, m := range r.modules {
		for _, dep := range m.Dependencies() {
			if _, exists := r.modules[dep]; !exists {
				return nil, fmt.Errorf("module %s depends on unregistered module %s", id, dep)
			}
			inDegree[id]++
			dependents[dep] = append(dependents[dep], id)
		}
	}

	// Kahn's algorithm
	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// 排序保证稳定顺序
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		// 取出入度为 0 的节点
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// 更新依赖此模块的节点的入度
		deps := dependents[current]
		sort.Strings(deps)
		for _, dep := range deps {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(result) != len(r.modules) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return result, nil
}
