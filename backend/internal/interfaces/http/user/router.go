package https_user

import (
	"IM_backend/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func AddUserRouter(ug *gin.RouterGroup, uh *UserHandle, auth *middleware.AuthMiddleware) {
	ug.POST("/login", uh.Login)
	ug.POST("/register", uh.Register)

	ug.Use(auth.JWTAuthMiddleware())
	ug.GET("/:userId", uh.GetUserByID)
	ug.POST("/logout", uh.Logout)
}
