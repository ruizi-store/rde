// Package testutil 提供测试共享工具
package testutil

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SetupTestDB 创建内存 SQLite 测试数据库并自动迁移给定模型
func SetupTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	if len(models) > 0 {
		err = db.AutoMigrate(models...)
		require.NoError(t, err)
	}

	return db
}

// TestLogger 返回静默的 zap.Logger
func TestLogger() *zap.Logger {
	return zap.NewNop()
}
