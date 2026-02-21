// Package backup 提供备份还原功能
package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// FileManifest 文件清单（用于增量备份）
type FileManifest struct {
	Version   int                    `json:"version"`
	CreatedAt time.Time              `json:"created_at"`
	TaskID    string                 `json:"task_id"`
	Files     map[string]FileEntry   `json:"files"`
}

// FileEntry 文件条目
type FileEntry struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	Checksum string    `json:"checksum,omitempty"` // 可选的内容校验
	IsDir    bool      `json:"is_dir"`
}

// NewFileManifest 创建新的文件清单
func NewFileManifest(taskID string) *FileManifest {
	return &FileManifest{
		Version:   1,
		CreatedAt: time.Now(),
		TaskID:    taskID,
		Files:     make(map[string]FileEntry),
	}
}

// ScanDirectory 扫描目录建立文件清单
func (m *FileManifest) ScanDirectory(rootPath string) error {
	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			relPath = path
		}

		m.Files[relPath] = FileEntry{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		}

		return nil
	})
}

// GetChangedFiles 获取相对于旧清单的变化文件
func (m *FileManifest) GetChangedFiles(old *FileManifest) []string {
	var changed []string

	for path, entry := range m.Files {
		if entry.IsDir {
			continue
		}

		oldEntry, exists := old.Files[path]
		if !exists {
			// 新文件
			changed = append(changed, path)
		} else if entry.ModTime.After(oldEntry.ModTime) || entry.Size != oldEntry.Size {
			// 修改过的文件
			changed = append(changed, path)
		}
	}

	return changed
}

// GetDeletedFiles 获取已删除的文件
func (m *FileManifest) GetDeletedFiles(old *FileManifest) []string {
	var deleted []string

	for path, entry := range old.Files {
		if entry.IsDir {
			continue
		}

		if _, exists := m.Files[path]; !exists {
			deleted = append(deleted, path)
		}
	}

	return deleted
}

// Save 保存清单到文件
func (m *FileManifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadManifest 从文件加载清单
func LoadManifest(path string) (*FileManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m FileManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

// IncrementalBackupInfo 增量备份信息
type IncrementalBackupInfo struct {
	BaseRecordID   string    `json:"base_record_id"`   // 基于哪个完整备份
	ChangedFiles   []string  `json:"changed_files"`    // 变更的文件列表
	DeletedFiles   []string  `json:"deleted_files"`    // 删除的文件列表
	ManifestPath   string    `json:"manifest_path"`    // 清单文件路径
	CreatedAt      time.Time `json:"created_at"`
}
