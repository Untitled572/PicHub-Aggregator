package handler

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
)

// ListSources GET /api/sources 数据源列表
// @Summary 数据源列表
// @Description 返回全部数据源
// @Tags Sources
// @Produce json
// @Success 200 {array} model.Source
// @Failure 500 {object} map[string]interface{}
// @Router /api/sources [get]
func (h *Handler) ListSources(c *gin.Context) {
	sources, err := h.store.ListSources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sources)
}

// GetSource GET /api/sources/:id 单个数据源
// @Summary 单个数据源
// @Tags Sources
// @Produce json
// @Param id path int true "数据源 ID"
// @Success 200 {object} model.Source
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/sources/{id} [get]
func (h *Handler) GetSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	src, err := h.store.GetSource(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}
	c.JSON(http.StatusOK, src)
}

// CreateSource POST /api/sources 新建数据源
// @Summary 新建数据源
// @Tags Sources
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body model.Source true "数据源配置"
// @Success 201 {object} model.Source
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/sources [post]
func (h *Handler) CreateSource(c *gin.Context) {
	var src model.Source
	if err := c.ShouldBindJSON(&src); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
	if src.Status == "" {
		src.Status = "normal"
	}
	if src.SuccessRate <= 0 {
		src.SuccessRate = 100.0
	}
	src.Enabled = true

	id, err := h.store.CreateSource(&src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	src.ID = id
	if h.healthChecker != nil {
		go h.healthChecker.CheckAll()
	}
	c.JSON(http.StatusCreated, src)
}

func parseDefaultName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "Custom Source"
	}
	return u.Host + " Source"
}

// UpdateSource PUT /api/sources/:id 更新数据源
// @Summary 更新数据源
// @Tags Sources
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "数据源 ID"
// @Param body body model.Source true "数据源配置"
// @Success 200 {object} model.Source
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/sources/{id} [put]
func (h *Handler) UpdateSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var src model.Source
	if err := c.ShouldBindJSON(&src); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	src.ID = id
	if src.Weight <= 0 {
		src.Weight = 10
	}
	if src.Weight > 100 {
		src.Weight = 100
	}
	if src.Name == "" {
		src.Name = parseDefaultName(src.URL)
	}
	if err := h.store.UpdateSource(&src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, src)
}

// DeleteSource DELETE /api/sources/:id 删除数据源
// @Summary 删除数据源
// @Tags Sources
// @Security BearerAuth
// @Param id path int true "数据源 ID"
// @Success 204 "已删除"
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/sources/{id} [delete]
func (h *Handler) DeleteSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if h.imageStore != nil {
		go h.imageStore.DeleteSourceFolder(id)
	}
	if h.engine != nil {
		h.engine.RemoveSourceFromPool(id)
	}
	if err := h.store.DeleteSource(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// ToggleSource POST /api/sources/:id/toggle 启停数据源
// @Summary 启停数据源
// @Tags Sources
// @Security BearerAuth
// @Param id path int true "数据源 ID"
// @Success 200 {object} model.Source
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/sources/{id}/toggle [post]
func (h *Handler) ToggleSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	src, err := h.store.GetSource(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}
	src.Enabled = !src.Enabled
	if err := h.store.UpdateSource(src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !src.Enabled {
		if h.engine != nil {
			h.engine.RemoveSourceFromPool(src.ID)
		}
		if h.imageStore != nil {
			go h.imageStore.ClearSourceImages(src.ID)
		}
	}
	c.JSON(http.StatusOK, src)
}
