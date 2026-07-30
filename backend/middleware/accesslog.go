package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/logger"
)

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start).Milliseconds()
		logger.WithCtx(c, "ACCESS", "%d %s %s %dms %s",
			c.Writer.Status(),
			c.Request.Method,
			c.Request.URL.Path,
			latency,
			c.ClientIP(),
		)
	}
}
