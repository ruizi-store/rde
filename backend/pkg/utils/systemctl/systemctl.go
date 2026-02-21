package systemctl

import (
	"os/exec"
	"strings"
)

// IsServiceRunning 检查服务是否运行
func IsServiceRunning(serviceName string) bool {
	cmd := exec.Command("systemctl", "is-active", serviceName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "active"
}

// StartService 启动服务
func StartService(serviceName string) error {
	return exec.Command("systemctl", "start", serviceName).Run()
}

// StopService 停止服务
func StopService(serviceName string) error {
	return exec.Command("systemctl", "stop", serviceName).Run()
}

// RestartService 重启服务
func RestartService(serviceName string) error {
	return exec.Command("systemctl", "restart", serviceName).Run()
}

// EnableService 设置服务开机启动
func EnableService(serviceName string) error {
	return exec.Command("systemctl", "enable", serviceName).Run()
}

// DisableService 禁止服务开机启动
func DisableService(serviceName string) error {
	return exec.Command("systemctl", "disable", serviceName).Run()
}

// GetServiceStatus 获取服务状态
func GetServiceStatus(serviceName string) string {
	cmd := exec.Command("systemctl", "status", serviceName)
	output, _ := cmd.Output()
	return string(output)
}
