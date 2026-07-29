package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ServeImage(c *gin.Context) {
	fileID := c.Param("file_id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_id required"})
		return
	}

	filePath, contentType, err := h.imageStore.GetImage(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(filePath)
}

func (h *Handler) SaveImage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.imageStore.SaveImage(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "image saved"})
}

func (h *Handler) UnsaveImage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.imageStore.UnsaveImage(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "image unsaved"})
}

func (h *Handler) ListSavedImages(c *gin.Context) {
	limit := 20
	offset := 0
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	if o, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && o >= 0 {
		offset = o
	}

	images, total, err := h.store.ListSavedImages(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"images": images,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
