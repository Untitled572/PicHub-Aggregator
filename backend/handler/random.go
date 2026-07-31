package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/service"
)

// RandomImage GET /random 随机分发图片
// @Summary 随机分发图片
// @Description 从分发池随机返回一张图片。category 为空时使用 settings.bound_tags。format=json 返回 JSON，否则 302 重定向到图片 URL。
// @Tags Distribution
// @Produce json
// @Param category query string false "标签(可逗号分隔多个)"
// @Param format query string false "响应格式" Enums(json)
// @Param orientation query string false "根据 UA 推断的屏幕方向" Enums(mobile,desktop)
// @Success 200 {object} service.Result
// @Success 302 "重定向到图片 URL (format != json)"
// @Failure 503 {object} map[string]interface{} "无可用图片"
// @Router /random [get]
func (h *Handler) RandomImage(c *gin.Context) {
	category := c.Query("category")
	if category == "" {
		settings, err := h.store.GetSettings()
		if err == nil && len(settings.BoundTags) > 0 {
			category = strings.Join(settings.BoundTags, ",")
		}
	}
	h.serveRandom(c, category)
}

// EndpointImage GET /e/{name} 自定义分发端点, 与 /random 完全同功能
// @Summary 自定义端点分发
// @Description ?category 为空时 fallback 到端点自身绑定的标签
// @Tags Distribution
// @Produce json
// @Param name path string true "端点名称"
// @Param category query string false "标签(可逗号分隔多个)"
// @Param format query string false "响应格式" Enums(json)
// @Param orientation query string false "根据 UA 推断的屏幕方向" Enums(mobile,desktop)
// @Success 200 {object} service.Result
// @Success 302 "重定向到图片 URL (format != json)"
// @Failure 404 {object} map[string]interface{} "端点不存在或已禁用"
// @Failure 503 {object} map[string]interface{} "无可用图片"
// @Router /e/{name} [get]
func (h *Handler) EndpointImage(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	ep, err := h.store.GetEndpointByName(name)
	if err != nil || ep == nil || !ep.Enabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
		return
	}
	category := c.Query("category")
	if category == "" && len(ep.BoundTags) > 0 {
		category = strings.Join(ep.BoundTags, ",")
	}
	h.serveRandom(c, category)
}

func (h *Handler) serveRandom(c *gin.Context, category string) {
	format := c.Query("format")
	orientation := c.Query("orientation")
	clientUA := c.GetHeader("User-Agent")

	result, statusCode, err := h.engine.RandomImage(category, format, orientation, clientUA)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	redirectURL := result.URL
	if result.LocalURL != "" {
		redirectURL = result.LocalURL
	}

	// 分发后异步移动文件：从子目录移到根目录
	if result.FileID != "" && result.SourceID > 0 && h.imageStore != nil {
		go h.imageStore.MoveToRoot(result.FileID, result.SourceID)
	}

	if format == "json" {
		c.JSON(http.StatusOK, result)
		return
	}

	if statusCode == http.StatusFound {
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	c.JSON(http.StatusOK, result)
}

// DetectURL POST /random/detect 探测数据源 URL 结构
// @Summary 探测数据源结构
// @Description 抓取目标 URL 并推断响应类型、headers 与可能的图片 URL 路径
// @Tags Distribution
// @Accept json
// @Produce json
// @Param body body DetectURLRequest true "目标 URL"
// @Success 200 {object} service.DetectResult
// @Failure 400 {object} map[string]interface{} "请求体无效"
// @Failure 500 {object} map[string]interface{} "探测失败"
// @Router /random/detect [post]
func (h *Handler) DetectURL(c *gin.Context) {
	var req DetectURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}
	result, err := service.DetectURL(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// BatchHealthCheck POST /api/sources/health-check 批量健康检查
// @Summary 批量健康检查
// @Description 对全部启用数据源执行一次健康检查并返回结果
// @Tags Health
// @Produce json
// @Success 200 {array} service.HealthResult
// @Router /api/sources/health-check [post]
func (h *Handler) BatchHealthCheck(c *gin.Context) {
	if h.healthChecker != nil {
		results := h.healthChecker.CheckAll()
		c.JSON(http.StatusOK, results)
		return
	}
	checker := service.NewHealthChecker(h.store)
	results := checker.CheckAll()
	c.JSON(http.StatusOK, results)
}

// GetHealthStatus GET /api/health 最近一次健康检查结果
// @Summary 健康检查结果
// @Description 返回最近一次健康检查的结果与运行时间；若首次检查未完成则异步触发
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{} "健康检查器不可用"
// @Router /api/health [get]
func (h *Handler) GetHealthStatus(c *gin.Context) {
	if h.healthChecker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "health checker not available"})
		return
	}
	results := h.healthChecker.GetLastResult()
	if results == nil {
		// 首次检查未完成: 异步触发, 立即返回空结果, 不阻塞请求
		results = []service.HealthResult{}
		go h.healthChecker.CheckAll()
	}
	c.JSON(http.StatusOK, gin.H{
		"results":  results,
		"last_run": h.healthChecker.LastRunAt(),
	})
}
