package https_room

import (
	application_room "IM_backend/internal/application/room"
	"IM_backend/internal/interfaces/http/response"
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RoomHandle struct {
	app *application_room.RoomApplication
}

func NewRoomHandle(app *application_room.RoomApplication) *RoomHandle {
	return &RoomHandle{
		app: app,
	}
}

func (rh *RoomHandle) Create(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("panic ", r)
			c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "网络错误"))
		}
	}()

	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	var req CreateRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	ctx := context.TODO()
	roomApp, err := rh.app.Create(ctx, userId, req.RoomName, req.Avatar, req.Description)

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(CreateRoomRes{
		RoomId:      roomApp.RoomId,
		Description: roomApp.Description,
		RoomName:    roomApp.RoomName,
		Avatar:      roomApp.Avatar,
		MemberCount: roomApp.MemberCount,
		InviteCode:  roomApp.InviteCode,
	}))
}

func (rh *RoomHandle) Invite(c *gin.Context) {
	userId := c.GetString("userId")

	roomId := c.Param("roomId")

	if roomId == "" {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	ctx := context.TODO()

	inviteCode, err := rh.app.Invite(ctx, userId, roomId)

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(inviteCode))
}

func (rh *RoomHandle) Join(c *gin.Context) {
	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	var req JoinRoomReq

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}
	ctx := c.Request.Context()
	roomUserApp, roomApp, err := rh.app.Join(ctx, userId, req.InviteCode)

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(JoinRoomRes{
		RoomId:      roomApp.RoomId,
		RoomName:    roomApp.RoomName,
		Avatar:      roomApp.Avatar,
		MemberCount: roomApp.MemberCount,
		Role:        roomUserApp.Role,
		JoinTime:    roomUserApp.JoinTime,
	}))
}
