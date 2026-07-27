package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
)

func (h *Handler) ExportRules(c *gin.Context) {
	sources, err := h.store.ListSources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var sanitized []gin.H
	for _, src := range sources {
		s := gin.H{
			"name":        src.Name,
			"url":         src.URL,
			"resp_type":   src.RespType,
			"json_path":   src.JsonPath,
			"weight":      src.Weight,
			"categories":  src.Categories,
			"enabled":     src.Enabled,
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
	var body struct {
		Sources []struct {
			Name       string            `json:"name"`
			URL        string            `json:"url"`
			RespType   string            `json:"resp_type"`
			JsonPath   string            `json:"json_path"`
			Weight     int               `json:"weight"`
			Categories []string          `json:"categories"`
			Headers    map[string]string `json:"headers"`
			Enabled    bool              `json:"enabled"`
		} `json:"sources"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var imported int
	for _, s := range body.Sources {
		src := model.Source{
			Name:       s.Name,
			URL:        s.URL,
			RespType:   s.RespType,
			JsonPath:   s.JsonPath,
			Weight:     s.Weight,
			Categories: s.Categories,
			Headers:    s.Headers,
			Enabled:    s.Enabled,
		}
		if src.Weight <= 0 {
			src.Weight = 3
		}
		if src.Weight > 5 {
			src.Weight = 5
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
	sensitive := []string{"api-key", "authorization", "token", "secret", "cookie"}
	lower := key
	for _, s := range sensitive {
		if lower == s {
			return true
		}
	}
	return false
}
