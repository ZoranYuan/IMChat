package friend

import (
	"IM_backend/internal/apis/response"
	application_friend "IM_backend/internal/applications/friend"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type FriendHandle struct {
	app *application_friend.FriendApplication
}

func NewUserHandler(app *application_friend.FriendApplication) *FriendHandle {
	return &FriendHandle{
		app: app,
	}
}

func (fh *FriendHandle) NewFriendRequest(c *gin.Context) {
	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(201, "登录过期"))
		return
	}

	var newFriendRequest FriendRequestReq
	if err := c.ShouldBindJSON(&newFriendRequest); err != nil {
		log.Println("failed to parse newFriendRequest, ", err)
		c.JSON(http.StatusBadRequest, response.Error(201, "参数错误"))
		return
	}

	_, err := fh.app.NewFriendRequest(userId, newFriendRequest.RequestId, newFriendRequest.Message)

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(201, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(nil))
}
