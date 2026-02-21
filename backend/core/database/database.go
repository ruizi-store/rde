// Package database 提供数据库连接和管理
package database

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Manager 数据库管理器
type Manager struct {
	db     *gorm.DB
	dbPath string
	logger *zap.Logger
}

// Config 数据库配置
type Config struct {
	Path        string
	Debug       bool
	MaxOpenConn int
	MaxIdleConn int
}

// New 创建数据库管理器
func New(cfg Config, zapLogger *zap.Logger) (*Manager, error) {
	// 确保目录存在
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// 配置 GORM logger
	var gormLogger logger.Interface
	if cfg.Debug {
		gormLogger = logger.Default.LogMode(logger.Info)
	} else {
		gormLogger = logger.Default.LogMode(logger.Silent)
	}

	// 打开数据库
	db, err := gorm.Open(sqlite.Open(cfg.Path+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	maxOpen := cfg.MaxOpenConn
	if maxOpen <= 0 {
		maxOpen = 10
	}
	maxIdle := cfg.MaxIdleConn
	if maxIdle <= 0 {
		maxIdle = 5
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)

	manager := &Manager{
		db:     db,
		dbPath: cfg.Path,
		logger: zapLogger,
	}

	zapLogger.Info("Database connected", zap.String("path", cfg.Path))
	return manager, nil
}

// DB 获取 GORM 数据库实例
func (m *Manager) DB() *gorm.DB {
	return m.db
}

// Close 关闭数据库连接
func (m *Manager) Close() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// AutoMigrate 自动迁移数据库表
func (m *Manager) AutoMigrate(models ...interface{}) error {
	return m.db.AutoMigrate(models...)
}

// Transaction 执行事务
func (m *Manager) Transaction(fc func(tx *gorm.DB) error) error {
	return m.db.Transaction(fc)
}

// HealthCheck 健康检查
func (m *Manager) HealthCheck() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Stats 获取数据库统计信息
func (m *Manager) Stats() map[string]interface{} {
	sqlDB, err := m.db.DB()
	if err != nil {
		return nil
	}

	stats := sqlDB.Stats()
	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
	}
}
