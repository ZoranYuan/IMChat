package apis

import (
	https_friend "IM_backend/internal/apis/https/friend"
	https_friend_request "IM_backend/internal/apis/https/friend_request"
	https_message "IM_backend/internal/apis/https/message"
	"IM_backend/internal/apis/https/middleware"
	https_room "IM_backend/internal/apis/https/room"
	https_user "IM_backend/internal/apis/https/user"

	"github.com/gin-gonic/gin"
)

func RegisterUserRouter(r *gin.RouterGroup, uh *https_user.UserHandle, auth *middleware.AuthMiddleware) {
	ug := r.Group("/user")
	https_user.AddUserRouter(ug, uh, auth)
}

func RegisterFriendRequestRouter(r *gin.RouterGroup, fh *https_friend_request.FriendRequestHandle, authMiddle *middleware.AuthMiddleware) {
	fg := r.Group("/friend/request").Use(authMiddle.JWTAuthMiddleware())
	https_friend_request.AddFriendRequstRouter(fg, fh)
}

func RegisterFriendRouter(r *gin.RouterGroup, fh *https_friend.FriendHandle, authMiddle *middleware.AuthMiddleware) {
	fr := r.Group("/friend").Use(authMiddle.JWTAuthMiddleware())
	https_friend.AddFriendRouter(fr, fh)
}

func RegisterRoomRouter(r *gin.RouterGroup, rh *https_room.RoomHandle, authMiddle *middleware.AuthMiddleware) {
	fr := r.Group("/room").Use(authMiddle.JWTAuthMiddleware())
	https_room.AddRoomRouter(fr, rh)
}

func RegisterMessagesRouter(r *gin.RouterGroup, mh *https_message.MessageHandle, authMiddle *middleware.AuthMiddleware) {
	mr := r.Group("/messages").Use(authMiddle.JWTAuthMiddleware())
	https_message.AddMessageRouter(mr, mh)
}
