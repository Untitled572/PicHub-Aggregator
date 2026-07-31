package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck GET /ping 存活探针
// @Summary 存活探针
// @Description 返回服务是否存活
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /ping [get]
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
