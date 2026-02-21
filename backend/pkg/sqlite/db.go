package sqlite

import (
	"github.com/glebarez/sqlite"
	"github.com/ruizi-store/rde/backend/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var gdb *gorm.DB

// GetDb 获取数据库连接
func GetDb(dbPath string) *gorm.DB {
	if gdb != nil {
		return gdb
	}

	var err error
	gdb, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		logger.Error("Failed to connect to database", zap.Error(err))
		panic(err)
	}

	return gdb
}

// Close 关闭数据库连接
func Close() error {
	if gdb == nil {
		return nil
	}
	
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	
	return sqlDB.Close()
}
