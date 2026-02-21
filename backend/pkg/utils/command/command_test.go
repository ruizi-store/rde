package command

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecResultStr(t *testing.T) {
	// 测试简单命令
	result, err := ExecResultStr("echo hello")
	if err != nil {
		t.Fatalf("ExecResultStr 失败: %v", err)
	}
	
	result = strings.TrimSpace(result)
	if result != "hello" {
		t.Errorf("ExecResultStr('echo hello') = %q, 期望 'hello'", result)
	}
}

func TestExecResultStrWithPipe(t *testing.T) {
	// 测试管道命令
	result, err := ExecResultStr("echo 'hello world' | wc -w")
	if err != nil {
		t.Fatalf("管道命令执行失败: %v", err)
	}
	
	result = strings.TrimSpace(result)
	if result != "2" {
		t.Errorf("管道命令结果 = %q, 期望 '2'", result)
	}
}

func TestExecResultStrError(t *testing.T) {
	// 测试错误命令
	_, err := ExecResultStr("nonexistent_command_12345")
	if err == nil {
		t.Error("执行不存在的命令应返回错误")
	}
}

func TestExecSuccess(t *testing.T) {
	// 测试成功命令
	if !ExecSuccess("true") {
		t.Error("'true' 命令应返回 success")
	}
	
	// 测试失败命令
	if ExecSuccess("false") {
		t.Error("'false' 命令应返回 failure")
	}
}

func TestExecWithTimeout(t *testing.T) {
	// 测试正常命令
	result, err := ExecWithTimeout("echo timeout_test", 5*time.Second)
	if err != nil {
		t.Fatalf("ExecWithTimeout 失败: %v", err)
	}
	
	if !strings.Contains(result, "timeout_test") {
		t.Errorf("结果应包含 'timeout_test': %s", result)
	}
}

func TestExecWithTimeoutExpired(t *testing.T) {
	// 测试超时命令
	_, err := ExecWithTimeout("sleep 10", 100*time.Millisecond)
	if err == nil {
		t.Error("超时命令应返回错误")
	}
}

func TestExecBackground(t *testing.T) {
	// 创建临时文件
	tmpFile := "/tmp/test_exec_background_" + time.Now().Format("20060102150405")
	
	// 后台执行写文件命令
	_, err := ExecBackground("echo 'background' > " + tmpFile)
	if err != nil {
		t.Fatalf("ExecBackground 失败: %v", err)
	}
	
	// 等待后台命令执行
	time.Sleep(200 * time.Millisecond)
	
	// 检查文件是否被创建
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("后台命令应该创建文件")
	}
	
	// 清理
	os.Remove(tmpFile)
}

func TestExecResultStrWithEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("跳过 Windows 测试")
	}
	
	// 测试环境变量
	result, err := ExecResultStr("echo $HOME")
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	
	result = strings.TrimSpace(result)
	if result == "" || result == "$HOME" {
		t.Error("应该能读取环境变量")
	}
}

func TestExecResultStrMultiLine(t *testing.T) {
	// 测试多行输出
	result, err := ExecResultStr("echo -e 'line1\\nline2\\nline3'")
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 3 {
		t.Errorf("应该有 3 行输出, 实际有 %d 行", len(lines))
	}
}

func TestExecResultStrExitCode(t *testing.T) {
	// 测试非零退出码
	_, err := ExecResultStr("exit 1")
	if err == nil {
		t.Error("非零退出码应返回错误")
	}
}

func TestExecWithArgs(t *testing.T) {
	// 测试带参数的命令
	result, err := ExecResultStr("printf '%s %s' hello world")
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	
	result = strings.TrimSpace(result)
	if result != "hello world" {
		t.Errorf("结果 = %q, 期望 'hello world'", result)
	}
}

func TestExecSpecialCharacters(t *testing.T) {
	// 测试特殊字符
	result, err := ExecResultStr("echo 'hello, world!'")
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	
	result = strings.TrimSpace(result)
	if result != "hello, world!" {
		t.Errorf("结果 = %q, 期望 'hello, world!'", result)
	}
}

// 基准测试
func BenchmarkExecResultStr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ExecResultStr("echo benchmark")
	}
}

func BenchmarkExecSuccess(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ExecSuccess("true")
	}
}
