package conversation

import (
	conversationapp "IM_backend/internal/application/conversation"
	"IM_backend/internal/transport/http/response"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserConversationHandle struct {
	app *conversationapp.UserConvApplication
}

func NewUserConversationHandle(app *conversationapp.UserConvApplication) *UserConversationHandle {
	return &UserConversationHandle{
		app: app,
	}
}

func (h *UserConversationHandle) GetUserConversations(c *gin.Context) {
	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	convs, err := h.app.GetUserConversationsById(c, userId)

	if err != nil {
		log.Println("用户会话获取失败", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "用户会话获取失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toUserConversationsRes(convs)))
}
