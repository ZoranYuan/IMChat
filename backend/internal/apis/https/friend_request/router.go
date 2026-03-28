package https_friend_request

import (
	"github.com/gin-gonic/gin"
)

func AddFriendRouter(fg gin.IRoutes, fh *FriendHandle) {
	fg.POST("", fh.Request)
	fg.GET("/:userId", fh.RequestList)
	fg.POST("/opreate", fh.OperateRequest)
}
