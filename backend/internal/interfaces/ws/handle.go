package ws

import (
	"IM_backend/configs"
	application_message "IM_backend/internal/application/message"
	"IM_backend/internal/interfaces/http/response"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WSHandler struct {
	upgrader   websocket.Upgrader
	app        *application_message.MessageApplication
	config     configs.Config
	dispatcher *Dispatcher
	gateway    *Gateway
}

func NewWSHandler(app *application_message.MessageApplication, config configs.Config, dispatcher *Dispatcher, gateway *Gateway) *WSHandler {
	wh := &WSHandler{
		app:        app,
		config:     config,
		gateway:    gateway,
		dispatcher: dispatcher,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	dispatcher.RegisterHandler(protocol.EventTypeMessage, wh.handleSendMessage)
	dispatcher.RegisterHandler(protocol.EventMessageReadAck, wh.handleHistoryMessageRead)

	return wh
}

func (wh *WSHandler) handleHistoryMessageRead(ctx context.Context, c *Client, data []byte) error {
	var req MessageReadAckReq

	if err := json.Unmarshal(data, &req); err != nil {
		return err
	}

	return wh.app.HandleMessageReadAck(ctx, c.userId, req.ConversationId, req.LastReadSeq)
}

func (wh *WSHandler) handleSendMessage(ctx context.Context, c *Client, data []byte) error {
	var req MessageReq
	if err := json.Unmarshal(data, &req); err != nil {
		return err
	}

	messageApp, err := wh.app.HandleMessage(ctx, application_message.MessageAppeDTO{
		SendId:      c.userId,
		ClientMsgId: req.ClientMsgId,
		RecvId:      req.RecvId,
		ConvType:    req.ConvType,
		CType:       req.CType,
		Content:     req.Content,
		VideoTime:   req.VideoTime,
	})

	var ackEvent *protocol.MessageAckEvent = &protocol.MessageAckEvent{
		ClientMsgId: messageApp.ClientMsgId,
		MessageId:   messageApp.MessageId,
		Status:      protocol.AckStatus(messageApp.Status),
	}

	if err != nil {
		ackEvent.Extra = err.Error()
		log.Println("failed to handle message, ", err)
	}

	data, err = json.Marshal(ackEvent)

	if err != nil {
		log.Println("marshal ack failed ", err)
		return err
	}

	return wh.gateway.SendToClient(protocol.EventTypeMsgAck, c.userId, data)
}

func (wh *WSHandler) readLoop(ctx context.Context, client *Client) {
	defer func() {
		client.Close()
		wh.gateway.RemoveClient(client)
	}()

	client.conn.SetReadDeadline(time.Now().Add(time.Duration(wh.config.WebSocket.PongWaitSeconds) * time.Second))
	client.conn.SetPongHandler(func(string) error {
		// TODO 实际业务可以自行实现 heart 来优化
		client.mu.Lock()
		client.idle = time.Now().UnixMilli()
		client.mu.Unlock()

		client.conn.SetReadDeadline(time.Now().Add(time.Duration(wh.config.WebSocket.PongWaitSeconds) * time.Second))
		return nil
	})

	for {
		msg, err := client.Read(wh.config.WebSocket.PongWaitSeconds)
		if err != nil {
			log.Println("failed to read message, ", err)
			return
		}

		data, err := json.Marshal(msg.Data)
		if err != nil {
			log.Println("failed to read message, ", err)
			return
		}

		wh.dispatcher.Dispatch(ctx, client, msg.Op, data)
	}
}

func (wh *WSHandler) writeLoop(client *Client) {
	ticker := time.NewTicker(time.Duration(wh.config.WebSocket.PingPeriodSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-client.send:
			if !ok {
				return
			}

			m, _ := json.Marshal(msg)

			if err := client.Write(websocket.TextMessage, m, wh.config.WebSocket.WriteWaitSeconds); err != nil {
				return
			}
		case <-ticker.C:
			if err := client.Write(websocket.PingMessage, nil, wh.config.WebSocket.WriteWaitSeconds); err != nil {
				return
			}
		}
	}
}

func (wh *WSHandler) Handler(c *gin.Context) {
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
	client := NewClient(ctx, cancel, conn, userId, sessionId, wh.config.WebSocket.MaxMessageSendBufferSize)
	wh.gateway.AddClient(client)

	go wh.readLoop(client.ctx, client)
	go wh.writeLoop(client)
}
