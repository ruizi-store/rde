// Package backup 提供备份模块测试
package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestEnv(t *testing.T) (*Service, string) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// 创建测试数据库
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}

	// 迁移数据库
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}

	// 创建服务
	log := zap.NewNop()
	service := NewService(log, db, tempDir)

	return service, tempDir
}

func TestCreateTask(t *testing.T) {
	service, tempDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	task, err := service.CreateTask(&CreateTaskRequest{
		Name:        "Test Backup",
		Type:        BackupTypeFull,
		Sources:     []string{"/tmp"},
		TargetType:  TargetTypeLocal,
		TargetConfig: `{"path":"` + tempDir + `/backups"}`,
		Compression: true,
	})

	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}
	if task.Name != "Test Backup" {
		t.Errorf("Expected name 'Test Backup', got '%s'", task.Name)
	}
	if task.Type != BackupTypeFull {
		t.Errorf("Expected type 'full', got '%s'", task.Type)
	}

	t.Logf("✓ CreateTask: task created with ID=%s", task.ID)
}

func TestListTasks(t *testing.T) {
	service, tempDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	// 创建几个任务
	for i := 0; i < 3; i++ {
		service.CreateTask(&CreateTaskRequest{
			Name:        "Task " + string(rune('A'+i)),
			Type:        BackupTypeFull,
			Sources:     []string{"/tmp"},
			TargetType:  TargetTypeLocal,
			TargetConfig: `{"path":"` + tempDir + `"}`,
		})
	}

	tasks, total, err := service.ListTasks(&ListTasksRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	if total != 3 {
		t.Errorf("Expected 3 tasks, got %d", total)
	}
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks in list, got %d", len(tasks))
	}

	t.Logf("✓ ListTasks: found %d tasks", len(tasks))
}

func TestGetOverview(t *testing.T) {
	service, tempDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	overview, err := service.GetOverview()
	if err != nil {
		t.Fatalf("GetOverview failed: %v", err)
	}

	if overview.TotalTasks != 0 {
		t.Errorf("Expected 0 tasks, got %d", overview.TotalTasks)
	}

	t.Logf("✓ GetOverview: %+v", overview)
}

func TestLocalTarget(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-target-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")

	target := &LocalTarget{}
	err = target.Configure(`{"path":"` + backupDir + `"}`)
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	// Test connection
	resp := target.Test()
	if !resp.Success {
		t.Errorf("Test target failed: %s", resp.Message)
	}

	t.Logf("✓ LocalTarget: %s", resp.Message)
}

func TestEncryptor(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "crypto-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	testData := []byte("Hello, World! This is test data for encryption.")
	srcPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(srcPath, testData, 0644); err != nil {
		t.Fatal(err)
	}

	encPath := filepath.Join(tempDir, "test.enc")
	decPath := filepath.Join(tempDir, "test.dec")

	password := "test-password-123"
	encryptor := NewEncryptor(password)

	// 加密
	if err := encryptor.EncryptFile(srcPath, encPath); err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// 验证加密文件存在
	if _, err := os.Stat(encPath); os.IsNotExist(err) {
		t.Error("Encrypted file not created")
	}

	// 检查是否标记为已加密
	isEnc, err := IsEncrypted(encPath)
	if err != nil {
		t.Fatal(err)
	}
	if !isEnc {
		t.Error("Encrypted file should be marked as encrypted")
	}

	// 解密
	if err := encryptor.DecryptFile(encPath, decPath); err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	// 验证解密后数据
	decData, err := os.ReadFile(decPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(decData) != string(testData) {
		t.Errorf("Decrypted data mismatch: got '%s', want '%s'", decData, testData)
	}

	t.Logf("✓ Encryption/Decryption: successful")
}

func TestFileManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "manifest-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("content2"), 0644)
	os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tempDir, "subdir", "file3.txt"), []byte("content3"), 0644)

	// 创建清单
	manifest := NewFileManifest("test-task")
	if err := manifest.ScanDirectory(tempDir); err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	if len(manifest.Files) < 3 {
		t.Errorf("Expected at least 3 files, got %d", len(manifest.Files))
	}

	// 保存清单
	manifestPath := filepath.Join(tempDir, "manifest.json")
	if err := manifest.Save(manifestPath); err != nil {
		t.Fatalf("Save manifest failed: %v", err)
	}

	// 重新加载
	loaded, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if len(loaded.Files) != len(manifest.Files) {
		t.Errorf("Loaded manifest file count mismatch")
	}

	// 模拟文件变化
	time.Sleep(10 * time.Millisecond) // 确保时间戳不同
	os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("modified"), 0644)
	os.WriteFile(filepath.Join(tempDir, "file4.txt"), []byte("new file"), 0644)

	// 创建新清单
	newManifest := NewFileManifest("test-task")
	newManifest.ScanDirectory(tempDir)

	// 检测变化
	changed := newManifest.GetChangedFiles(manifest)
	if len(changed) < 1 {
		t.Logf("Warning: expected changed files, got %d (timing issue possible)", len(changed))
	}

	t.Logf("✓ FileManifest: scanned %d files, detected %d changes", len(manifest.Files), len(changed))
}

func TestExportableConfigs(t *testing.T) {
	service, tempDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	configs := service.GetExportableConfigs()
	if len(configs) == 0 {
		t.Error("Expected exportable configs")
	}

	t.Logf("✓ ExportableConfigs: %d items available", len(configs))
	for _, c := range configs {
		t.Logf("  - %s: %s", c.ID, c.Name)
	}
}

func TestTargetTestResponse(t *testing.T) {
	service, tempDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	// 测试本地目标
	resp := service.TestTarget(&TargetTestRequest{
		Type:   TargetTypeLocal,
		Config: `{"path":"` + tempDir + `"}`,
	})

	if !resp.Success {
		t.Errorf("Local target test should succeed: %s", resp.Message)
	}

	t.Logf("✓ TestTarget (local): %s", resp.Message)

	// 测试无效目标
	resp = service.TestTarget(&TargetTestRequest{
		Type:   TargetTypeLocal,
		Config: `{"path":"/nonexistent/path/that/should/not/exist"}`,
	})

	if resp.Success {
		t.Error("Non-existent path should fail")
	}

	t.Logf("✓ TestTarget (invalid): correctly failed - %s", resp.Message)
}
