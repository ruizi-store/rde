package file

import (
	"bufio"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Exists 检查文件或目录是否存在
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

// IsDir 检查是否为目录
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsFile 检查是否为文件
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// IsSymlink 检查是否为符号链接
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// ReadFile 读取文件内容
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ReadFileString 读取文件内容为字符串
func ReadFileString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadLines 按行读取文件
func ReadLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// WriteFile 写入文件
func WriteFile(path string, data []byte, perm os.FileMode) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// WriteFileString 写入字符串到文件
func WriteFileString(path string, content string, perm os.FileMode) error {
	return WriteFile(path, []byte(content), perm)
}

// AppendFile 追加内容到文件
func AppendFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

// AppendLine 追加一行到文件
func AppendLine(path string, line string) error {
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	return AppendFile(path, []byte(line))
}

// GetSize 获取文件大小
func GetSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// GetDirSize 获取目录总大小
func GetDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// FormatSize 格式化文件大小
func FormatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// ParseSize 解析文件大小字符串
func ParseSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))
	
	units := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
		"K":  1024,
		"M":  1024 * 1024,
		"G":  1024 * 1024 * 1024,
		"T":  1024 * 1024 * 1024 * 1024,
	}
	
	for suffix, multiplier := range units {
		if strings.HasSuffix(sizeStr, suffix) {
			numStr := strings.TrimSuffix(sizeStr, suffix)
			var num float64
			_, err := fmt.Sscanf(numStr, "%f", &num)
			if err != nil {
				return 0, err
			}
			return int64(num * float64(multiplier)), nil
		}
	}
	
	var size int64
	_, err := fmt.Sscanf(sizeStr, "%d", &size)
	return size, err
}

// MD5 计算文件 MD5 哈希
func MD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// SHA256 计算文件 SHA256 哈希
func SHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Chmod 修改文件权限
func Chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

// Chown 修改文件所有者
func Chown(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}

// GetExtension 获取文件扩展名
func GetExtension(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}

// GetBaseName 获取文件名（不含扩展名）
func GetBaseName(path string) string {
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}

// GetFileName 获取文件名（含扩展名）
func GetFileName(path string) string {
	return filepath.Base(path)
}

// GetDir 获取目录路径
func GetDir(path string) string {
	return filepath.Dir(path)
}

// JoinPath 连接路径
func JoinPath(elem ...string) string {
	return filepath.Join(elem...)
}

// AbsPath 获取绝对路径
func AbsPath(path string) (string, error) {
	return filepath.Abs(path)
}

// CleanPath 清理路径
func CleanPath(path string) string {
	return filepath.Clean(path)
}

// MkdirAll 递归创建目录
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Remove 删除文件或空目录
func Remove(path string) error {
	return os.Remove(path)
}

// RemoveAll 递归删除文件或目录
func RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Rename 重命名或移动文件
func Rename(oldPath, newPath string) error {
	// 确保目标目录存在
	dir := filepath.Dir(newPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}

// ListFiles 列出目录下的所有文件（非递归）
func ListFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files, nil
}

// ListDirs 列出目录下的所有子目录（非递归）
func ListDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(dir, entry.Name()))
		}
	}
	return dirs, nil
}

// ListAll 列出目录下的所有文件和目录（非递归）
func ListAll(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var items []string
	for _, entry := range entries {
		items = append(items, filepath.Join(dir, entry.Name()))
	}
	return items, nil
}

// WalkFiles 递归遍历目录下的所有文件
func WalkFiles(dir string, fn func(path string) error) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fn(path)
		}
		return nil
	})
}

// FindFiles 查找匹配模式的文件
func FindFiles(dir string, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			matched, err := filepath.Match(pattern, info.Name())
			if err != nil {
				return err
			}
			if matched {
				matches = append(matches, path)
			}
		}
		return nil
	})
	return matches, err
}

// CountFiles 统计目录下的文件数量
func CountFiles(dir string) (int, error) {
	count := 0
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// Copy 复制文件
func Copy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 获取源文件权限
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// CopyDir 递归复制目录
func CopyDir(src, dst string, style ...string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// 检查是否有 style 参数（CasaOS 兼容）
	// style: "skip" 跳过已存在的文件, "overwrite" 覆盖
	copyStyle := "overwrite"
	if len(style) > 0 {
		copyStyle = style[0]
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath, copyStyle); err != nil {
				return err
			}
		} else {
			// 检查目标文件是否存在
			if copyStyle == "skip" && Exists(dstPath) {
				continue
			}
			if err := Copy(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// Move 移动文件或目录
func Move(src, dst string) error {
	// 先尝试重命名（同一文件系统）
	err := Rename(src, dst)
	if err == nil {
		return nil
	}

	// 如果重命名失败，尝试复制后删除
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		if err := CopyDir(src, dst); err != nil {
			return err
		}
	} else {
		if err := Copy(src, dst); err != nil {
			return err
		}
	}

	return RemoveAll(src)
}

// Touch 创建空文件或更新文件时间戳
func Touch(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

// Truncate 截断文件到指定大小
func Truncate(path string, size int64) error {
	return os.Truncate(path, size)
}

// IsEmpty 检查文件或目录是否为空
func IsEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return false, err
		}
		return len(entries) == 0, nil
	}

	return info.Size() == 0, nil
}

// SameFile 检查两个路径是否指向同一文件
func SameFile(path1, path2 string) (bool, error) {
	info1, err := os.Stat(path1)
	if err != nil {
		return false, err
	}
	info2, err := os.Stat(path2)
	if err != nil {
		return false, err
	}
	return os.SameFile(info1, info2), nil
}
