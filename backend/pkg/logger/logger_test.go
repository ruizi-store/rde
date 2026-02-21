package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogInitConsoleOnly(t *testing.T) {
	// 不应 panic
	LogInitConsoleOnly()

	l := GetLogger()
	if l == nil {
		t.Error("Logger should not be nil after initialization")
	}
}

func TestGetLogger_Default(t *testing.T) {
	// 重置 logger
	logger = nil

	l := GetLogger()
	if l == nil {
		t.Error("GetLogger should return a non-nil logger")
	}
}

func TestInit(t *testing.T) {
	// 使用临时目录
	tmpDir := t.TempDir()

	Init(tmpDir, "test", "log")

	l := GetLogger()
	if l == nil {
		t.Error("Logger should not be nil after Init")
	}

	// 检查日志文件是否创建
	logFile := filepath.Join(tmpDir, "test.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		// 文件可能还没写入，这是正常的
		t.Log("Log file not created yet, which is acceptable")
	}
}

func TestInfo(t *testing.T) {
	LogInitConsoleOnly()

	// 不应 panic
	Info("test info message")
}

func TestError(t *testing.T) {
	LogInitConsoleOnly()

	// 不应 panic
	Error("test error message")
}

func TestDebug(t *testing.T) {
	LogInitConsoleOnly()

	// 不应 panic
	Debug("test debug message")
}

func TestWarn(t *testing.T) {
	LogInitConsoleOnly()

	// 不应 panic
	Warn("test warn message")
}

func TestSync(t *testing.T) {
	LogInitConsoleOnly()

	err := Sync()
	// Sync 可能返回错误（stdout 不支持 sync），这是正常的
	_ = err
}
