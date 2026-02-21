// Package samba Samba 类型定义
package samba

// SambaShare 共享配置
type SambaShare struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	Comment      string   `json:"comment"`
	Browseable   bool     `json:"browseable"`
	Writable     bool     `json:"writable"`
	GuestOK      bool     `json:"guest_ok"`
	ReadOnly     bool     `json:"read_only"`
	ValidUsers   []string `json:"valid_users"`
	InvalidUsers []string `json:"invalid_users"`
	WriteList    []string `json:"write_list"`
	ReadList     []string `json:"read_list"`
	CreateMask   string   `json:"create_mask"`
	DirMask      string   `json:"dir_mask"`
	ForceUser    string   `json:"force_user"`
	ForceGroup   string   `json:"force_group"`
	VFSObjects   []string `json:"vfs_objects"`
	RecycleBin   string   `json:"recycle_bin"`
}

// SambaGlobalConfig 全局配置
type SambaGlobalConfig struct {
	Workgroup    string `json:"workgroup"`
	ServerString string `json:"server_string"`
	NetbiosName  string `json:"netbios_name"`
	Security     string `json:"security"`
	MapToGuest   string `json:"map_to_guest"`
	LogLevel     string `json:"log_level"`
	MaxLogSize   string `json:"max_log_size"`
	ServerRole   string `json:"server_role"`
}

// SambaUser Samba 用户
type SambaUser struct {
	Username string `json:"username"`
	UID      int    `json:"uid"`
	Enabled  bool   `json:"enabled"`
}

// ServiceStatus 服务状态
type ServiceStatus struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Enabled   bool   `json:"enabled"`
	Version   string `json:"version"`
}

// SambaConfig 完整的 Samba 配置
type SambaConfig struct {
	Global SambaGlobalConfig `json:"global"`
	Shares []SambaShare      `json:"shares"`
}

// SambaSession 活动会话信息
type SambaSession struct {
	PID         int    `json:"pid"`
	Username    string `json:"username"`
	Group       string `json:"group"`
	Machine     string `json:"machine"`
	IPAddress   string `json:"ip_address"`
	ProtocolVer string `json:"protocol_ver"`
	Encryption  string `json:"encryption"`
	Signing     string `json:"signing"`
}

// SambaOpenFile 打开的文件信息
type SambaOpenFile struct {
	PID       int    `json:"pid"`
	Username  string `json:"username"`
	DenyMode  string `json:"deny_mode"`
	Access    string `json:"access"`
	ReadWrite string `json:"read_write"`
	OpLock    string `json:"oplock"`
	SharePath string `json:"share_path"`
	FileName  string `json:"file_name"`
}

// SambaShareConnection 共享连接信息
type SambaShareConnection struct {
	Service    string `json:"service"`
	PID        int    `json:"pid"`
	Machine    string `json:"machine"`
	Encryption string `json:"encryption"`
	Signing    string `json:"signing"`
}

// SessionsInfo 完整的会话信息
type SessionsInfo struct {
	Sessions    []SambaSession         `json:"sessions"`
	Shares      []SambaShareConnection `json:"shares"`
	OpenFiles   []SambaOpenFile        `json:"open_files"`
	TotalCount  int                    `json:"total_count"`
	UniqueUsers int                    `json:"unique_users"`
}

// CreateShareRequest 创建共享请求
type CreateShareRequest struct {
	Name       string   `json:"name" binding:"required"`
	Path       string   `json:"path" binding:"required"`
	Comment    string   `json:"comment"`
	Browseable *bool    `json:"browseable"`
	Writable   *bool    `json:"writable"`
	GuestOK    bool     `json:"guest_ok"`
	ValidUsers []string `json:"valid_users"`
}

// UpdateShareRequest 更新共享请求
type UpdateShareRequest struct {
	Path         string   `json:"path"`
	Comment      string   `json:"comment"`
	Browseable   *bool    `json:"browseable"`
	Writable     *bool    `json:"writable"`
	GuestOK      *bool    `json:"guest_ok"`
	ValidUsers   []string `json:"valid_users"`
	InvalidUsers []string `json:"invalid_users"`
	ReadOnly     *bool    `json:"read_only"`
	CreateMask   string   `json:"create_mask"`
	DirMask      string   `json:"dir_mask"`
}

// UpdateGlobalConfigRequest 更新全局配置请求
type UpdateGlobalConfigRequest struct {
	Workgroup    string `json:"workgroup"`
	ServerString string `json:"server_string"`
	NetbiosName  string `json:"netbios_name"`
}

// AddUserRequest 添加用户请求
type AddUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// SetPasswordRequest 设置密码请求
type SetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	Password string `json:"password" binding:"required"`
}
