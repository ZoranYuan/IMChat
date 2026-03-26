package https_friend

import (
	"github.com/gin-gonic/gin"
)

func AddFriendRouter(fg *gin.RouterGroup, fh *FriendHandle) {
	fg.POST("/", fh.Request)
	fg.GET("/:userId", fh.RequestList)
	fg.POST("/opreate", fh.OperateRequest)
}
