package cloudbackup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CloudRestoreService 云端恢复服务
type CloudRestoreService struct {
	db      *gorm.DB
	dataDir string
	logger  *zap.Logger
}

// NewCloudRestoreService 创建云端恢复服务
func NewCloudRestoreService(db *gorm.DB, dataDir string, logger *zap.Logger) *CloudRestoreService {
	return &CloudRestoreService{db: db, dataDir: dataDir, logger: logger}
}

// CloudBackupListItem 云端备份列表项
type CloudBackupListItem struct {
	ID        string `json:"id"`
	DeviceID  string `json:"device_id"`
	Hostname  string `json:"hostname"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
	Version   string `json:"version"`
}

// ListCloudBackups 列出云端可用备份
func (s *CloudRestoreService) ListCloudBackups(cloudToken, cloudURL string) ([]CloudBackupListItem, error) {
	target := &CloudTarget{}
	cfg, _ := json.Marshal(CloudTargetConfig{
		CloudToken: cloudToken,
		CloudURL:   cloudURL,
	})
	if err := target.Configure(string(cfg)); err != nil {
		return nil, fmt.Errorf("配置云目标失败: %w", err)
	}

	files, err := target.List()
	if err != nil {
		return nil, fmt.Errorf("获取备份列表失败: %w", err)
	}

	var items []CloudBackupListItem
	for _, f := range files {
		items = append(items, CloudBackupListItem{
			ID:        f.Name,
			Size:      f.Size,
			CreatedAt: fmt.Sprintf("%d", f.ModTime),
		})
	}

	return items, nil
}

// RestoreFromCloud 从云端恢复
func (s *CloudRestoreService) RestoreFromCloud(cloudToken, cloudURL, backupID, password string, progress func(stage string, pct int)) error {
	if progress == nil {
		progress = func(string, int) {}
	}

	// 1. 配置云目标
	progress("connecting", 5)
	target := &CloudTarget{}
	cfg, _ := json.Marshal(CloudTargetConfig{
		CloudToken: cloudToken,
		CloudURL:   cloudURL,
	})
	if err := target.Configure(string(cfg)); err != nil {
		return fmt.Errorf("配置云目标失败: %w", err)
	}

	// 2. 下载备份文件
	progress("downloading", 10)
	tmpDir, err := os.MkdirTemp("", "rde-cloud-restore-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	encryptedPath := filepath.Join(tmpDir, "backup.enc")
	remotePath := fmt.Sprintf("cloud://%s", backupID)
	if err := target.Download(remotePath, encryptedPath, func(pct int) {
		progress("downloading", 10+pct*40/100)
	}); err != nil {
		return fmt.Errorf("下载备份失败: %w", err)
	}

	// 3. 解密
	progress("decrypting", 55)
	decryptedPath := filepath.Join(tmpDir, "backup.tar.gz")
	if err := DecryptFile(encryptedPath, decryptedPath, password); err != nil {
		return fmt.Errorf("解密失败（密码错误？）: %w", err)
	}

	// 4. 解压
	progress("extracting", 65)
	extractDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("创建解压目录失败: %w", err)
	}
	if err := extractTarGz(decryptedPath, extractDir); err != nil {
		return fmt.Errorf("解压失败: %w", err)
	}

	// 5. 导入数据库数据
	progress("importing", 75)
	dataFile := filepath.Join(extractDir, "data.json")
	if _, err := os.Stat(dataFile); err == nil {
		if err := s.importCollectedData(dataFile); err != nil {
			s.logger.Warn("导入数据库数据部分失败", zap.Error(err))
			// 继续执行，不中断
		}
	}

	// 6. 恢复配置文件
	progress("restoring_files", 85)
	s.restoreFiles(extractDir)

	progress("completed", 100)
	s.logger.Info("云端恢复完成", zap.String("backup_id", backupID))
	return nil
}

// importCollectedData 导入收集的数据到数据库
func (s *CloudRestoreService) importCollectedData(dataFile string) error {
	raw, err := os.ReadFile(dataFile)
	if err != nil {
		return fmt.Errorf("读取数据文件失败: %w", err)
	}

	var data CollectedData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("解析数据文件失败: %w", err)
	}

	var errs []string

	// P0 核心配置
	if len(data.Settings) > 0 {
		if err := s.upsertRows("setting_items", data.Settings); err != nil {
			errs = append(errs, "settings: "+err.Error())
		}
	}
	if len(data.UserPrefs) > 0 {
		if err := s.upsertRows("user_preferences", data.UserPrefs); err != nil {
			errs = append(errs, "user_preferences: "+err.Error())
		}
	}
	if len(data.DesktopIcons) > 0 {
		if err := s.upsertRows("desktop_icons", data.DesktopIcons); err != nil {
			errs = append(errs, "desktop_icons: "+err.Error())
		}
	}
	if len(data.SystemApps) > 0 {
		if err := s.upsertRows("system_apps", data.SystemApps); err != nil {
			errs = append(errs, "system_apps: "+err.Error())
		}
	}
	if len(data.Connections) > 0 {
		if err := s.upsertRows("connections", data.Connections); err != nil {
			errs = append(errs, "connections: "+err.Error())
		}
	}
	if len(data.ProxyConfigs) > 0 {
		if err := s.upsertRows("proxy_configs", data.ProxyConfigs); err != nil {
			errs = append(errs, "proxy_configs: "+err.Error())
		}
	}

	// P1 服务配置
	if len(data.SambaShares) > 0 {
		if err := s.upsertRows("samba_shares", data.SambaShares); err != nil {
			errs = append(errs, "samba_shares: "+err.Error())
		}
	}
	if len(data.BackupTasks) > 0 {
		if err := s.upsertRows("backup_tasks", data.BackupTasks); err != nil {
			errs = append(errs, "backup_tasks: "+err.Error())
		}
	}
	if len(data.Notifications) > 0 {
		if err := s.upsertRows("notifications", data.Notifications); err != nil {
			errs = append(errs, "notifications: "+err.Error())
		}
	}
	if len(data.DDNSConfigs) > 0 {
		if err := s.upsertRows("ddns_configs", data.DDNSConfigs); err != nil {
			errs = append(errs, "ddns_configs: "+err.Error())
		}
	}
	if len(data.ModuleSettings) > 0 {
		if err := s.upsertRows("module_settings", data.ModuleSettings); err != nil {
			errs = append(errs, "module_settings: "+err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("部分表导入失败: %s", strings.Join(errs, "; "))
	}
	return nil
}

// upsertRows 批量 upsert 数据（有 id 则更新，无则插入）
func (s *CloudRestoreService) upsertRows(table string, rows []map[string]interface{}) error {
	for _, row := range rows {
		// 使用 GORM 的 Clauses 实现 upsert
		result := s.db.Table(table).Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(row)
		if result.Error != nil {
			// 如果 upsert 失败，尝试普通插入
			s.db.Table(table).Create(row)
		}
	}
	return nil
}

// restoreFiles 恢复额外的配置文件
func (s *CloudRestoreService) restoreFiles(extractDir string) {
	filesDir := filepath.Join(extractDir, "files")
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		return
	}

	// 遍历 files/ 目录，按相对路径还原到数据目录
	filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(filesDir, path)
		destPath := filepath.Join(s.dataDir, relPath)

		// 创建目标目录
		os.MkdirAll(filepath.Dir(destPath), 0755)

		// 复制文件
		src, err := os.Open(path)
		if err != nil {
			s.logger.Warn("打开备份文件失败", zap.String("path", path), zap.Error(err))
			return nil
		}
		defer src.Close()

		dst, err := os.Create(destPath)
		if err != nil {
			s.logger.Warn("创建目标文件失败", zap.String("path", destPath), zap.Error(err))
			return nil
		}
		defer dst.Close()

		io.Copy(dst, src)
		os.Chmod(destPath, info.Mode())
		s.logger.Debug("已恢复文件", zap.String("path", relPath))
		return nil
	})
}

// extractTarGz 解压 tar.gz 文件
func extractTarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dst, hdr.Name)

		// 安全检查：防止路径穿越
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dst)+string(os.PathSeparator)) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
			os.Chmod(target, os.FileMode(hdr.Mode))
		}
	}
	return nil
}
