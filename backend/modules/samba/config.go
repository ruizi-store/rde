// Package samba 提供 smb.conf 配置解析和写入
package samba

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	DefaultConfigPath = "/etc/samba/smb.conf"
)

// ConfigParser smb.conf 配置解析器
type ConfigParser struct {
	configPath string
}

// NewConfigParser 创建配置解析器
func NewConfigParser(configPath string) *ConfigParser {
	if configPath == "" {
		configPath = DefaultConfigPath
	}
	return &ConfigParser{configPath: configPath}
}

// Parse 解析 smb.conf
func (p *ConfigParser) Parse() (*SambaConfig, error) {
	file, err := os.Open(p.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open smb.conf: %w", err)
	}
	defer file.Close()

	config := &SambaConfig{
		Shares: make([]SambaShare, 0),
	}

	scanner := bufio.NewScanner(file)
	var currentSection string
	var currentShare *SambaShare
	sectionPattern := regexp.MustCompile(`^\s*\[([^\]]+)\]`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// 检查 section 头
		if matches := sectionPattern.FindStringSubmatch(line); len(matches) > 1 {
			// 保存之前的 share
			if currentShare != nil {
				config.Shares = append(config.Shares, *currentShare)
			}

			currentSection = strings.ToLower(matches[1])
			if currentSection != "global" {
				currentShare = &SambaShare{Name: matches[1]}
			} else {
				currentShare = nil
			}
			continue
		}

		// 解析键值对
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		if currentSection == "global" {
			p.parseGlobalOption(&config.Global, key, value)
		} else if currentShare != nil {
			p.parseShareOption(currentShare, key, value)
		}
	}

	// 保存最后一个 share
	if currentShare != nil {
		config.Shares = append(config.Shares, *currentShare)
	}

	return config, scanner.Err()
}

func (p *ConfigParser) parseGlobalOption(global *SambaGlobalConfig, key, value string) {
	switch key {
	case "workgroup":
		global.Workgroup = value
	case "server string":
		global.ServerString = value
	case "netbios name":
		global.NetbiosName = value
	case "security":
		global.Security = value
	case "map to guest":
		global.MapToGuest = value
	case "log level":
		global.LogLevel = value
	case "max log size":
		global.MaxLogSize = value
	case "server role":
		global.ServerRole = value
	}
}

func (p *ConfigParser) parseShareOption(share *SambaShare, key, value string) {
	switch key {
	case "path":
		share.Path = value
	case "comment":
		share.Comment = value
	case "browseable", "browsable":
		share.Browseable = parseBool(value)
	case "writable", "writeable":
		share.Writable = parseBool(value)
	case "read only":
		share.ReadOnly = parseBool(value)
	case "guest ok", "public":
		share.GuestOK = parseBool(value)
	case "valid users":
		share.ValidUsers = parseUserList(value)
	case "invalid users":
		share.InvalidUsers = parseUserList(value)
	case "write list":
		share.WriteList = parseUserList(value)
	case "read list":
		share.ReadList = parseUserList(value)
	case "create mask", "create mode":
		share.CreateMask = value
	case "directory mask", "directory mode":
		share.DirMask = value
	case "force user":
		share.ForceUser = value
	case "force group":
		share.ForceGroup = value
	case "vfs objects":
		share.VFSObjects = strings.Fields(value)
	case "recycle:repository":
		share.RecycleBin = value
	}
}

// GetShare 获取指定共享
func (p *ConfigParser) GetShare(name string) (*SambaShare, error) {
	config, err := p.Parse()
	if err != nil {
		return nil, err
	}

	for _, share := range config.Shares {
		if strings.EqualFold(share.Name, name) {
			return &share, nil
		}
	}
	return nil, fmt.Errorf("share '%s' not found", name)
}

// AddShare 添加共享
func (p *ConfigParser) AddShare(share SambaShare) error {
	config, err := p.Parse()
	if err != nil {
		return err
	}

	// 检查是否已存在
	for _, s := range config.Shares {
		if strings.EqualFold(s.Name, share.Name) {
			return fmt.Errorf("share '%s' already exists", share.Name)
		}
	}

	config.Shares = append(config.Shares, share)
	return p.Write(config)
}

// UpdateShare 更新共享
func (p *ConfigParser) UpdateShare(share SambaShare) error {
	config, err := p.Parse()
	if err != nil {
		return err
	}

	found := false
	for i, s := range config.Shares {
		if strings.EqualFold(s.Name, share.Name) {
			config.Shares[i] = share
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("share '%s' not found", share.Name)
	}

	return p.Write(config)
}

// DeleteShare 删除共享
func (p *ConfigParser) DeleteShare(name string) error {
	config, err := p.Parse()
	if err != nil {
		return err
	}

	found := false
	shares := make([]SambaShare, 0, len(config.Shares))
	for _, s := range config.Shares {
		if strings.EqualFold(s.Name, name) {
			found = true
			continue
		}
		shares = append(shares, s)
	}

	if !found {
		return fmt.Errorf("share '%s' not found", name)
	}

	config.Shares = shares
	return p.Write(config)
}

// Write 写入配置
func (p *ConfigParser) Write(config *SambaConfig) error {
	// 先备份
	backupPath := p.configPath + ".bak"
	if data, err := os.ReadFile(p.configPath); err == nil {
		os.WriteFile(backupPath, data, 0644)
	}

	// 确保目录存在
	dir := filepath.Dir(p.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	file, err := os.Create(p.configPath)
	if err != nil {
		return fmt.Errorf("failed to create smb.conf: %w", err)
	}
	defer file.Close()

	// 写入头部注释
	fmt.Fprintln(file, "# Samba configuration file")
	fmt.Fprintln(file, "# Generated by RDE")
	fmt.Fprintln(file)

	// 写入 [global] 部分
	fmt.Fprintln(file, "[global]")
	p.writeGlobalSection(file, &config.Global)
	fmt.Fprintln(file)

	// 写入共享
	for _, share := range config.Shares {
		fmt.Fprintf(file, "[%s]\n", share.Name)
		p.writeShareSection(file, &share)
		fmt.Fprintln(file)
	}

	return nil
}

func (p *ConfigParser) writeGlobalSection(file *os.File, global *SambaGlobalConfig) {
	// 写入基本配置，使用默认值
	workgroup := global.Workgroup
	if workgroup == "" {
		workgroup = "WORKGROUP"
	}
	fmt.Fprintf(file, "   workgroup = %s\n", workgroup)

	serverString := global.ServerString
	if serverString == "" {
		serverString = "RDE Samba Server"
	}
	fmt.Fprintf(file, "   server string = %s\n", serverString)

	if global.NetbiosName != "" {
		fmt.Fprintf(file, "   netbios name = %s\n", global.NetbiosName)
	}

	security := global.Security
	if security == "" {
		security = "user"
	}
	fmt.Fprintf(file, "   security = %s\n", security)

	if global.MapToGuest != "" {
		fmt.Fprintf(file, "   map to guest = %s\n", global.MapToGuest)
	}

	if global.LogLevel != "" {
		fmt.Fprintf(file, "   log level = %s\n", global.LogLevel)
	}

	if global.MaxLogSize != "" {
		fmt.Fprintf(file, "   max log size = %s\n", global.MaxLogSize)
	}

	if global.ServerRole != "" {
		fmt.Fprintf(file, "   server role = %s\n", global.ServerRole)
	}

	// 默认设置
	fmt.Fprintln(file, "   unix charset = UTF-8")
	fmt.Fprintln(file, "   dos charset = CP936")
	fmt.Fprintln(file, "   logging = systemd")
}

func (p *ConfigParser) writeShareSection(file *os.File, share *SambaShare) {
	fmt.Fprintf(file, "   path = %s\n", share.Path)

	if share.Comment != "" {
		fmt.Fprintf(file, "   comment = %s\n", share.Comment)
	}

	fmt.Fprintf(file, "   browseable = %s\n", boolToStr(share.Browseable))
	fmt.Fprintf(file, "   writable = %s\n", boolToStr(share.Writable))
	fmt.Fprintf(file, "   read only = %s\n", boolToStr(share.ReadOnly))
	fmt.Fprintf(file, "   guest ok = %s\n", boolToStr(share.GuestOK))

	if len(share.ValidUsers) > 0 {
		fmt.Fprintf(file, "   valid users = %s\n", strings.Join(share.ValidUsers, " "))
	}

	if len(share.InvalidUsers) > 0 {
		fmt.Fprintf(file, "   invalid users = %s\n", strings.Join(share.InvalidUsers, " "))
	}

	if len(share.WriteList) > 0 {
		fmt.Fprintf(file, "   write list = %s\n", strings.Join(share.WriteList, " "))
	}

	if len(share.ReadList) > 0 {
		fmt.Fprintf(file, "   read list = %s\n", strings.Join(share.ReadList, " "))
	}

	if share.CreateMask != "" {
		fmt.Fprintf(file, "   create mask = %s\n", share.CreateMask)
	}

	if share.DirMask != "" {
		fmt.Fprintf(file, "   directory mask = %s\n", share.DirMask)
	}

	if share.ForceUser != "" {
		fmt.Fprintf(file, "   force user = %s\n", share.ForceUser)
	}

	if share.ForceGroup != "" {
		fmt.Fprintf(file, "   force group = %s\n", share.ForceGroup)
	}

	if len(share.VFSObjects) > 0 {
		fmt.Fprintf(file, "   vfs objects = %s\n", strings.Join(share.VFSObjects, " "))
	}

	if share.RecycleBin != "" {
		fmt.Fprintf(file, "   recycle:repository = %s\n", share.RecycleBin)
	}
}

// ==================== 辅助函数 ====================

func parseBool(value string) bool {
	v := strings.ToLower(value)
	return v == "yes" || v == "true" || v == "1"
}

func boolToStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func parseUserList(value string) []string {
	users := make([]string, 0)
	for _, u := range strings.Fields(value) {
		u = strings.Trim(u, ",")
		if u != "" {
			users = append(users, u)
		}
	}
	return users
}

// ValidateShareName 验证共享名称
func ValidateShareName(name string) error {
	if name == "" {
		return fmt.Errorf("share name cannot be empty")
	}
	if len(name) > 80 {
		return fmt.Errorf("share name too long (max 80 chars)")
	}
	reserved := []string{"global", "homes", "printers", "print$", "ipc$"}
	lowerName := strings.ToLower(name)
	for _, r := range reserved {
		if lowerName == r {
			return fmt.Errorf("'%s' is a reserved share name", name)
		}
	}
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "[", "]"}
	for _, c := range invalidChars {
		if strings.Contains(name, c) {
			return fmt.Errorf("share name contains invalid character: %s", c)
		}
	}
	return nil
}

// ValidateSharePath 验证共享路径
func ValidateSharePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute")
	}
	return nil
}
