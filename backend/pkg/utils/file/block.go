package file

import (
	"os"
	"syscall"
)

// BlockDevice 块设备信息
type BlockDevice struct {
	Path       string
	Size       int64
	MountPoint string
}

// GetBlockDeviceSize 获取块设备大小
func GetBlockDeviceSize(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// 使用 seek 到文件末尾获取大小
	size, err := file.Seek(0, 2)
	if err != nil {
		return 0, err
	}

	return size, nil
}

// IsBlockDevice 检查是否为块设备
func IsBlockDevice(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeDevice != 0
}

// GetDiskUsage 获取磁盘使用情况
type DiskUsage struct {
	Total     uint64  // 总空间
	Free      uint64  // 可用空间
	Used      uint64  // 已用空间
	UsedPct   float64 // 使用百分比
	Available uint64  // 用户可用空间
}

// GetDiskUsage 获取指定路径的磁盘使用情况
func GetDiskUsage(path string) (*DiskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	var usedPct float64
	if total > 0 {
		usedPct = float64(used) / float64(total) * 100
	}

	return &DiskUsage{
		Total:     total,
		Free:      free,
		Used:      used,
		UsedPct:   usedPct,
		Available: available,
	}, nil
}

// GetInodeUsage 获取 inode 使用情况
type InodeUsage struct {
	Total   uint64
	Free    uint64
	Used    uint64
	UsedPct float64
}

// GetInodeUsage 获取指定路径的 inode 使用情况
func GetInodeUsage(path string) (*InodeUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, err
	}

	total := stat.Files
	free := stat.Ffree
	used := total - free

	var usedPct float64
	if total > 0 {
		usedPct = float64(used) / float64(total) * 100
	}

	return &InodeUsage{
		Total:   total,
		Free:    free,
		Used:    used,
		UsedPct: usedPct,
	}, nil
}

// HasEnoughSpace 检查路径是否有足够空间
func HasEnoughSpace(path string, required uint64) (bool, error) {
	usage, err := GetDiskUsage(path)
	if err != nil {
		return false, err
	}
	return usage.Available >= required, nil
}
