package handler

import (
	"encoding/json"
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

	// 未提交新密码时保留旧哈希 (前端密码框留空=不修改)
	credentialsChanged := false
	if oldSettings != nil {
		if settings.AdminPassword == "" {
			settings.AdminPasswordHash = oldSettings.AdminPasswordHash
		} else {
			settings.AdminPasswordHash = settings.AdminPassword
			credentialsChanged = true
		}
		// admin_token 不回显: 留空则保持现有令牌, 避免保存设置时误清空
		if settings.AdminToken == "" {
			settings.AdminToken = oldSettings.AdminToken
		}
		if settings.AdminUsername != oldSettings.AdminUsername {
			credentialsChanged = true
		}
		if settings.LoginEnabled != oldSettings.LoginEnabled {
			credentialsChanged = true
		}
	}

	if err := h.store.UpdateSettings(&settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if credentialsChanged {
		// 凭据变更/登录开关切换: 全量踢下线
		h.store.Sessions().Clear()
	}
	settings.AdminPassword = ""
	if h.proxyConfig != nil {
		h.proxyConfig.Update(settings.ProxyEnabled, settings.ProxyURL)
	}
	if tagsChanged && h.engine != nil {
		go h.engine.ReplenishPool()
	}
	c.JSON(http.StatusOK, settingsResponseWithToken(&settings))
}

func settingsResponseWithToken(s *model.Settings) map[string]interface{} {
	b, _ := json.Marshal(s)
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		m = gin.H{}
	}
	// admin_token 仅对已鉴权的写请求返回 (GET /api/settings 公开, 不回显)
	m["admin_token"] = s.AdminToken
	return m
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
