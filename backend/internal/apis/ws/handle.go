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
	upgrader   websocket.Upgrader
	app        *application_message.MessageApplication
	confg      configs.Config
	dispacther *Dispatcher
	getway     *GetWay
}

func NewWshandler(app *application_message.MessageApplication, confg configs.Config, dispacther *Dispatcher, getway *GetWay) *WsHandler {
	wh := &WsHandler{
		app:        app,
		confg:      confg,
		getway:     getway,
		dispacther: dispacther,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	dispacther.RegisterHandler("chat", wh.handleSendMessage)

	return wh
}

func (wh *WsHandler) handleSendMessage(ctx context.Context, c *Client, data []byte) error {
	// 对数据进行统一管理，然后调用 application 服务
	var req MessageReqData

	// TODO 后期优化为 decode 校验字段
	if err := json.Unmarshal(data, &req); err != nil {
		log.Println("failed to unmarsha, ", err)
		return err
	}

	return wh.app.HandleMessage(ctx, application_message.MessageAppeDTO{
		SendId:    c.userId,
		SessionId: c.sessionId,
		RecvId:    req.RecvId,
		ConvType:  req.ConvType,
		Ctype:     req.CType,
		Content:   req.Content,
		VideoTime: req.VideoTime,
	})
}

func (wh *WsHandler) readLoop(ctx context.Context, client *Client) {
	defer func() {
		client.Close()
		wh.getway.RemoveClient(client)
	}()

	client.conn.SetReadDeadline(time.Now().Add(time.Duration(wh.confg.WebSocket.PongWaitSeconds) * time.Second))
	client.conn.SetPongHandler(func(string) error {
		// TODO 实际业务可以自行实现 heart 来优化
		client.mu.Lock()
		client.idle = time.Now().Unix()
		client.mu.Unlock()

		client.conn.SetReadDeadline(time.Now().Add(time.Duration(wh.confg.WebSocket.PongWaitSeconds) * time.Second))
		return nil
	})

	for {
		msg, err := client.Read(wh.confg.WebSocket.PongWaitSeconds)
		if err != nil {
			log.Println("failed to read message, ", err)
			return
		}

		log.Println("read message , time is :", time.Now().Local())

		data, err := json.Marshal(msg.Data)
		if err != nil {
			log.Println("failed to read message, ", err)
			return
		}

		wh.dispacther.Dispatch(ctx, client, msg.Op, data)
	}
}

func (wh *WsHandler) writeLoop(client *Client) {
	ticker := time.NewTicker(time.Duration(wh.confg.WebSocket.PingPeriodSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-client.send:
			if !ok {
				return
			}

			m, _ := json.Marshal(msg)

			if err := client.Write(websocket.TextMessage, m, wh.confg.WebSocket.WriteWaitSeconds); err != nil {
				return
			}

			log.Println("send message , time is :", time.Now().Local())
		case <-ticker.C:
			if err := client.Write(websocket.PingMessage, nil, wh.confg.WebSocket.WriteWaitSeconds); err != nil {
				return
			}
		}
	}
}

func (wh *WsHandler) Handler(c *gin.Context) {
	userId := c.GetString("userId")
	ctx, cancel := context.WithCancel(context.Background())

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		cancel()
		return
	}

	conn, err := wh.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "网络异常，无法连接服务器"))
		cancel()
		return
	}

	sessionId := uuid.NewString()
	client := NewClient(ctx, cancel, conn, userId, sessionId, wh.confg.WebSocket.MaxMessageSendBufferSize)
	wh.getway.AddClient(client)

	go wh.readLoop(client.ctx, client)
	go wh.writeLoop(client)
}
