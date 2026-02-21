package command

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ExecResultStr 执行命令并返回输出字符串
func ExecResultStr(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s: %s", err.Error(), stderr.String())
		}
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecResultBytes 执行命令并返回字节数组
func ExecResultBytes(command string) ([]byte, error) {
	cmd := exec.Command("bash", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s: %s", err.Error(), stderr.String())
		}
		return nil, err
	}

	return stdout.Bytes(), nil
}

// ExecSuccess 执行命令返回是否成功
func ExecSuccess(command string) bool {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// ExecWithTimeout 带超时的命令执行
func ExecWithTimeout(command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after %v", timeout)
	}
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s: %s", err.Error(), stderr.String())
		}
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecBackground 后台执行命令（不等待完成）
func ExecBackground(command string) (*exec.Cmd, error) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	// 设置进程组，便于管理
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}

// ExecStream 流式输出命令执行结果
func ExecStream(command string, onOutput func(line string)) error {
	cmd := exec.Command("bash", "-c", command)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// 读取 stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if onOutput != nil {
				onOutput(scanner.Text())
			}
		}
	}()

	// 读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if onOutput != nil {
				onOutput("[stderr] " + scanner.Text())
			}
		}
	}()

	return cmd.Wait()
}

// ExecStreamWithContext 带上下文的流式命令执行
func ExecStreamWithContext(ctx context.Context, command string, onOutput func(line string)) error {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan struct{})

	// 读取 stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if onOutput != nil {
				onOutput(scanner.Text())
			}
		}
	}()

	// 读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if onOutput != nil {
				onOutput("[stderr] " + scanner.Text())
			}
		}
		close(done)
	}()

	select {
	case <-ctx.Done():
		cmd.Process.Kill()
		return ctx.Err()
	case <-done:
		return cmd.Wait()
	}
}

// ExecWithEnv 带环境变量执行命令
func ExecWithEnv(command string, env map[string]string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	
	// 继承当前环境变量
	cmd.Env = os.Environ()
	
	// 添加自定义环境变量
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s: %s", err.Error(), stderr.String())
		}
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecWithDir 在指定目录执行命令
func ExecWithDir(command string, dir string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s: %s", err.Error(), stderr.String())
		}
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecCombined 执行命令并返回合并的输出
func ExecCombined(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return strings.TrimSpace(string(output)), nil
}

// Which 查找命令路径
func Which(command string) (string, error) {
	return exec.LookPath(command)
}

// CommandExists 检查命令是否存在
func CommandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// KillProcess 终止进程
func KillProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

// KillProcessGroup 终止进程组
func KillProcessGroup(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGKILL)
}
