package ws

import (
	"IM_backend/configs"
	"IM_backend/internal/apis/response"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WsHandle struct {
	upgrader      websocket.Upgrader
	clientManager *ClientManager
	config        configs.Config
}

func NewWshandle(clientManager *ClientManager, config configs.Config) *WsHandle {
	return &WsHandle{
		clientManager: clientManager,
		config:        config,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (wh *WsHandle) Handle(c *gin.Context) {
	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	// 增加重试机制
	conn, err := wh.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "网络异常，无法连接服务器"))
		return
	}

	sessionId := uuid.NewString()
	client := NewClient(conn, userId, sessionId, wh.config.WebSocket.MaxMessageSendBufferSize)

	wh.clientManager.AddClient(userId, sessionId, client)

	go client.readHandle()
	go client.writeHandle()
}
