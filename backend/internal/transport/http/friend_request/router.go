package friendrequest

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(fg gin.IRoutes, fh *FriendRequestHandle) {
	fg.POST("", fh.Create)
	fg.GET("", fh.List)
	fg.POST("/actions", fh.OperateRequest)
}
