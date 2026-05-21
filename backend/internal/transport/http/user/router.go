package user

import (
	"IM_backend/internal/transport/http/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(ug *gin.RouterGroup, uh *UserHandle, auth *middleware.AuthMiddleware) {
	ug.POST("/login", uh.Login)
	ug.POST("/register", uh.Register)

	ug.Use(auth.JWTAuthMiddleware())
	ug.GET("/resolve", uh.ResolveUser)
	ug.GET("/:userId", uh.GetUserByID)
	ug.POST("/logout", uh.Logout)
}
