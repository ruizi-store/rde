// Package files 提供文件管理 HTTP 处理器
package files

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/common"
	"github.com/ruizi-store/rde/backend/core/auth"
	"github.com/ruizi-store/rde/backend/pkg/runas"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Handler HTTP 处理器
type Handler struct {
	service      *Service
	thumbnails   *ThumbnailService
	db           *gorm.DB
	elevationMgr *ElevationManager
}

// NewHandler 创建处理器
func NewHandler(service *Service, thumbnails *ThumbnailService, db *gorm.DB) *Handler {
	return &Handler{
		service:      service,
		thumbnails:   thumbnails,
		db:           db,
		elevationMgr: NewElevationManager(),
	}
}

// response 通用响应结构
type response struct {
	Success int         `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, response{
		Success: 200,
		Message: "success",
		Data:    data,
	})
}

func fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, response{
		Success: code,
		Message: msg,
	})
}

func serverError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, response{
		Success: 500,
		Message: msg,
	})
}

// getPathContext 从 gin 上下文获取虚拟路径解析上下文
func getPathContext(c *gin.Context) *common.VirtualPathContext {
	return &common.VirtualPathContext{
		Username: auth.GetUsername(c),
		IsAdmin:  auth.IsAdmin(c),
	}
}

// resolvePathWithContext 解析虚拟路径为实际路径（带用户上下文）
func resolvePathWithContext(c *gin.Context, virtualPath string) string {
	return common.ResolveVirtualPathWithContext(virtualPath, getPathContext(c))
}

// toVirtualPathWithContext 将实际路径转换为虚拟路径（带用户上下文）
func toVirtualPathWithContext(c *gin.Context, realPath string) string {
	return common.ToVirtualPathWithContext(realPath, getPathContext(c))
}

// resolvePath 解析虚拟路径为实际路径（无上下文，兼容旧代码）
func resolvePath(virtualPath string) string {
	return common.ResolveVirtualPath(virtualPath)
}

// toVirtualPath 将实际路径转换为虚拟路径（无上下文，兼容旧代码）
func toVirtualPath(realPath string) string {
	return common.ToVirtualPath(realPath)
}

// permissionDeniedResponse 权限不足响应（包含是否可以提权的信息）
type permissionDeniedResponse struct {
	Success     int    `json:"success"`
	Message     string `json:"message"`
	NeedElevate bool   `json:"need_elevate"` // 提示前端可以通过管理员提权访问
}

// failPermission 返回权限不足错误，对 admin 角色附带 need_elevate 标记
func (h *Handler) failPermission(c *gin.Context, path string) {
	isAdmin := auth.IsAdmin(c)
	userID := c.GetString("user_id")
	elevated := h.elevationMgr.IsElevated(userID)

	c.JSON(http.StatusOK, permissionDeniedResponse{
		Success:     403,
		Message:     "权限不足，无法访问此路径",
		NeedElevate: isAdmin && !elevated,
	})
}

// checkReadPermission 检查用户对路径的读权限
// 返回 true 表示有权限，false 表示无权限（已向客户端写入返回）
func (h *Handler) checkReadPermission(c *gin.Context, realPath string) bool {
	username := auth.GetUsername(c)
	userID := c.GetString("user_id")

	// 如果用户处于提权状态，跳过权限检查（后端本身以 root 运行）
	if h.elevationMgr.IsElevated(userID) {
		return true
	}

	// 检查目标路径
	info, err := os.Stat(realPath)
	if err != nil {
		return true // 路径不存在时让后续逻辑处理
	}

	if info.IsDir() {
		// 目录需要 r+x
		if !common.CheckUserDirAccess(username, realPath) {
			h.failPermission(c, realPath)
			return false
		}
	} else {
		// 文件需要 r
		if !common.CheckUserReadAccess(username, realPath) {
			h.failPermission(c, realPath)
			return false
		}
	}

	return true
}

// checkPathAccess 统一路径访问检查：IsPathAllowed + 敏感路径提权
// 返回 true 表示可以继续，false 表示已向客户端返回错误
func (h *Handler) checkPathAccess(c *gin.Context, virtualPath string, pathCtx *common.VirtualPathContext) bool {
	if !common.IsPathAllowed(virtualPath, pathCtx) {
		fail(c, 403, "permission denied")
		return false
	}

	// 敏感路径需要管理员提权
	if common.IsSensitivePath(virtualPath) {
		userID := c.GetString("user_id")
		if !h.elevationMgr.IsElevated(userID) {
			h.failPermission(c, virtualPath)
			return false
		}
	}

	return true
}

// checkWritePermission 检查用户对路径的写权限
func (h *Handler) checkWritePermission(c *gin.Context, realPath string) bool {
	username := auth.GetUsername(c)
	userID := c.GetString("user_id")

	if h.elevationMgr.IsElevated(userID) {
		return true
	}

	// 如果路径不存在，检查父目录的写权限
	targetPath := realPath
	if _, err := os.Stat(realPath); os.IsNotExist(err) {
		targetPath = filepath.Dir(realPath)
	}

	if !common.CheckUserWriteAccess(username, targetPath) {
		h.failPermission(c, realPath)
		return false
	}

	return true
}

// List 列出目录内容
// @Summary 列出目录内容
// @Tags files
// @Accept json
// @Produce json
// @Param path query string true "目录路径"
// @Param index query int false "页码" default(1)
// @Param size query int false "每页数量" default(50)
// @Success 200 {object} ListResponse
// @Router /api/v1/files/list [get]
func (h *Handler) List(c *gin.Context) {
	virtualPath := c.Query("path")
	if virtualPath == "" {
		fail(c, 400, "path is required")
		return
	}

	pathCtx := getPathContext(c)

	// 权限检查
	if !h.checkPathAccess(c, virtualPath, pathCtx) {
		return
	}

	// 解析虚拟路径为实际路径
	realPath := common.ResolveVirtualPathWithContext(virtualPath, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkReadPermission(c, realPath) {
		return
	}

	// 解析符号链接，获取真实路径（避免符号链接目录无限递归）
	resolvedPath := realPath
	if resolved, err := filepath.EvalSymlinks(realPath); err == nil {
		resolvedPath = resolved
	}

	req := &ListRequest{
		Path:       resolvedPath,
		Index:      parseInt(c.Query("index"), 1),
		Size:       parseInt(c.Query("size"), 50),
		ShowHidden: c.Query("show_hidden") == "true",
	}

	resp, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		handleError(c, err)
		return
	}

	// 如果路径被解析过（含符号链接），告诉前端真实路径
	if resolvedPath != realPath {
		resp.ResolvedPath = resolvedPath
	}

	ok(c, resp)
}

// GetInfo 获取文件/目录信息
// @Summary 获取文件/目录信息
// @Tags files
// @Accept json
// @Produce json
// @Param path query string true "路径"
// @Success 200 {object} FileInfo
// @Router /api/v1/files/info [get]
func (h *Handler) GetInfo(c *gin.Context) {
	virtualPath := c.Query("path")
	if virtualPath == "" {
		fail(c, 400, "path is required")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, virtualPath, pathCtx) {
		return
	}

	realPath := common.ResolveVirtualPathWithContext(virtualPath, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkReadPermission(c, realPath) {
		return
	}

	info, err := h.service.GetInfo(c.Request.Context(), realPath)
	if err != nil {
		handleError(c, err)
		return
	}

	// 转换路径回虚拟路径
	if info != nil {
		info.Path = common.ToVirtualPathWithContext(info.Path, pathCtx)
	}

	ok(c, info)
}

// Read 读取文件内容
// @Summary 读取文件内容
// @Tags files
// @Accept json
// @Produce json
// @Param path query string true "文件路径"
// @Success 200 {string} string "文件内容"
// @Router /api/v1/files/read [get]
func (h *Handler) Read(c *gin.Context) {
	virtualPath := c.Query("path")
	if virtualPath == "" {
		fail(c, 400, "path is required")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, virtualPath, pathCtx) {
		return
	}

	realPath := common.ResolveVirtualPathWithContext(virtualPath, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkReadPermission(c, realPath) {
		return
	}

	content, err := h.service.ReadFile(c.Request.Context(), realPath)
	if err != nil {
		handleError(c, err)
		return
	}

	ok(c, string(content))
}

// Download 下载文件
// @Summary 下载文件
// @Tags files
// @Accept json
// @Produce octet-stream
// @Param path query string true "文件路径"
// @Param inline query string false "是否内嵌显示（1为内嵌，否则下载）"
// @Success 200 {file} binary
// @Router /api/v1/files/download [get]
func (h *Handler) Download(c *gin.Context) {
	virtualPath := c.Query("path")
	if virtualPath == "" {
		fail(c, 400, "path is required")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, virtualPath, pathCtx) {
		return
	}

	realPath := common.ResolveVirtualPathWithContext(virtualPath, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkReadPermission(c, realPath) {
		return
	}

	info, err := os.Stat(realPath)
	if err != nil {
		handleError(c, ErrPathNotExist)
		return
	}

	if info.IsDir() {
		fail(c, 400, "cannot download directory directly")
		return
	}

	fileName := filepath.Base(realPath)
	inline := c.Query("inline") == "1"

	// 根据文件扩展名设置 Content-Type
	ext := strings.ToLower(filepath.Ext(fileName))
	contentType := getContentType(ext)

	c.Header("Content-Type", contentType)
	
	if inline {
		// 内嵌显示（用于预览）- 直接返回文件，不设置 Content-Disposition
		c.File(realPath)
	} else {
		// 强制下载 - 使用 FileAttachment 自动设置正确的 Content-Disposition
		c.FileAttachment(realPath, fileName)
	}
}

// getContentType 根据文件扩展名返回 MIME 类型
func getContentType(ext string) string {
	mimeTypes := map[string]string{
		// 图片
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".bmp":  "image/bmp",
		".ico":  "image/x-icon",
		// 视频
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".mkv":  "video/x-matroska",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".flv":  "video/x-flv",
		".m4v":  "video/x-m4v",
		// 音频
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".flac": "audio/flac",
		".aac":  "audio/aac",
		".m4a":  "audio/mp4",
		".wma":  "audio/x-ms-wma",
		// 文档
		".pdf":  "application/pdf",
		".txt":  "text/plain; charset=utf-8",
		".md":   "text/markdown; charset=utf-8",
		".json": "application/json; charset=utf-8",
		".xml":  "application/xml; charset=utf-8",
		".html": "text/html; charset=utf-8",
		".css":  "text/css; charset=utf-8",
		".js":   "application/javascript; charset=utf-8",
		".ts":   "text/typescript; charset=utf-8",
		".yaml": "text/yaml; charset=utf-8",
		".yml":  "text/yaml; charset=utf-8",
	}

	if ct, ok := mimeTypes[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}

// CreateDir 创建目录
// @Summary 创建目录
// @Tags files
// @Accept json
// @Produce json
// @Param body body CreateRequest true "创建请求"
// @Success 200 {object} response
// @Router /api/v1/files/mkdir [post]
func (h *Handler) CreateDir(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request")
		return
	}

	if req.Path == "" {
		fail(c, 400, "path is required")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, req.Path, pathCtx) {
		return
	}

	realPath := common.ResolveVirtualPathWithContext(req.Path, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// 如果提供了 Name，拼接到路径后面
	if req.Name != "" {
		realPath = filepath.Join(realPath, req.Name)
	}

	// Linux 文件权限检查 - 检查父目录的写权限
	parentDir := filepath.Dir(realPath)
	if !h.checkWritePermission(c, parentDir) {
		return
	}

	username := auth.GetUsername(c)
	if err := h.service.CreateDir(c.Request.Context(), realPath, username); err != nil {
		handleError(c, err)
		return
	}

	ok(c, nil)
}

// CreateFile 创建文件
// @Summary 创建文件
// @Tags files
// @Accept json
// @Produce json
// @Param body body CreateRequest true "创建请求"
// @Success 200 {object} response
// @Router /api/v1/files/create [post]
func (h *Handler) CreateFile(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request")
		return
	}

	if req.Path == "" {
		fail(c, 400, "path is required")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, req.Path, pathCtx) {
		return
	}

	var content []byte
	if req.Content != "" {
		content = []byte(req.Content)
	}

	realPath := common.ResolveVirtualPathWithContext(req.Path, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// 如果提供了 Name，拼接到路径后面
	if req.Name != "" {
		realPath = filepath.Join(realPath, req.Name)
	}

	// Linux 文件权限检查 - 检查父目录的写权限
	parentDir := filepath.Dir(realPath)
	if !h.checkWritePermission(c, parentDir) {
		return
	}

	username := auth.GetUsername(c)
	if err := h.service.CreateFile(c.Request.Context(), realPath, content, username); err != nil {
		handleError(c, err)
		return
	}

	ok(c, nil)
}

// Rename 重命名
// @Summary 重命名文件/目录
// @Tags files
// @Accept json
// @Produce json
// @Param body body RenameRequest true "重命名请求"
// @Success 200 {object} response
// @Router /api/v1/files/rename [put]
func (h *Handler) Rename(c *gin.Context) {
	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, req.OldPath, pathCtx) || !h.checkPathAccess(c, req.NewPath, pathCtx) {
		return
	}

	oldPath := common.ResolveVirtualPathWithContext(req.OldPath, pathCtx)
	newPath := common.ResolveVirtualPathWithContext(req.NewPath, pathCtx)
	if oldPath == "" || newPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkWritePermission(c, oldPath) {
		return
	}

	username := auth.GetUsername(c)
	if err := h.service.Rename(c.Request.Context(), oldPath, newPath, username); err != nil {
		handleError(c, err)
		return
	}

	ok(c, nil)
}

// Update 更新文件内容
// @Summary 更新文件内容
// @Tags files
// @Accept json
// @Produce json
// @Param body body UpdateContentRequest true "更新请求"
// @Success 200 {object} response
// @Router /api/v1/files/update [put]
func (h *Handler) Update(c *gin.Context) {
	var req UpdateContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, req.Path, pathCtx) {
		return
	}

	realPath := common.ResolveVirtualPathWithContext(req.Path, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkWritePermission(c, realPath) {
		return
	}

	username := auth.GetUsername(c)
	if err := h.service.WriteFile(c.Request.Context(), realPath, []byte(req.Content), 0644, username); err != nil {
		handleError(c, err)
		return
	}

	ok(c, nil)
}

// Delete 删除文件/目录
// @Summary 删除文件/目录
// @Tags files
// @Accept json
// @Produce json
// @Param body body []string true "要删除的路径列表"
// @Success 200 {object} response
// @Router /api/v1/files/delete [delete]
func (h *Handler) Delete(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request")
		return
	}

	paths := req.Paths
	if len(paths) == 0 {
		fail(c, 400, "paths is required")
		return
	}

	pathCtx := getPathContext(c)

	// 转换所有路径
	realPaths := make([]string, len(paths))
	for i, p := range paths {
		if !h.checkPathAccess(c, p, pathCtx) {
			return
		}
		realPaths[i] = common.ResolveVirtualPathWithContext(p, pathCtx)
		if realPaths[i] == "" {
			fail(c, 403, "permission denied")
			return
		}
	}

	// Linux 文件权限检查（检查每个路径的父目录写权限）
	for _, rp := range realPaths {
		if !h.checkWritePermission(c, filepath.Dir(rp)) {
			return
		}
	}

	username := auth.GetUsername(c)
	if err := h.service.Delete(c.Request.Context(), realPaths, username); err != nil {
		handleError(c, err)
		return
	}

	ok(c, nil)
}

// Operate 文件操作（复制/移动）
// @Summary 复制或移动文件
// @Tags files
// @Accept json
// @Produce json
// @Param body body FileOperation true "操作请求"
// @Success 200 {object} response
// @Router /api/v1/files/operate [post]
func (h *Handler) Operate(c *gin.Context) {
	var op FileOperation
	if err := c.ShouldBindJSON(&op); err != nil {
		fail(c, 400, "invalid request")
		return
	}

	pathCtx := getPathContext(c)

	// 权限检查和路径转换
	if !h.checkPathAccess(c, op.Destination, pathCtx) {
		return
	}
	for i := range op.Items {
		if !h.checkPathAccess(c, op.Items[i].Path, pathCtx) {
			return
		}
		op.Items[i].Path = common.ResolveVirtualPathWithContext(op.Items[i].Path, pathCtx)
		if op.Items[i].Path == "" {
			fail(c, 403, "permission denied")
			return
		}
	}
	op.Destination = common.ResolveVirtualPathWithContext(op.Destination, pathCtx)
	if op.Destination == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查：目标目录需要写权限
	if !h.checkWritePermission(c, op.Destination) {
		return
	}
	// 源文件需要读权限；如果是移动还需要源父目录写权限
	for _, item := range op.Items {
		if !h.checkReadPermission(c, item.Path) {
			return
		}
		if op.Type == "move" {
			if !h.checkWritePermission(c, filepath.Dir(item.Path)) {
				return
			}
		}
	}

	// 设置执行操作的用户名
	op.Username = auth.GetUsername(c)

	opID, err := h.service.StartOperation(c.Request.Context(), &op)
	if err != nil {
		handleError(c, err)
		return
	}

	ok(c, gin.H{"operation_id": opID})
}

// GetOperationStatus 获取操作状态
// @Summary 获取文件操作状态
// @Tags files
// @Accept json
// @Produce json
// @Param id query string true "操作ID"
// @Success 200 {object} OperationStatus
// @Router /api/v1/files/operate/status [get]
func (h *Handler) GetOperationStatus(c *gin.Context) {
	opID := c.Query("id")
	if opID == "" {
		fail(c, 400, "id is required")
		return
	}

	status, err := h.service.GetOperationStatus(c.Request.Context(), opID)
	if err != nil {
		fail(c, 404, err.Error())
		return
	}

	ok(c, status)
}

// CancelOperation 取消操作
// @Summary 取消文件操作
// @Tags files
// @Accept json
// @Produce json
// @Param id query string true "操作ID"
// @Success 200 {object} response
// @Router /api/v1/files/operate/cancel [delete]
func (h *Handler) CancelOperation(c *gin.Context) {
	opID := c.Query("id")
	if opID == "" {
		fail(c, 400, "id is required")
		return
	}

	if err := h.service.CancelOperation(c.Request.Context(), opID); err != nil {
		fail(c, 500, err.Error())
		return
	}

	ok(c, nil)
}

// Upload 上传文件
// @Summary 上传文件（支持分片）
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Param file formance file true "文件"
// @Param path formance string true "目标目录"
// @Param filename formance string true "文件名"
// @Param chunkNumber formance int false "分片编号"
// @Param totalChunks formance int false "总分片数"
// @Success 200 {object} response
// @Router /api/v1/files/upload [post]
func (h *Handler) Upload(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, 400, "file is required")
		return
	}
	defer file.Close()

	virtualPath := c.PostForm("path")
	filename := c.PostForm("filename")
	relativePath := c.PostForm("relativePath")
	chunkNumber := parseInt(c.PostForm("chunkNumber"), 1)
	totalChunks := parseInt(c.PostForm("totalChunks"), 1)

	if virtualPath == "" || filename == "" {
		fail(c, 400, "path and filename are required")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, virtualPath, pathCtx) {
		return
	}

	// 解析虚拟路径
	path := common.ResolveVirtualPathWithContext(virtualPath, pathCtx)
	if path == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkWritePermission(c, path) {
		return
	}

	username := auth.GetUsername(c)
	homeDir := filepath.Join("/home", username)

	// 处理相对路径（目录上传）
	// 情况1: relativePath 参数
	if relativePath != "" && relativePath != filename {
		dirPath := strings.TrimSuffix(relativePath, filename)
		fullDirPath := filepath.Join(path, dirPath)
		if err := runas.MkdirAllAndChown(homeDir, fullDirPath, username); err != nil {
			serverError(c, err.Error())
			return
		}
		path = fullDirPath
	}

	// 情况2: filename 包含路径（如 "folder/sub/file.txt"）
	if strings.Contains(filename, "/") {
		// 分离目录和文件名
		dir := filepath.Dir(filename)
		filename = filepath.Base(filename)
		// 创建父目录
		targetDir := filepath.Join(path, dir)
		if err := runas.MkdirAllAndChown(homeDir, targetDir, username); err != nil {
			serverError(c, err.Error())
			return
		}
		path = targetDir
	}

	destPath := filepath.Join(path, filename)

	if totalChunks > 1 {
		// 分片上传
		tempDir := h.service.GetUploadTempDir(path, filename, totalChunks)
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			serverError(c, err.Error())
			return
		}

		chunkPath := filepath.Join(tempDir, strconv.Itoa(chunkNumber))
		out, err := os.Create(chunkPath)
		if err != nil {
			serverError(c, err.Error())
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			serverError(c, err.Error())
			return
		}

		// 检查是否所有分片都已上传
		entries, _ := os.ReadDir(tempDir)
		if len(entries) == totalChunks {
			if err := h.service.MergeChunks(c.Request.Context(), tempDir, destPath, totalChunks); err != nil {
				serverError(c, err.Error())
				return
			}
			// 修改文件所有权为登录用户
			username := auth.GetUsername(c)
			if username != "" {
				runas.ChownToUser(destPath, username)
			}
		}
	} else {
		// 单文件上传
		out, err := os.Create(destPath)
		if err != nil {
			serverError(c, err.Error())
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			serverError(c, err.Error())
			return
		}

		// 修改文件所有权为登录用户
		username := auth.GetUsername(c)
		if username != "" {
			runas.ChownToUser(destPath, username)
		}
	}

	ok(c, nil)
}

// InitUploadRequest 初始化上传请求
type InitUploadRequest struct {
	Path      string `json:"path"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	ChunkSize int64  `json:"chunk_size"`
}

// InitUpload 初始化分片上传
// @Summary 初始化分片上传
// @Tags files
// @Accept json
// @Produce json
// @Param body body InitUploadRequest true "上传信息"
// @Success 200 {object} response
// @Router /api/v1/files/upload/init [post]
func (h *Handler) InitUpload(c *gin.Context) {
	var req InitUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	if req.Path == "" || req.Filename == "" {
		fail(c, 400, "path and filename are required")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, req.Path, pathCtx) {
		return
	}

	// 解析虚拟路径
	realPath := common.ResolveVirtualPathWithContext(req.Path, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkWritePermission(c, realPath) {
		return
	}

	// 生成上传ID
	uploadId := h.service.GenerateUploadID(realPath, req.Filename)

	// 创建临时目录
	tempDir := h.service.GetUploadTempDirByID(uploadId)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		serverError(c, err.Error())
		return
	}

	// 计算分片数
	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 50 * 1024 * 1024 // 默认 50MB
	}
	totalChunks := (req.Size + chunkSize - 1) / chunkSize

	// 保存上传元信息（使用真实路径）
	h.service.SaveUploadMeta(uploadId, realPath, req.Filename, req.Size, int(totalChunks))

	ok(c, gin.H{
		"upload_id":    uploadId,
		"chunk_size":   chunkSize,
		"total_chunks": totalChunks,
	})
}

// UploadChunk 上传分片
// @Summary 上传分片
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Param upload_id formData string true "上传ID"
// @Param chunk_index formData int true "分片索引"
// @Param file formData file true "分片数据"
// @Success 200 {object} response
// @Router /api/v1/files/upload/chunk [post]
func (h *Handler) UploadChunk(c *gin.Context) {
	uploadId := c.PostForm("upload_id")
	chunkIndex := parseInt(c.PostForm("chunk_index"), -1)

	if uploadId == "" || chunkIndex < 0 {
		fail(c, 400, "upload_id and chunk_index are required")
		return
	}

	// 优先使用 "chunk" 字段名，兼容 "file"
	file, _, err := c.Request.FormFile("chunk")
	if err != nil {
		file, _, err = c.Request.FormFile("file")
		if err != nil {
			fail(c, 400, "chunk or file is required")
			return
		}
	}
	defer file.Close()

	// 保存分片
	tempDir := h.service.GetUploadTempDirByID(uploadId)

	// 确保临时目录存在
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		serverError(c, "failed to create temp dir: "+err.Error())
		return
	}

	chunkPath := filepath.Join(tempDir, strconv.Itoa(chunkIndex))

	out, err := os.Create(chunkPath)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		serverError(c, err.Error())
		return
	}

	ok(c, gin.H{
		"chunk_index": chunkIndex,
		"received":    true,
	})
}

// CompleteUploadRequest 完成上传请求
type CompleteUploadRequest struct {
	UploadID string `json:"upload_id"`
}

// CompleteUpload 完成分片上传
// @Summary 完成分片上传
// @Tags files
// @Accept json
// @Produce json
// @Param body body CompleteUploadRequest true "上传信息"
// @Success 200 {object} response
// @Router /api/v1/files/upload/complete [post]
func (h *Handler) CompleteUpload(c *gin.Context) {
	var req CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	if req.UploadID == "" {
		fail(c, 400, "upload_id is required")
		return
	}

	// 获取上传元信息
	meta := h.service.GetUploadMeta(req.UploadID)
	if meta == nil {
		fail(c, 404, "upload not found")
		return
	}

	// 合并分片
	tempDir := h.service.GetUploadTempDirByID(req.UploadID)

	username := auth.GetUsername(c)

	// 处理包含路径的 filename（如 "folder/sub/file.txt"）
	filename := meta.Filename
	targetPath := meta.Path
	homeDir := filepath.Join("/home", username)
	if strings.Contains(filename, "/") {
		dir := filepath.Dir(filename)
		filename = filepath.Base(filename)
		targetPath = filepath.Join(meta.Path, dir)
		// 创建父目录并 chown
		if err := runas.MkdirAllAndChown(homeDir, targetPath, username); err != nil {
			serverError(c, err.Error())
			return
		}
	}

	destPath := filepath.Join(targetPath, filename)

	if err := h.service.MergeChunks(c.Request.Context(), tempDir, destPath, meta.TotalChunks); err != nil {
		serverError(c, err.Error())
		return
	}

	// 修改文件所有权为登录用户
	if username != "" {
		runas.ChownToUser(destPath, username)
	}

	// 清理元信息
	h.service.DeleteUploadMeta(req.UploadID)

	ok(c, gin.H{
		"path": toVirtualPath(destPath),
	})
}

// AbortUpload 取消上传
// @Summary 取消分片上传
// @Tags files
// @Accept json
// @Produce json
// @Param upload_id query string true "上传ID"
// @Success 200 {object} response
// @Router /api/v1/files/upload/abort [delete]
func (h *Handler) AbortUpload(c *gin.Context) {
	uploadId := c.Query("upload_id")
	if uploadId == "" {
		fail(c, 400, "upload_id is required")
		return
	}

	// 删除临时目录
	tempDir := h.service.GetUploadTempDirByID(uploadId)
	os.RemoveAll(tempDir)

	// 清理元信息
	h.service.DeleteUploadMeta(uploadId)

	ok(c, nil)
}

// Search 搜索文件
// @Summary 搜索文件
// @Tags files
// @Accept json
// @Produce json
// @Param path query string true "搜索路径"
// @Param keyword query string true "关键字"
// @Param recursive query bool false "是否递归" default(false)
// @Param file_type query string false "文件类型" Enums(all, file, dir)
// @Param max_results query int false "最大结果数" default(100)
// @Success 200 {object} SearchResult
// @Router /api/v1/files/search [get]
func (h *Handler) Search(c *gin.Context) {
	virtualPath := c.Query("path")
	if virtualPath == "" || c.Query("keyword") == "" {
		fail(c, 400, "path and keyword are required")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, virtualPath, pathCtx) {
		return
	}

	realPath := common.ResolveVirtualPathWithContext(virtualPath, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkReadPermission(c, realPath) {
		return
	}

	req := &SearchRequest{
		Path:       realPath,
		Keyword:    c.Query("keyword"),
		Recursive:  c.Query("recursive") == "true",
		FileType:   c.DefaultQuery("file_type", "all"),
		MaxResults: parseInt(c.Query("max_results"), 100),
	}

	result, err := h.service.Search(c.Request.Context(), req)
	if err != nil {
		handleError(c, err)
		return
	}

	// 转换结果中的路径
	if result != nil {
		for i := range result.Files {
			result.Files[i].Path = common.ToVirtualPathWithContext(result.Files[i].Path, pathCtx)
		}
	}

	ok(c, result)
}

// GetStats 获取目录统计
// @Summary 获取目录统计
// @Tags files
// @Accept json
// @Produce json
// @Param path query string true "目录路径"
// @Success 200 {object} FileStats
// @Router /api/v1/files/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	virtualPath := c.Query("path")
	if virtualPath == "" {
		fail(c, 400, "path is required")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, virtualPath, pathCtx) {
		return
	}

	realPath := common.ResolveVirtualPathWithContext(virtualPath, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkReadPermission(c, realPath) {
		return
	}

	stats, err := h.service.GetStats(c.Request.Context(), realPath)
	if err != nil {
		handleError(c, err)
		return
	}

	ok(c, stats)
}

// Thumbnail 获取文件缩略图
// @Summary 获取文件缩略图
// @Tags files
// @Accept json
// @Produce image/jpeg
// @Param path query string true "文件路径"
// @Param size query int false "缩略图大小" default(256)
// @Param token query string false "认证 token（用于 img src）"
// @Success 200 {file} binary
// @Router /api/v1/files/thumbnail [get]
func (h *Handler) Thumbnail(c *gin.Context) {
	virtualPath := c.Query("path")
	if virtualPath == "" {
		fail(c, 400, "path is required")
		return
	}

	pathCtx := getPathContext(c)
	if !h.checkPathAccess(c, virtualPath, pathCtx) {
		return
	}

	realPath := common.ResolveVirtualPathWithContext(virtualPath, pathCtx)
	if realPath == "" {
		fail(c, 403, "permission denied")
		return
	}

	// Linux 文件权限检查
	if !h.checkReadPermission(c, realPath) {
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(realPath); os.IsNotExist(err) {
		fail(c, 404, "file not found")
		return
	}

	// 检查是否支持缩略图
	if !SupportsThumbnail(realPath) {
		fail(c, 400, "file type not supported for thumbnail")
		return
	}

	// 解析尺寸参数
	sizeParam := c.DefaultQuery("size", "256")
	size := parseInt(sizeParam, 256)
	var thumbSize ThumbnailSize
	switch {
	case size <= 128:
		thumbSize = ThumbnailSmall
	case size <= 256:
		thumbSize = ThumbnailMedium
	case size <= 512:
		thumbSize = ThumbnailLarge
	default:
		thumbSize = ThumbnailXLarge
	}

	// 检查 thumbnails 服务是否可用
	if h.thumbnails == nil {
		serverError(c, "thumbnail service not available")
		return
	}

	// 获取缩略图
	result, err := h.thumbnails.GetThumbnail(realPath, thumbSize)
	if err != nil {
		serverError(c, "failed to generate thumbnail: "+err.Error())
		return
	}

	// 设置缓存头（缩略图可以长期缓存）
	c.Header("Cache-Control", "public, max-age=86400") // 24 小时
	c.Header("Content-Type", result.MimeType)
	c.Data(200, result.MimeType, result.Data)
}

// Elevate 管理员提权
// @Summary 管理员提权（输入密码后获取5分钟的超级权限）
// @Tags files
// @Accept json
// @Produce json
// @Param body body object true "密码" SchemaExample({"password": "xxx"})
// @Success 200 {object} response
// @Router /api/v1/files/elevate [post]
func (h *Handler) Elevate(c *gin.Context) {
	if !auth.IsAdmin(c) {
		fail(c, 403, "only admin can elevate")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		fail(c, 400, "password is required")
		return
	}

	userID := c.GetString("user_id")

	// 从数据库获取用户密码 hash
	var account struct {
		Password string
	}
	if err := h.db.Table("users_accounts").Select("password").Where("id = ?", userID).Scan(&account).Error; err != nil {
		fail(c, 500, "failed to verify password")
		return
	}
	if account.Password == "" {
		fail(c, 500, "failed to verify password")
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(req.Password)); err != nil {
		fail(c, 401, "密码错误")
		return
	}

	// 创建提权会话
	username := auth.GetUsername(c)
	session := h.elevationMgr.Elevate(userID, username)

	ok(c, gin.H{
		"elevated":   true,
		"expires_at": session.ExpiresAt,
		"duration":   ElevationDuration.Seconds(),
	})
}

// RevokeElevation 撤销提权
// @Summary 撤销管理员提权
// @Tags files
// @Produce json
// @Success 200 {object} response
// @Router /api/v1/files/elevate [delete]
func (h *Handler) RevokeElevation(c *gin.Context) {
	userID := c.GetString("user_id")
	h.elevationMgr.Revoke(userID)
	ok(c, gin.H{"elevated": false})
}

// GetElevationStatus 获取提权状态
// @Summary 获取管理员提权状态
// @Tags files
// @Produce json
// @Success 200 {object} response
// @Router /api/v1/files/elevate [get]
func (h *Handler) GetElevationStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	status := h.elevationMgr.GetStatus(userID)

	if status == nil {
		ok(c, gin.H{
			"elevated":  false,
			"remaining": 0,
		})
		return
	}

	ok(c, gin.H{
		"elevated":   true,
		"remaining":  time.Until(status.ExpiresAt).Seconds(),
		"expires_at": status.ExpiresAt,
	})
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	files := group.Group("/files")
	{
		// GET /files?path=xxx 作为 /files/list 的别名
		files.GET("", h.List)
		files.GET("/list", h.List)
		files.GET("/info", h.GetInfo)
		files.GET("/read", h.Read)
		files.GET("/download", h.Download)
		files.GET("/bookmarks", h.GetBookmarks)
		files.POST("/mkdir", h.CreateDir)
		files.POST("/create", h.CreateFile)
		files.PUT("/rename", h.Rename)
		files.PUT("/update", h.Update)
		files.DELETE("/delete", h.Delete)
		files.POST("/operate", h.Operate)
		files.GET("/operate/status", h.GetOperationStatus)
		files.DELETE("/operate/cancel", h.CancelOperation)
		files.POST("/upload", h.Upload)
		files.POST("/upload/init", h.InitUpload)
		files.POST("/upload/chunk", h.UploadChunk)
		files.POST("/upload/complete", h.CompleteUpload)
		files.DELETE("/upload/abort", h.AbortUpload)
		files.GET("/search", h.Search)
		files.GET("/stats", h.GetStats)
		files.GET("/thumbnail", h.Thumbnail)
		// 管理员提权
		files.POST("/elevate", h.Elevate)
		files.DELETE("/elevate", h.RevokeElevation)
		files.GET("/elevate", h.GetElevationStatus)
		// 音频元数据
		files.GET("/audio/metadata", h.GetAudioMetadata)
		files.GET("/audio/lyrics", h.GetAudioLyrics)
	}
}

// BookmarkItem 书签条目
type BookmarkItem struct {
	Icon  string `json:"icon"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

// BookmarksResponse 书签响应
type BookmarksResponse struct {
	Default  []BookmarkItem `json:"default"`
	System   []BookmarkItem `json:"system"`
	HomePath string         `json:"home_path"`
}

// GetBookmarks 获取当前用户的快捷访问书签
func (h *Handler) GetBookmarks(c *gin.Context) {
	username := auth.GetUsername(c)
	isAdmin := auth.IsAdmin(c)

	homeDir := filepath.Join("/home", username)

	// 如果 home 目录不存在，尝试创建
	if _, err := os.Stat(homeDir); os.IsNotExist(err) {
		// 尝试创建用户主目录
		if mkErr := os.MkdirAll(homeDir, 0755); mkErr != nil {
			// 创建失败，回退到 /root（兼容 root 用户运行的情况）
			homeDir = "/root"
		} else {
			// 修改目录所有权为用户
			runas.ChownToUser(homeDir, username)
		}
	}

	// 用户默认书签
	defaultBookmarks := []BookmarkItem{
		{Icon: "mdi:home", Label: "主目录", Path: homeDir},
		{Icon: "mdi:desktop-mac", Label: "桌面", Path: filepath.Join(homeDir, "Desktop")},
		{Icon: "mdi:download", Label: "下载", Path: filepath.Join(homeDir, "Downloads")},
		{Icon: "mdi:file-document-outline", Label: "文档", Path: filepath.Join(homeDir, "Documents")},
		{Icon: "mdi:image-outline", Label: "图片", Path: filepath.Join(homeDir, "Pictures")},
		{Icon: "mdi:movie-outline", Label: "视频", Path: filepath.Join(homeDir, "Videos")},
		{Icon: "mdi:music-note", Label: "音乐", Path: filepath.Join(homeDir, "Music")},
	}

	// 过滤掉实际不存在的目录
	var filtered []BookmarkItem
	for _, bm := range defaultBookmarks {
		if _, err := os.Stat(bm.Path); err == nil {
			filtered = append(filtered, bm)
		}
	}
	if len(filtered) == 0 {
		// 至少保留主目录
		filtered = []BookmarkItem{
			{Icon: "mdi:home", Label: "主目录", Path: homeDir},
		}
	}

	// 系统目录（仅管理员可见）
	var systemBookmarks []BookmarkItem
	if isAdmin {
		systemBookmarks = []BookmarkItem{
			{Icon: "mdi:folder-home", Label: "根目录", Path: "/"},
		}
	}

	ok(c, BookmarksResponse{
		Default:  filtered,
		System:   systemBookmarks,
		HomePath: homeDir,
	})
}

// handleError 处理错误
func handleError(c *gin.Context, err error) {
	switch err {
	case ErrPathEmpty:
		fail(c, 400, "path is empty")
	case ErrPathNotExist:
		fail(c, 404, "path does not exist")
	case ErrPathExists:
		fail(c, 409, "path already exists")
	case ErrNotFile:
		fail(c, 400, "path is not a file")
	case ErrNotDir:
		fail(c, 400, "path is not a directory")
	case ErrPermissionDenied:
		fail(c, 403, "permission denied")
	case ErrMountedPath:
		fail(c, 400, "cannot operate on mounted path")
	case ErrSameSourceDest:
		fail(c, 400, "source and destination are the same")
	case ErrInvalidOperation:
		fail(c, 400, "invalid operation type")
	default:
		serverError(c, err.Error())
	}
}

// parseInt 解析整数
func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}
