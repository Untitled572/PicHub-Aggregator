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
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err == nil && id > 0 {
		if err := h.imageStore.SaveImage(id); err == nil {
			c.JSON(http.StatusOK, gin.H{"message": "image saved"})
			return
		}
	}

	img, err := h.store.GetImageByFileID(idParam)
	if err == nil && img != nil {
		if err := h.imageStore.SaveImage(img.ID); err == nil {
			c.JSON(http.StatusOK, gin.H{"message": "image saved"})
			return
		}
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "image not found or unable to save"})
}

func (h *Handler) LikeImage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	img, err := h.store.GetImageByID(id)
	if err != nil || img == nil || img.SourceID <= 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}

	h.store.IncrementSourceWeight(img.SourceID, 1)
	c.JSON(http.StatusOK, gin.H{"message": "liked"})
}

func (h *Handler) DislikeImage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	img, err := h.store.GetImageByID(id)
	if err != nil || img == nil || img.SourceID <= 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}

	h.store.IncrementSourceWeight(img.SourceID, -1)
	c.JSON(http.StatusOK, gin.H{"message": "disliked"})
}

func (h *Handler) UnsaveImage(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err == nil && id > 0 {
		if err := h.imageStore.UnsaveImage(id); err == nil {
			c.JSON(http.StatusOK, gin.H{"message": "image unsaved"})
			return
		}
	}

	img, err := h.store.GetImageByFileID(idParam)
	if err == nil && img != nil {
		if err := h.imageStore.UnsaveImage(img.ID); err == nil {
			c.JSON(http.StatusOK, gin.H{"message": "image unsaved"})
			return
		}
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "image not found or unable to unsave"})
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
