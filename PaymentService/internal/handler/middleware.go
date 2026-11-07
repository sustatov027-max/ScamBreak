package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireIdempotencyKey проверка ключа идемпотентности в запросе
func RequireIdempotencyKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" || len(key) > 64 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "заголовок Idempotency-Key обязателен и не должен превышать 64 символа",
			})
			return
		}

		c.Set("idempotency_key", key)
		c.Next()
	}
}
