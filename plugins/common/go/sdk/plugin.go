// Package sdk 提供 RDE 插件开发的标准化辅助库
// 封装了 Unix Socket 服务器、健康检查、信号处理等通用逻辑
// 插件开发者只需关注业务逻辑
package sdk

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// PluginModule 定义插件中的一个功能模块
type PluginModule interface {
	// ID 模块唯一标识
	ID() string

	// Init 初始化模块
	Init(ctx *PluginContext) error

	// Start 启动模块（后台任务等）
	Start() error

	// Stop 停止模块
	Stop() error

	// RegisterRoutes 注册模块路由到 /api/v1 组
	RegisterRoutes(group *gin.RouterGroup)
}

// PluginContext 传递给模块的上下文
type PluginContext struct {
	Logger  *zap.Logger
	DataDir string   // 插件数据目录
	BaseDir string   // RDE 安装目录
	DB      *gorm.DB // 可选：共享 SQLite 数据库
	Debug   bool
}

// Plugin 插件实例，管理多个 PluginModule
type Plugin struct {
	name    string
	version string

	// 命令行参数
	socketPath string
	dataDir    string
	baseDir    string
	debug      bool

	logger  *zap.Logger
	router  *gin.Engine
	modules []PluginModule
	server  *http.Server

	// 可选数据库
	enableDB bool
	db       *gorm.DB

	// 自定义中间件（如 license 检查）
	apiMiddlewares []gin.HandlerFunc

	// 前端静态文件目录，key 为路由前缀，value 为文件系统路径
	frontendDirs map[string]string

	// 嵌入的前端文件系统，key 为路由前缀
	frontendFSMap map[string]fs.FS
}

// New 创建插件实例
func New(name, version string) *Plugin {
	return &Plugin{
		name:         name,
		version:      version,
		frontendDirs: make(map[string]string),
		frontendFSMap: make(map[string]fs.FS),
	}
}

// UseAPIMiddleware 添加 API 路由组中间件（在模块路由注册前调用）
// 典型用例：License 验证中间件
func (p *Plugin) UseAPIMiddleware(middleware ...gin.HandlerFunc) {
	p.apiMiddlewares = append(p.apiMiddlewares, middleware...)
}

// EnableDB 启用 SQLite 数据库支持
// 数据库文件存储在 {dataDir}/plugin.db
func (p *Plugin) EnableDB() {
	p.enableDB = true
}

// AddModule 注册功能模块
func (p *Plugin) AddModule(m PluginModule) {
	p.modules = append(p.modules, m)
}

// ServeFrontend 注册前端静态文件服务
// routePrefix 是路由路径（如 "/app/ai"），dir 是前端构建输出目录
// 前端 SPA 的入口 index.html 将作为回退提供（SPA 路由支持）
func (p *Plugin) ServeFrontend(routePrefix, dir string) {
	p.frontendDirs[routePrefix] = dir
}

// ServeFrontendFS 注册嵌入的前端静态文件服务
// routePrefix 是路由路径（如 "/app/android"），fsys 是 embed.FS 或其子目录
func (p *Plugin) ServeFrontendFS(routePrefix string, fsys fs.FS) {
	p.frontendFSMap[routePrefix] = fsys
}

// GetLogger 获取日志实例
func (p *Plugin) GetLogger() *zap.Logger {
	return p.logger
}

// GetDataDir 获取数据目录
func (p *Plugin) GetDataDir() string {
	return p.dataDir
}

// Run 解析参数、初始化、启动服务、阻塞等待退出
// 这是插件的主入口，调用后不会返回（除非出错）
func (p *Plugin) Run() error {
	// 解析命令行参数
	flag.StringVar(&p.socketPath, "socket", "/var/run/rde/plugins/plugin.sock", "Unix socket path")
	flag.StringVar(&p.dataDir, "data-dir", "/var/lib/rde/plugin-data/plugin", "Plugin data directory")
	flag.StringVar(&p.baseDir, "base-dir", "/opt/rde", "RDE base directory")
	flag.BoolVar(&p.debug, "debug", false, "Enable debug mode")
	flag.Parse()

	// 初始化日志
	var err error
	if p.debug {
		p.logger, err = zap.NewDevelopment()
	} else {
		p.logger, err = zap.NewProduction()
	}
	if err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}
	defer p.logger.Sync()

	p.logger.Info("Plugin starting",
		zap.String("name", p.name),
		zap.String("version", p.version),
		zap.String("socket", p.socketPath),
		zap.String("data_dir", p.dataDir),
	)

	// 设置 Gin
	if !p.debug {
		gin.SetMode(gin.ReleaseMode)
	}
	p.router = gin.New()
	p.router.Use(gin.Recovery())

	// 健康检查端点（不需要认证）
	p.router.GET("/health", p.handleHealth)

	// API 路由组
	api := p.router.Group("/api/v1")
	for _, mw := range p.apiMiddlewares {
		api.Use(mw)
	}

	// 初始化并注册模块
	ctx := &PluginContext{
		Logger:  p.logger,
		DataDir: p.dataDir,
		BaseDir: p.baseDir,
		Debug:   p.debug,
	}

	// 可选：初始化 SQLite
	if p.enableDB {
		dbPath := filepath.Join(p.dataDir, "plugin.db")
		gormCfg := &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		}
		if p.debug {
			gormCfg.Logger = gormlogger.Default.LogMode(gormlogger.Info)
		}
		p.db, err = gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL"), gormCfg)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		ctx.DB = p.db
		p.logger.Info("Database initialized", zap.String("path", dbPath))
	}

	for _, m := range p.modules {
		moduleLogger := p.logger.Named(m.ID())
		moduleCtx := &PluginContext{
			Logger:  moduleLogger,
			DataDir: p.dataDir,
			BaseDir: p.baseDir,
			DB:      p.db,
			Debug:   ctx.Debug,
		}

		if err := m.Init(moduleCtx); err != nil {
			p.logger.Error("Failed to init module",
				zap.String("module", m.ID()),
				zap.Error(err))
			continue
		}

		if err := m.Start(); err != nil {
			p.logger.Error("Failed to start module",
				zap.String("module", m.ID()),
				zap.Error(err))
			continue
		}

		m.RegisterRoutes(api)
		p.logger.Info("Module registered",
			zap.String("module", m.ID()))
	}

	// 注册前端静态文件路由
	for routePrefix, dir := range p.frontendDirs {
		p.registerFrontendRoute(api, routePrefix, dir)
	}
	for routePrefix, fsys := range p.frontendFSMap {
		p.registerFrontendFSRoute(api, routePrefix, fsys)
	}

	// 创建 Unix Socket
	socketDir := filepath.Dir(p.socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}
	os.Remove(p.socketPath)

	listener, err := net.Listen("unix", p.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket %s: %w", p.socketPath, err)
	}
	defer listener.Close()

	os.Chmod(p.socketPath, 0660)

	p.logger.Info("Plugin started",
		zap.String("socket", p.socketPath))

	// 启动 HTTP 服务
	p.server = &http.Server{Handler: p.router}
	go func() {
		if err := p.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			p.logger.Fatal("server error", zap.Error(err))
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	p.logger.Info("Shutting down...")

	// 优雅关闭
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.server.Shutdown(shutdownCtx)

	// 停止模块（逆序）
	for i := len(p.modules) - 1; i >= 0; i-- {
		p.modules[i].Stop()
	}

	p.logger.Info("Plugin stopped")
	return nil
}

// handleHealth 健康检查端点
func (p *Plugin) handleHealth(c *gin.Context) {
	moduleStatuses := make([]gin.H, 0, len(p.modules))
	for _, m := range p.modules {
		moduleStatuses = append(moduleStatuses, gin.H{
			"id": m.ID(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"name":    p.name,
		"version": p.version,
		"modules": moduleStatuses,
	})
}

// registerFrontendRoute 注册前端 SPA 静态文件服务
// 支持 SPA 回退：未匹配到静态文件时返回 index.html
func (p *Plugin) registerFrontendRoute(api *gin.RouterGroup, routePrefix, dir string) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		p.logger.Warn("Invalid frontend directory",
			zap.String("route", routePrefix),
			zap.String("dir", dir),
			zap.Error(err))
		return
	}

	// 检查目录是否存在
	if _, err := os.Stat(absDir); os.IsNotExist(err) {
		p.logger.Warn("Frontend directory not found",
			zap.String("route", routePrefix),
			zap.String("dir", absDir))
		return
	}

	// 注册 GET 路由处理静态文件
	route := routePrefix + "/*filepath"
	api.GET(route, func(c *gin.Context) {
		filePath := c.Param("filepath")
		if filePath == "" || filePath == "/" {
			filePath = "/index.html"
		}

		// 安全检查：防止目录遍历
		fullPath := filepath.Join(absDir, filepath.Clean(filePath))
		if !strings.HasPrefix(fullPath, absDir) {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid path"})
			return
		}

		// 尝试提供静态文件
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			// 不可变资源（带 hash 的构建产物）永久缓存
			if strings.Contains(filePath, "/assets/") || strings.Contains(filePath, "/immutable/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else if strings.HasSuffix(filePath, ".html") {
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			c.File(fullPath)
			return
		}

		// SPA 回退：返回 index.html
		indexPath := filepath.Join(absDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.File(indexPath)
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	p.logger.Info("Frontend route registered",
		zap.String("route", routePrefix),
		zap.String("dir", absDir))
}

// registerFrontendFSRoute 注册嵌入式前端 SPA 静态文件服务
func (p *Plugin) registerFrontendFSRoute(api *gin.RouterGroup, routePrefix string, fsys fs.FS) {
	route := routePrefix + "/*filepath"
	api.GET(route, func(c *gin.Context) {
		filePath := c.Param("filepath")
		if filePath == "" || filePath == "/" {
			filePath = "/index.html"
		}
		filePath = strings.TrimPrefix(filePath, "/")

		// 尝试提供静态文件
		f, err := fsys.Open(filePath)
		if err == nil {
			defer f.Close()
			stat, statErr := f.Stat()
			if statErr == nil && !stat.IsDir() {
				// 缓存策略
				if strings.Contains(filePath, "/assets/") || strings.Contains(filePath, "/immutable/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				} else if strings.HasSuffix(filePath, ".html") {
					c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				}
				http.ServeContent(c.Writer, c.Request, stat.Name(), stat.ModTime(), f.(io.ReadSeeker))
				return
			}
		}

		// SPA 回退：返回 index.html
		indexFile, err := fsys.Open("index.html")
		if err == nil {
			defer indexFile.Close()
			stat, _ := indexFile.Stat()
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), indexFile.(io.ReadSeeker))
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	p.logger.Info("Frontend FS route registered",
		zap.String("route", routePrefix))
}
