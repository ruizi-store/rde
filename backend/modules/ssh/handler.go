// Package ssh SSH远程连接模块 - HTTP/WebSocket 处理器
package ssh

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/ruizi-store/rde/backend/core/auth"
	"go.uber.org/zap"
)

// Handler HTTP 处理器
type Handler struct {
	service  *Service
	upgrader websocket.Upgrader
	logger   *zap.Logger
}

// NewHandler 创建处理器
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  WebSocketBufferSize,
			WriteBufferSize: WebSocketBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup, tokenManager *auth.TokenManager) {
	// SSH连接配置管理
	connections := router.Group("/ssh/connections")
	connections.Use(auth.Middleware(tokenManager), auth.RequireAdmin())
	{
		connections.POST("", h.CreateConnection)
		connections.GET("", h.ListConnections)
		connections.GET("/:id", h.GetConnection)
		connections.PUT("/:id", h.UpdateConnection)
		connections.DELETE("/:id", h.DeleteConnection)
		connections.POST("/:id/test", h.TestConnectionByID)
		connections.POST("/test", h.TestConnection)
	}

	// SSH会话管理
	sessions := router.Group("/ssh/sessions")
	sessions.Use(auth.Middleware(tokenManager), auth.RequireAdmin())
	{
		sessions.POST("", h.CreateSession)
		sessions.GET("", h.ListSessions)
		sessions.DELETE("/:id", h.CloseSession)
		sessions.POST("/:id/resize", h.Resize)
	}

	// SSH WebSocket（单独处理认证）
	router.GET("/ssh/sessions/:id/ws", h.handleWebSocketAuth(tokenManager), h.WebSocket)

	// SFTP操作
	sftp := router.Group("/sftp/:session_id")
	sftp.Use(auth.Middleware(tokenManager), auth.RequireAdmin())
	{
		sftp.GET("/list", h.ListDir)
		sftp.GET("/stat", h.Stat)
		sftp.POST("/mkdir", h.Mkdir)
		sftp.POST("/rename", h.Rename)
		sftp.DELETE("/delete", h.Delete)
		sftp.POST("/upload", h.Upload)
		sftp.POST("/download", h.Download)
	}

	// 传输队列
	transfers := router.Group("/sftp/transfers")
	transfers.Use(auth.Middleware(tokenManager), auth.RequireAdmin())
	{
		transfers.GET("", h.ListTransfers)
		transfers.POST("", h.CreateTransfer)
		transfers.DELETE("/:id", h.CancelTransfer)
		transfers.DELETE("", h.ClearTransfers)
	}

	// 传输进度 WebSocket
	router.GET("/sftp/transfers/ws", h.handleWebSocketAuth(tokenManager), h.TransferProgressWS)
}

// handleWebSocketAuth WebSocket 认证中间件
func (h *Handler) handleWebSocketAuth(tokenManager *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			token = auth.ExtractToken(c)
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authentication required",
			})
			c.Abort()
			return
		}

		claims, err := tokenManager.ParseAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid token",
			})
			c.Abort()
			return
		}

		if claims.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Admin access required",
			})
			c.Abort()
			return
		}

		c.Set(auth.ContextKeyUserID, claims.UserID)
		c.Set(auth.ContextKeyUsername, claims.Username)
		c.Set(auth.ContextKeyRole, claims.Role)
		c.Set(auth.ContextKeyClaims, claims)

		c.Next()
	}
}

// ==================== 连接配置管理 ====================

// CreateConnection 创建SSH连接配置
func (h *Handler) CreateConnection(c *gin.Context) {
	var req CreateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	conn, err := h.service.CreateConnection(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    conn,
	})
}

// ListConnections 获取所有连接配置
func (h *Handler) ListConnections(c *gin.Context) {
	connections, err := h.service.ListConnections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    connections,
	})
}

// GetConnection 获取单个连接配置
func (h *Handler) GetConnection(c *gin.Context) {
	id := c.Param("id")

	conn, err := h.service.GetConnection(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    conn,
	})
}

// UpdateConnection 更新连接配置
func (h *Handler) UpdateConnection(c *gin.Context) {
	id := c.Param("id")

	var req UpdateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	conn, err := h.service.UpdateConnection(id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    conn,
	})
}

// DeleteConnection 删除连接配置
func (h *Handler) DeleteConnection(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteConnection(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "连接已删除",
	})
}

// TestConnection 测试连接（不保存）
func (h *Handler) TestConnection(c *gin.Context) {
	var req TestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := h.service.TestConnection(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "连接失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "连接成功",
	})
}

// TestConnectionByID 测试已保存的连接
func (h *Handler) TestConnectionByID(c *gin.Context) {
	id := c.Param("id")

	conn, err := h.service.GetConnection(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 先检查端口
	if err := h.service.CheckPort(conn.Host, conn.Port); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法连接到主机: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "主机可达",
	})
}

// ==================== SSH会话管理 ====================

// CreateSession 创建SSH会话
func (h *Handler) CreateSession(c *gin.Context) {
	var req ConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	session, err := h.service.Connect(req.ConnectionID, req.Cols, req.Rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    session.ToInfo(),
	})
}

// ListSessions 获取所有会话
func (h *Handler) ListSessions(c *gin.Context) {
	sessions := h.service.ListSessions()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sessions,
	})
}

// CloseSession 关闭会话
func (h *Handler) CloseSession(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.CloseSession(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "会话已关闭",
	})
}

// Resize 调整终端大小
func (h *Handler) Resize(c *gin.Context) {
	id := c.Param("id")

	var req ResizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := h.service.ResizeTerminal(id, req.Cols, req.Rows); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// WebSocket 处理WebSocket连接
func (h *Handler) WebSocket(c *gin.Context) {
	sessionID := c.Param("id")

	session, err := h.service.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 升级为WebSocket
	ws, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade websocket", zap.Error(err))
		return
	}

	session.mu.Lock()
	session.ws = ws
	session.mu.Unlock()

	// 获取PTY的stdin/stdout
	stdin, err := session.ptySession.StdinPipe()
	if err != nil {
		h.logger.Error("Failed to get stdin pipe", zap.Error(err))
		ws.Close()
		return
	}

	stdout, err := session.ptySession.StdoutPipe()
	if err != nil {
		h.logger.Error("Failed to get stdout pipe", zap.Error(err))
		ws.Close()
		return
	}

	// 启动shell
	if err := session.ptySession.Shell(); err != nil {
		h.logger.Error("Failed to start shell", zap.Error(err))
		ws.Close()
		return
	}

	// 处理WebSocket消息
	go h.handleWSRead(session, stdin, ws)
	go h.handleWSWrite(session, stdout, ws)

	// 等待会话结束
	<-session.done
}

// handleWSRead 处理WebSocket读取（用户输入）
func (h *Handler) handleWSRead(session *Session, stdin io.WriteCloser, ws *websocket.Conn) {
	defer func() {
		stdin.Close()
		h.service.CloseSession(session.ID)
	}()

	for {
		select {
		case <-session.done:
			return
		default:
		}

		msgType, data, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("WebSocket read error", zap.Error(err))
			}
			return
		}

		if msgType == websocket.TextMessage {
			// 检查是否是控制消息
			if len(data) > 0 && data[0] == '{' {
				var msg ControlMessage
				if err := json.Unmarshal(data, &msg); err == nil && msg.Type == "resize" {
					h.service.ResizeTerminal(session.ID, msg.Cols, msg.Rows)
					continue
				}
			}
		}

		// 写入PTY
		if _, err := stdin.Write(data); err != nil {
			h.logger.Error("Failed to write to pty", zap.Error(err))
			return
		}
	}
}

// handleWSWrite 处理WebSocket写入（终端输出）
func (h *Handler) handleWSWrite(session *Session, stdout io.Reader, ws *websocket.Conn) {
	buf := make([]byte, 8192)

	for {
		select {
		case <-session.done:
			return
		default:
		}

		n, err := stdout.Read(buf)
		if err != nil {
			if err != io.EOF {
				h.logger.Error("Failed to read from pty", zap.Error(err))
			}
			return
		}

		session.mu.Lock()
		err = ws.WriteMessage(websocket.BinaryMessage, buf[:n])
		session.mu.Unlock()

		if err != nil {
			h.logger.Error("Failed to write to websocket", zap.Error(err))
			return
		}
	}
}

// ==================== SFTP操作 ====================

// ListDir 列出目录
func (h *Handler) ListDir(c *gin.Context) {
	sessionID := c.Param("session_id")
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	files, err := h.service.ListDir(sessionID, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    files,
	})
}

// Stat 获取文件信息
func (h *Handler) Stat(c *gin.Context) {
	sessionID := c.Param("session_id")
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "path is required",
		})
		return
	}

	info, err := h.service.Stat(sessionID, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    info,
	})
}

// Mkdir 创建目录
func (h *Handler) Mkdir(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req MkdirRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := h.service.Mkdir(sessionID, req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// Rename 重命名/移动
func (h *Handler) Rename(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := h.service.Rename(sessionID, req.OldPath, req.NewPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// Delete 删除文件/目录
func (h *Handler) Delete(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req DeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := h.service.Delete(sessionID, req.Paths); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// Upload 上传文件（multipart）
func (h *Handler) Upload(c *gin.Context) {
	sessionID := c.Param("session_id")
	remoteDir := c.PostForm("remote_dir")
	if remoteDir == "" {
		remoteDir = "/"
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的表单数据",
		})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "没有文件",
		})
		return
	}

	var uploaded []string
	for _, file := range files {
		src, err := file.Open()
		if err != nil {
			continue
		}

		remotePath := filepath.Join(remoteDir, file.Filename)
		if err := h.service.UploadFromReader(sessionID, remotePath, src, file.Size); err != nil {
			src.Close()
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "上传失败: " + err.Error(),
			})
			return
		}
		src.Close()
		uploaded = append(uploaded, remotePath)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"uploaded": uploaded,
		},
	})
}

// Download 下载文件到本地
func (h *Handler) Download(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var downloaded []string
	for _, remotePath := range req.RemotePaths {
		fileName := filepath.Base(remotePath)
		localPath := filepath.Join(req.LocalDir, fileName)

		if err := h.service.DownloadFile(sessionID, remotePath, localPath, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "下载失败: " + err.Error(),
			})
			return
		}
		downloaded = append(downloaded, localPath)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"downloaded": downloaded,
		},
	})
}

// ==================== 传输队列管理 ====================

// ListTransfers 列出传输任务
func (h *Handler) ListTransfers(c *gin.Context) {
	sessionID := c.Query("session_id")
	tasks := h.service.ListTransferTasks(sessionID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tasks,
	})
}

// CreateTransfer 创建传输任务
func (h *Handler) CreateTransfer(c *gin.Context) {
	var req CreateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var tasks []*TransferTask

	if req.Type == "download" {
		// 下载任务
		for _, remotePath := range req.RemotePaths {
			fileName := filepath.Base(remotePath)
			localPath := filepath.Join(req.LocalDir, fileName)

			// 获取文件大小
			fileInfo, err := h.service.Stat(req.SessionID, remotePath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "获取文件信息失败: " + err.Error(),
				})
				return
			}

			task, err := h.service.CreateTransferTask(
				req.SessionID, "download", localPath, remotePath, fileName, fileInfo.Size,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "创建任务失败: " + err.Error(),
				})
				return
			}
			tasks = append(tasks, task)
		}
	} else if req.Type == "upload" {
		// 上传任务
		for _, localPath := range req.LocalPaths {
			fileName := filepath.Base(localPath)
			remotePath := filepath.Join(req.RemoteDir, fileName)

			// 获取本地文件大小
			stat, err := os.Stat(localPath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "获取本地文件信息失败: " + err.Error(),
				})
				return
			}

			task, err := h.service.CreateTransferTask(
				req.SessionID, "upload", localPath, remotePath, fileName, stat.Size(),
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "创建任务失败: " + err.Error(),
				})
				return
			}
			tasks = append(tasks, task)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tasks,
	})
}

// CancelTransfer 取消传输任务
func (h *Handler) CancelTransfer(c *gin.Context) {
	taskID := c.Param("id")

	if err := h.service.CancelTransferTask(taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "任务已取消",
	})
}

// ClearTransfers 清除已完成的任务
func (h *Handler) ClearTransfers(c *gin.Context) {
	h.service.ClearCompletedTasks()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已清除完成的任务",
	})
}

// TransferProgressWS 传输进度 WebSocket
func (h *Handler) TransferProgressWS(c *gin.Context) {
	ws, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade websocket", zap.Error(err))
		return
	}
	defer ws.Close()

	progressChan := h.service.GetProgressChannel()

	// 发送心跳
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case progress, ok := <-progressChan:
			if !ok {
				return
			}
			if err := ws.WriteJSON(progress); err != nil {
				h.logger.Error("Failed to send progress", zap.Error(err))
				return
			}
		case <-ticker.C:
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
