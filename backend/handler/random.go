package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/service"
)

func (h *Handler) RandomImage(c *gin.Context) {
	category := c.Query("category")
	format := c.Query("format")
	orientation := c.Query("orientation")
	clientUA := c.GetHeader("User-Agent")

	if category == "" {
		settings, err := h.store.GetSettings()
		if err == nil && len(settings.BoundTags) > 0 {
			category = strings.Join(settings.BoundTags, ",")
		}
	}

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

func (h *Handler) DetectURL(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
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

func (h *Handler) GetHealthStatus(c *gin.Context) {
	if h.healthChecker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "health checker not available"})
		return
	}
	results := h.healthChecker.GetLastResult()
	if results == nil {
		results = h.healthChecker.CheckAll()
	}
	c.JSON(http.StatusOK, gin.H{
		"results":   results,
		"last_run":  h.healthChecker.LastRunAt(),
	})
}
