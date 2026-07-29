package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/store"
)

func AdminAuth(st *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := st.GetSettings()
		if err != nil || settings.AdminToken == "" {
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
