package handler

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
)

func (h *Handler) ListSources(c *gin.Context) {
	sources, err := h.store.ListSources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sources)
}

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
