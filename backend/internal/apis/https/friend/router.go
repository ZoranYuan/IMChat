package https_friend

import "github.com/gin-gonic/gin"

func AddFriendRouter(fg gin.IRoutes, fh *FriendHandle) {
	fg.GET("", fh.GetFriendList)
}
