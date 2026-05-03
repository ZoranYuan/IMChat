package https_message

import (
	application_message "IM_backend/internal/application/message"
	"IM_backend/internal/interfaces/http/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type MessageHandle struct {
	app *application_message.MessageApplication
}

func NewMessageHandle(app *application_message.MessageApplication) *MessageHandle {
	return &MessageHandle{
		app: app,
	}
}

func (mh *MessageHandle) GetHistoryMessages(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}
	var req MessageHistoryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}
	messagesApp, nextCursor, hasMore, err := mh.app.GetHistoryMessages(c.Request.Context(), req.ConversationId, userId, req.Limit, req.Cursor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "获取消息失败，请稍后再试"))
		return
	}
	c.JSON(http.StatusOK, response.Success(toHistoryMessageRes(messagesApp, nextCursor, hasMore)))
}

func (mh *MessageHandle) GetOfflineMessages(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	messagesApp, unreadMap, err := mh.app.GetOfflineMessages(c.Request.Context(), userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "获取消息失败，请稍后再试"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toOfflineMessageRes(messagesApp, unreadMap)))
}
