package apis

import (
	https_friend_request "IM_backend/internal/apis/https/friend_request"
	"IM_backend/internal/apis/https/middleware"
	https_user "IM_backend/internal/apis/https/user"

	"github.com/gin-gonic/gin"
)

func RegisterUserRouter(r *gin.RouterGroup, uh *https_user.UserHandler) {
	ug := r.Group("/user")
	https_user.AddUserRouter(ug, uh)
}

func RegisterFriendRouter(r *gin.RouterGroup, fh *https_friend_request.FriendHandle, authMiddle *middleware.AuthMiddleware) {
	fg := r.Group("/friend/request").Use(authMiddle.JWTAuthMiddleware())
	https_friend_request.AddFriendRouter(fg, fh)
}
