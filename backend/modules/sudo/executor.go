package sudo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Executor sudo 执行器
type Executor struct {
	logger  *zap.Logger
	db      *gorm.DB
	actions map[string]*Action
}

// NewExecutor 创建执行器
func NewExecutor(logger *zap.Logger, db *gorm.DB) *Executor {
	e := &Executor{
		logger:  logger,
		db:      db,
		actions: make(map[string]*Action),
	}
	e.registerDefaultActions()
	return e
}

// registerDefaultActions 注册默认的白名单操作
func (e *Executor) registerDefaultActions() {
	actions := []*Action{
		// 系统服务管理
		{
			ID:          "service_start",
			Name:        "启动服务",
			Description: "启动系统服务",
			Command:     "systemctl start %s",
			ArgCount:    1,
			Dangerous:   false,
		},
		{
			ID:          "service_stop",
			Name:        "停止服务",
			Description: "停止系统服务",
			Command:     "systemctl stop %s",
			ArgCount:    1,
			Dangerous:   true,
		},
		{
			ID:          "service_restart",
			Name:        "重启服务",
			Description: "重启系统服务",
			Command:     "systemctl restart %s",
			ArgCount:    1,
			Dangerous:   false,
		},
		{
			ID:          "service_enable",
			Name:        "启用服务",
			Description: "设置服务开机自启",
			Command:     "systemctl enable %s",
			ArgCount:    1,
			Dangerous:   false,
		},
		{
			ID:          "service_disable",
			Name:        "禁用服务",
			Description: "取消服务开机自启",
			Command:     "systemctl disable %s",
			ArgCount:    1,
			Dangerous:   true,
		},

		// 软件包管理
		{
			ID:          "apt_update",
			Name:        "更新软件源",
			Description: "更新 APT 软件源列表",
			Command:     "apt update",
			ArgCount:    0,
			Dangerous:   false,
		},
		{
			ID:          "apt_install",
			Name:        "安装软件包",
			Description: "安装指定的软件包",
			Command:     "apt install -y %s",
			ArgCount:    1,
			Dangerous:   false,
		},
		{
			ID:          "apt_remove",
			Name:        "卸载软件包",
			Description: "卸载指定的软件包",
			Command:     "apt remove -y %s",
			ArgCount:    1,
			Dangerous:   true,
		},
		{
			ID:          "apt_upgrade",
			Name:        "升级所有软件包",
			Description: "升级所有已安装的软件包",
			Command:     "apt upgrade -y",
			ArgCount:    0,
			Dangerous:   true,
		},

		// Docker 应用
		{
			ID:          "docker_start",
			Name:        "启动容器",
			Description: "启动 Docker 容器",
			Command:     "docker start %s",
			ArgCount:    1,
			Dangerous:   false,
		},
		{
			ID:          "docker_stop",
			Name:        "停止容器",
			Description: "停止 Docker 容器",
			Command:     "docker stop %s",
			ArgCount:    1,
			Dangerous:   false,
		},
		{
			ID:          "docker_restart",
			Name:        "重启容器",
			Description: "重启 Docker 容器",
			Command:     "docker restart %s",
			ArgCount:    1,
			Dangerous:   false,
		},
		{
			ID:          "docker_rm",
			Name:        "删除容器",
			Description: "删除 Docker 容器",
			Command:     "docker rm -f %s",
			ArgCount:    1,
			Dangerous:   true,
		},
		{
			ID:          "docker_pull",
			Name:        "拉取镜像",
			Description: "从仓库拉取 Docker 镜像",
			Command:     "docker pull %s",
			ArgCount:    1,
			Dangerous:   false,
		},

		// Samba 用户管理
		{
			ID:          "samba_add_user",
			Name:        "添加 Samba 用户",
			Description: "添加 Samba 共享用户",
			Command:     "smbpasswd -a -n %s",
			ArgCount:    1,
			Dangerous:   false,
		},
		{
			ID:          "samba_del_user",
			Name:        "删除 Samba 用户",
			Description: "删除 Samba 共享用户",
			Command:     "smbpasswd -x %s",
			ArgCount:    1,
			Dangerous:   true,
		},
		{
			ID:          "samba_enable_user",
			Name:        "启用 Samba 用户",
			Description: "启用 Samba 用户账户",
			Command:     "smbpasswd -e %s",
			ArgCount:    1,
			Dangerous:   false,
		},
		{
			ID:          "samba_disable_user",
			Name:        "禁用 Samba 用户",
			Description: "禁用 Samba 用户账户",
			Command:     "smbpasswd -d %s",
			ArgCount:    1,
			Dangerous:   false,
		},

		// 系统操作
		{
			ID:          "system_reboot",
			Name:        "重启系统",
			Description: "立即重启系统",
			Command:     "reboot",
			ArgCount:    0,
			Dangerous:   true,
		},
		{
			ID:          "system_shutdown",
			Name:        "关闭系统",
			Description: "立即关闭系统",
			Command:     "shutdown -h now",
			ArgCount:    0,
			Dangerous:   true,
		},

		// 文件系统
		{
			ID:          "mount",
			Name:        "挂载分区",
			Description: "挂载存储分区",
			Command:     "mount %s %s",
			ArgCount:    2,
			Dangerous:   false,
		},
		{
			ID:          "umount",
			Name:        "卸载分区",
			Description: "卸载存储分区",
			Command:     "umount %s",
			ArgCount:    1,
			Dangerous:   true,
		},

		// 用户管理
		{
			ID:          "useradd",
			Name:        "创建系统用户",
			Description: "创建新的系统用户",
			Command:     "useradd -m -s /bin/bash %s",
			ArgCount:    1,
			Dangerous:   false,
		},
		{
			ID:          "userdel",
			Name:        "删除系统用户",
			Description: "删除系统用户及其主目录",
			Command:     "userdel -r %s",
			ArgCount:    1,
			Dangerous:   true,
		},
		{
			ID:          "chown",
			Name:        "更改文件所有者",
			Description: "更改文件或目录的所有者",
			Command:     "chown -R %s %s",
			ArgCount:    2,
			Dangerous:   false,
		},
		{
			ID:          "chmod",
			Name:        "更改文件权限",
			Description: "更改文件或目录的权限",
			Command:     "chmod -R %s %s",
			ArgCount:    2,
			Dangerous:   false,
		},
	}

	for _, action := range actions {
		e.actions[action.ID] = action
	}

	e.logger.Info("Registered sudo actions", zap.Int("count", len(actions)))
}

// GetAction 获取操作定义
func (e *Executor) GetAction(id string) *Action {
	return e.actions[id]
}

// GetAllActions 获取所有操作
func (e *Executor) GetAllActions() []*Action {
	result := make([]*Action, 0, len(e.actions))
	for _, action := range e.actions {
		result = append(result, action)
	}
	return result
}

// BuildCommand 构建完整命令
func (e *Executor) BuildCommand(action *Action, args []string) (string, error) {
	if len(args) != action.ArgCount {
		return "", fmt.Errorf("expected %d arguments, got %d", action.ArgCount, len(args))
	}

	// 参数安全检查
	for _, arg := range args {
		if err := validateArg(arg); err != nil {
			return "", err
		}
	}

	// 构建命令
	if action.ArgCount == 0 {
		return action.Command, nil
	}

	// 将参数转换为 interface{} 切片
	iArgs := make([]interface{}, len(args))
	for i, arg := range args {
		iArgs[i] = arg
	}

	return fmt.Sprintf(action.Command, iArgs...), nil
}

// validateArg 验证参数安全性
func validateArg(arg string) error {
	// 禁止危险字符
	dangerous := []string{";", "&&", "||", "|", "`", "$(", "${", ">", "<", "\n", "\r"}
	for _, d := range dangerous {
		if strings.Contains(arg, d) {
			return fmt.Errorf("invalid character in argument: %s", d)
		}
	}
	return nil
}

// Execute 执行 sudo 操作
func (e *Executor) Execute(ctx context.Context, req *ExecuteRequest, userID, username, clientIP string) (*ExecuteResponse, error) {
	action := e.GetAction(req.ActionID)
	if action == nil {
		return nil, fmt.Errorf("unknown action: %s", req.ActionID)
	}

	// 危险操作需要确认
	if action.Dangerous && !req.Confirmed {
		return nil, fmt.Errorf("dangerous operation requires confirmation")
	}

	// 构建命令
	cmdStr, err := e.BuildCommand(action, req.Args)
	if err != nil {
		return nil, err
	}

	// 执行
	start := time.Now()
	cmd := exec.CommandContext(ctx, "sudo", strings.Split(cmdStr, " ")...)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	duration := time.Since(start).Milliseconds()

	// 构建响应
	resp := &ExecuteResponse{
		Success:  err == nil,
		Output:   stdout.String(),
		Duration: duration,
	}

	if err != nil {
		resp.Error = stderr.String()
		if exitErr, ok := err.(*exec.ExitError); ok {
			resp.ExitCode = exitErr.ExitCode()
		} else {
			resp.ExitCode = -1
			resp.Error = err.Error()
		}
	}

	// 记录审计日志
	argsJSON, _ := json.Marshal(req.Args)
	log := &AuditLog{
		UserID:   userID,
		Username: username,
		ActionID: req.ActionID,
		Args:     string(argsJSON),
		Command:  "sudo " + cmdStr,
		Success:  resp.Success,
		Output:   truncateString(resp.Output, 1000),
		Error:    truncateString(resp.Error, 1000),
		ExitCode: resp.ExitCode,
		Duration: duration,
		ClientIP: clientIP,
	}
	
	if err := e.db.Create(log).Error; err != nil {
		e.logger.Error("Failed to save audit log", zap.Error(err))
	}

	e.logger.Info("Sudo command executed",
		zap.String("action", req.ActionID),
		zap.String("command", cmdStr),
		zap.Bool("success", resp.Success),
		zap.Int64("duration_ms", duration),
		zap.String("user", username),
	)

	return resp, nil
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
