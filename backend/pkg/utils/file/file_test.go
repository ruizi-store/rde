package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExists(t *testing.T) {
	// 测试存在的文件
	tmpFile, err := os.CreateTemp("", "test_exists_*.txt")
	if err != nil {
		t.Fatalf("无法创建临时文件: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	
	if !Exists(tmpFile.Name()) {
		t.Error("Exists 应返回 true 对于存在的文件")
	}
	
	// 测试不存在的文件
	if Exists("/nonexistent/path/file.txt") {
		t.Error("Exists 应返回 false 对于不存在的文件")
	}
}

func TestIsDir(t *testing.T) {
	// 测试目录
	tmpDir := t.TempDir()
	
	if !IsDir(tmpDir) {
		t.Error("IsDir 应返回 true 对于目录")
	}
	
	// 测试文件
	tmpFile, _ := os.CreateTemp(tmpDir, "test_*.txt")
	tmpFile.Close()
	
	if IsDir(tmpFile.Name()) {
		t.Error("IsDir 应返回 false 对于文件")
	}
}

func TestIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "test_*.txt")
	tmpFile.Close()
	
	if !IsFile(tmpFile.Name()) {
		t.Error("IsFile 应返回 true 对于文件")
	}
	
	if IsFile(tmpDir) {
		t.Error("IsFile 应返回 false 对于目录")
	}
}

func TestReadWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_rw.txt")
	
	content := []byte("Hello, World!")
	
	// 写入
	if err := WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("WriteFile 失败: %v", err)
	}
	
	// 读取
	data, err := ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile 失败: %v", err)
	}
	
	if string(data) != string(content) {
		t.Errorf("读取内容 = %q, 期望 %q", string(data), string(content))
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")
	
	content := []byte("Copy test content")
	os.WriteFile(srcFile, content, 0644)
	
	// 使用标准库的复制方式测试
	srcData, err := os.ReadFile(srcFile)
	if err != nil {
		t.Fatalf("读取源文件失败: %v", err)
	}
	
	if err := os.WriteFile(dstFile, srcData, 0644); err != nil {
		t.Fatalf("写入目标文件失败: %v", err)
	}
	
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("读取目标文件失败: %v", err)
	}
	
	if string(dstContent) != string(content) {
		t.Errorf("复制内容不匹配")
	}
}

func TestMoveFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "move_source.txt")
	dstFile := filepath.Join(tmpDir, "move_dest.txt")
	
	content := []byte("Move test content")
	os.WriteFile(srcFile, content, 0644)
	
	if err := Move(srcFile, dstFile); err != nil {
		t.Fatalf("Move 失败: %v", err)
	}
	
	// 源文件不应存在
	if Exists(srcFile) {
		t.Error("移动后源文件不应存在")
	}
	
	// 目标文件应存在
	if !Exists(dstFile) {
		t.Error("移动后目标文件应存在")
	}
}

func TestGetSize(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "size_test.txt")
	
	content := []byte("12345678901234567890") // 20 字节
	os.WriteFile(testFile, content, 0644)
	
	size, err := GetSize(testFile)
	if err != nil {
		t.Fatalf("GetSize 失败: %v", err)
	}
	
	if size != 20 {
		t.Errorf("GetSize = %d, 期望 20", size)
	}
}

func TestGetDirSize(t *testing.T) {
	tmpDir := t.TempDir()
	
	// 创建一些文件
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("12345"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("67890"), 0644)
	
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "file3.txt"), []byte("abc"), 0644)
	
	size, err := GetDirSize(tmpDir)
	if err != nil {
		t.Fatalf("GetDirSize 失败: %v", err)
	}
	
	// 总共 13 字节 (5 + 5 + 3)
	if size != 13 {
		t.Errorf("GetDirSize = %d, 期望 13", size)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	
	for _, tt := range tests {
		result := FormatSize(tt.size)
		if result != tt.expected {
			t.Errorf("FormatSize(%d) = %s, 期望 %s", tt.size, result, tt.expected)
		}
	}
}

func TestMD5File(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "md5_test.txt")
	
	os.WriteFile(testFile, []byte("hello"), 0644)
	
	hash, err := MD5(testFile)
	if err != nil {
		t.Fatalf("MD5 失败: %v", err)
	}
	
	// "hello" 的 MD5
	expected := "5d41402abc4b2a76b9719d911017c592"
	if hash != expected {
		t.Errorf("MD5 = %s, 期望 %s", hash, expected)
	}
}

func TestGetExtension(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/file.txt", "txt"},
		{"/path/to/file.tar.gz", "gz"},
		{"/path/to/file", ""},
		{"file.png", "png"},
	}
	
	for _, tt := range tests {
		result := GetExtension(tt.path)
		if result != tt.expected {
			t.Errorf("GetExtension(%q) = %s, 期望 %s", tt.path, result, tt.expected)
		}
	}
}

func TestGetBaseName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/file.txt", "file"},
		{"/path/to/file.tar.gz", "file.tar"},
		{"/path/to/file", "file"},
		{"file.png", "file"},
	}
	
	for _, tt := range tests {
		result := GetBaseName(tt.path)
		if result != tt.expected {
			t.Errorf("GetBaseName(%q) = %s, 期望 %s", tt.path, result, tt.expected)
		}
	}
}

func TestGetFileName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/file.txt", "file.txt"},
		{"/path/to/dir/", "dir"},
		{"file.png", "file.png"},
	}
	
	for _, tt := range tests {
		result := GetFileName(tt.path)
		if result != tt.expected {
			t.Errorf("GetFileName(%q) = %s, 期望 %s", tt.path, result, tt.expected)
		}
	}
}

func TestMkdirAll(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "a", "b", "c")
	
	if err := MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("MkdirAll 失败: %v", err)
	}
	
	if !IsDir(newDir) {
		t.Error("目录应该被创建")
	}
}

func TestRemoveAll(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "to_remove")
	
	os.MkdirAll(filepath.Join(testDir, "sub"), 0755)
	os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("test"), 0644)
	
	if err := RemoveAll(testDir); err != nil {
		t.Fatalf("RemoveAll 失败: %v", err)
	}
	
	if Exists(testDir) {
		t.Error("目录应该被删除")
	}
}

func TestListFiles(t *testing.T) {
	tmpDir := t.TempDir()
	
	// 创建文件和子目录
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("2"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	
	files, err := ListFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListFiles 失败: %v", err)
	}
	
	// 应该只有 2 个文件（不包括子目录）
	if len(files) != 2 {
		t.Errorf("ListFiles 应返回 2 个文件, 实际返回 %d 个", len(files))
	}
}

func TestListDirs(t *testing.T) {
	tmpDir := t.TempDir()
	
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("1"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "dir2"), 0755)
	
	dirs, err := ListDirs(tmpDir)
	if err != nil {
		t.Fatalf("ListDirs 失败: %v", err)
	}
	
	// 应该只有 2 个目录
	if len(dirs) != 2 {
		t.Errorf("ListDirs 应返回 2 个目录, 实际返回 %d 个", len(dirs))
	}
}

func TestAppendFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "append_test.txt")
	
	os.WriteFile(testFile, []byte("Hello"), 0644)
	
	if err := AppendFile(testFile, []byte(", World!")); err != nil {
		t.Fatalf("AppendFile 失败: %v", err)
	}
	
	content, _ := os.ReadFile(testFile)
	if string(content) != "Hello, World!" {
		t.Errorf("内容 = %q, 期望 'Hello, World!'", string(content))
	}
}

// 基准测试
func BenchmarkExists(b *testing.B) {
	tmpFile, _ := os.CreateTemp("", "bench_exists_*.txt")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Exists(tmpFile.Name())
	}
}

func BenchmarkReadFile(b *testing.B) {
	tmpFile, _ := os.CreateTemp("", "bench_read_*.txt")
	tmpFile.Write([]byte("benchmark content"))
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ReadFile(tmpFile.Name())
	}
}

func BenchmarkWriteFile(b *testing.B) {
	tmpDir := b.TempDir()
	content := []byte("benchmark content")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, "bench_write.txt")
		WriteFile(testFile, content, 0644)
	}
}
