package user

import (
	shared_ratelimit "IM_backend/internal/shared/ratelimit"
	"IM_backend/internal/transport/http/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(ug *gin.RouterGroup, uh *UserHandle, auth *middleware.AuthMiddleware, limter *middleware.LimitMiddleware) {
	limiterPolicy := shared_ratelimit.Policy{
		// 每分钟补充 10 个令牌。
		Rate:  2.0 / 60.0,
		Burst: 5,
	}

	ug.POST("/login", limter.ByIP(
		"login",
		limiterPolicy,
	), uh.Login)
	ug.POST("/register", limter.ByIP(
		"register",
		limiterPolicy,
	), uh.Register)
	ug.POST("/refresh", limter.ByIP("refresh", limiterPolicy), uh.Refresh)

	ug.Use(auth.JWTAuthMiddleware())
	ug.GET("/resolve", uh.ResolveUser)
	ug.GET("/:userId", uh.GetUserByID)
	ug.POST("/logout", uh.Logout)
}
