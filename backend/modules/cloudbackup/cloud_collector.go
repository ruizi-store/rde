package cloudbackup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"gorm.io/gorm"
)

// CloudBackupCollector 云备份数据收集器
// 按 P0/P1/P2 优先级收集 RDE 配置和关键数据，打包加密后上传。
type CloudBackupCollector struct {
	db      *gorm.DB
	dataDir string // RDE 数据目录
}

// NewCloudBackupCollector 创建云备份收集器
func NewCloudBackupCollector(db *gorm.DB, dataDir string) *CloudBackupCollector {
	return &CloudBackupCollector{db: db, dataDir: dataDir}
}

// CollectedData 收集到的数据
type CollectedData struct {
	// P0 核心配置
	Settings       []map[string]interface{} `json:"settings,omitempty"`
	UserPrefs      []map[string]interface{} `json:"user_preferences,omitempty"`
	DesktopIcons   []map[string]interface{} `json:"desktop_icons,omitempty"`
	SystemApps     []map[string]interface{} `json:"system_apps,omitempty"`
	Connections    []map[string]interface{} `json:"connections,omitempty"`
	ProxyConfigs   []map[string]interface{} `json:"proxy_configs,omitempty"`

	// P1 服务配置
	SambaShares     []map[string]interface{} `json:"samba_shares,omitempty"`
	BackupTasks     []map[string]interface{} `json:"backup_tasks,omitempty"`
	Notifications   []map[string]interface{} `json:"notifications,omitempty"`
	DDNSConfigs     []map[string]interface{} `json:"ddns_configs,omitempty"`
	ModuleSettings  []map[string]interface{} `json:"module_settings,omitempty"`

	// 元数据
	Metadata BackupMetadata `json:"metadata"`
}

// BackupMetadata 备份元数据
type BackupMetadata struct {
	Version    string    `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	DeviceID   string    `json:"device_id,omitempty"`
	Hostname   string    `json:"hostname,omitempty"`
	Priorities []string  `json:"priorities"` // 包含的优先级 ["P0", "P1", "P2"]
}

// CollectOptions 收集选项
type CollectOptions struct {
	IncludeP0 bool   // 核心配置
	IncludeP1 bool   // 服务配置
	IncludeP2 bool   // 个性化数据
	DeviceID  string // 设备标识
}

// Collect 收集备份数据
func (c *CloudBackupCollector) Collect(opts CollectOptions) (*CollectedData, []string, error) {
	data := &CollectedData{
		Metadata: BackupMetadata{
			Version:   "1.0",
			CreatedAt: time.Now(),
			DeviceID:  opts.DeviceID,
		},
	}

	hostname, _ := os.Hostname()
	data.Metadata.Hostname = hostname

	var extraFiles []string // 额外需要打包的文件路径

	// P0: 核心配置
	if opts.IncludeP0 {
		data.Metadata.Priorities = append(data.Metadata.Priorities, "P0")
		c.collectP0(data)
	}

	// P1: 服务配置
	if opts.IncludeP1 {
		data.Metadata.Priorities = append(data.Metadata.Priorities, "P1")
		c.collectP1(data)
		extraFiles = append(extraFiles, c.collectP1Files()...)
	}

	// P2: 个性化数据
	if opts.IncludeP2 {
		data.Metadata.Priorities = append(data.Metadata.Priorities, "P2")
		extraFiles = append(extraFiles, c.collectP2Files()...)
	}

	return data, extraFiles, nil
}

// collectP0 收集 P0 核心配置（数据库表）
func (c *CloudBackupCollector) collectP0(data *CollectedData) {
	// 系统设置
	c.db.Table("setting_items").Find(&data.Settings)

	// 用户偏好
	c.db.Table("user_preferences").Find(&data.UserPrefs)

	// 桌面图标
	c.db.Table("desktop_icons").Find(&data.DesktopIcons)

	// 应用注册表
	c.db.Table("system_apps").Find(&data.SystemApps)

	// 远程连接（密码需要在恢复时解密）
	c.db.Table("connections").Find(&data.Connections)

	// 代理设置
	c.db.Table("system_proxy_configs").Find(&data.ProxyConfigs)
}

// collectP1 收集 P1 服务配置（数据库表）
func (c *CloudBackupCollector) collectP1(data *CollectedData) {
	// Samba 共享
	c.db.Table("samba_shares").Find(&data.SambaShares)

	// 备份任务
	c.db.Table("backup_tasks").Find(&data.BackupTasks)

	// 通知渠道
	c.db.Table("notification_channels").Find(&data.Notifications)

	// DDNS 配置
	c.db.Table("ddns_configs").Find(&data.DDNSConfigs)

	// 模块设置
	c.db.Table("module_settings").Find(&data.ModuleSettings)
}

// collectP1Files 收集 P1 需要打包的文件
func (c *CloudBackupCollector) collectP1Files() []string {
	var files []string

	// Docker Compose 文件
	composeDir := filepath.Join(c.dataDir, "docker", "compose")
	if entries, err := os.ReadDir(composeDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, filepath.Join(composeDir, e.Name()))
			}
		}
	}

	// Syncthing 配置
	syncthingConfig := filepath.Join(c.dataDir, "syncthing", "config.xml")
	if _, err := os.Stat(syncthingConfig); err == nil {
		files = append(files, syncthingConfig)
	}

	// aria2 配置
	aria2Config := filepath.Join(c.dataDir, "aria2", "aria2.conf")
	if _, err := os.Stat(aria2Config); err == nil {
		files = append(files, aria2Config)
	}

	return files
}

// collectP2Files 收集 P2 个性化数据文件
func (c *CloudBackupCollector) collectP2Files() []string {
	var files []string

	// 自定义壁纸
	wallpaperDir := filepath.Join(c.dataDir, "wallpapers", "custom")
	if entries, err := os.ReadDir(wallpaperDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, filepath.Join(wallpaperDir, e.Name()))
			}
		}
	}

	// 用户头像
	avatarDir := filepath.Join(c.dataDir, "avatars")
	if entries, err := os.ReadDir(avatarDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, filepath.Join(avatarDir, e.Name()))
			}
		}
	}

	return files
}

// PackageBackup 将收集的数据打包为 tar.gz 文件
// 返回打包后的文件路径
func (c *CloudBackupCollector) PackageBackup(data *CollectedData, extraFiles []string, outputDir string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("rde-cloud-backup-%s.tar.gz", timestamp)
	outputPath := filepath.Join(outputDir, backupName)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// 1. 写入数据库数据（JSON）
	dbJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("marshal data: %w", err)
	}

	if err := tarWriter.WriteHeader(&tar.Header{
		Name:    "database.json",
		Size:    int64(len(dbJSON)),
		Mode:    0644,
		ModTime: time.Now(),
	}); err != nil {
		os.Remove(outputPath)
		return "", err
	}
	if _, err := tarWriter.Write(dbJSON); err != nil {
		os.Remove(outputPath)
		return "", err
	}

	// 2. 写入额外文件
	for _, filePath := range extraFiles {
		if err := addFileToTar(tarWriter, filePath, c.dataDir); err != nil {
			// 跳过不存在的文件，不中断备份
			continue
		}
	}

	return outputPath, nil
}

// addFileToTar 添加文件到 tar 归档
func addFileToTar(tw *tar.Writer, filePath, baseDir string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	// 使用相对于 dataDir 的路径
	relPath, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		relPath = filepath.Base(filePath)
	}

	header := &tar.Header{
		Name:    "files/" + relPath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, f)
	return err
}

// EncryptFile 使用 AES-256-GCM 加密文件
// 密钥通过 PBKDF2 从密码派生
func EncryptFile(inputPath, outputPath, password string) error {
	// 生成随机 salt（16 字节）
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}

	// PBKDF2 派生 256-bit 密钥
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// 创建 AES-GCM
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	// 读取输入文件
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	// 加密
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// 写入输出文件：[salt(16)] + [ciphertext(nonce + encrypted + tag)]
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	if _, err := out.Write(salt); err != nil {
		os.Remove(outputPath)
		return err
	}
	if _, err := out.Write(ciphertext); err != nil {
		os.Remove(outputPath)
		return err
	}

	return nil
}

// DecryptFile 解密 AES-256-GCM 加密的文件
func DecryptFile(inputPath, outputPath, password string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	if len(data) < 16 {
		return fmt.Errorf("invalid encrypted file: too short")
	}

	// 提取 salt
	salt := data[:16]
	ciphertext := data[16:]

	// PBKDF2 派生密钥
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// AES-GCM 解密
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("invalid encrypted data")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decrypt failed (wrong password?): %w", err)
	}

	return os.WriteFile(outputPath, plaintext, 0600)
}

// CalculateSHA256 计算文件的 SHA256 校验和
func CalculateSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
