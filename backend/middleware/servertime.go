package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ServerTime 在响应头附加服务器时间戳, 供前端做客户端/服务端时差漂移检测
func ServerTime() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Server-Time", strconv.FormatInt(time.Now().UnixMilli(), 10))
		c.Next()
	}
}
