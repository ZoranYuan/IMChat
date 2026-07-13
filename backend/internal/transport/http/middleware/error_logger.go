package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func ErrorLoggerMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()

		ctx.Next()

		status := ctx.Writer.Status()
		elapsed := time.Since(start)

		if len(ctx.Errors) > 0 || status >= 500 {
			errText := ctx.Errors.String()
			if errText == "" {
				if err := ctx.Err(); err != nil {
					errText = err.Error()
				}
			}
			if errText == "" {
				errText = "n/a"
			}
			log.Printf("[ERROR] %d | %s %s | %v | %v",
				status,
				ctx.Request.Method,
				ctx.Request.URL.Path,
				elapsed,
				errText,
			)
			return
		}

		if status >= 400 || elapsed >= 500*time.Millisecond {
			log.Printf("[WARN] %d | %s %s | %v",
				status,
				ctx.Request.Method,
				ctx.Request.URL.Path,
				elapsed,
			)
		}
	}
}
