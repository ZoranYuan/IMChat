package apis

import (
	https_user "IM_backend/internal/apis/https/user"

	"github.com/gin-gonic/gin"
)

func RegisterUserRouter(r *gin.Engine, uh *https_user.UserHandler) {
	apiGroup := r.Group("/api/v1")

	ug := apiGroup.Group("/user")
	https_user.AddUserRouter(ug, uh)
}
