package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
)

func (h *Handler) GetTags(c *gin.Context) {
	tags, err := h.store.GetTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}

func (h *Handler) UpdateTags(c *gin.Context) {
	var tags []model.Tag
	if err := c.ShouldBindJSON(&tags); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.UpdateTags(tags); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}
