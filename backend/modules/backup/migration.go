// Package backup 提供备份还原功能
package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MigrationPackage 迁移包结构
type MigrationPackage struct {
	Version     string                 `json:"version"`
	CreatedAt   time.Time              `json:"created_at"`
	SourceHost  string                 `json:"source_host"`
	Description string                 `json:"description"`
	Contents    []MigrationContent     `json:"contents"`
	PathMapping map[string]string      `json:"path_mapping"` // 路径映射
	Metadata    map[string]interface{} `json:"metadata"`
}

// MigrationContent 迁移内容条目
type MigrationContent struct {
	Type       string `json:"type"`        // config, data, file
	Category   string `json:"category"`    // users, docker, samba, etc.
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	Size       int64  `json:"size"`
}

// MigrationService 迁移服务
type MigrationService struct {
	service *Service
	dataDir string
}

// NewMigrationService 创建迁移服务
func NewMigrationService(service *Service) *MigrationService {
	return &MigrationService{
		service: service,
		dataDir: service.dataDir,
	}
}

// ExportMigrationPackage 导出迁移包
func (m *MigrationService) ExportMigrationPackage(req *MigrationExportRequest) (string, error) {
	pkg := &MigrationPackage{
		Version:     "1.0",
		CreatedAt:   time.Now(),
		SourceHost:  hostname(),
		Description: req.Description,
		Contents:    []MigrationContent{},
		PathMapping: make(map[string]string),
		Metadata:    make(map[string]interface{}),
	}

	// 创建迁移包存储路径
	timestamp := time.Now().Format("20060102-150405")
	packageName := fmt.Sprintf("rde-migration-%s.tar.gz", timestamp)
	packagePath := filepath.Join(m.dataDir, "backups", "migrations", packageName)

	if err := os.MkdirAll(filepath.Dir(packagePath), 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建归档文件
	f, err := os.Create(packagePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzWriter := gzip.NewWriter(f)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// 收集要迁移的内容
	for _, item := range req.IncludeItems {
		switch item {
		case "users":
			// 导出用户数据
			if err := m.addUsersToPackage(tarWriter, pkg); err != nil {
				return "", fmt.Errorf("导出用户失败: %w", err)
			}

		case "docker":
			// 导出Docker配置
			if err := m.addDockerToPackage(tarWriter, pkg); err != nil {
				return "", fmt.Errorf("导出Docker配置失败: %w", err)
			}

		case "samba":
			// 导出Samba配置
			if err := m.addSambaToPackage(tarWriter, pkg); err != nil {
				return "", fmt.Errorf("导出Samba配置失败: %w", err)
			}

		case "settings":
			// 导出系统设置
			if err := m.addSettingsToPackage(tarWriter, pkg); err != nil {
				return "", fmt.Errorf("导出设置失败: %w", err)
			}
		}
	}

	// 写入包元数据
	metaData, _ := json.MarshalIndent(pkg, "", "  ")
	header := &tar.Header{
		Name: "migration.json",
		Mode: 0644,
		Size: int64(len(metaData)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return "", err
	}
	if _, err := tarWriter.Write(metaData); err != nil {
		return "", err
	}

	return packagePath, nil
}

// ImportMigrationPackage 导入迁移包
func (m *MigrationService) ImportMigrationPackage(packagePath string, req *MigrationImportRequest) (*MigrationResult, error) {
	result := &MigrationResult{
		StartedAt: time.Now(),
		Items:     []MigrationItemResult{},
	}

	// 打开迁移包
	f, err := os.Open(packagePath)
	if err != nil {
		return nil, fmt.Errorf("打开迁移包失败: %w", err)
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("解压失败: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	// 临时目录存放解压内容
	tempDir := filepath.Join(m.dataDir, "backups", "temp", "migration-import")
	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	var pkg *MigrationPackage

	// 解压并读取元数据
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		targetPath := filepath.Join(tempDir, header.Name)

		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(targetPath, 0755)
			continue
		}

		// 确保目录存在
		os.MkdirAll(filepath.Dir(targetPath), 0755)

		outFile, err := os.Create(targetPath)
		if err != nil {
			return nil, err
		}
		io.Copy(outFile, tarReader)
		outFile.Close()

		// 读取元数据
		if header.Name == "migration.json" {
			data, _ := os.ReadFile(targetPath)
			json.Unmarshal(data, &pkg)
		}
	}

	if pkg == nil {
		return nil, fmt.Errorf("无效的迁移包：缺少元数据")
	}

	result.Package = pkg

	// 执行迁移
	for _, content := range pkg.Contents {
		itemResult := MigrationItemResult{
			Category: content.Category,
			Type:     content.Type,
		}

		// 检查是否在导入列表中
		if !contains(req.IncludeItems, content.Category) && len(req.IncludeItems) > 0 {
			itemResult.Status = "skipped"
			result.Items = append(result.Items, itemResult)
			continue
		}

		// 根据类型执行导入
		sourcePath := filepath.Join(tempDir, content.SourcePath)
		targetPath := m.mapPath(content.TargetPath, req.PathMapping)

		if err := m.importContent(content, sourcePath, targetPath, req.Overwrite); err != nil {
			itemResult.Status = "failed"
			itemResult.Error = err.Error()
		} else {
			itemResult.Status = "success"
		}

		result.Items = append(result.Items, itemResult)
	}

	result.CompletedAt = time.Now()
	return result, nil
}

// addUsersToPackage 添加用户数据到迁移包
func (m *MigrationService) addUsersToPackage(tw *tar.Writer, pkg *MigrationPackage) error {
	// 从数据库导出用户信息
	var users []map[string]interface{}
	m.service.db.Table("users").Select("id, username, email, role, created_at, avatar, ssh_public_key").Find(&users)

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name: "users/users.json",
		Mode: 0644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}

	pkg.Contents = append(pkg.Contents, MigrationContent{
		Type:       "config",
		Category:   "users",
		SourcePath: "users/users.json",
		Size:       int64(len(data)),
	})

	return nil
}

// addDockerToPackage 添加Docker配置到迁移包
func (m *MigrationService) addDockerToPackage(tw *tar.Writer, pkg *MigrationPackage) error {
	// 收集Docker compose文件
	dockerDir := filepath.Join(m.dataDir, "docker")
	if _, err := os.Stat(dockerDir); os.IsNotExist(err) {
		return nil // 目录不存在则跳过
	}

	return filepath.Walk(dockerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// 只收集 compose 文件
		if !strings.HasSuffix(info.Name(), ".yml") && !strings.HasSuffix(info.Name(), ".yaml") {
			return nil
		}

		relPath, _ := filepath.Rel(m.dataDir, path)
		tarPath := filepath.Join("docker", relPath)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		header := &tar.Header{
			Name: tarPath,
			Mode: 0644,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}

		pkg.Contents = append(pkg.Contents, MigrationContent{
			Type:       "config",
			Category:   "docker",
			SourcePath: tarPath,
			TargetPath: path,
			Size:       int64(len(data)),
		})

		return nil
	})
}

// addSambaToPackage 添加Samba配置到迁移包
func (m *MigrationService) addSambaToPackage(tw *tar.Writer, pkg *MigrationPackage) error {
	// 从数据库导出Samba共享配置
	var shares []map[string]interface{}
	m.service.db.Table("samba_shares").Find(&shares)

	if len(shares) == 0 {
		return nil
	}

	data, err := json.MarshalIndent(shares, "", "  ")
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name: "samba/shares.json",
		Mode: 0644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}

	pkg.Contents = append(pkg.Contents, MigrationContent{
		Type:       "config",
		Category:   "samba",
		SourcePath: "samba/shares.json",
		Size:       int64(len(data)),
	})

	return nil
}

// addSettingsToPackage 添加系统设置到迁移包
func (m *MigrationService) addSettingsToPackage(tw *tar.Writer, pkg *MigrationPackage) error {
	var settings []map[string]interface{}
	m.service.db.Table("settings").Find(&settings)

	if len(settings) == 0 {
		return nil
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name: "settings/settings.json",
		Mode: 0644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}

	pkg.Contents = append(pkg.Contents, MigrationContent{
		Type:       "config",
		Category:   "settings",
		SourcePath: "settings/settings.json",
		Size:       int64(len(data)),
	})

	return nil
}

// mapPath 映射路径
func (m *MigrationService) mapPath(originalPath string, mapping map[string]string) string {
	if mapping == nil {
		return originalPath
	}

	for old, new := range mapping {
		if strings.HasPrefix(originalPath, old) {
			return strings.Replace(originalPath, old, new, 1)
		}
	}

	return originalPath
}

// importContent 导入内容
func (m *MigrationService) importContent(content MigrationContent, sourcePath, targetPath string, overwrite bool) error {
	switch content.Type {
	case "config":
		// 配置类型需要解析并导入数据库
		return m.importConfigContent(content, sourcePath)
	case "file":
		// 文件类型直接复制
		return copyFile(sourcePath, targetPath, overwrite)
	}
	return nil
}

// importConfigContent 导入配置内容
func (m *MigrationService) importConfigContent(content MigrationContent, sourcePath string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	switch content.Category {
	case "settings":
		var settings []map[string]interface{}
		if err := json.Unmarshal(data, &settings); err != nil {
			return err
		}
		for _, s := range settings {
			// 使用 upsert 逻辑
			key, _ := s["key"].(string)
			if key != "" {
				m.service.db.Table("settings").Where("key = ?", key).Assign(s).FirstOrCreate(&map[string]interface{}{})
			}
		}
	// 可以继续添加其他类型的处理
	}

	return nil
}

// copyFile 复制文件
func copyFile(src, dst string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(dst); err == nil {
			return nil // 文件已存在，跳过
		}
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// hostname 获取主机名
func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// contains 检查切片是否包含元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// MigrationExportRequest 迁移导出请求
type MigrationExportRequest struct {
	Description  string   `json:"description"`
	IncludeItems []string `json:"include_items"` // users, docker, samba, settings, etc.
}

// MigrationImportRequest 迁移导入请求
type MigrationImportRequest struct {
	PackagePath  string            `json:"package_path"`
	IncludeItems []string          `json:"include_items"` // 空则导入全部
	PathMapping  map[string]string `json:"path_mapping"`  // 路径映射
	Overwrite    bool              `json:"overwrite"`
}

// MigrationResult 迁移结果
type MigrationResult struct {
	Package     *MigrationPackage       `json:"package"`
	Items       []MigrationItemResult   `json:"items"`
	StartedAt   time.Time               `json:"started_at"`
	CompletedAt time.Time               `json:"completed_at"`
}

// MigrationItemResult 迁移项结果
type MigrationItemResult struct {
	Category string `json:"category"`
	Type     string `json:"type"`
	Status   string `json:"status"` // success, failed, skipped
	Error    string `json:"error,omitempty"`
}
