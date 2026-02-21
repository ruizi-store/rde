// Package files 提供文件管理服务
package files

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/ruizi-store/rde/backend/pkg/runas"
	"go.uber.org/zap"
)

var (
	ErrPathEmpty        = errors.New("path is empty")
	ErrPathNotExist     = errors.New("path does not exist")
	ErrPathExists       = errors.New("path already exists")
	ErrNotFile          = errors.New("path is not a file")
	ErrNotDir           = errors.New("path is not a directory")
	ErrPermissionDenied = errors.New("permission denied")
	ErrMountedPath      = errors.New("cannot operate on mounted path")
	ErrSameSourceDest   = errors.New("source and destination are the same")
	ErrInvalidOperation = errors.New("invalid operation type")
)

// Service 文件管理服务
type Service struct {
	logger       *zap.Logger
	rootPaths    []string // 允许访问的根路径列表
	operations   sync.Map // 存储进行中的操作
	opQueue      []string // 操作队列
	opMutex      sync.Mutex
	mountChecker MountChecker // 检查挂载点的接口
}

// MountChecker 挂载点检查器接口
type MountChecker interface {
	IsMounted(path string) bool
}

// defaultMountChecker 默认挂载检查器
type defaultMountChecker struct{}

func (d *defaultMountChecker) IsMounted(path string) bool {
	// 简化实现，实际应调用系统API
	return false
}

// NewService 创建文件服务
func NewService(logger *zap.Logger, rootPaths []string) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		logger:       logger,
		rootPaths:    rootPaths,
		mountChecker: &defaultMountChecker{},
	}
}

// SetMountChecker 设置挂载检查器
func (s *Service) SetMountChecker(mc MountChecker) {
	s.mountChecker = mc
}

// validatePath 验证路径安全性
func (s *Service) validatePath(path string) error {
	if path == "" {
		return ErrPathEmpty
	}
	// 清理路径，防止路径遍历攻击
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return ErrPermissionDenied
	}
	return nil
}

// ensureUserHomeDir 确保用户主目录存在
// 如果主目录不存在，以 root 身份创建并 chown 给用户
func (s *Service) ensureUserHomeDir(username string) error {
	homeDir := filepath.Join("/home", username)

	// 检查主目录是否存在
	if _, err := os.Stat(homeDir); err == nil {
		return nil // 已存在
	}

	// 以 root 身份创建主目录
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		return fmt.Errorf("create home dir failed: %w", err)
	}

	// 修改所有权为用户
	if err := runas.ChownToUser(homeDir, username); err != nil {
		return fmt.Errorf("chown home dir failed: %w", err)
	}

	return nil
}

// getFileOwnerGroup 获取文件的所有者和组名称
func getFileOwnerGroup(info os.FileInfo) (owner, group string) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", ""
	}

	uid := strconv.Itoa(int(stat.Uid))
	gid := strconv.Itoa(int(stat.Gid))

	// 尝试获取用户名
	if u, err := user.LookupId(uid); err == nil {
		owner = u.Username
	} else {
		owner = uid
	}

	// 尝试获取组名
	if g, err := user.LookupGroupId(gid); err == nil {
		group = g.Name
	} else {
		group = gid
	}

	return owner, group
}
// List 列出目录内容
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	if err := s.validatePath(req.Path); err != nil {
		return nil, err
	}

	// 设置默认分页
	if req.Index <= 0 {
		req.Index = 1
	}
	if req.Size <= 0 {
		req.Size = 50
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPathNotExist
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, ErrNotDir
	}

	entries, err := os.ReadDir(req.Path)
	if err != nil {
		return nil, err
	}

	// 构建文件信息列表
	var files []FileInfo
	for _, entry := range entries {
		// 跳过临时目录
		if entry.Name() == ".temp" && entry.IsDir() {
			continue
		}

		// 过滤隐藏文件（以 . 开头）
		if !req.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(req.Path, entry.Name())
		isSymlink := entry.Type()&os.ModeSymlink != 0
		isDir := entry.IsDir()
		size := info.Size()
		var linkTarget string

		// 符号链接：跟随链接获取目标类型
		if isSymlink {
			if target, err := os.Readlink(fullPath); err == nil {
				linkTarget = target
			}
			if targetInfo, err := os.Stat(fullPath); err == nil {
				isDir = targetInfo.IsDir()
				size = targetInfo.Size()
			}
		}

		// 获取所有者和组
		owner, group := getFileOwnerGroup(info)

		fileInfo := FileInfo{
			Name:       entry.Name(),
			Path:       fullPath,
			Size:       size,
			IsDir:      isDir,
			IsSymlink:  isSymlink,
			LinkTarget: linkTarget,
			ModTime:    info.ModTime(),
			Mode:       info.Mode().String(),
			Owner:      owner,
			Group:      group,
		}

		// 获取 MIME 类型
		if !entry.IsDir() {
			ext := filepath.Ext(entry.Name())
			if mimeType := mime.TypeByExtension(ext); mimeType != "" {
				fileInfo.MimeType = mimeType
			}
		}

		files = append(files, fileInfo)
	}

	// 排序：目录在前，然后按名称排序
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	total := len(files)

	// 分页
	start := (req.Index - 1) * req.Size
	end := start + req.Size
	if start >= total {
		files = []FileInfo{}
	} else {
		if end > total {
			end = total
		}
		files = files[start:end]
	}

	return &ListResponse{
		Content: files,
		Total:   int64(total),
		Index:   req.Index,
		Size:    req.Size,
	}, nil
}

// GetInfo 获取文件/目录信息
func (s *Service) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	if err := s.validatePath(path); err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPathNotExist
		}
		return nil, err
	}

	fileInfo := &FileInfo{
		Name:    info.Name(),
		Path:    path,
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
		Mode:    info.Mode().String(),
	}

	// 检查是否为符号链接
	if linfo, err := os.Lstat(path); err == nil {
		if linfo.Mode()&os.ModeSymlink != 0 {
			fileInfo.IsSymlink = true
			if target, err := os.Readlink(path); err == nil {
				fileInfo.LinkTarget = target
			}
		}
	}

	if !info.IsDir() {
		ext := filepath.Ext(info.Name())
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			fileInfo.MimeType = mimeType
		}
	}

	return fileInfo, nil
}

// ReadFile 读取文件内容
func (s *Service) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := s.validatePath(path); err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPathNotExist
		}
		return nil, err
	}

	if info.IsDir() {
		return nil, ErrNotFile
	}

	return os.ReadFile(path)
}

// WriteFile 写入文件内容
func (s *Service) WriteFile(ctx context.Context, path string, content []byte, perm os.FileMode, username string) error {
	if err := s.validatePath(path); err != nil {
		return err
	}

	if perm == 0 {
		perm = 0644
	}

	// 确保用户主目录存在
	if err := s.ensureUserHomeDir(username); err != nil {
		return fmt.Errorf("ensure home dir failed: %w", err)
	}

	// 以用户身份执行
	exec, err := runas.NewExecutor(username)
	if err != nil {
		return fmt.Errorf("create executor failed: %w", err)
	}

	if err := exec.WriteFile(path, content); err != nil {
		return err
	}

	// 设置权限
	return exec.Chmod(path, perm)
}

// CreateDir 创建目录
func (s *Service) CreateDir(ctx context.Context, path string, username string) error {
	if err := s.validatePath(path); err != nil {
		return err
	}

	// 确保用户主目录存在
	if err := s.ensureUserHomeDir(username); err != nil {
		return fmt.Errorf("ensure home dir failed: %w", err)
	}

	// 以用户身份执行
	exec, err := runas.NewExecutor(username)
	if err != nil {
		return fmt.Errorf("create executor failed: %w", err)
	}

	return exec.Mkdir(path)
}

// CreateFile 创建文件
func (s *Service) CreateFile(ctx context.Context, path string, content []byte, username string) error {
	if err := s.validatePath(path); err != nil {
		return err
	}

	// 检查文件是否已存在
	if _, err := os.Stat(path); err == nil {
		return ErrPathExists
	}

	// 确保用户主目录存在
	if err := s.ensureUserHomeDir(username); err != nil {
		return fmt.Errorf("ensure home dir failed: %w", err)
	}

	// 以用户身份执行
	exec, err := runas.NewExecutor(username)
	if err != nil {
		return fmt.Errorf("create executor failed: %w", err)
	}

	if len(content) > 0 {
		return exec.WriteFile(path, content)
	}
	return exec.Touch(path)
}

// Rename 重命名文件/目录
func (s *Service) Rename(ctx context.Context, oldPath, newPath string, username string) error {
	if err := s.validatePath(oldPath); err != nil {
		return err
	}
	if err := s.validatePath(newPath); err != nil {
		return err
	}

	// 检查原路径是否存在
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return ErrPathNotExist
	}

	// 检查是否为挂载点
	if s.mountChecker.IsMounted(oldPath) {
		return ErrMountedPath
	}

	// 以用户身份执行
	exec, err := runas.NewExecutor(username)
	if err != nil {
		return fmt.Errorf("create executor failed: %w", err)
	}

	return exec.Move(oldPath, newPath)
}

// Delete 删除文件/目录
func (s *Service) Delete(ctx context.Context, paths []string, username string) error {
	// 以用户身份执行
	exec, err := runas.NewExecutor(username)
	if err != nil {
		return fmt.Errorf("create executor failed: %w", err)
	}

	for _, path := range paths {
		if err := s.validatePath(path); err != nil {
			return err
		}

		// 检查是否为挂载点
		if s.mountChecker.IsMounted(path) {
			return ErrMountedPath
		}

		if err := exec.Remove(path, true); err != nil {
			return err
		}
	}
	return nil
}

// StartOperation 开始文件操作（复制/移动）
func (s *Service) StartOperation(ctx context.Context, op *FileOperation) (string, error) {
	if op.Type != "copy" && op.Type != "move" {
		return "", ErrInvalidOperation
	}

	if err := s.validatePath(op.Destination); err != nil {
		return "", err
	}

	// 验证所有源路径
	for i := range op.Items {
		if err := s.validatePath(op.Items[i].Path); err != nil {
			return "", err
		}

		// 检查源路径是否存在
		info, err := os.Stat(op.Items[i].Path)
		if os.IsNotExist(err) {
			return "", ErrPathNotExist
		}

		// 计算大小
		if info.IsDir() {
			size, _ := s.getDirSize(op.Items[i].Path)
			op.Items[i].Size = size
		} else {
			op.Items[i].Size = info.Size()
		}
		op.TotalSize += op.Items[i].Size

		// 移动时检查挂载点
		if op.Type == "move" && s.mountChecker.IsMounted(op.Items[i].Path) {
			return "", ErrMountedPath
		}
	}

	// 检查源和目标是否相同
	if len(op.Items) > 0 {
		srcDir := filepath.Dir(op.Items[0].Path)
		if srcDir == op.Destination {
			return "", ErrSameSourceDest
		}
	}

	// 生成操作ID
	opID := uuid.NewString()
	s.operations.Store(opID, op)

	// 添加到队列
	s.opMutex.Lock()
	s.opQueue = append(s.opQueue, opID)
	if len(s.opQueue) == 1 {
		go s.processOperations()
	}
	s.opMutex.Unlock()

	return opID, nil
}

// GetOperationStatus 获取操作状态
func (s *Service) GetOperationStatus(ctx context.Context, opID string) (*OperationStatus, error) {
	val, ok := s.operations.Load(opID)
	if !ok {
		return nil, errors.New("operation not found")
	}

	op := val.(*FileOperation)
	progress := 0
	if op.TotalSize > 0 {
		progress = int(op.ProcessedSize * 100 / op.TotalSize)
	}

	return &OperationStatus{
		ID:            opID,
		Type:          op.Type,
		TotalSize:     op.TotalSize,
		ProcessedSize: op.ProcessedSize,
		Progress:      progress,
		Finished:      op.Finished,
	}, nil
}

// CancelOperation 取消操作
func (s *Service) CancelOperation(ctx context.Context, opID string) error {
	s.operations.Delete(opID)

	s.opMutex.Lock()
	defer s.opMutex.Unlock()

	for i, id := range s.opQueue {
		if id == opID {
			s.opQueue = append(s.opQueue[:i], s.opQueue[i+1:]...)
			break
		}
	}
	return nil
}

// processOperations 处理操作队列
func (s *Service) processOperations() {
	for {
		s.opMutex.Lock()
		if len(s.opQueue) == 0 {
			s.opMutex.Unlock()
			return
		}
		opID := s.opQueue[0]
		s.opMutex.Unlock()

		val, ok := s.operations.Load(opID)
		if !ok {
			s.removeFromQueue(opID)
			continue
		}

		op := val.(*FileOperation)
		s.executeOperation(op)
		op.Finished = true
		s.operations.Store(opID, op)

		s.removeFromQueue(opID)

		// 延迟清理已完成的操作
		go func(id string) {
			time.Sleep(30 * time.Second)
			s.operations.Delete(id)
		}(opID)
	}
}

func (s *Service) removeFromQueue(opID string) {
	s.opMutex.Lock()
	defer s.opMutex.Unlock()

	for i, id := range s.opQueue {
		if id == opID {
			s.opQueue = append(s.opQueue[:i], s.opQueue[i+1:]...)
			break
		}
	}
}

// executeOperation 执行单个操作
func (s *Service) executeOperation(op *FileOperation) {
	// 创建用户执行器
	exec, err := runas.NewExecutor(op.Username)
	if err != nil {
		s.logger.Error("create executor failed",
			zap.String("username", op.Username),
			zap.Error(err))
		return
	}

	for i := range op.Items {
		item := &op.Items[i]
		destPath := filepath.Join(op.Destination, filepath.Base(item.Path))

		// 处理冲突
		if _, err := os.Stat(destPath); err == nil {
			if op.ConflictStyle == "skip" {
				item.Finished = true
				continue
			}
			// overwrite: 先删除目标
			exec.Remove(destPath, true)
		}

		var opErr error
		info, _ := os.Stat(item.Path)
		isDir := info != nil && info.IsDir()

		if op.Type == "copy" {
			opErr = exec.Copy(item.Path, destPath, isDir)
		} else {
			// 移动操作
			opErr = exec.Move(item.Path, destPath)
		}

		if opErr != nil {
			s.logger.Error("operation failed",
				zap.String("type", op.Type),
				zap.String("src", item.Path),
				zap.String("dest", destPath),
				zap.Error(opErr))
		}

		item.Finished = true
		item.ProcessedSize = item.Size
		op.ProcessedSize += item.Size
	}
}

// copyPath 复制文件或目录
func (s *Service) copyPath(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return s.copyDir(src, dest)
	}
	return s.copyFile(src, dest, info.Mode())
}

// copyFile 复制单个文件
func (s *Service) copyFile(src, dest string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

// copyDir 递归复制目录
func (s *Service) copyDir(src, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dest, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			if err := s.copyDir(srcPath, destPath); err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := s.copyFile(srcPath, destPath, info.Mode()); err != nil {
				return err
			}
		}
	}

	return nil
}

// getDirSize 获取目录大小
func (s *Service) getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// Search 搜索文件
func (s *Service) Search(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	if err := s.validatePath(req.Path); err != nil {
		return nil, err
	}

	if req.MaxResults <= 0 {
		req.MaxResults = 100
	}
	if req.FileType == "" {
		req.FileType = "all"
	}

	keyword := strings.ToLower(req.Keyword)
	var results []FileInfo

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}

		// 检查结果数量
		if len(results) >= req.MaxResults {
			return filepath.SkipAll
		}

		// 跳过根目录
		if path == req.Path {
			return nil
		}

		// 非递归时跳过子目录内容
		if !req.Recursive && filepath.Dir(path) != req.Path {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 过滤文件类型
		if req.FileType == "file" && info.IsDir() {
			return nil
		}
		if req.FileType == "dir" && !info.IsDir() {
			return nil
		}

		// 匹配关键字
		if strings.Contains(strings.ToLower(info.Name()), keyword) {
			results = append(results, FileInfo{
				Name:    info.Name(),
				Path:    path,
				Size:    info.Size(),
				IsDir:   info.IsDir(),
				ModTime: info.ModTime(),
				Mode:    info.Mode().String(),
			})
		}

		return nil
	}

	if err := filepath.Walk(req.Path, walkFn); err != nil && err != filepath.SkipAll {
		return nil, err
	}

	return &SearchResult{
		Files: results,
		Total: len(results),
	}, nil
}

// GetStats 获取目录统计
func (s *Service) GetStats(ctx context.Context, path string) (*FileStats, error) {
	if err := s.validatePath(path); err != nil {
		return nil, err
	}

	stats := &FileStats{}
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			stats.TotalDirs++
		} else {
			stats.TotalFiles++
			stats.TotalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 不计算根目录本身
	if stats.TotalDirs > 0 {
		stats.TotalDirs--
	}

	return stats, nil
}

// GetHash 计算文件哈希
func (s *Service) GetHash(ctx context.Context, path string, algorithm string) (string, error) {
	if err := s.validatePath(path); err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrPathNotExist
		}
		return "", err
	}

	if info.IsDir() {
		return "", ErrNotFile
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New() // 默认使用 MD5
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GetUploadTempDir 获取上传临时目录
func (s *Service) GetUploadTempDir(destDir, filename string, totalChunks int) string {
	hash := md5.Sum([]byte(filename))
	hashStr := hex.EncodeToString(hash[:])
	return filepath.Join(destDir, ".temp", fmt.Sprintf("%s%d", hashStr, totalChunks))
}

// GetUploadTempDirByID 根据上传ID获取临时目录
func (s *Service) GetUploadTempDirByID(uploadId string) string {
	return filepath.Join(os.TempDir(), "rde-upload", uploadId)
}

// GenerateUploadID 生成上传ID
func (s *Service) GenerateUploadID(path, filename string) string {
	data := fmt.Sprintf("%s:%s:%d", path, filename, time.Now().UnixNano())
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// UploadMeta 上传元信息
type UploadMeta struct {
	Path        string
	Filename    string
	Size        int64
	TotalChunks int
	CreatedAt   time.Time
}

// 上传元信息缓存
var uploadMetaCache = make(map[string]*UploadMeta)
var uploadMetaMutex sync.RWMutex

// SaveUploadMeta 保存上传元信息
func (s *Service) SaveUploadMeta(uploadId, path, filename string, size int64, totalChunks int) {
	uploadMetaMutex.Lock()
	defer uploadMetaMutex.Unlock()
	uploadMetaCache[uploadId] = &UploadMeta{
		Path:        path,
		Filename:    filename,
		Size:        size,
		TotalChunks: totalChunks,
		CreatedAt:   time.Now(),
	}
}

// GetUploadMeta 获取上传元信息
func (s *Service) GetUploadMeta(uploadId string) *UploadMeta {
	uploadMetaMutex.RLock()
	defer uploadMetaMutex.RUnlock()
	return uploadMetaCache[uploadId]
}

// DeleteUploadMeta 删除上传元信息
func (s *Service) DeleteUploadMeta(uploadId string) {
	uploadMetaMutex.Lock()
	defer uploadMetaMutex.Unlock()
	delete(uploadMetaCache, uploadId)
}

// MergeChunks 合并分片文件
func (s *Service) MergeChunks(ctx context.Context, tempDir, destPath string, totalChunks int) error {
	// 确保目标目录存在
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// 分片索引从 0 开始（新 API）或从 1 开始（旧 API）
	// 尝试从 0 开始
	startIndex := 0
	firstChunk := filepath.Join(tempDir, "0")
	if _, err := os.Stat(firstChunk); os.IsNotExist(err) {
		startIndex = 1
	}

	for i := startIndex; i < startIndex+totalChunks; i++ {
		chunkPath := filepath.Join(tempDir, fmt.Sprintf("%d", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return fmt.Errorf("failed to open chunk %d: %w", i, err)
		}

		_, err = io.Copy(destFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy chunk %d: %w", i, err)
		}
	}

	// 清理临时目录
	go func() {
		time.Sleep(5 * time.Second)
		os.RemoveAll(tempDir)
	}()

	return nil
}
