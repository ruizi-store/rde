package backup

import (
	"time"

	"gorm.io/gorm"
)

// BackupTaskModel 备份任务数据库模型
type BackupTaskModel struct {
	ID           string     `gorm:"primaryKey;size:36"`
	Name         string     `gorm:"not null;size:255"`
	Description  string     `gorm:"size:500"`
	Type         string     `gorm:"not null;size:20"` // full, incremental, config
	Sources      string     `gorm:"type:text"`        // JSON 数组
	TargetType   string     `gorm:"not null;size:20"` // local, s3, webdav, sftp
	TargetConfig string     `gorm:"type:text"`        // JSON 配置
	Schedule     string     `gorm:"size:100"`         // cron 表达式
	Retention    int        `gorm:"default:7"`
	Compression  bool       `gorm:"default:true"`
	Encryption   bool       `gorm:"default:false"`
	Enabled      bool       `gorm:"default:true"`
	LastRunAt    *time.Time `gorm:"index"`
	NextRunAt    *time.Time `gorm:"index"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}

func (BackupTaskModel) TableName() string {
	return "backup_tasks"
}

// BackupRecordModel 备份记录数据库模型
type BackupRecordModel struct {
	ID          string     `gorm:"primaryKey;size:36"`
	TaskID      string     `gorm:"not null;size:36;index"`
	Type        string     `gorm:"not null;size:20"`
	Size        int64      `gorm:"default:0"`
	FileCount   int        `gorm:"default:0"`
	FilePath    string     `gorm:"size:500"`
	Checksum    string     `gorm:"size:64"` // SHA256
	Status      string     `gorm:"not null;size:20;index"`
	Progress    int        `gorm:"default:0"`
	Message     string     `gorm:"size:500"`
	Error       string     `gorm:"type:text"`
	StartedAt   time.Time  `gorm:"not null"`
	CompletedAt *time.Time
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (BackupRecordModel) TableName() string {
	return "backup_records"
}

// RestoreRecordModel 还原记录数据库模型
type RestoreRecordModel struct {
	ID           string     `gorm:"primaryKey;size:36"`
	BackupID     string     `gorm:"not null;size:36;index"` // 关联的备份记录
	TargetPath   string     `gorm:"size:500"`
	Status       string     `gorm:"not null;size:20"`
	Progress     int        `gorm:"default:0"`
	CurrentFile  string     `gorm:"size:500"`
	Message      string     `gorm:"size:500"`
	Error        string     `gorm:"type:text"`
	StartedAt    time.Time  `gorm:"not null"`
	CompletedAt  *time.Time
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

func (RestoreRecordModel) TableName() string {
	return "restore_records"
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&BackupTaskModel{},
		&BackupRecordModel{},
		&RestoreRecordModel{},
	)
}

// ToBackupTask 转换为 BackupTask
func (m *BackupTaskModel) ToBackupTask() *BackupTask {
	return &BackupTask{
		ID:           m.ID,
		Name:         m.Name,
		Description:  m.Description,
		Type:         BackupType(m.Type),
		Sources:      parseJSONStringArray(m.Sources),
		TargetType:   TargetType(m.TargetType),
		TargetConfig: m.TargetConfig,
		Schedule:     m.Schedule,
		Retention:    m.Retention,
		Compression:  m.Compression,
		Encryption:   m.Encryption,
		Enabled:      m.Enabled,
		LastRunAt:    m.LastRunAt,
		NextRunAt:    m.NextRunAt,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

// ToBackupRecord 转换为 BackupRecord
func (m *BackupRecordModel) ToBackupRecord() *BackupRecord {
	return &BackupRecord{
		ID:          m.ID,
		TaskID:      m.TaskID,
		Type:        BackupType(m.Type),
		Size:        m.Size,
		FileCount:   m.FileCount,
		FilePath:    m.FilePath,
		Checksum:    m.Checksum,
		Status:      BackupStatus(m.Status),
		Progress:    m.Progress,
		Message:     m.Message,
		Error:       m.Error,
		StartedAt:   m.StartedAt,
		CompletedAt: m.CompletedAt,
	}
}

// ToRestoreStatus 转换为 RestoreStatus
func (m *RestoreRecordModel) ToRestoreStatus() *RestoreStatus {
	return &RestoreStatus{
		ID:          m.ID,
		RecordID:    m.BackupID,
		Status:      BackupStatus(m.Status),
		Progress:    m.Progress,
		CurrentFile: m.CurrentFile,
		Message:     m.Message,
		Error:       m.Error,
		StartedAt:   m.StartedAt,
		CompletedAt: m.CompletedAt,
	}
}
