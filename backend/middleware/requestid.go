package middleware

import (
	"crypto/rand"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/common/ctxkey"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		b := make([]byte, 4)
		rand.Read(b)
		id := fmt.Sprintf("%x", b)
		c.Set(string(ctxkey.RequestID), id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}
