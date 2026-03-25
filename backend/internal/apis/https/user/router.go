package https_user

import (
	"github.com/gin-gonic/gin"
)

func AddUserRouter(ug *gin.RouterGroup, uh *UserHandler) {
	ug.GET("/:userId", uh.GetUserByUserId)
	ug.POST("/login", uh.Login)
	ug.POST("/logout", uh.Logout)
	ug.POST("/register", uh.Register)
}
