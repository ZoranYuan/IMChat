package apis

import (
	https_friend "IM_backend/internal/apis/https/friend"
	https_user "IM_backend/internal/apis/https/user"

	"github.com/gin-gonic/gin"
)

func RegisterUserRouter(r *gin.RouterGroup, uh *https_user.UserHandler) {
	ug := r.Group("/user")
	https_user.AddUserRouter(ug, uh)
}

func RegisterFriendRouter(r *gin.RouterGroup, fh *https_friend.FriendHandle) {
	fg := r.Group("/friend/request")
	https_friend.AddFriendRouter(fg, fh)
}
