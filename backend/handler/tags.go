package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
)

// GetTags GET /api/tags 标签列表
// @Summary 标签列表
// @Tags Tags
// @Produce json
// @Success 200 {array} model.Tag
// @Failure 500 {object} map[string]interface{}
// @Router /api/tags [get]
func (h *Handler) GetTags(c *gin.Context) {
	tags, err := h.store.GetTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}

// UpdateTags PUT /api/tags 更新标签
// @Summary 更新标签
// @Tags Tags
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body []model.Tag true "完整标签列表"
// @Success 200 {array} model.Tag
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/tags [put]
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
