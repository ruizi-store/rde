// Package docker 端口检测服务
package docker

import (
	"fmt"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PortHandler 端口检测 HTTP 处理器
type PortHandler struct{}

// NewPortHandler 创建端口处理器
func NewPortHandler() *PortHandler {
	return &PortHandler{}
}

// RegisterRoutes 注册路由
func (h *PortHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/check", h.CheckPort)
	r.GET("/suggest", h.SuggestPort)
}

// CheckPort 检查端口是否可用
// GET /docker/ports/check?port=8080
func (h *PortHandler) CheckPort(c *gin.Context) {
	port := c.Query("port")
	if port == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "port is required"})
		return
	}

	available := isPortAvailable(port)
	c.JSON(http.StatusOK, gin.H{
		"port":      port,
		"available": available,
	})
}

// SuggestPort 建议可用端口
// GET /docker/ports/suggest?preferred=8080
func (h *PortHandler) SuggestPort(c *gin.Context) {
	preferred := c.Query("preferred")
	if preferred == "" {
		preferred = "8080"
	}

	var portNum int
	fmt.Sscanf(preferred, "%d", &portNum)
	if portNum < 1 || portNum > 65535 {
		portNum = 8080
	}

	// 从首选端口开始查找可用端口
	for i := 0; i < 100; i++ {
		testPort := fmt.Sprintf("%d", portNum+i)
		if isPortAvailable(testPort) {
			c.JSON(http.StatusOK, gin.H{
				"suggested": portNum + i,
				"preferred": portNum,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"suggested": 0,
		"error":     "No available port found in range",
	})
}

// isPortAvailable 检查端口是否可用
func isPortAvailable(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
