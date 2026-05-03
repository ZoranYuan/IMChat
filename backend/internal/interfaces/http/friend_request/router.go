package https_friend_request

import (
	"github.com/gin-gonic/gin"
)

func AddFriendRequestRouter(fg gin.IRoutes, fh *FriendRequestHandle) {
	fg.POST("", fh.Create)
	fg.GET("/:userId", fh.List)
	fg.POST("/opreate", fh.OperateRequest)
}
