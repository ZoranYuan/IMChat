package user

import (
	shared_ratelimit "IM_backend/internal/shared/ratelimit"
	"IM_backend/internal/transport/http/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(
	ug *gin.RouterGroup,
	uh *UserHandle,
	auth *middleware.AuthMiddleware,
	limiter *middleware.LimitMiddleware,
	userAPIPolicy shared_ratelimit.Policy,
) {
	if userAPIPolicy.Rate <= 0 || userAPIPolicy.Burst <= 0 {
		userAPIPolicy = shared_ratelimit.Policy{Rate: 2.0 / 60.0, Burst: 5}
	}

	ug.POST("/login", limiter.ByIP("login", userAPIPolicy), uh.Login)
	ug.POST("/register", limiter.ByIP("register", userAPIPolicy), uh.Register)
	ug.POST("/refresh", limiter.ByIP("refresh", userAPIPolicy), uh.Refresh)

	ug.Use(auth.JWTAuthMiddleware())
	ug.PATCH("/me", limiter.ByIP("user_profile_update", userAPIPolicy), uh.UpdateUserProfile)
	ug.GET("/resolve", uh.FindUserByPhoneAndUserName)
	ug.POST("/logout", uh.Logout)
}
