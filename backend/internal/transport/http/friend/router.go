package friend

import "github.com/gin-gonic/gin"

func RegisterRoutes(fr gin.IRoutes, fh *FriendHandle) {
	fr.GET("", fh.GetFriendList)
}
