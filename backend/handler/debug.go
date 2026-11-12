package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// @Summary 读取临时图源调试状态
// @Tags Debug
// @Security BearerAuth
// @Produce json
// @Success 200 {object} service.DebugSourceState
// @Router /api/debug/source [get]
func (h *Handler) GetDebugSource(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h.engine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "取图服务不可用"})
		return
	}
	c.JSON(http.StatusOK, h.engine.DebugSource())
}

// @Summary 开启临时图源调试
// @Tags Debug
// @Security BearerAuth
// @Produce json
// @Success 200 {object} service.DebugSourceState
// @Accept json
// @Param body body DebugSourceRequest true "图源 ID"
// @Router /api/debug/source [put]
func (h *Handler) SetDebugSource(c *gin.Context) {
	var body DebugSourceRequest
	if err := c.ShouldBindJSON(&body); err != nil || body.SourceID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择有效的图源"})
		return
	}
	if h.engine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "取图服务不可用"})
		return
	}
	state, err := h.engine.SetDebugSource(body.SourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, state)
}

// @Summary 退出临时图源调试
// @Tags Debug
// @Security BearerAuth
// @Produce json
// @Success 200 {object} service.DebugSourceState
// @Router /api/debug/source [delete]
func (h *Handler) ClearDebugSource(c *gin.Context) {
	if h.engine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "取图服务不可用"})
		return
	}
	h.engine.ClearDebugSource()
	c.JSON(http.StatusOK, h.engine.DebugSource())
}

type DebugSourceRequest struct {
	SourceID int64 `json:"source_id"`
}
