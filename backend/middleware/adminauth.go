package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/store"
)

// AdminAuth 双通道鉴权:
//   - LoginEnabled 时: 校验 session token (登录产生) 或旧版 admin_token (兼容), 均无效则 401
//   - 未启用登录: 维持现状 (有 admin_token 则校验, 无配置则放行)
func AdminAuth(st *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := st.GetSettings()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load settings"})
			return
		}

		if settings.LoginEnabled {
			// 首次初始化: 登录已启用但未配置账号密码 → 放行写操作 (允许设置凭据)
			if settings.AdminUsername == "" || settings.AdminPasswordHash == "" {
				c.Next()
				return
			}
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token := auth[7:]
				if st.Sessions().Validate(token) {
					c.Next()
					return
				}
				if token != "" && settings.AdminToken != "" && token == settings.AdminToken {
					c.Next()
					return
				}
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		if settings.AdminToken == "" {
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || auth[7:] != settings.AdminToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
