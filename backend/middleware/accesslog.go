package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/logger"
)

// 跳过静态资源与前端轮询接口, 避免访问日志刷屏
func shouldSkip(c *gin.Context) bool {
	p := c.Request.URL.Path
	if p == "/" || p == "/favicon.ico" || strings.HasPrefix(p, "/assets/") {
		return true
	}
	// 前端健康轮询接口 (App.vue 每 8 秒轮询)
	if c.Request.Method == "GET" && (p == "/api/settings" || p == "/ping") {
		return true
	}
	for _, ext := range []string{".js", ".css", ".png", ".svg", ".ico", ".woff2", ".map", ".jpg", ".jpeg", ".gif", ".webp", ".webmanifest"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if shouldSkip(c) {
			return
		}
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
