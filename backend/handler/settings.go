package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
)

func (h *Handler) GetSettings(c *gin.Context) {
	settings, err := h.store.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var settings model.Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检测 bound_tags 是否变更，变更后立即补充分发池
	oldSettings, _ := h.store.GetSettings()
	tagsChanged := oldSettings != nil && !stringSliceEqual(oldSettings.BoundTags, settings.BoundTags)

	if err := h.store.UpdateSettings(&settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.proxyConfig != nil {
		h.proxyConfig.Update(settings.ProxyEnabled, settings.ProxyURL)
	}
	if tagsChanged && h.engine != nil {
		go h.engine.ReplenishPool()
	}
	c.JSON(http.StatusOK, settings)
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
