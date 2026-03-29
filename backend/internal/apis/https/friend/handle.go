package https_friend

import (
	"IM_backend/internal/apis/response"
	application_friend "IM_backend/internal/applications/friend"
	"net/http"

	"github.com/gin-gonic/gin"
)

type FriendHandle struct {
	app *application_friend.FriendApplication
}

func NewFriendHandle(app *application_friend.FriendApplication) *FriendHandle {
	return &FriendHandle{
		app: app,
	}
}

func (fh *FriendHandle) GetFriendList(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	friendListApp, err := fh.app.GetUserFriendList(userId)

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "网络错误"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toRes(friendListApp)))
}
