package common

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

// FilePermission 文件权限检查模式
type FilePermission int

const (
	PermRead    FilePermission = 4 // r
	PermWrite   FilePermission = 2 // w
	PermExecute FilePermission = 1 // x
)

// CheckUserPermission 检查指定用户对文件/目录是否有指定权限
// 模拟 Linux 文件权限检查逻辑（owner → group → other）
// 对于目录，进入需要 r+x 权限
func CheckUserPermission(username, path string, perm FilePermission) bool {
	// root 用户拥有所有权限
	if username == "root" {
		return true
	}

	// 获取用户信息
	u, err := user.Lookup(username)
	if err != nil {
		return false
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return false
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return false
	}

	// 获取用户补充组
	groupIDs := getUserGroupIDs(u)

	// 获取文件信息
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	fileUID := int(stat.Uid)
	fileGID := int(stat.Gid)
	mode := info.Mode()

	// Linux 权限检查顺序: owner → group → other
	var bits FilePermission

	if uid == fileUID {
		// 用户是文件所有者
		bits = FilePermission((mode >> 6) & 0x7)
	} else if gid == fileGID || containsInt(groupIDs, fileGID) {
		// 用户在文件所属组中
		bits = FilePermission((mode >> 3) & 0x7)
	} else {
		// 其他用户
		bits = FilePermission(mode & 0x7)
	}

	return (bits & perm) == perm
}

// CheckUserDirAccess 检查用户是否能访问（进入并列出）目录
// 目录需要 r+x 权限
func CheckUserDirAccess(username, path string) bool {
	return CheckUserPermission(username, path, PermRead|PermExecute)
}

// CheckUserReadAccess 检查用户是否能读取文件
func CheckUserReadAccess(username, path string) bool {
	return CheckUserPermission(username, path, PermRead)
}

// CheckUserWriteAccess 检查用户是否能写入文件/目录
func CheckUserWriteAccess(username, path string) bool {
	return CheckUserPermission(username, path, PermWrite)
}

// getUserGroupIDs 获取用户的所有补充组 GID
func getUserGroupIDs(u *user.User) []int {
	gids, err := u.GroupIds()
	if err != nil {
		return nil
	}

	var result []int
	for _, gidStr := range gids {
		if id, err := strconv.Atoi(gidStr); err == nil {
			result = append(result, id)
		}
	}
	return result
}

// containsInt 检查 int 切片是否包含指定值
func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// CheckPathChainAccess 检查从根目录到目标路径的每一级目录是否都有 x 权限
// Linux 访问 /home/user/Documents 需要 / , /home, /home/user, /home/user/Documents 每层都有 x
func CheckPathChainAccess(username, path string) bool {
	if username == "root" {
		return true
	}

	cleaned := ResolvePath(path)
	parts := strings.Split(cleaned, "/")

	// 从根目录开始逐级检查
	current := "/"
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = current + part
		if !CheckUserPermission(username, current, PermExecute) {
			return false
		}
		current += "/"
	}

	return true
}
