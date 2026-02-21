// Package sudo 提供特权操作执行模块
package sudo

import "time"

// Action 定义允许的 sudo 操作
type Action struct {
	ID          string   // 操作标识
	Name        string   // 显示名称
	Description string   // 描述
	Command     string   // 命令模板 (使用 %s 作为参数占位符)
	ArgCount    int      // 期望的参数数量
	Dangerous   bool     // 是否危险操作（需要额外确认）
	AllowedArgs []string // 允许的参数值（为空表示不限制）
}

// ExecuteRequest 执行请求
type ExecuteRequest struct {
	ActionID  string   `json:"action_id" binding:"required"` // 操作 ID
	Args      []string `json:"args"`                         // 参数列表
	Confirmed bool     `json:"confirmed"`                    // 是否已确认（危险操作需要）
}

// ExecuteResponse 执行响应
type ExecuteResponse struct {
	Success  bool   `json:"success"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code"`
	Duration int64  `json:"duration_ms"` // 执行时长（毫秒）
}

// ActionInfo 操作信息（用于前端显示）
type ActionInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ArgCount    int    `json:"arg_count"`
	Dangerous   bool   `json:"dangerous"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Timestamp time.Time `gorm:"autoCreateTime" json:"timestamp"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	ActionID  string    `json:"action_id"`
	Args      string    `json:"args"` // JSON 序列化的参数
	Command   string    `json:"command"`
	Success   bool      `json:"success"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	ExitCode  int       `json:"exit_code"`
	Duration  int64     `json:"duration_ms"`
	ClientIP  string    `json:"client_ip"`
}

func (AuditLog) TableName() string {
	return "sudo_audit_logs"
}

// PreviewRequest 预览请求（返回将要执行的命令）
type PreviewRequest struct {
	ActionID string   `json:"action_id" binding:"required"`
	Args     []string `json:"args"`
}

// PreviewResponse 预览响应
type PreviewResponse struct {
	Action  ActionInfo `json:"action"`
	Command string     `json:"command"` // 将要执行的完整命令
}
