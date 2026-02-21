package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruizi-store/rde/backend/testutil"
)

func TestNewService(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})

	service := NewService(logger, db, t.TempDir())

	assert.NotNil(t, service)
	assert.Equal(t, logger, service.logger)
	assert.Equal(t, db, service.db)
}

func TestNewService_NilLogger(t *testing.T) {
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})

	service := NewService(nil, db, t.TempDir())

	assert.NotNil(t, service)
	assert.NotNil(t, service.logger) // 应该使用 zap.NewNop()
}

func TestService_Init(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	err := service.Init()

	require.NoError(t, err)

	// 验证目录已创建
	assert.DirExists(t, filepath.Join(dataDir, "sync", "uploads"))
	assert.DirExists(t, filepath.Join(dataDir, "sync", "files"))

	// 验证 TUS handler 已初始化
	assert.NotNil(t, service.GetTusHandler())
}

func TestService_GetStatus(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	status := service.GetStatus()

	assert.True(t, status.Running)
	assert.Equal(t, 0, status.TotalFiles)
	assert.Equal(t, int64(0), status.TotalSize)
	assert.Equal(t, 0, status.Uploading)
	assert.Contains(t, status.StoragePath, "sync/files")
}

func TestService_ListFiles_Empty(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	resp, err := service.ListFiles(&ListFilesRequest{Limit: 10})

	require.NoError(t, err)
	assert.Empty(t, resp.Files)
	assert.Equal(t, 0, resp.Total)
}

func TestService_ListFiles_WithFiles(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	// 插入测试数据
	testFiles := []SyncFileModel{
		{ID: "file-1", Filename: "test1.jpg", Size: 1024, MimeType: "image/jpeg", Status: "completed", CreatedAt: time.Now()},
		{ID: "file-2", Filename: "test2.mp4", Size: 2048, MimeType: "video/mp4", Status: "completed", CreatedAt: time.Now()},
		{ID: "file-3", Filename: "test3.pdf", Size: 512, MimeType: "application/pdf", Status: "completed", CreatedAt: time.Now()},
	}
	for _, f := range testFiles {
		require.NoError(t, db.Create(&f).Error)
	}

	resp, err := service.ListFiles(&ListFilesRequest{Limit: 10})

	require.NoError(t, err)
	assert.Len(t, resp.Files, 3)
	assert.Equal(t, 3, resp.Total)
}

func TestService_ListFiles_Pagination(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	// 插入 5 个文件
	for i := 0; i < 5; i++ {
		f := SyncFileModel{
			ID:        "file-" + string(rune('a'+i)),
			Filename:  "file" + string(rune('a'+i)) + ".txt",
			Size:      int64(100 * (i + 1)),
			Status:    "completed",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Hour),
		}
		require.NoError(t, db.Create(&f).Error)
	}

	// 第一页
	resp1, err := service.ListFiles(&ListFilesRequest{Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, resp1.Files, 2)
	assert.Equal(t, 5, resp1.Total)

	// 第二页
	resp2, err := service.ListFiles(&ListFilesRequest{Limit: 2, Offset: 2})
	require.NoError(t, err)
	assert.Len(t, resp2.Files, 2)
}

func TestService_GetFile(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	// 插入测试数据
	testFile := SyncFileModel{
		ID:       "test-file-id",
		Filename: "photo.jpg",
		Size:     4096,
		MimeType: "image/jpeg",
		SHA256:   "abc123",
		Path:     "/path/to/file.jpg",
		Status:   "completed",
	}
	require.NoError(t, db.Create(&testFile).Error)

	file, err := service.GetFile("test-file-id")

	require.NoError(t, err)
	assert.Equal(t, "test-file-id", file.ID)
	assert.Equal(t, "photo.jpg", file.Filename)
	assert.Equal(t, int64(4096), file.Size)
	assert.Equal(t, "image/jpeg", file.MimeType)
}

func TestService_GetFile_NotFound(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	file, err := service.GetFile("non-existent")

	assert.Error(t, err)
	assert.Nil(t, file)
}

func TestService_DeleteFile(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	// 创建物理文件
	filePath := filepath.Join(dataDir, "test-file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("test content"), 0644))

	// 插入数据库记录
	testFile := SyncFileModel{
		ID:       "delete-test",
		Filename: "test-file.txt",
		Size:     12,
		Path:     filePath,
		Status:   "completed",
	}
	require.NoError(t, db.Create(&testFile).Error)

	// 删除
	err := service.DeleteFile("delete-test")

	require.NoError(t, err)

	// 验证物理文件已删除
	_, statErr := os.Stat(filePath)
	assert.True(t, os.IsNotExist(statErr))

	// 验证数据库记录已删除
	var count int64
	db.Model(&SyncFileModel{}).Where("id = ?", "delete-test").Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestService_DeleteFile_NotFound(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	err := service.DeleteFile("non-existent")

	assert.Error(t, err)
}

func TestService_ListActiveUploads(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	// 插入上传会话
	sessions := []UploadSessionModel{
		{ID: "upload-1", Filename: "big-file.zip", Size: 1000000, Offset: 500000, ExpiresAt: time.Now().Add(1 * time.Hour)},
		{ID: "upload-2", Filename: "video.mp4", Size: 2000000, Offset: 0, ExpiresAt: time.Now().Add(2 * time.Hour)},
		{ID: "upload-3", Filename: "expired.txt", Size: 100, Offset: 50, ExpiresAt: time.Now().Add(-1 * time.Hour)}, // 已过期
	}
	for _, s := range sessions {
		require.NoError(t, db.Create(&s).Error)
	}

	uploads, err := service.ListActiveUploads()

	require.NoError(t, err)
	assert.Len(t, uploads, 2) // 排除已过期的
}

func TestService_GetUpload(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	// 插入上传会话
	session := UploadSessionModel{
		ID:        "upload-test",
		Filename:  "uploading.mp4",
		Size:      10000,
		Offset:    5000,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	require.NoError(t, db.Create(&session).Error)

	upload, err := service.GetUpload("upload-test")

	require.NoError(t, err)
	assert.Equal(t, "upload-test", upload.ID)
	assert.Equal(t, "uploading.mp4", upload.Filename)
	assert.Equal(t, 0.5, upload.Progress) // 5000/10000 = 0.5
}

func TestService_DownloadFile(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	// 插入测试数据
	testFile := SyncFileModel{
		ID:       "download-test",
		Filename: "document.pdf",
		Size:     2048,
		Path:     "/data/sync/files/document.pdf",
		Status:   "completed",
	}
	require.NoError(t, db.Create(&testFile).Error)

	path, filename, err := service.DownloadFile("download-test")

	require.NoError(t, err)
	assert.Equal(t, "/data/sync/files/document.pdf", path)
	assert.Equal(t, "document.pdf", filename)
}

func TestService_CalculateSHA256(t *testing.T) {
	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)

	// 创建测试文件
	testFile := filepath.Join(dataDir, "hashtest.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	hash, err := service.calculateSHA256(testFile)

	require.NoError(t, err)
	// "hello world" 的 SHA256
	assert.Equal(t, "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", hash)
}
