package https_friend_request

import (
	"IM_backend/internal/apis/response"
	application_friend_request "IM_backend/internal/applications/friend_request"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type FriendHandle struct {
	app *application_friend_request.FriendApplication
}

func NewUserHandler(app *application_friend_request.FriendApplication) *FriendHandle {
	return &FriendHandle{
		app: app,
	}
}

func (fh *FriendHandle) Request(c *gin.Context) {
	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	var newFriendRequest FriendRequestReq
	if err := c.ShouldBindJSON(&newFriendRequest); err != nil {
		log.Println("failed to parse newFriendRequest, ", err)
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	friendRequestApp, err := fh.app.NewFriendRequest(userId, newFriendRequest.ToUserId, newFriendRequest.Message)

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(&FriendRequestRes{
		RequestId: friendRequestApp.RequestId,
		ToUserId:  friendRequestApp.ToUserId,
		Message:   friendRequestApp.Message,
		Status:    friendRequestApp.Status,
		ApplyTime: friendRequestApp.ApplyTime,
	}))
}

func (fh *FriendHandle) OperateRequest(c *gin.Context) {
	var res OperateRequestReq

	if err := c.ShouldBindJSON(&res); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	var err error
	if res.IsAccept {
		err = fh.app.Accept(res.RequestId)
	} else {
		err = fh.app.Refuse(res.RequestId)
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(nil))
}

func (fh *FriendHandle) RequestList(c *gin.Context) {
	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	requestListApp, err := fh.app.GetFriendRequstListByUserId(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "服务端错误"))
		return
	}

	res := make([]FriendRequestRes, 0, len(requestListApp))
	for _, r := range requestListApp {
		res = append(res, FriendRequestRes{
			RequestId: r.RequestId,
			ToUserId:  r.ToUserId,
			Status:    r.Status,
			ApplyTime: r.ApplyTime,
		})
	}

	c.JSON(http.StatusOK, response.Success(res))
}
