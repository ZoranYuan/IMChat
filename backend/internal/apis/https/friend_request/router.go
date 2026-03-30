package https_friend_request

import (
	"github.com/gin-gonic/gin"
)

func AddFriendRequstRouter(fg gin.IRoutes, fh *FriendRequestHandle) {
	fg.POST("", fh.Request)
	fg.GET("/:userId", fh.RequestList)
	fg.POST("/opreate", fh.OperateRequest)
}
