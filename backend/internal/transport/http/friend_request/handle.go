package friendrequest

import (
	friendrequestapp "IM_backend/internal/application/friend_request"
	"IM_backend/internal/transport/http/response"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type FriendRequestHandle struct {
	app *friendrequestapp.FriendApplication
}

func NewFriendRequestHandle(app *friendrequestapp.FriendApplication) *FriendRequestHandle {
	return &FriendRequestHandle{
		app: app,
	}
}

func (fh *FriendRequestHandle) Create(c *gin.Context) {
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

	friendRequestApp, err := fh.app.CreateFriendRequest(userId, newFriendRequest.ToUserId, newFriendRequest.Message)

	if err != nil {
		if errors.Is(err, friendrequestapp.ErrRequestSentTooFrequently) {
			c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, err.Error()))
		} else {
			c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "网络错误"))
		}
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

func (fh *FriendRequestHandle) OperateRequest(c *gin.Context) {
	var res OperateRequestReq

	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	if err := c.ShouldBindJSON(&res); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	var err error
	if res.Action == ActionAccept {
		err = fh.app.Accept(res.RequestId, userId)
	} else {
		err = fh.app.Refuse(res.RequestId, userId)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(nil))
}

func (fh *FriendRequestHandle) List(c *gin.Context) {
	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	requestListApp, err := fh.app.ListFriendRequestsByUserID(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "服务端错误"))
		return
	}

	res := make([]FriendRequestRes, 0, len(requestListApp))
	for _, r := range requestListApp {
		res = append(res, FriendRequestRes{
			RequestId:       r.RequestId,
			FromUserId:      r.FromUserId,
			FromUsername:    r.FromUsername,
			FromDisplayName: r.FromDisplayName,
			ToUserId:        r.ToUserId,
			Message:         r.Message,
			Status:          r.Status,
			ApplyTime:       r.ApplyTime,
		})
	}

	c.JSON(http.StatusOK, response.Success(res))
}
