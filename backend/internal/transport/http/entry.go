package api

import (
	shared_ratelimit "IM_backend/internal/shared/ratelimit"
	agenthttp "IM_backend/internal/transport/http/agent"
	conversationhttp "IM_backend/internal/transport/http/conversation"
	filehttp "IM_backend/internal/transport/http/file"
	friendhttp "IM_backend/internal/transport/http/friend"
	friendrequesthttp "IM_backend/internal/transport/http/friend_request"
	messagehttp "IM_backend/internal/transport/http/message"
	"IM_backend/internal/transport/http/middleware"
	roomhttp "IM_backend/internal/transport/http/room"
	userhttp "IM_backend/internal/transport/http/user"

	"github.com/gin-gonic/gin"
)

func RegisterUserRouter(r *gin.RouterGroup, uh *userhttp.UserHandle, auth *middleware.AuthMiddleware, limter *middleware.LimitMiddleware) {
	userGroup := r.Group("/users")
	userhttp.RegisterRoutes(userGroup, uh, auth, limter)
}

func RegisterUserConversationRouter(r *gin.RouterGroup, ch *conversationhttp.UserConversationHandle, auth *middleware.AuthMiddleware, limter *middleware.LimitMiddleware) {
	ucGroup := r.Group("/conversations").Use(auth.JWTAuthMiddleware())
	conversationhttp.RegisterRoutes(ucGroup, ch)
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

func RegisterFileRouter(r *gin.RouterGroup, fh *filehttp.Handle, authMiddle *middleware.AuthMiddleware, limiter *middleware.LimitMiddleware, policy shared_ratelimit.Policy) {
	fileGroup := r.Group("/files").Use(authMiddle.JWTAuthMiddleware())
	filehttp.RegisterRoutes(fileGroup, fh, limiter, policy)
}

func RegisterAgentRouter(r *gin.RouterGroup, ah *agenthttp.Handle, authMiddle *middleware.AuthMiddleware) {
	agenthttp.RegisterRoutes(r, ah, authMiddle)
}

// func RegisterTestDataRouter(r *gin.RouterGroup, th *testdatahttp.Handle) {
// 	testdataGroup := r.Group("/testdata")
// 	testdatahttp.RegisterRoutes(testdataGroup, th)
// }
