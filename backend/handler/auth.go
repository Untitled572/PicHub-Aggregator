package handler

import (
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"` // 前端 MD5 后的摘要
}

// Login POST /api/login 用户名+MD5密码校验, 成功后签发 session token
// @Summary 登录
// @Description 用户名 + MD5 后的密码摘要校验，成功后签发 session token
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "登录凭据"
// @Success 200 {object} map[string]interface{} "token"
// @Failure 400 {object} map[string]interface{} "请求体无效"
// @Failure 403 {object} map[string]interface{} "未启用登录或未配置"
// @Failure 401 {object} map[string]interface{} "用户名或密码错误"
// @Router /api/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load settings"})
		return
	}
	if !settings.LoginEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "login is not enabled"})
		return
	}
	if settings.AdminUsername == "" || settings.AdminPasswordHash == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "login is not configured"})
		return
	}
	if req.Username != settings.AdminUsername || req.Password != settings.AdminPasswordHash {
		// 防爆破: 失败随机延迟 500~2000ms
		time.Sleep(time.Duration(500+rand.IntN(1500)) * time.Millisecond)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	token := h.store.Sessions().Create(time.Duration(settings.SessionHours) * time.Hour)
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// Logout POST /api/logout 使当前 token 失效 (幂等)
// @Summary 登出
// @Description 使当前 Bearer token 失效（幂等）
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		h.store.Sessions().Delete(auth[7:])
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// CheckAuth GET /api/auth/check 公开接口: 校验当前 Bearer token 是否仍有效
// (服务重启后内存会话清空, 前端据此将残留 token 清空并跳登录页)
// @Summary 校验登录态
// @Description 校验当前 Bearer token 是否仍有效
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/auth/check [get]
func (h *Handler) CheckAuth(c *gin.Context) {
	settings, err := h.store.GetSettings()
	if err != nil || settings == nil || !settings.LoginEnabled {
		c.JSON(http.StatusOK, gin.H{"valid": true, "login_enabled": false})
		return
	}
	valid := false
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := auth[7:]
		valid = h.store.Sessions().Validate(token) ||
			(settings.AdminToken != "" && token == settings.AdminToken)
	}
	c.JSON(http.StatusOK, gin.H{"valid": valid, "login_enabled": true})
}
