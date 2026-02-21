package retrogame

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	EmulatorJSVersion = "4.2.3"
	EmulatorJSTag     = "v" + EmulatorJSVersion
)

// git clone 源（优先使用 Gitee，国内 git 协议不触发验证码）
var gitSources = []string{
	"https://gitee.com/liduanjun/EmulatorJS.git",
	"https://github.com/EmulatorJS/EmulatorJS.git",
}

// HTTP tar.gz 下载源（git 不可用时的备选）
var httpSources = []string{
	"https://github.com/EmulatorJS/EmulatorJS/archive/refs/tags/" + EmulatorJSTag + ".tar.gz",
}

// Service 复古游戏服务
type Service struct {
	logger      *zap.Logger
	emulatorDir string
	mu          sync.Mutex
	installing  bool
}

// NewService 创建服务
func NewService(logger *zap.Logger, dataDir string) *Service {
	emulatorDir := filepath.Join(dataDir, "emulatorjs")
	return &Service{
		logger:      logger,
		emulatorDir: emulatorDir,
	}
}

// GetEmulatorDir 返回 EmulatorJS 安装目录
func (s *Service) GetEmulatorDir() string {
	return s.emulatorDir
}

// IsInstalled 检查 EmulatorJS 是否已安装（同时检查 version.json 和 emulator.min.js）
func (s *Service) IsInstalled() bool {
	versionFile := filepath.Join(s.emulatorDir, "version.json")
	minJS := filepath.Join(s.emulatorDir, "emulator.min.js")
	_, err1 := os.Stat(versionFile)
	_, err2 := os.Stat(minJS)
	return err1 == nil && err2 == nil
}

// GetStatus 获取安装状态
func (s *Service) GetStatus() *SetupStatus {
	return &SetupStatus{
		Installed:   s.IsInstalled(),
		Version:     EmulatorJSVersion,
		EmulatorDir: s.emulatorDir,
	}
}

// Setup 下载并安装 EmulatorJS，通过 channel 报告进度
func (s *Service) Setup(progressChan chan<- ProgressEvent) {
	defer close(progressChan)

	// 防止并发安装
	s.mu.Lock()
	if s.installing {
		s.mu.Unlock()
		progressChan <- ProgressEvent{
			Status:  "failed",
			Message: "安装正在进行中，请稍后",
		}
		return
	}
	s.installing = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.installing = false
		s.mu.Unlock()
	}()

	if s.IsInstalled() {
		progressChan <- ProgressEvent{
			Status:   "completed",
			Message:  "EmulatorJS 已安装",
			Progress: 100,
		}
		return
	}

	// 确保目标目录存在
	os.MkdirAll(s.emulatorDir, 0755)

	progressChan <- ProgressEvent{
		Status:   "downloading",
		Message:  "正在准备下载 EmulatorJS...",
		Progress: 0,
	}

	var lastErr error

	// 阶段一：尝试 git clone（Gitee 优先，国内 git 协议不触发验证码）
	if gitPath, err := exec.LookPath("git"); err == nil {
		for i, source := range gitSources {
			progressChan <- ProgressEvent{
				Status:   "downloading",
				Message:  fmt.Sprintf("正在从 Git 源 %d/%d 克隆...", i+1, len(gitSources)),
				Progress: 5,
			}

			s.logger.Info("Trying git clone", zap.String("repo", source))

			err := s.gitCloneAndInstall(gitPath, source, progressChan)
			if err == nil {
				progressChan <- ProgressEvent{
					Status:   "completed",
					Message:  "EmulatorJS 安装完成！",
					Progress: 100,
				}
				s.logger.Info("EmulatorJS installed successfully via git clone", zap.String("dir", s.emulatorDir))
				return
			}

			lastErr = err
			s.logger.Warn("Git clone failed, trying next",
				zap.String("source", source),
				zap.Error(err))
		}
	} else {
		s.logger.Info("git not found, skipping git clone sources")
	}

	// 阶段二：回退到 HTTP tar.gz 下载
	for i, source := range httpSources {
		progressChan <- ProgressEvent{
			Status:   "downloading",
			Message:  fmt.Sprintf("尝试 HTTP 下载源 %d/%d...", i+1, len(httpSources)),
			Progress: 2,
		}

		s.logger.Info("Trying HTTP download", zap.String("url", source))

		err := s.downloadAndExtract(source, progressChan)
		if err == nil {
			progressChan <- ProgressEvent{
				Status:   "completed",
				Message:  "EmulatorJS 安装完成！",
				Progress: 100,
			}
			s.logger.Info("EmulatorJS installed successfully via HTTP", zap.String("dir", s.emulatorDir))
			return
		}

		lastErr = err
		s.logger.Warn("HTTP download failed, trying next",
			zap.String("source", source),
			zap.Error(err))
	}

	progressChan <- ProgressEvent{
		Status:   "failed",
		Message:  fmt.Sprintf("所有下载源均失败: %v", lastErr),
		Progress: 0,
	}
}

// gitCloneAndInstall 通过 git clone --depth 1 安装 EmulatorJS
func (s *Service) gitCloneAndInstall(gitPath, repoURL string, progressChan chan<- ProgressEvent) error {
	tmpDir, err := os.MkdirTemp(filepath.Dir(s.emulatorDir), "emulatorjs-clone-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 先删除空临时目录，git clone 需要目标不存在
	os.Remove(tmpDir)

	cmd := exec.Command(gitPath, "clone", "--depth", "1", "--branch", EmulatorJSTag, repoURL, tmpDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone 失败: %w, output: %s", err, string(output))
	}

	progressChan <- ProgressEvent{
		Status:   "extracting",
		Message:  "克隆完成，正在安装...",
		Progress: 80,
	}

	// EmulatorJS 仓库中 dist 文件在 data/ 子目录
	sourceDir := filepath.Join(tmpDir, "data")
	if _, err := os.Stat(filepath.Join(sourceDir, "loader.js")); err != nil {
		return fmt.Errorf("无效的仓库结构：data/loader.js 不存在")
	}

	// 替换目标目录
	os.RemoveAll(s.emulatorDir)
	if err := os.Rename(sourceDir, s.emulatorDir); err != nil {
		// Rename 跨文件系统可能失败，回退到复制
		if copyErr := s.copyDir(sourceDir, s.emulatorDir); copyErr != nil {
			return fmt.Errorf("安装文件失败: %w", copyErr)
		}
	}

	// 生成 emulator.min.js / emulator.min.css（仓库只有源码，需要拼接）
	progressChan <- ProgressEvent{
		Status:   "extracting",
		Message:  "正在生成运行文件...",
		Progress: 90,
	}
	if err := s.buildMinifiedFiles(); err != nil {
		s.logger.Warn("Build minified files failed", zap.Error(err))
		return fmt.Errorf("生成运行文件失败: %w", err)
	}

	if !s.IsInstalled() {
		return fmt.Errorf("安装完成但验证失败：version.json 不存在")
	}

	progressChan <- ProgressEvent{
		Status:   "extracting",
		Message:  "验证安装...",
		Progress: 95,
	}

	return nil
}

// downloadAndExtract 从指定 URL 下载并解压
func (s *Service) downloadAndExtract(url string, progressChan chan<- ProgressEvent) error {
	client := &http.Client{
		Timeout: 10 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength

	// 在目标目录同级创建临时文件（确保同一文件系统，方便 rename）
	tmpFile, err := os.CreateTemp(filepath.Dir(s.emulatorDir), "emulatorjs-dl-*.tar.gz")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	defer tmpFile.Close()

	// 下载到临时文件，报告进度
	var downloaded int64
	buf := make([]byte, 64*1024)
	lastReport := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tmpFile.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("写入临时文件失败: %w", writeErr)
			}
			downloaded += int64(n)

			// 限制进度报告频率（200ms 一次）
			if time.Since(lastReport) > 200*time.Millisecond {
				lastReport = time.Now()
				pct := 5
				if totalSize > 0 {
					pct = int(float64(downloaded)/float64(totalSize)*75) + 5
				}
				progressChan <- ProgressEvent{
					Status:   "downloading",
					Message:  fmt.Sprintf("已下载 %s / %s", formatSize(downloaded), formatSize(totalSize)),
					Progress: pct,
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("下载中断: %w", readErr)
		}
	}

	// 解压
	progressChan <- ProgressEvent{
		Status:   "extracting",
		Message:  "正在解压...",
		Progress: 85,
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		return fmt.Errorf("seek 失败: %w", err)
	}

	if err := s.extractTarGz(tmpFile); err != nil {
		// 解压失败，清理残留
		os.RemoveAll(s.emulatorDir)
		os.MkdirAll(s.emulatorDir, 0755)
		return fmt.Errorf("解压失败: %w", err)
	}

	// 验证安装
	if !s.IsInstalled() {
		return fmt.Errorf("解压完成但 version.json 不存在，归档结构可能不正确")
	}

	progressChan <- ProgressEvent{
		Status:   "extracting",
		Message:  "验证安装...",
		Progress: 95,
	}

	return nil
}

// extractTarGz 解压 tar.gz 到 emulatorDir
// 自动检测并去除顶层单一目录前缀
func (s *Service) extractTarGz(r io.Reader) error {
	// 先解压到临时目录
	tmpDir, err := os.MkdirTemp(filepath.Dir(s.emulatorDir), "emulatorjs-extract-*")
	if err != nil {
		return fmt.Errorf("创建临时解压目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip 解压失败: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 tar 条目失败: %w", err)
		}

		name := filepath.Clean(header.Name)
		if name == "." {
			continue
		}

		target := filepath.Join(tmpDir, name)

		// 防止路径遍历攻击
		if !strings.HasPrefix(target, tmpDir+string(filepath.Separator)) && target != tmpDir {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("创建文件 %s 失败: %w", name, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("写入文件 %s 失败: %w", name, err)
			}
			f.Close()
		}
	}

	// 确定实际的 emulatorjs 根目录
	// 如果解压后只有一个顶层目录（如 EmulatorJS-v4.2.3/），则进入该目录
	sourceDir := tmpDir
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return fmt.Errorf("读取解压目录失败: %w", err)
	}
	if len(entries) == 1 && entries[0].IsDir() {
		sourceDir = filepath.Join(tmpDir, entries[0].Name())
	}

	// 完整 EmulatorJS 仓库归档中，dist 文件在 data/ 子目录下
	dataDir := filepath.Join(sourceDir, "data")
	if _, err := os.Stat(filepath.Join(dataDir, "loader.js")); err == nil {
		sourceDir = dataDir
	}

	// 验证解压内容（检查 loader.js，这是 EmulatorJS 的入口文件）
	if _, err := os.Stat(filepath.Join(sourceDir, "loader.js")); err != nil {
		// 尝试再下一层（兼容其他归档结构）
		subEntries, _ := os.ReadDir(sourceDir)
		if len(subEntries) == 1 && subEntries[0].IsDir() {
			sourceDir = filepath.Join(sourceDir, subEntries[0].Name())
		}
		if _, err := os.Stat(filepath.Join(sourceDir, "loader.js")); err != nil {
			return fmt.Errorf("无效的归档：找不到 loader.js")
		}
	}

	// 替换目标目录
	os.RemoveAll(s.emulatorDir)
	if err := os.Rename(sourceDir, s.emulatorDir); err != nil {
		// Rename 跨文件系统可能失败，退而求次复制
		if copyErr := s.copyDir(sourceDir, s.emulatorDir); copyErr != nil {
			return copyErr
		}
	}

	// 生成 emulator.min.js / emulator.min.css（仓库只有源码）
	if err := s.buildMinifiedFiles(); err != nil {
		s.logger.Warn("Build minified files failed", zap.Error(err))
		return fmt.Errorf("生成运行文件失败: %w", err)
	}

	return nil
}

// copyDir 递归复制目录
func (s *Service) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		os.MkdirAll(filepath.Dir(dstPath), 0755)
		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// formatSize 格式化文件大小
func formatSize(bytes int64) string {
	if bytes <= 0 {
		return "未知"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// buildMinifiedFiles 拼接 src/*.js 生成 emulator.min.js，复制 emulator.css 生成 emulator.min.css
// EmulatorJS 仓库源码没有预构建的 min 文件，原始构建需要 Node.js + terser
// 这里直接拼接源文件（效果等价，只是没有压缩）
func (s *Service) buildMinifiedFiles() error {
	srcDir := filepath.Join(s.emulatorDir, "src")

	// 检查 src/ 目录是否存在
	if _, err := os.Stat(srcDir); err != nil {
		// 如果已经有 emulator.min.js，跳过
		if _, err := os.Stat(filepath.Join(s.emulatorDir, "emulator.min.js")); err == nil {
			return nil
		}
		return fmt.Errorf("src/ 目录不存在，且没有预构建的 emulator.min.js")
	}

	// 读取 src/ 下所有 .js 文件
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("读取 src/ 目录失败: %w", err)
	}

	// loader.js 中定义的正确加载顺序（EmulatorJS 核心依赖顺序）
	loaderOrder := []string{
		"emulator.js",
		"nipplejs.js",
		"shaders.js",
		"storage.js",
		"gamepad.js",
		"GameManager.js",
		"socket.io.min.js",
		"compression.js",
	}

	// 按 loader.js 定义的顺序排列，未在列表中的文件追加到末尾
	orderMap := make(map[string]int, len(loaderOrder))
	for i, name := range loaderOrder {
		orderMap[name] = i
	}

	var jsFiles []string
	var extraFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
			if _, ok := orderMap[e.Name()]; ok {
				jsFiles = append(jsFiles, e.Name())
			} else {
				extraFiles = append(extraFiles, e.Name())
			}
		}
	}

	// 按 loader.js 顺序排序
	sort.Slice(jsFiles, func(i, j int) bool {
		return orderMap[jsFiles[i]] < orderMap[jsFiles[j]]
	})
	sort.Strings(extraFiles)
	jsFiles = append(jsFiles, extraFiles...)

	s.logger.Info("Building emulator.min.js", zap.Strings("sources", jsFiles))

	// 拼接所有 JS 文件到 emulator.min.js
	outFile, err := os.Create(filepath.Join(s.emulatorDir, "emulator.min.js"))
	if err != nil {
		return fmt.Errorf("创建 emulator.min.js 失败: %w", err)
	}
	defer outFile.Close()

	for _, name := range jsFiles {
		data, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			return fmt.Errorf("读取 %s 失败: %w", name, err)
		}
		if _, err := outFile.Write(data); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", name, err)
		}
		// 确保文件之间有换行分隔
		outFile.WriteString("\n")
	}

	// 复制 emulator.css -> emulator.min.css
	cssFile := filepath.Join(s.emulatorDir, "emulator.css")
	if _, err := os.Stat(cssFile); err == nil {
		data, err := os.ReadFile(cssFile)
		if err != nil {
			return fmt.Errorf("读取 emulator.css 失败: %w", err)
		}
		if err := os.WriteFile(filepath.Join(s.emulatorDir, "emulator.min.css"), data, 0644); err != nil {
			return fmt.Errorf("写入 emulator.min.css 失败: %w", err)
		}
	}

	s.logger.Info("Built minified files",
		zap.Int("js_files", len(jsFiles)),
		zap.String("output", filepath.Join(s.emulatorDir, "emulator.min.js")))

	return nil
}

// ==================== ROM 扫描 ====================

// romExtToPlatform 将 ROM 文件扩展名映射到平台 ID
// 注意：.zip 不在此映射中，zip 文件需要检查内部内容来判断平台
var romExtToPlatform = map[string]string{
	".nes": "nes",
	".smc": "snes",
	".sfc": "snes",
	".gb":  "gb",
	".gbc": "gbc",
	".gba": "gba",
	".n64": "n64",
	".z64": "n64",
	".v64": "n64",
	".nds": "nds",
	".pbp": "psx",
	".cue": "psx",
	".cso": "psp",
	".md":  "genesis",
	".gen": "genesis",
}

// allRomExts 所有有效的 ROM 扩展名（用于检测 zip 内部文件）
var allRomExts = func() map[string]bool {
	m := make(map[string]bool, len(romExtToPlatform))
	for ext := range romExtToPlatform {
		m[ext] = true
	}
	// 同时包含多平台共享的扩展名
	m[".bin"] = true
	m[".iso"] = true
	return m
}()

// ScanRoms 扫描目录中的 ROM 文件，对 .zip 文件会检查内部内容以正确识别平台
func (s *Service) ScanRoms(directory string) ([]RomFileInfo, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var roms []RomFileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		fullPath := filepath.Join(directory, name)

		info, err := entry.Info()
		if err != nil {
			continue
		}

		var platform string

		if ext == ".zip" || ext == ".7z" {
			// 压缩包：检查内部文件扩展名来判断真实平台
			platform = s.detectZipPlatform(fullPath)
		} else if p, ok := romExtToPlatform[ext]; ok {
			platform = p
		}

		if platform != "" {
			roms = append(roms, RomFileInfo{
				Name:     strings.TrimSuffix(name, ext),
				Path:     fullPath,
				Size:     info.Size(),
				Platform: platform,
			})
		}
	}

	return roms, nil
}

// detectZipPlatform 通过检查 ZIP 文件内部的 ROM 文件扩展名来判断平台
// 例如：一个包含 .gba 文件的 zip 会被识别为 GBA 平台而非街机
func (s *Service) detectZipPlatform(zipPath string) string {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		s.logger.Debug("无法打开 zip 文件进行平台检测，回退为 arcade",
			zap.String("path", zipPath), zap.Error(err))
		return "arcade"
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		innerExt := strings.ToLower(filepath.Ext(f.Name))
		if platform, ok := romExtToPlatform[innerExt]; ok {
			return platform
		}
	}

	// 没有识别出具体平台的 ROM 文件，默认按街机处理
	// （街机 ROM 通常是多个 .bin 文件打包在 zip 中，以 ROM set 名称命名）
	return "arcade"
}
