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

		end := time.Since(start)
		if status > 400 {
			log.Printf("[WARN] %d | %s %s | %v\n",
				status,
				ctx.Request.Method,
				ctx.Request.URL.Path,
				end,
			)
		} else if len(ctx.Errors) > 0 {
			log.Printf("[ERROR] %d | %s %s | %v | %v\n",
				status,
				ctx.Request.Method,
				ctx.Request.URL.Path,
				end,
				ctx.Err(),
			)
		}
	}
}
