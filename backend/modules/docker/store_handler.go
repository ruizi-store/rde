// Package docker 应用商店 HTTP 处理器
package docker

import (
	"net/http"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// StoreHandler 应用商店 HTTP 处理器
type StoreHandler struct {
	catalog  *CatalogService
	logger   *zap.Logger
	iconsDir string
}

// NewStoreHandler 创建商店处理器
func NewStoreHandler(catalog *CatalogService, logger *zap.Logger) *StoreHandler {
	return &StoreHandler{
		catalog: catalog,
		logger:  logger,
	}
}

// SetIconsDir 设置图标目录路径
func (h *StoreHandler) SetIconsDir(dir string) {
	h.iconsDir = dir
}

// RegisterRoutes 注册商店路由
func (h *StoreHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/apps", h.ListApps)
	r.GET("/apps/:id", h.GetApp)
	r.GET("/categories", h.ListCategories)

	// 图标静态文件（从前端 static 目录或数据目录提供）
	if h.iconsDir != "" {
		r.Static("/icons", h.iconsDir)
	}
}

// ListApps 获取应用列表
// GET /docker/store/apps?category=xxx&search=xxx
func (h *StoreHandler) ListApps(c *gin.Context) {
	if h.catalog == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "App catalog not loaded"})
		return
	}

	category := c.Query("category")
	search := c.Query("search")

	apps := h.catalog.GetApps(category, search)

	// 将 Go 架构名映射为 Docker 架构名（amd64→amd64, arm64→arm64）
	goArch := runtime.GOARCH
	dockerArch := goArch // Go 和 Docker 对 amd64/arm64 的命名一致
	if goArch == "arm" {
		dockerArch = "arm/v7"
	}

	// 返回精简列表（不含 compose 和 form 详情）
	list := make([]gin.H, 0, len(apps))
	for _, app := range apps {
		// 判断是否兼容当前架构
		compatible := len(app.Architectures) == 0 // 无声明则视为兼容
		for _, a := range app.Architectures {
			if strings.EqualFold(a, dockerArch) {
				compatible = true
				break
			}
		}

		list = append(list, gin.H{
			"id":            app.ID,
			"name":          app.Name,
			"title":         app.Title,
			"description":   app.Description,
			"title_i18n":    app.TitleI18n,
			"desc_i18n":     app.DescI18n,
			"category":      app.Category,
			"icon":          app.Icon,
			"version":       app.Version,
			"author":        app.Author,
			"tags":          app.Tags,
			"architectures": app.Architectures,
			"compatible":    compatible,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": list})
}

// GetApp 获取应用详情
// GET /docker/store/apps/:id
func (h *StoreHandler) GetApp(c *gin.Context) {
	if h.catalog == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "App catalog not loaded"})
		return
	}

	id := c.Param("id")
	app := h.catalog.GetApp(id)
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": app})
}

// ListCategories 获取分类列表
// GET /docker/store/categories
func (h *StoreHandler) ListCategories(c *gin.Context) {
	if h.catalog == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "App catalog not loaded"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": h.catalog.GetCategories()})
}
