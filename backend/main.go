// Package main RDE Backend 应用入口点
//
// 采用模块化架构，通过 bootstrap 包初始化所有模块。
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/ruizi-store/rde/backend/common"
	"github.com/ruizi-store/rde/backend/core/bootstrap"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	DEFAULT_PORT   = "3080"
	DEFAULT_CONFIG = "/etc/rde/rde.conf"
)

var (
	commit = "private build"
	date   = "private build"

	configFlag    = flag.String("c", DEFAULT_CONFIG, "config file path")
	dbFlag        = flag.String("db", "", "database path")
	portFlag      = flag.String("p", "", "server port (overrides config)")
	versionFlag   = flag.Bool("v", false, "show version")
	debugFlag     = flag.Bool("debug", false, "enable debug mode")
	dataFlag      = flag.String("data", "/var/lib/rde", "data directory")
	infoFlag      = flag.Bool("info", false, "show server info (port, credentials)")
	initFlag      = flag.Bool("init", false, "initialize database and create admin user")
	adminUserFlag = flag.String("admin-user", "admin", "admin username for init")
	adminPassFlag = flag.String("admin-pass", "", "admin password for init")
)

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Println("v" + common.VERSION)
		return
	}

	// 确定数据库路径（用于检查初始化状态）
	dbPath := *dbFlag
	if dbPath == "" {
		dbPath = filepath.Join(*dataFlag, "db", "rde.db")
	}

	// 决定端口：未初始化用默认 3080，已初始化读配置文件
	port := DEFAULT_PORT
	if isSetupCompleted(dbPath) {
		port = getPortFromConfig(*configFlag)
	}
	if *portFlag != "" {
		port = *portFlag
	}

	// 显示服务器信息
	if *infoFlag {
		showServerInfo(*configFlag, port, isSetupCompleted(dbPath))
		return
	}

	// 初始化模式：创建数据库和管理员用户
	if *initFlag {
		if err := initializeSystem(); err != nil {
			fmt.Printf("Initialization failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Initialization completed successfully")
		return
	}

	println("RDE Backend")
	println("git commit:", commit)
	println("build date:", date)

	// 确定数据目录
	dataDir := *dataFlag
	if len(*dbFlag) == 0 {
		*dbFlag = filepath.Join(dataDir, "db", "rde.db")
	}

	// 确保数据库目录存在
	dbDir := filepath.Dir(*dbFlag)
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		fmt.Printf("Failed to create database directory: %v\n", err)
		os.Exit(1)
	}

	// 创建应用实例
	app, err := bootstrap.New(&bootstrap.Options{
		DBPath:  *dbFlag,
		DataDir: dataDir,
		LogPath: filepath.Join(dataDir, "logs"),
		Debug:   *debugFlag,
	})
	if err != nil {
		fmt.Printf("Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	// 启动应用
	if err := app.Start(); err != nil {
		app.Logger.Fatal("Failed to start application", zap.Error(err))
	}

	// 监听端口（从配置文件读取监听地址）
	host := getHostFromConfig(*configFlag)
	addr := net.JoinHostPort(host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		app.Logger.Fatal("Failed to listen", zap.String("addr", addr), zap.Error(err))
	}

	// 创建 HTTP 服务器
	server := &http.Server{
		Handler:           app.GetRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		app.Logger.Info("Shutdown signal received")

		// 关闭 HTTP 服务器
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			app.Logger.Error("HTTP server shutdown error", zap.Error(err))
		}

		// 停止应用
		if err := app.Stop(); err != nil {
			app.Logger.Error("Application stop error", zap.Error(err))
		}

		os.Exit(0)
	}()

	app.Logger.Info("RDE backend is listening...", zap.String("address", addr))
	fmt.Printf("\n🚀 Server running at http://localhost:%s\n\n", port)

	if err = server.Serve(listener); err != nil && err != http.ErrServerClosed {
		app.Logger.Fatal("Server error", zap.Error(err))
	}
}

// getHostFromConfig 从配置文件读取监听地址
func getHostFromConfig(configPath string) string {
	if configPath == "" {
		return "0.0.0.0"
	}

	file, err := os.Open(configPath)
	if err != nil {
		return "0.0.0.0"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "HttpHost") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				host := strings.TrimSpace(parts[1])
				if host != "" {
					return host
				}
			}
		}
	}

	return "0.0.0.0"
}

// getPortFromConfig 从配置文件读取端口
func getPortFromConfig(configPath string) string {
	if configPath == "" {
		return DEFAULT_PORT
	}

	file, err := os.Open(configPath)
	if err != nil {
		return DEFAULT_PORT
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "HttpPort") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				port := strings.TrimSpace(parts[1])
				if port != "" {
					return port
				}
			}
		}
	}

	return DEFAULT_PORT
}

// isSetupCompleted 检查数据库中 setup 是否已完成
// DB 不存在或查询失败视为未初始化
func isSetupCompleted(dbPath string) bool {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return false
	}

	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000&mode=ro"), &gorm.Config{})
	if err != nil {
		return false
	}
	sqlDB, err := db.DB()
	if err != nil {
		return false
	}
	defer sqlDB.Close()

	var completed bool
	err = db.Raw("SELECT setup_completed FROM setup_settings LIMIT 1").Scan(&completed).Error
	if err != nil {
		return false
	}
	return completed
}

// showServerInfo 显示服务器信息
func showServerInfo(configPath, port string, setupDone bool) {
	status := "未初始化 (使用默认端口)"
	if setupDone {
		status = "已初始化"
	}
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║          RDE Server Info             ║")
	fmt.Println("╠══════════════════════════════════════════╣")
	fmt.Printf("║  Port: %-33s║\n", port)
	fmt.Printf("║  URL:  http://localhost:%-17s║\n", port)
	fmt.Printf("║  Status: %-31s║\n", status)
	fmt.Println("╠══════════════════════════════════════════╣")
	fmt.Println("║  Config: /etc/rde/rde.conf       ║")
	fmt.Println("║  Data:   /var/lib/rde                ║")
	fmt.Println("║  Logs:   /var/log/rde                ║")
	fmt.Println("╠══════════════════════════════════════════╣")
	fmt.Println("║  Commands:                               ║")
	fmt.Println("║    rde -info      Show this info     ║")
	fmt.Println("║    rde -v         Show version       ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println("")
}

// initializeSystem 初始化系统（创建数据库和管理员用户）
func initializeSystem() error {
	dataDir := *dataFlag
	dbPath := *dbFlag
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "db", "rde.db")
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// 创建应用实例（这会自动创建数据库表）
	app, err := bootstrap.New(&bootstrap.Options{
		DBPath:  dbPath,
		DataDir: dataDir,
		LogPath: filepath.Join(dataDir, "logs"),
		Debug:   false,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// 如果提供了管理员密码，创建管理员用户
	if *adminPassFlag != "" {
		if err := createAdminUser(app, *adminUserFlag, *adminPassFlag); err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}
		fmt.Printf("Admin user '%s' created successfully\n", *adminUserFlag)
	}

	// 标记 setup 完成
	if err := markSetupCompleted(app); err != nil {
		return fmt.Errorf("failed to mark setup as completed: %w", err)
	}

	return nil
}

// createAdminUser 创建管理员用户
func createAdminUser(app *bootstrap.App, username, password string) error {
	// 使用 users 模块创建用户
	if app.Users != nil {
		return app.Users.CreateAdminUser(username, password)
	}
	return fmt.Errorf("users module not available")
}

// markSetupCompleted 标记安装完成
func markSetupCompleted(app *bootstrap.App) error {
	if app.Setup != nil {
		return app.Setup.MarkCompleted()
	}
	return nil
}
