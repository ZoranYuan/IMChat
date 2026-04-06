package ws

import (
	"IM_backend/configs"
	"IM_backend/internal/apis/response"
	application_message "IM_backend/internal/applications/message"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WsHandler struct {
	upgrader      websocket.Upgrader
	app           *application_message.MessageApplication
	confg         configs.Config
	clientManager *ClientManager
	dispacther    *Dispatcher
}

func NewWshandler(app *application_message.MessageApplication, clientManager *ClientManager, confg configs.Config, dispacther *Dispatcher) *WsHandler {
	wh := &WsHandler{
		app:           app,
		confg:         confg,
		clientManager: clientManager,
		dispacther:    dispacther,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	dispacther.RegisterHandler("send_message", wh.handlerSendMessage)
	dispacther.RegisterHandler("join_room", wh.handlerJoinRoom)

	return wh
}

func (wh *WsHandler) handlerSendMessage(ctx context.Context, c *Client, data json.RawMessage) error {
	// 对数据进行统一管理，然后调用 application 服务
	var req SendMessageReq

	if err := json.Unmarshal(data, &req); err != nil {
		log.Println("failed to unmarsha, ", err)
		return err
	}

	return wh.app.HandleMessage(ctx, application_message.MessageAppeDTO{
		UserId:    c.userId,
		SessionId: c.sessionId,
		RecvId:    req.RecvId,
		Convtype:  req.Convtype,
		Ctype:     req.CType,
		Content:   req.Content,
		VideoTime: req.VideoTime,
	})
}

func (wh *WsHandler) handlerJoinRoom(ctx context.Context, c *Client, data json.RawMessage) error {
	return nil
}

func (wh *WsHandler) readLoop(ctx context.Context, client *Client) {
	for {
		select {
		case <-client.close:
			// 通知 clientManager 去关闭通道
			wh.clientManager.RemoveClient(client)

		default:
			msg, err := client.Read()
			if err != nil {
				log.Println("failed to read message, ", err)
				client.Close()
				return
			}

			wh.dispacther.Dispatch(ctx, client, msg.Op, msg.Data)
		}
	}
}

func (wh *WsHandler) writeLoop() {}

func (wh *WsHandler) Handler(c *gin.Context) {
	userId := c.GetString("userId")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)

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
	client := NewClient(ctx, cancel, conn, userId, sessionId, wh.confg.WebSocket.MaxMessageSendBufferSize)

	wh.clientManager.AddClient(client)

	go wh.readLoop(client.ctx, client)
	go wh.writeLoop()
}
