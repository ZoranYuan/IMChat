package https_user

import (
	"github.com/gin-gonic/gin"
)

func AddUserRouter(ug *gin.RouterGroup, uh *UserHandler) {
	ug.POST("/login", uh.Login)
}
