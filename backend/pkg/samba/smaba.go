package samba

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetSambaSharesList 获取 Samba 共享列表
// host: Samba 服务器地址
// port: 端口
// username: 用户名
// password: 密码
// 返回: 共享目录列表
func GetSambaSharesList(host, port, username, password string) ([]string, error) {
	// 使用 smbclient 列出共享
	// smbclient -L //host -U username%password
	
	var shares []string
	
	// 构建命令
	authStr := fmt.Sprintf("%s%%%s", username, password)
	targetHost := fmt.Sprintf("//%s", host)
	
	cmd := exec.Command("smbclient", "-L", targetHost, "-U", authStr, "-p", port, "-g")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("smbclient error: %v, output: %s", err, string(output))
	}
	
	// 解析输出
	// 格式: Disk|share_name|comment
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Disk|") {
			parts := strings.Split(line, "|")
			if len(parts) >= 2 {
				shareName := parts[1]
				// 过滤隐藏共享（以 $ 结尾）
				if !strings.HasSuffix(shareName, "$") {
					shares = append(shares, shareName)
				}
			}
		}
	}
	
	return shares, nil
}

// TestConnection 测试 Samba 连接
func TestConnection(host, port, username, password string) error {
	_, err := GetSambaSharesList(host, port, username, password)
	return err
}

// MountShare 挂载 Samba 共享
func MountShare(host, shareName, mountPoint, username, password string) error {
	// mount -t cifs //host/share /mount/point -o username=user,password=pass
	
	source := fmt.Sprintf("//%s/%s", host, shareName)
	options := fmt.Sprintf("username=%s,password=%s", username, password)
	
	cmd := exec.Command("mount", "-t", "cifs", source, mountPoint, "-o", options)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount error: %v, output: %s", err, string(output))
	}
	
	return nil
}

// UnmountShare 卸载 Samba 共享
func UnmountShare(mountPoint string) error {
	cmd := exec.Command("umount", mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("umount error: %v, output: %s", err, string(output))
	}
	
	return nil
}

// IsMounted 检查挂载点是否已挂载
func IsMounted(mountPoint string) bool {
	cmd := exec.Command("mountpoint", "-q", mountPoint)
	err := cmd.Run()
	return err == nil
}

// GetMountedShares 获取已挂载的 Samba 共享列表
func GetMountedShares() ([]string, error) {
	cmd := exec.Command("mount", "-t", "cifs")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mount error: %v, output: %s", err, string(output))
	}
	
	var shares []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				shares = append(shares, parts[2]) // mount point
			}
		}
	}
	
	return shares, nil
}
