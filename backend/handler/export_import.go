package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
)

var sensitiveHeaders = []string{"api-key", "authorization", "token", "secret", "cookie"}

// ExportRules POST /api/export 导出规则
// @Summary 导出规则
// @Description 导出数据源规则，自动剔除敏感 headers
// @Tags Export
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/export [post]
func (h *Handler) ExportRules(c *gin.Context) {
	sources, err := h.store.ListSources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var sanitized []gin.H
	for _, src := range sources {
		s := gin.H{
			"name":          src.Name,
			"url":           src.URL,
			"resp_type":     src.RespType,
			"json_path":     src.JsonPath,
			"weight":        src.Weight,
			"categories":    src.Categories,
			"enabled":       src.Enabled,
			"default_query": src.DefaultQuery,
			"params":        src.Params,
		}
		headers := make(map[string]string)
		for k, v := range src.Headers {
			if isSensitive(k) {
				continue
			}
			headers[k] = v
		}
		s["headers"] = headers
		sanitized = append(sanitized, s)
	}
	c.JSON(http.StatusOK, gin.H{"sources": sanitized})
}

func (h *Handler) ImportRules(c *gin.Context) {
	var body ImportRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var imported int
	for _, s := range body.Sources {
		src := model.Source{
			Name:         s.Name,
			URL:          s.URL,
			RespType:     s.RespType,
			JsonPath:     s.JsonPath,
			Weight:       s.Weight,
			Categories:   s.Categories,
			Headers:      s.Headers,
			Enabled:      s.Enabled,
			DefaultQuery: s.DefaultQuery,
			Params:       s.Params,
		}
		if src.Weight <= 0 {
			src.Weight = 10
		}
		if src.Weight > 100 {
			src.Weight = 100
		}
		if src.Name == "" {
			src.Name = parseDefaultName(src.URL)
		}
		if src.Headers == nil {
			src.Headers = make(map[string]string)
		}
		if src.Categories == nil {
			src.Categories = []string{}
		}
		if _, err := h.store.CreateSource(&src); err != nil {
			continue
		}
		imported++
	}
	c.JSON(http.StatusOK, gin.H{"imported": imported})
}

func isSensitive(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range sensitiveHeaders {
		if lower == s {
			return true
		}
	}
	return false
}
