package api

import (
	friendhttp "IM_backend/internal/transport/http/friend"
	friendrequesthttp "IM_backend/internal/transport/http/friend_request"
	messagehttp "IM_backend/internal/transport/http/message"
	"IM_backend/internal/transport/http/middleware"
	roomhttp "IM_backend/internal/transport/http/room"
	userhttp "IM_backend/internal/transport/http/user"

	"github.com/gin-gonic/gin"
)

func RegisterUserRouter(r *gin.RouterGroup, uh *userhttp.UserHandle, auth *middleware.AuthMiddleware) {
	userGroup := r.Group("/users")
	userhttp.RegisterRoutes(userGroup, uh, auth)
}

func RegisterFriendRequestRouter(r *gin.RouterGroup, fh *friendrequesthttp.FriendRequestHandle, authMiddle *middleware.AuthMiddleware) {
	friendRequestGroup := r.Group("/friend-requests").Use(authMiddle.JWTAuthMiddleware())
	friendrequesthttp.RegisterRoutes(friendRequestGroup, fh)
}

func RegisterFriendRouter(r *gin.RouterGroup, fh *friendhttp.FriendHandle, authMiddle *middleware.AuthMiddleware) {
	friendGroup := r.Group("/friends").Use(authMiddle.JWTAuthMiddleware())
	friendhttp.RegisterRoutes(friendGroup, fh)
}

func RegisterRoomRouter(r *gin.RouterGroup, rh *roomhttp.RoomHandle, authMiddle *middleware.AuthMiddleware) {
	roomGroup := r.Group("/rooms").Use(authMiddle.JWTAuthMiddleware())
	roomhttp.RegisterRoutes(roomGroup, rh)
}

func RegisterMessagesRouter(r *gin.RouterGroup, mh *messagehttp.MessageHandle, authMiddle *middleware.AuthMiddleware) {
	messageGroup := r.Group("/messages").Use(authMiddle.JWTAuthMiddleware())
	messagehttp.RegisterRoutes(messageGroup, mh)
}
