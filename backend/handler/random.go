package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/service"
)

func (h *Handler) RandomImage(c *gin.Context) {
	category := c.Query("category")
	format := c.Query("format")

	engine := service.NewEngine(h.store)
	result, statusCode, err := engine.RandomImage(category, format)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	if format == "json" {
		c.JSON(http.StatusOK, result)
		return
	}

	if statusCode == http.StatusFound {
		c.Redirect(http.StatusFound, result.URL)
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
	checker := service.NewHealthChecker(h.store)
	results := checker.CheckAll()
	c.JSON(http.StatusOK, results)
}
