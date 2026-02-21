// Package runas 提供以指定用户身份执行命令的能力
package runas

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

// Executor 用户命令执行器
type Executor struct {
	Username string
	uid      uint32
	gid      uint32
	homeDir  string
}

// NewExecutor 创建一个新的用户执行器
func NewExecutor(username string) (*Executor, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid uid: %s", u.Uid)
	}

	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid gid: %s", u.Gid)
	}

	return &Executor{
		Username: username,
		uid:      uint32(uid),
		gid:      uint32(gid),
		homeDir:  u.HomeDir,
	}, nil
}

// Run 以用户身份执行命令
func (e *Executor) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: e.uid,
			Gid: e.gid,
		},
	}
	cmd.Env = []string{
		"HOME=" + e.homeDir,
		"USER=" + e.Username,
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}
	return cmd.CombinedOutput()
}

// RunWithStdin 以用户身份执行命令并传入 stdin
func (e *Executor) RunWithStdin(stdin io.Reader, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: e.uid,
			Gid: e.gid,
		},
	}
	cmd.Env = []string{
		"HOME=" + e.homeDir,
		"USER=" + e.Username,
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}
	cmd.Stdin = stdin
	return cmd.CombinedOutput()
}

// Mkdir 以用户身份创建目录
func (e *Executor) Mkdir(path string) error {
	output, err := e.Run("mkdir", "-p", path)
	if err != nil {
		return fmt.Errorf("mkdir failed: %s - %s", err, string(output))
	}
	return nil
}

// Remove 以用户身份删除文件或目录
func (e *Executor) Remove(path string, recursive bool) error {
	args := []string{"-f", path}
	if recursive {
		args = []string{"-rf", path}
	}
	output, err := e.Run("rm", args...)
	if err != nil {
		return fmt.Errorf("rm failed: %s - %s", err, string(output))
	}
	return nil
}

// Move 以用户身份移动/重命名文件
func (e *Executor) Move(src, dest string) error {
	output, err := e.Run("mv", "-f", src, dest)
	if err != nil {
		return fmt.Errorf("mv failed: %s - %s", err, string(output))
	}
	return nil
}

// Copy 以用户身份复制文件或目录
func (e *Executor) Copy(src, dest string, recursive bool) error {
	args := []string{"-f", src, dest}
	if recursive {
		args = []string{"-rf", src, dest}
	}
	output, err := e.Run("cp", args...)
	if err != nil {
		return fmt.Errorf("cp failed: %s - %s", err, string(output))
	}
	return nil
}

// Touch 以用户身份创建空文件
func (e *Executor) Touch(path string) error {
	// 确保父目录存在
	dir := filepath.Dir(path)
	if err := e.Mkdir(dir); err != nil {
		return err
	}
	output, err := e.Run("touch", path)
	if err != nil {
		return fmt.Errorf("touch failed: %s - %s", err, string(output))
	}
	return nil
}

// WriteFile 以用户身份写入文件内容
func (e *Executor) WriteFile(path string, content []byte) error {
	// 确保父目录存在
	dir := filepath.Dir(path)
	if err := e.Mkdir(dir); err != nil {
		return err
	}

	// 使用 tee 写入文件
	output, err := e.RunWithStdin(bytes.NewReader(content), "tee", path)
	if err != nil {
		return fmt.Errorf("write file failed: %s - %s", err, string(output))
	}
	return nil
}

// AppendFile 以用户身份追加内容到文件
func (e *Executor) AppendFile(path string, content []byte) error {
	output, err := e.RunWithStdin(bytes.NewReader(content), "tee", "-a", path)
	if err != nil {
		return fmt.Errorf("append file failed: %s - %s", err, string(output))
	}
	return nil
}

// Chmod 以用户身份修改文件权限
func (e *Executor) Chmod(path string, mode os.FileMode) error {
	modeStr := fmt.Sprintf("%04o", mode)
	output, err := e.Run("chmod", modeStr, path)
	if err != nil {
		return fmt.Errorf("chmod failed: %s - %s", err, string(output))
	}
	return nil
}

// Stat 检查文件/目录是否存在
func (e *Executor) Stat(path string) (bool, bool, error) {
	// 使用 test 命令检查
	// -e 存在, -d 是目录
	_, err := e.Run("test", "-e", path)
	if err != nil {
		return false, false, nil // 不存在
	}
	_, err = e.Run("test", "-d", path)
	isDir := err == nil
	return true, isDir, nil
}

// GetUID 返回用户的 UID
func (e *Executor) GetUID() uint32 {
	return e.uid
}

// GetGID 返回用户的 GID
func (e *Executor) GetGID() uint32 {
	return e.gid
}

// ChownToUser 将文件/目录的所有权改为指定用户
// 这是一个便捷函数，使用 root 权限执行 chown
func ChownToUser(path, username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("user not found: %s", username)
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid uid: %s", u.Uid)
	}

	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid gid: %s", u.Gid)
	}

	return os.Chown(path, int(uid), int(gid))
}

// ChownToUserRecursive 递归将目录及其内容的所有权改为指定用户
func ChownToUserRecursive(path, username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("user not found: %s", username)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("invalid uid: %s", u.Uid)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("invalid gid: %s", u.Gid)
	}

	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, uid, gid)
	})
}

// MkdirAllAndChown 创建目录（含父目录）并将所有新建的目录 chown 给用户
// basePath 是用户已有权限的基础路径（如用户主目录）
// targetPath 是最终要创建的路径
func MkdirAllAndChown(basePath, targetPath, username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("user not found: %s", username)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("invalid uid: %s", u.Uid)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("invalid gid: %s", u.Gid)
	}

	// 找出需要创建的目录链
	var dirsToCreate []string
	current := targetPath
	for current != basePath && current != "/" && current != "." {
		if _, err := os.Stat(current); os.IsNotExist(err) {
			dirsToCreate = append([]string{current}, dirsToCreate...)
		} else {
			break
		}
		current = filepath.Dir(current)
	}

	// 创建目录
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}

	// chown 所有新创建的目录
	for _, dir := range dirsToCreate {
		if err := os.Chown(dir, uid, gid); err != nil {
			return fmt.Errorf("chown failed for %s: %w", dir, err)
		}
	}

	return nil
}
