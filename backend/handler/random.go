package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/pichub/backend/common/ctxkey"
	"github.com/pichub/backend/logger"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/service"
	"github.com/pichub/backend/store"
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
	c.Header("Cache-Control", "no-store")
	format := c.Query("format")
	orientation := c.Query("orientation")
	clientUA := c.GetHeader("User-Agent")

	ctx := c.Request.Context()
	if c.GetHeader("X-PicHub-Debug-Image") == "true" {
		ctx = service.WithDebugImageResponse(ctx)
	}
	result, statusCode, err := h.engine.RandomImageContext(ctx, category, format, orientation, clientUA)
	if err != nil {
		logger.Error("[%s] distribution failed: %v", c.GetString(string(ctxkey.RequestID)), err)
		if errors.Is(err, service.ErrDistributionModeChanged) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "code": "distribution_mode_changed"})
			return
		}
		var debugErr *service.DebugFetchError
		if errors.As(err, &debugErr) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "code": "debug_distribution_failed"})
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "上游取图超时", "code": "distribution_timeout"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	redirectURL := result.URL
	if result.LocalURL != "" {
		redirectURL = result.LocalURL
	}

	if result.Debug {
		c.Header("Cache-Control", "no-store")
		c.Header("X-PicHub-Debug", "true")
		if len(result.ImageData) > 0 {
			if c.GetHeader("X-PicHub-Debug-Image") == "true" {
				metadata, err := json.Marshal(gin.H{"source": result.SourceName, "categories": result.Categories})
				if err == nil && len(metadata) <= 4096 {
					c.Header("X-PicHub-Debug-Metadata", base64.RawURLEncoding.EncodeToString(metadata))
				}
			}
			c.Data(http.StatusOK, result.ContentType, result.ImageData)
			return
		}
	}

	if format == "json" {
		vote, supported := h.feedbackForResult(result)
		c.JSON(http.StatusOK, struct {
			*service.Result
			Feedback          string `json:"feedback"`
			FeedbackSupported bool   `json:"feedback_supported"`
		}{result, store.FeedbackValue(vote), supported})
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
// @Security BearerAuth
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
// @Security BearerAuth
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

// SourceHealthCheck checks only the selected source, including its branches.
// @Summary 单个图源健康检查
// @Description 仅检测指定图源的主地址和配置分支，更新该图源结果
// @Tags Health
// @Produce json
// @Param id path int true "图源 ID"
// @Success 200 {object} service.HealthResult
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security BearerAuth
// @Router /api/sources/{id}/health-check [post]
func (h *Handler) SourceHealthCheck(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	src, err := h.store.GetSource(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}
	if h.healthChecker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "health checker not available"})
		return
	}
	c.JSON(http.StatusOK, h.healthChecker.CheckOne(*src))
}

// GetHealthStatus GET /api/health 最近一次健康检查结果
// @Summary 健康检查结果
// @Description 返回最近一次健康检查的结果与运行时间；若首次检查未完成则异步触发
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{} "健康检查器不可用"
// @Security BearerAuth
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
