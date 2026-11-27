package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/store"
)

func (h *Handler) requireCuratorSettingsAuth(c *gin.Context) bool {
	settings, err := h.store.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取鉴权设置"})
		return false
	}
	// AdminAuth permits open installation and open-mode management.
	if settings.AdminToken == "" && (!settings.LoginEnabled || settings.AdminUsername == "" || settings.AdminPasswordHash == "") {
		c.JSON(http.StatusForbidden, gin.H{"error": "请先配置 PicHub 登录保护或管理令牌"})
		return false
	}
	return true
}

func curatorSettingsResponse(c *gin.Context, cfg store.CuratorFeedbackConfig) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"enabled": cfg.Enabled, "address": cfg.Address, "token_configured": cfg.Token != ""})
}

func (h *Handler) GetCuratorFeedbackSettings(c *gin.Context) {
	if !h.requireCuratorSettingsAuth(c) {
		return
	}
	cfg, err := h.store.GetCuratorFeedbackConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取 Curator 联动设置"})
		return
	}
	curatorSettingsResponse(c, cfg)
}

func (h *Handler) UpdateCuratorFeedbackSettings(c *gin.Context) {
	if !h.requireCuratorSettingsAuth(c) {
		return
	}
	var input struct {
		Enabled    bool   `json:"enabled"`
		Address    string `json:"address"`
		Token      string `json:"token"`
		ClearToken bool   `json:"clear_token"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32768)
	if err := c.ShouldBindJSON(&input); err != nil || (input.ClearToken && strings.TrimSpace(input.Token) != "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Curator 联动设置格式无效"})
		return
	}
	var token *string
	if input.ClearToken {
		empty := ""
		token = &empty
	} else if strings.TrimSpace(input.Token) != "" {
		token = &input.Token
	}
	cfg, err := h.store.UpdateCuratorFeedbackConfig(input.Enabled, input.Address, token)
	switch {
	case errors.Is(err, store.ErrInvalidCuratorFeedbackAddress):
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写 Curator 的 HTTPS 服务地址，地址只需包含主机和端口"})
	case errors.Is(err, store.ErrInvalidCuratorFeedbackToken):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Curator 管理令牌格式无效"})
	case errors.Is(err, store.ErrMissingCuratorFeedbackConfig):
		c.JSON(http.StatusBadRequest, gin.H{"error": "开启联动需要填写 Curator 地址和对应的管理令牌"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法保存 Curator 联动设置"})
	default:
		curatorSettingsResponse(c, cfg)
	}
}

func (h *Handler) TestCuratorFeedbackSettings(c *gin.Context) {
	if !h.requireCuratorSettingsAuth(c) {
		return
	}
	c.Header("Cache-Control", "no-store")
	var input struct {
		Address string  `json:"address"`
		Token   *string `json:"token"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32768)
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Curator 联动设置格式无效"})
		return
	}
	cfg, err := h.store.CuratorFeedbackConnection(input.Address, input.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写 Curator 的 HTTPS 地址和对应管理令牌"})
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, cfg.Address+"/admin/api/settings/preferences", nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Curator 地址无效"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	resp, err := curatorFeedbackHTTP.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "无法连接 Curator，请检查地址或网络"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Curator 拒绝鉴权，请检查管理令牌"})
		return
	}
	var reply struct {
		Rules   map[string]any `json:"rules"`
		Version *int64         `json:"version"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&reply) != nil ||
		reply.Rules == nil || reply.Version == nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Curator 未返回预期的设置响应，请检查服务地址和版本"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
