// Package bootstrap 提供应用启动引导功能
// 负责初始化核心服务和加载所有模块
package bootstrap

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/auth"
	"github.com/ruizi-store/rde/backend/core/config"
	"github.com/ruizi-store/rde/backend/core/database"
	"github.com/ruizi-store/rde/backend/core/event"
	"github.com/ruizi-store/rde/backend/core/i18n"
	"github.com/ruizi-store/rde/backend/core/module"
	"github.com/ruizi-store/rde/backend/core/plugin"

	// 导入核心模块
	"github.com/ruizi-store/rde/backend/modules/backup"
	"github.com/ruizi-store/rde/backend/modules/docker"
	"github.com/ruizi-store/rde/backend/modules/download"
	"github.com/ruizi-store/rde/backend/modules/files"
	"github.com/ruizi-store/rde/backend/modules/flatpak"
	"github.com/ruizi-store/rde/backend/modules/linuxlab"
	"github.com/ruizi-store/rde/backend/modules/notification"
	"github.com/ruizi-store/rde/backend/modules/photos"
	"github.com/ruizi-store/rde/backend/modules/retrogame"
	"github.com/ruizi-store/rde/backend/modules/samba"
	"github.com/ruizi-store/rde/backend/modules/setup"
	"github.com/ruizi-store/rde/backend/modules/ssh"
	"github.com/ruizi-store/rde/backend/modules/sudo"
	"github.com/ruizi-store/rde/backend/modules/sync"
	"github.com/ruizi-store/rde/backend/modules/system"
	"github.com/ruizi-store/rde/backend/modules/terminal"
	"github.com/ruizi-store/rde/backend/modules/users"
	"github.com/ruizi-store/rde/backend/modules/video"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 代表应用实例
type App struct {
	DB           *gorm.DB
	dbMgr        *database.Manager
	Config       *config.Config
	EventBus     *event.Bus
	Registry     *module.Registry
	Logger       *zap.Logger
	Router       *gin.Engine
	TokenManager *auth.TokenManager

	// 核心模块快捷访问
	Setup        *setup.Module
	Users        *users.Module
	Files        *files.Module
	System       *system.Module
	Notification *notification.Module
	Sudo         *sudo.Module
	Terminal     *terminal.Module
	Docker       *docker.Module
	Samba        *samba.Module
	SSH          *ssh.Module
	Sync         *sync.Module
	Download     *download.Module
	Flatpak      *flatpak.Module
	Retrogame    *retrogame.Module
	Photos       *photos.Module
	Video        *video.Module
	Backup       *backup.Module
	LinuxLab     *linuxlab.Module

	// 插件管理器
	PluginManager *plugin.Manager
}

// Options 启动选项
type Options struct {
	// DBPath 数据库文件路径
	DBPath string

	// DataDir 数据目录
	DataDir string

	// LogPath 日志路径
	LogPath string

	// Debug 是否开启调试模式
	Debug bool
}

// DefaultOptions 返回默认选项
func DefaultOptions() *Options {
	return &Options{
		DBPath:  "/var/lib/rde/db/rde.db",
		DataDir: "/var/lib/rde",
		LogPath: "/var/log/rde",
		Debug:   false,
	}
}

// New 创建并初始化应用
func New(opts *Options) (*App, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	// 1. 初始化日志
	var logger *zap.Logger
	var err error
	if opts.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("Initializing RDE application",
		zap.String("data_dir", opts.DataDir),
		zap.String("db_path", opts.DBPath),
		zap.Bool("debug", opts.Debug),
	)

	// 2. 初始化配置
	cfg := config.New()
	cfg.Set("data_dir", opts.DataDir)
	cfg.Set("db_path", opts.DBPath)
	cfg.Set("log_path", opts.LogPath)
	cfg.Set("debug", opts.Debug)

	// 设置各模块数据目录
	cfg.Set("files.root_path", filepath.Join(opts.DataDir, "files"))
	cfg.Set("appstore.index_url", "https://ruizi.io/api/appstore")

	// 3. 初始化数据库
	dbMgr, err := database.New(database.Config{
		Path:  opts.DBPath,
		Debug: opts.Debug,
	}, logger)
	if err != nil {
		logger.Error("Failed to open database", zap.Error(err))
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 4. 创建事件总线
	eventBus := event.NewBus(logger)

	// 5. 创建事件总线适配器（实现 module.EventBus 接口）
	eventBusAdapter := &eventBusAdapter{bus: eventBus}

	// 6. 创建 JWT TokenManager
	// 优先从配置文件读取 JWT 密钥，支持 TOML [auth] 段
	jwtSecret := cfg.GetString("auth.jwt_secret")
	if jwtSecret == "" {
		jwtSecret = cfg.GetString("jwt.secret")
	}
	if jwtSecret == "" || jwtSecret == "rde-default-secret-change-me" {
		// 尝试从持久化文件读取
		secretFile := filepath.Join(opts.DataDir, "jwt_secret")
		if data, err := os.ReadFile(secretFile); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			jwtSecret = strings.TrimSpace(string(data))
			logger.Info("JWT secret loaded from persistent file")
		} else {
			// 首次启动：随机生成并持久化
			b := make([]byte, 32)
			if _, err := rand.Read(b); err == nil {
				jwtSecret = fmt.Sprintf("%x", b)
			} else {
				jwtSecret = fmt.Sprintf("rde-auto-%d", time.Now().UnixNano())
			}
			os.MkdirAll(opts.DataDir, 0o755)
			if err := os.WriteFile(secretFile, []byte(jwtSecret), 0o600); err != nil {
				logger.Warn("Failed to persist JWT secret", zap.Error(err))
			} else {
				logger.Info("JWT secret generated and persisted")
			}
		}
	}
	tokenManager := auth.NewTokenManager(&auth.Config{
		JWTSecret:     jwtSecret,
		AccessExpiry:  24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
		Issuer:        "rde",
	})

	// 6.1 创建持久化令牌黑名单
	tokenBlacklist := auth.NewPersistentBlacklist(dbMgr.DB())

	// 7. 创建模块注册中心（传递 TokenManager 等额外组件）
	extra := map[string]interface{}{
		"tokenManager":   tokenManager,
		"tokenBlacklist": tokenBlacklist,
	}
	registry := module.NewRegistry(dbMgr.DB(), cfg, eventBusAdapter, logger, extra)

	// 8. 创建 Gin 路由器
	if !opts.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	if opts.Debug {
		router.Use(gin.Logger())
	}

	// 添加 CORS 中间件
	router.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	app := &App{
		DB:           dbMgr.DB(),
		dbMgr:        dbMgr,
		Config:       cfg,
		EventBus:     eventBus,
		Registry:     registry,
		Logger:       logger,
		Router:       router,
		TokenManager: tokenManager,
	}

	// 9. 初始化插件管理器
	pluginDir := filepath.Join(opts.DataDir, "plugins")
	socketDir := filepath.Join("/var/run/rde", "plugins")
	baseDir := "/opt/rde"
	if bd := cfg.GetString("base_dir"); bd != "" {
		baseDir = bd
	}
	app.PluginManager = plugin.NewManager(pluginDir, socketDir, opts.DataDir, baseDir, opts.Debug, logger, cfg)

	// 10. 注册所有内置模块
	if err := app.registerModules(); err != nil {
		return nil, err
	}

	return app, nil
}

// eventBusAdapter 适配 event.Bus 到 module.EventBus 接口
type eventBusAdapter struct {
	bus *event.Bus
}

func (a *eventBusAdapter) Publish(eventType string, data interface{}) {
	a.bus.Publish(eventType, data)
}

func (a *eventBusAdapter) Subscribe(eventType string, handler module.EventHandler) {
	a.bus.Subscribe(eventType, func(e event.Event) {
		handler(module.Event{
			Type:      e.Type,
			Source:    e.Source,
			Data:      e.Data,
			Timestamp: e.Timestamp,
		})
	})
}

func (a *eventBusAdapter) Unsubscribe(eventType string, handler module.EventHandler) {
	// 简化实现：事件总线使用返回的取消函数来取消订阅
	// 这里不做处理，因为实际使用中很少需要取消订阅
}

// registerModules 注册所有功能模块
func (app *App) registerModules() error {
	// 创建核心模块实例
	app.Setup = setup.NewModule()
	app.Users = users.NewModule()
	app.Files = files.New()
	app.System = system.NewModule()
	app.Notification = notification.NewModule()
	app.Sudo = sudo.NewModule().(*sudo.Module)
	app.Terminal = terminal.New()
	app.Docker = docker.New()
	app.Samba = samba.New()
	app.SSH = ssh.New()
	app.Sync = sync.New()
	app.Download = download.New()
	app.Flatpak = flatpak.New()
	app.Retrogame = retrogame.New()
	app.Photos = photos.New()
	app.Video = video.New()
	app.Backup = backup.New()
	app.LinuxLab = linuxlab.New()

	// 核心模块（始终加载）
	coreModules := []module.Module{
		app.Setup,
		app.Users,
		app.Files,
		app.System,
		app.Notification,
		app.Sudo,
		app.Terminal,
		app.Docker,
		app.Samba,
		app.SSH,
		app.Sync,
		app.Download,
		app.Flatpak,
		app.Retrogame,
		app.Photos,
		app.Video,
		app.Backup,
		app.LinuxLab,
	}

	// 注册核心模块
	for _, m := range coreModules {
		if err := app.Registry.Register(m); err != nil {
			return fmt.Errorf("failed to register core module %s: %w", m.ID(), err)
		}
		app.Logger.Info("Core module registered", zap.String("id", m.ID()))
	}

	return nil
}

// Start 启动应用
func (app *App) Start() error {
	app.Logger.Info("Starting RDE application")

	// 0. 加载用户 i18n 区域设置（在模块启动前，确保镜像源配置正确）
	app.loadI18nRegion()

	// 1. 启动所有模块
	if err := app.Registry.Start(); err != nil {
		return fmt.Errorf("failed to start modules: %w", err)
	}

	// 2. 注册模块路由
	apiV1 := app.Router.Group("/api/v1")

	// 添加 JWT 认证中间件（跳过公开路由）
	authMiddleware := auth.MiddlewareWithConfig(app.TokenManager, &auth.MiddlewareConfig{
		Skipper: func(c *gin.Context) bool {
			path := c.Request.URL.Path
			method := c.Request.Method

			// 公开路由 - 不需要认证
			publicPaths := []string{
				"/api/v1/auth/login",
				"/api/v1/auth/refresh",
				"/api/v1/auth/verify-2fa",
				"/api/v1/setup/status",
				"/api/v1/setup/complete",
				"/api/v1/setup/initialize",
				"/api/v1/system/i18n", // 登录页面需要获取i18n设置
			}

			for _, p := range publicPaths {
				if path == p {
					return true
				}
			}

			// 健康检查
			if path == "/ping" || path == "/health" {
				return true
			}

			// Setup 流程相关路由（所有 setup 路由在初始化前都应公开）
			if strings.HasPrefix(path, "/api/v1/setup/") {
				return true
			}

			// Flatpak VNC 代理使用 cookie 认证，不跳过中间件
			// 首次加载 vnc.html?token=XXX 时由 handler 将 token 写入 cookie，
			// 后续 JS/CSS/WebSocket 请求自动携带 cookie 通过认证

			// GET 请求的某些路径（静态资源等）
			if method == "GET" {
				// WebSocket 升级请求（无法设置 Authorization header）
				// 排除 VNC 路径：VNC WebSocket 使用 cookie 认证
				if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") &&
					!strings.HasPrefix(path, "/api/v1/flatpak/vnc/") {
					return true
				}
				// 系统基本信息
				if strings.HasPrefix(path, "/api/v1/system/info") {
					return true
				}
				// 套件列表（桌面初始化时需要）
				if path == "/api/v1/packages" {
					return true
				}
				// 套件前端资源和图标（iframe 内加载不带 Authorization header）
				if strings.HasPrefix(path, "/api/v1/pkg-assets/") {
					return true
				}
				if strings.HasPrefix(path, "/api/v1/pkg-icon/") {
					return true
				}
				// 套件后端 API 代理（iframe 内请求不带 Authorization）
				if strings.HasPrefix(path, "/api/v1/pkg-api/") {
					return true
				}
				// Flatpak 应用图标（img src 不带 Authorization header）
				if strings.HasPrefix(path, "/api/v1/flatpak/icons/") {
					return true
				}
				// 照片缩略图/预览图/原图（img src 不带 Authorization header）
				if strings.HasPrefix(path, "/api/v1/photos/") &&
					(strings.HasSuffix(path, "/thumbnail") ||
						strings.HasSuffix(path, "/preview") ||
						strings.HasSuffix(path, "/original")) {
					return true
				}
			}

			return false
		},
		SuccessHandler: func(c *gin.Context, claims *auth.Claims) {
			// 记录用户活跃状态（用于在线检测）
			if app.Users != nil {
				app.Users.GetService().RecordActivity(claims.UserID)
			}
		},
	})
	apiV1.Use(authMiddleware)

	// 注册模块路由
	app.Registry.RegisterRoutes(apiV1)

	// 2.5 发现并启动外部插件，启动目录监听
	if err := app.PluginManager.Discover(); err != nil {
		app.Logger.Warn("Plugin discovery failed", zap.Error(err))
	}
	app.PluginManager.StartAll()
	app.PluginManager.StartWatching()

	// 3. 注册插件前端路由 (/app/*)
	// 这些路由需要认证，代理到各插件的前端服务
	appGroup := app.Router.Group("/app")
	appGroup.Use(authMiddleware)
	appGroup.Use(app.PluginManager.FrontendMiddleware())

	// 4. 注册核心路由
	app.registerCoreRoutes()

	// 4. 注册静态文件服务（前端 SPA）
	app.registerStaticFiles()

	app.Logger.Info("RDE application started successfully")
	return nil
}

// registerCoreRoutes 注册核心路由
func (app *App) registerCoreRoutes() {
	// 健康检查
	app.Router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	app.Router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"modules": app.Registry.GetAll(),
			"plugins": app.PluginManager.GetPlugins(),
		})
	})

	// 模块信息
	app.Router.GET("/api/v1/modules", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"data": app.Registry.GetAll(),
		})
	})

	// 插件信息
	app.Router.GET("/api/v1/plugins", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"data": app.PluginManager.GetPlugins(),
		})
	})

	// 插件提供的前端应用列表
	app.Router.GET("/api/v1/plugin-apps", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"data": app.PluginManager.GetPluginApps(),
		})
	})

	// 启用插件
	app.Router.POST("/api/v1/plugins/:id/enable", func(c *gin.Context) {
		id := c.Param("id")
		if err := app.PluginManager.EnablePlugin(id); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "plugin enabled", "id": id})
	})

	// 禁用插件
	app.Router.POST("/api/v1/plugins/:id/disable", func(c *gin.Context) {
		id := c.Param("id")
		if err := app.PluginManager.DisablePlugin(id); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "plugin disabled", "id": id})
	})
}

// registerStaticFiles 注册静态文件服务（前端 SPA）
func (app *App) registerStaticFiles() {
	dataDir := app.Config.GetString("data_dir")
	wwwDir := filepath.Join(dataDir, "www")

	// EmulatorJS 静态文件（按需下载后存放于 {dataDir}/emulatorjs/）
	emulatorjsDir := filepath.Join(dataDir, "emulatorjs")
	os.MkdirAll(emulatorjsDir, 0755)
	app.Router.Static("/emulatorjs", emulatorjsDir)
	app.Logger.Info("EmulatorJS static route registered", zap.String("path", emulatorjsDir))

	// 用户头像静态文件
	avatarsDir := filepath.Join(dataDir, "avatars")
	os.MkdirAll(avatarsDir, 0755)
	app.Router.Static("/avatars", avatarsDir)

	// 检查静态文件目录是否存在
	if _, err := os.Stat(wwwDir); os.IsNotExist(err) {
		app.Logger.Warn("Static files directory not found", zap.String("path", wwwDir))
		return
	}

	app.Logger.Info("Serving static files", zap.String("path", wwwDir))

	// SPA 路由处理：所有非 API 请求都返回 index.html
	app.Router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// 插件 API 路由 (/api/v1/xxx) - 代理到插件
		if strings.HasPrefix(path, "/api/v1/") {
			if app.PluginManager.HandleAPIRequest(c) {
				return
			}
			c.JSON(404, gin.H{"error": "not found"})
			return
		}

		// 其他 API 请求返回 404
		if len(path) >= 4 && path[:4] == "/api" {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}

		// 尝试提供静态文件
		filePath := filepath.Join(wwwDir, path)
		if _, err := os.Stat(filePath); err == nil {
			// 不可变资源（带 content hash）可以永久缓存
			if strings.HasPrefix(path, "/_app/immutable/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else if path == "/" || strings.HasSuffix(path, ".html") {
				// HTML 文件禁止缓存，确保部署后浏览器获取最新版本
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Header("Pragma", "no-cache")
				c.Header("Expires", "0")
			}
			c.File(filePath)
			return
		}

		// 不可变资源（带 hash 的 JS/CSS chunk）不存在时直接返回 404，
		// 避免 SPA 回退导致浏览器收到 text/html 而非预期的 JS/CSS MIME 类型
		if strings.HasPrefix(path, "/_app/immutable/") {
			c.JSON(404, gin.H{"error": "asset not found"})
			return
		}

		// SPA 回退：返回 index.html（禁止缓存，确保部署后立即生效）
		indexPath := filepath.Join(wwwDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
			c.File(indexPath)
			return
		}

		c.JSON(404, gin.H{"error": "not found"})
	})
}

// Stop 停止应用
func (app *App) Stop() error {
	app.Logger.Info("Stopping RDE application")

	// 停止所有插件
	app.PluginManager.Stop()

	// 停止所有内置模块
	if err := app.Registry.Stop(); err != nil {
		app.Logger.Error("Error stopping modules", zap.Error(err))
		return err
	}

	// 关闭日志
	app.Logger.Sync()

	return nil
}

// GetRouter 返回 Gin 路由器
func (app *App) GetRouter() *gin.Engine {
	return app.Router
}

// GetDB 返回数据库连接
func (app *App) GetDB() *gorm.DB {
	return app.DB
}

// GetModule 根据 ID 获取模块
func (app *App) GetModule(id string) module.Module {
	return app.Registry.Get(id)
}

// GetUsersService 获取用户服务
func (app *App) GetUsersService() *users.Service {
	if app.Users != nil {
		return app.Users.GetService()
	}
	return nil
}

// GetFilesService 获取文件服务
func (app *App) GetFilesService() *files.Service {
	if app.Files != nil {
		return app.Files.GetService()
	}
	return nil
}

// GetSystemService 获取系统服务
func (app *App) GetSystemService() *system.Service {
	if app.System != nil {
		return app.System.GetService()
	}
	return nil
}

// GetNotificationService 获取通知服务
func (app *App) GetNotificationService() *notification.Service {
	if app.Notification != nil {
		return app.Notification.GetService()
	}
	return nil
}

// loadI18nRegion 从 i18n.json 文件加载用户保存的区域设置
// 在所有模块启动前调用，确保镜像源等配置使用用户设置的区域
func (app *App) loadI18nRegion() {
	settingsPath := filepath.Join(app.Config.GetString("data_path"), "i18n.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		app.Logger.Debug("No i18n settings file found, using auto-detection",
			zap.String("path", settingsPath))
		return
	}

	var settings struct {
		Region string `json:"region"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		app.Logger.Warn("Failed to parse i18n settings", zap.Error(err))
		return
	}

	if settings.Region != "" {
		i18n.SetRegionOverride(settings.Region)
		app.Logger.Info("Loaded user region setting",
			zap.String("region", settings.Region))
	}
}
