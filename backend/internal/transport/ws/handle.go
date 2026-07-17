package ws

import (
	"IM_backend/configs"
	messageapp "IM_backend/internal/application/message"
	realtimews "IM_backend/internal/infrastructure/realtime/websocket"
	"IM_backend/internal/shared/protocol"
	shared_ratelimit "IM_backend/internal/shared/ratelimit"
	"IM_backend/internal/transport/http/response"
	wspb "IM_backend/internal/transport/ws/pb"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

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
	gateway    *realtimews.Gateway
	limiter    shared_ratelimit.Limit
}

func NewWSHandler(app *messageapp.MessageApplication, config configs.Config, dispatcher *Dispatcher, gateway *realtimews.Gateway) *WSHandler {
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

func (wh *WSHandler) SetLimiter(limiter shared_ratelimit.Limit) {
	wh.limiter = limiter
}

func (wh *WSHandler) allowEvent(ctx context.Context, op string, userId string, policy shared_ratelimit.Policy) bool {
	if wh.limiter == nil {
		return true
	}

	key := fmt.Sprintf(
		"ws:user:%s:op:%s",
		userId,
		op,
	)

	decision, err := wh.limiter.Allow(
		ctx,
		key,
		policy,
	)

	if err != nil {
		// WebSocket 普通消息采用 Fail Open。
		log.Printf(
			"websocket rate limiter failed: %v",
			err,
		)
		return true
	}

	return decision.Allowed
}

func (wh *WSHandler) handleHistoryMessageRead(ctx context.Context, session *realtimews.Session, data []byte) error {
	var pb wspb.MessageReadAckReq

	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}

	return wh.app.HandleMessageReadAck(ctx, session.UserID(), pb.GetConversationId(), pb.GetLastReadSeq(), pb.GetSenderId())
}

func (wh *WSHandler) handleSendMessage(ctx context.Context, session *realtimews.Session, data []byte) error {
	var pb wspb.MessageReq
	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}

	allowed := wh.allowEvent(
		ctx,
		protocol.EventTypeMessage,
		session.UserID(),
		shared_ratelimit.Policy{
			Rate:  10,
			Burst: 20,
		},
	)

	if !allowed {
		ack := protocol.MessageAckEvent{
			ClientMsgId: pb.GetClientMsgId(),
			Status:      protocol.AckStatusFailed,
			Extra:       "消息发送过于频繁",
		}

		payload, err := json.Marshal(ack)
		if err != nil {
			return err
		}

		return wh.replyToClient(
			session,
			protocol.EventTypeMsgAck,
			payload,
		)
	}

	req := messageReqFromPB(&pb)

	messageApp, err := wh.app.HandleMessage(ctx, messageapp.MessageAppeDTO{
		SendId:      session.UserID(),
		ClientMsgId: req.ClientMsgId,
		RecvId:      req.RecvId,
		ConvType:    req.ConvType,
		CType:       req.CType,
		Content:     req.Content,
		MediaURL:    req.MediaURL,
		ThumbURL:    req.ThumbURL,
		FileId:      req.FileId,
		ThumbFileId: req.ThumbFileId,
		FileName:    req.FileName,
		FileSize:    req.FileSize,
		Width:       req.Width,
		Height:      req.Height,
		DurationMs:  req.DurationMs,
		StickerId:   req.StickerId,
		PackId:      req.PackId,
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

	return wh.replyToClient(session, protocol.EventTypeMsgAck, data)
}

func (wh *WSHandler) replyToClient(session *realtimews.Session, op string, payload []byte) error {
	encodedPayload, err := realtimews.EncodePayload(op, payload)
	if err != nil {
		return err
	}
	if err := session.Enqueue(realtimews.Message{Op: op, Data: encodedPayload}); err != nil {
		if errors.Is(err, realtimews.ErrOutboundQueueFull) {
			session.Close()
		}
		return err
	}
	return nil
}

func (wh *WSHandler) handleClientClosed(session *realtimews.Session) {
	wh.gateway.Unregister(session)
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
	session := realtimews.NewSession(ctx, cancel, conn, userId, sessionId, wh.config.WebSocket.MaxMessageSendBufferSize)
	wh.gateway.Register(session)
	session.Start(
		wh.config.WebSocket.PongWaitSeconds,
		wh.config.WebSocket.PingPeriodSeconds,
		wh.config.WebSocket.WriteWaitSeconds,
		wh.dispatcher.Dispatch,
		wh.handleClientClosed,
	)
}
