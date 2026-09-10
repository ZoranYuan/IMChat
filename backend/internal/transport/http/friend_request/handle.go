package friendrequest

import (
	friendapp "IM_backend/internal/application/friend"
	"IM_backend/internal/transport/http/response"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type FriendRequestHandle struct {
	app *friendapp.RequestApplication
}

func NewFriendRequestHandle(app *friendapp.RequestApplication) *FriendRequestHandle {
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

	var newFriendRequest FriendRequestRequest
	if err := c.ShouldBindJSON(&newFriendRequest); err != nil {
		log.Println("解析好友申请参数失败：", err)
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	friendRequestApp, err := fh.app.CreateFriendRequest(userId, newFriendRequest.TargetUserID, newFriendRequest.Message)

	if err != nil {
		c.JSON(http.StatusConflict, response.Error(http.StatusBadRequest, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(&FriendRequestResponse{
		RequestID:         friendRequestApp.RequestID,
		ApplicantUserID:   friendRequestApp.ApplicantUserID,
		ApplicantNickName: friendRequestApp.ApplicantNickName,
		Message:           friendRequestApp.Message,
		Status:            friendRequestApp.Status,
		ApplyTime:         friendRequestApp.ApplyTime,
	}))
}

func (fh *FriendRequestHandle) OperateRequest(c *gin.Context) {
	var res OperateRequestRequest

	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	if err := c.ShouldBindJSON(&res); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}
	if res.Action != ActionAccept && res.Action != ActionReject {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "不支持的操作"))
		return
	}

	var err error
	if res.Action == ActionAccept {
		err = fh.app.Accept(res.RequestID, userId)
	} else {
		err = fh.app.Refuse(res.RequestID, userId)
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

	res := make([]FriendRequestResponse, 0, len(requestListApp))
	for _, r := range requestListApp {
		res = append(res, FriendRequestResponse{
			RequestID:         r.RequestID,
			ApplicantUserID:   r.ApplicantUserID,
			ApplicantNickName: r.ApplicantNickName,
			Message:           r.Message,
			Status:            r.Status,
			ApplyTime:         r.ApplyTime,
		})
	}

	c.JSON(http.StatusOK, response.Success(res))
}
