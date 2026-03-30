package https_friend

import "github.com/gin-gonic/gin"

func AddFriendRouter(fr gin.IRoutes, fh *FriendHandle) {
	fr.GET("", fh.GetFriendList)
}
