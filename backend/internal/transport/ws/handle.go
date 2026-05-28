package ws

import (
	"IM_backend/configs"
	messageapp "IM_backend/internal/application/message"
	"IM_backend/internal/shared/protocol"
	"IM_backend/internal/transport/http/response"
	wspb "IM_backend/internal/transport/ws/pb"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

type WSHandler struct {
	upgrader   websocket.Upgrader
	app        *messageapp.MessageApplication
	config     configs.Config
	dispatcher *Dispatcher
	gateway    *Gateway
}

func NewWSHandler(app *messageapp.MessageApplication, config configs.Config, dispatcher *Dispatcher, gateway *Gateway) *WSHandler {
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
	dispatcher.RegisterHandler(protocol.EventWatchVideoCtrl, wh.handleWatchVideoControl)

	return wh
}

func (wh *WSHandler) handleHistoryMessageRead(ctx context.Context, c *Client, data []byte) error {
	var pb wspb.MessageReadAckReq

	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}

	return wh.app.HandleMessageReadAck(ctx, c.userId, pb.GetConversationId(), pb.GetLastReadSeq())
}

func (wh *WSHandler) handleSendMessage(ctx context.Context, c *Client, data []byte) error {
	var pb wspb.MessageReq
	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}
	req := messageReqFromPB(&pb)

	videoId := ""
	if req.ConvType == 2 && req.VideoTime != nil {
		if state, ok := wh.gateway.GetWatchVideoState(req.RecvId); ok {
			videoId = state.VideoId
		}
	}

	messageApp, err := wh.app.HandleMessage(ctx, messageapp.MessageAppeDTO{
		SendId:      c.userId,
		ClientMsgId: req.ClientMsgId,
		RecvId:      req.RecvId,
		ConvType:    req.ConvType,
		CType:       req.CType,
		Content:     req.Content,
		VideoId:     videoId,
		VideoTime:   req.VideoTime,
	})
	if messageApp == nil {
		messageApp = &messageapp.MessageAppeDTO{
			ClientMsgId: req.ClientMsgId,
			Status:      string(protocol.AckStatusFailed),
		}
	}

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

func (wh *WSHandler) handleWatchVideoControl(ctx context.Context, c *Client, data []byte) error {
	var pb wspb.WatchVideoControl
	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}
	req := watchVideoControlFromPB(&pb)

	if req.RoomId == "" {
		return nil
	}
	if err := wh.app.CheckRoomMember(ctx, c.userId, req.RoomId); err != nil {
		return err
	}

	if req.Action == "get_state" {
		state, ok := wh.gateway.GetWatchVideoState(req.RoomId)
		if !ok {
			return nil
		}
		return wh.gateway.SendWatchVideoStateToUsers(protocol.EventWatchVideoSync, []string{c.userId}, state)
	}

	state, err := wh.gateway.UpsertWatchVideoState(req, c.userId)
	if err != nil {
		return err
	}

	members, err := wh.app.GetRoomMemberIDs(ctx, req.RoomId)
	if err != nil {
		return err
	}

	return wh.gateway.SendWatchVideoStateToUsers(protocol.EventWatchVideoSync, members, state)
}

func (wh *WSHandler) readLoop(ctx context.Context, client *Client) {
	defer func() {
		client.Close()
		wh.gateway.RemoveClient(client)
		released := wh.gateway.ReleaseWatchVideoStatesByUser(client.userId)
		for _, state := range released {
			members, err := wh.app.GetRoomMemberIDs(ctx, state.RoomId)
			if err != nil {
				log.Println("failed to fetch room members for release:", err)
				continue
			}
			if err := wh.gateway.SendWatchVideoStateToUsers(protocol.EventWatchVideoSync, members, state); err != nil {
				log.Println("failed to broadcast released watch state:", err)
			}
		}
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

		wh.dispatcher.Dispatch(ctx, client, msg.Op, msg.Data)
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

			m, err := proto.Marshal(&wspb.WsFrame{
				Op:   msg.Op,
				Data: msg.Data,
			})
			if err != nil {
				log.Println("marshal ws message failed ", err)
				return
			}

			if err := client.Write(websocket.BinaryMessage, m, wh.config.WebSocket.WriteWaitSeconds); err != nil {
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
