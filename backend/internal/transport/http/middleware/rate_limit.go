package middleware

import (
	shared_ratelimit "IM_backend/internal/shared/ratelimit"
	"IM_backend/internal/transport/http/response"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type LimitMiddleware struct {
	limiter  shared_ratelimit.Limit
	failOpen bool
}

func NewLimitMiddleware(limiter shared_ratelimit.Limit, failOpen bool) *LimitMiddleware {
	return &LimitMiddleware{
		limiter:  limiter,
		failOpen: failOpen,
	}
}

func (m *LimitMiddleware) ByIP(scope string, policy shared_ratelimit.Policy) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ip := ctx.ClientIP()

		key := fmt.Sprintf(
			"http:%s:ip:%s",
			scope,
			ip,
		)

		decision, err := m.limiter.Allow(
			ctx.Request.Context(),
			key,
			policy,
		)

		if err != nil {
			// 限流器不可用
			log.Printf(
				"限流器执行失败，key=%s：%v",
				key,
				err,
			)

			if m.failOpen {
				ctx.Next()
				return
			}

			// 降级处理
			ctx.AbortWithStatusJSON(http.StatusServiceUnavailable,
				response.Error(
					http.StatusServiceUnavailable,
					"服务暂时不可用",
				))

			return
		}

		if decision.Allowed {
			ctx.Next()
			return
		}

		retrySeconds := int64(math.Ceil(decision.RetryAfter.Seconds()))

		ctx.Header(
			"Retry-After",
			strconv.FormatInt(
				retrySeconds,
				10,
			),
		)

		ctx.AbortWithStatusJSON(http.StatusTooManyRequests,
			response.Error(
				http.StatusTooManyRequests,
				"请求过于频繁，请稍后重试",
			))
	}
}
