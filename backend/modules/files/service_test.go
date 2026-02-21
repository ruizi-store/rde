package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestService(t *testing.T) (*Service, string) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "files_test_*")
	require.NoError(t, err)

	logger := zap.NewNop()
	service := NewService(logger, []string{tempDir})

	return service, tempDir
}

func TestService_List(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	// 创建测试文件和目录
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("hello"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("world"), 0644))

	ctx := context.Background()
	resp, err := service.List(ctx, &ListRequest{
		Path:  tempDir,
		Index: 1,
		Size:  10,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(3), resp.Total)
	assert.Len(t, resp.Content, 3)
	// 目录应该在前面
	assert.True(t, resp.Content[0].IsDir)
	assert.Equal(t, "subdir", resp.Content[0].Name)
}

func TestService_List_Pagination(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	// 创建多个文件
	for i := 0; i < 10; i++ {
		require.NoError(t, os.WriteFile(
			filepath.Join(tempDir, filepath.Base(tempDir)+string(rune('a'+i))+".txt"),
			[]byte("content"),
			0644,
		))
	}

	ctx := context.Background()
	
	// 第一页
	resp1, err := service.List(ctx, &ListRequest{
		Path:  tempDir,
		Index: 1,
		Size:  3,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(10), resp1.Total)
	assert.Len(t, resp1.Content, 3)

	// 第二页
	resp2, err := service.List(ctx, &ListRequest{
		Path:  tempDir,
		Index: 2,
		Size:  3,
	})
	require.NoError(t, err)
	assert.Len(t, resp2.Content, 3)
}

func TestService_CreateDir(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	newDir := filepath.Join(tempDir, "new", "nested", "dir")

	err := service.CreateDir(ctx, newDir)
	require.NoError(t, err)

	info, err := os.Stat(newDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestService_CreateFile(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	newFile := filepath.Join(tempDir, "new_file.txt")
	content := []byte("test content")

	err := service.CreateFile(ctx, newFile, content)
	require.NoError(t, err)

	data, err := os.ReadFile(newFile)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestService_CreateFile_AlreadyExists(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	existingFile := filepath.Join(tempDir, "existing.txt")
	require.NoError(t, os.WriteFile(existingFile, []byte("existing"), 0644))

	err := service.CreateFile(ctx, existingFile, []byte("new"))
	assert.ErrorIs(t, err, ErrPathExists)
}

func TestService_ReadFile(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	testFile := filepath.Join(tempDir, "read_test.txt")
	content := []byte("test content for reading")
	require.NoError(t, os.WriteFile(testFile, content, 0644))

	data, err := service.ReadFile(ctx, testFile)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestService_ReadFile_NotExist(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	_, err := service.ReadFile(ctx, filepath.Join(tempDir, "nonexistent.txt"))
	assert.ErrorIs(t, err, ErrPathNotExist)
}

func TestService_ReadFile_IsDir(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	_, err := service.ReadFile(ctx, tempDir)
	assert.ErrorIs(t, err, ErrNotFile)
}

func TestService_Rename(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	oldPath := filepath.Join(tempDir, "old_name.txt")
	newPath := filepath.Join(tempDir, "new_name.txt")
	require.NoError(t, os.WriteFile(oldPath, []byte("content"), 0644))

	err := service.Rename(ctx, oldPath, newPath)
	require.NoError(t, err)

	assert.NoFileExists(t, oldPath)
	assert.FileExists(t, newPath)
}

func TestService_Delete(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	require.NoError(t, os.WriteFile(file1, []byte("1"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("2"), 0644))

	err := service.Delete(ctx, []string{file1, file2})
	require.NoError(t, err)

	assert.NoFileExists(t, file1)
	assert.NoFileExists(t, file2)
}

func TestService_Delete_Directory(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	subDir := filepath.Join(tempDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("content"), 0644))

	err := service.Delete(ctx, []string{subDir})
	require.NoError(t, err)

	assert.NoDirExists(t, subDir)
}

func TestService_Search(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	// 创建测试结构
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "test_file.txt"), []byte("1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "other.txt"), []byte("2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "subdir", "test_nested.txt"), []byte("3"), 0644))

	ctx := context.Background()

	// 非递归搜索
	result, err := service.Search(ctx, &SearchRequest{
		Path:      tempDir,
		Keyword:   "test",
		Recursive: false,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total) // 只有 test_file.txt

	// 递归搜索
	result, err = service.Search(ctx, &SearchRequest{
		Path:      tempDir,
		Keyword:   "test",
		Recursive: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total) // test_file.txt 和 test_nested.txt
}

func TestService_GetStats(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	// 创建测试结构
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "dir1"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "dir2"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("12345"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "dir1", "file2.txt"), []byte("67890"), 0644))

	ctx := context.Background()
	stats, err := service.GetStats(ctx, tempDir)
	require.NoError(t, err)

	assert.Equal(t, int64(2), stats.TotalFiles)
	assert.Equal(t, int64(2), stats.TotalDirs)
	assert.Equal(t, int64(10), stats.TotalSize)
}

func TestService_CopyFile(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	srcFile := filepath.Join(tempDir, "source.txt")
	content := []byte("content to copy")
	require.NoError(t, os.WriteFile(srcFile, content, 0644))

	// 创建目标目录
	destDir := filepath.Join(tempDir, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0755))

	// 开始复制操作
	op := &FileOperation{
		Type:        "copy",
		Items:       []FileItem{{Path: srcFile}},
		Destination: destDir,
	}

	opID, err := service.StartOperation(ctx, op)
	require.NoError(t, err)
	require.NotEmpty(t, opID)

	// 等待操作完成
	for {
		status, err := service.GetOperationStatus(ctx, opID)
		require.NoError(t, err)
		if status.Finished {
			break
		}
	}

	// 验证结果
	destFile := filepath.Join(destDir, "source.txt")
	assert.FileExists(t, srcFile)  // 源文件仍存在
	assert.FileExists(t, destFile) // 目标文件已创建

	data, _ := os.ReadFile(destFile)
	assert.Equal(t, content, data)
}

func TestService_MoveFile(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	srcFile := filepath.Join(tempDir, "to_move.txt")
	content := []byte("content to move")
	require.NoError(t, os.WriteFile(srcFile, content, 0644))

	destDir := filepath.Join(tempDir, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0755))

	op := &FileOperation{
		Type:        "move",
		Items:       []FileItem{{Path: srcFile}},
		Destination: destDir,
	}

	opID, err := service.StartOperation(ctx, op)
	require.NoError(t, err)

	// 等待操作完成
	for {
		status, err := service.GetOperationStatus(ctx, opID)
		require.NoError(t, err)
		if status.Finished {
			break
		}
	}

	// 验证结果
	destFile := filepath.Join(destDir, "to_move.txt")
	assert.NoFileExists(t, srcFile) // 源文件已删除
	assert.FileExists(t, destFile)  // 目标文件已创建

	data, _ := os.ReadFile(destFile)
	assert.Equal(t, content, data)
}

func TestService_ValidatePath_Empty(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	err := service.validatePath("")
	assert.ErrorIs(t, err, ErrPathEmpty)
}

func TestService_ValidatePath_Traversal(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	err := service.validatePath("../../../etc/passwd")
	assert.ErrorIs(t, err, ErrPermissionDenied)
}

func TestService_GetInfo(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	testFile := filepath.Join(tempDir, "info_test.txt")
	content := []byte("test content")
	require.NoError(t, os.WriteFile(testFile, content, 0644))

	info, err := service.GetInfo(ctx, testFile)
	require.NoError(t, err)

	assert.Equal(t, "info_test.txt", info.Name)
	assert.Equal(t, int64(len(content)), info.Size)
	assert.False(t, info.IsDir)
	assert.Equal(t, "text/plain; charset=utf-8", info.MimeType)
}

func TestService_WriteFile(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	testFile := filepath.Join(tempDir, "write_test.txt")
	content := []byte("initial content")

	// 第一次写入
	err := service.WriteFile(ctx, testFile, content, 0644)
	require.NoError(t, err)

	// 覆盖写入
	newContent := []byte("new content")
	err = service.WriteFile(ctx, testFile, newContent, 0644)
	require.NoError(t, err)

	data, _ := os.ReadFile(testFile)
	assert.Equal(t, newContent, data)
}
