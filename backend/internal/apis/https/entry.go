package apis

import (
	https_user "IM_backend/internal/apis/https/user"

	"github.com/gin-gonic/gin"
)

func RegisterUserRouter(r *gin.Engine, uh *https_user.UserHandler) {
	r.Group("/api/v1")

	ug := r.Group("/user")
	https_user.AddUserRouter(ug, uh)
}
