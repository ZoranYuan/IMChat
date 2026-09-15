package ws

import (
	"IM_backend/configs"
	messageapp "IM_backend/internal/application/message"
	"IM_backend/internal/infrastructure/id/snow"
	realtimews "IM_backend/internal/infrastructure/realtime/websocket"
	"IM_backend/internal/shared/protocol"
	shared_ratelimit "IM_backend/internal/shared/ratelimit"
	"IM_backend/internal/transport/http/response"
	wspb "IM_backend/internal/transport/ws/pb"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

type WSHandler struct {
	upgrader    websocket.Upgrader
	app         *messageapp.MessageApplication
	config      configs.Config
	dispatcher  *Dispatcher
	gateway     *realtimews.Gateway
	limiter     shared_ratelimit.Limit
	idGenerator *snow.Generator
}

func NewWSHandler(
	app *messageapp.MessageApplication,
	config configs.Config,
	dispatcher *Dispatcher,
	gateway *realtimews.Gateway,
	idGenerator *snow.Generator,
) *WSHandler {
	wh := &WSHandler{
		app:         app,
		config:      config,
		gateway:     gateway,
		dispatcher:  dispatcher,
		idGenerator: idGenerator,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" || config.App.Env == "development" {
					return true
				}
				for _, allowed := range strings.Split(config.Security.AllowedOrigins, ",") {
					if strings.TrimSpace(allowed) == origin {
						return true
					}
				}
				return false
			},
		},
	}

	dispatcher.RegisterHandler(protocol.EventTypeSendMessage, wh.handleSendMessage)
	dispatcher.RegisterHandler(protocol.EventReadMessageAck, wh.handleReadMessageAck)

	return wh
}

func (wh *WSHandler) SetLimiter(limiter shared_ratelimit.Limit) {
	wh.limiter = limiter
}

func (wh *WSHandler) allowEvent(ctx context.Context, op string, userId string, policy shared_ratelimit.Policy) (bool, error) {
	if wh.limiter == nil {
		return true, nil
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
			"WebSocket 限流器执行失败：%v",
			err,
		)
		return false, err
	}

	return decision.Allowed, nil
}

func (wh *WSHandler) handleReadMessageAck(ctx context.Context, session *realtimews.Session, data []byte) error {
	var pb wspb.MessageReadAckReq

	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}

	if err := wh.app.HandleReadMessage(ctx, session.UserID(), pb.GetConversationId(), pb.GetLastReadSeq()); err != nil {
		return err
	}
	return nil
}

func (wh *WSHandler) handleSendMessage(ctx context.Context, session *realtimews.Session, data []byte) error {
	var pb wspb.MessageReq
	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}

	sendRate := wh.config.WebSocket.SendMessageRate
	if sendRate <= 0 {
		sendRate = 10
	}
	sendBurst := int64(wh.config.WebSocket.SendMessageBurst)
	if sendBurst <= 0 {
		sendBurst = 20
	}

	// 用户发消息频率限制
	allowed, limitErr := wh.allowEvent(
		ctx,
		protocol.EventTypeSendMessage,
		session.UserID(),
		shared_ratelimit.Policy{
			Rate:  float64(sendRate),
			Burst: sendBurst,
		},
	)
	if limitErr != nil {
		ack := protocol.MessageAckEvent{ClientMsgId: pb.GetClientMsgId(), Status: protocol.AckStatusFailed, Extra: "限流服务暂不可用"}
		payload, err := json.Marshal(ack)
		if err != nil {
			return err
		}
		return wh.replyToClient(session, protocol.EventTypeMsgAck, payload)
	}

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

	messageApp, err := wh.app.HandleSendMessage(ctx, messageapp.SendMessageDTO{
		SenderID:         session.UserID(),
		ClientMessageID:  req.ClientMsgId,
		ReceiverID:       req.RecvId,
		ConversationType: req.ConvType,
		Type:             req.CType,
		Content:          req.Content,
		FileID:           req.FileId,
		StickerID:        req.StickerId,
		PackID:           req.PackId,
		VideoTime:        req.VideoTime,
	})

	if messageApp == nil {
		messageApp = &messageapp.MessageAckDTO{
			ClientMessageID: req.ClientMsgId,
			Status:          string(protocol.AckStatusFailed),
		}
	}

	var ackEvent *protocol.MessageAckEvent = &protocol.MessageAckEvent{
		ClientMsgId:    messageApp.ClientMessageID,
		MessageId:      messageApp.MessageID,
		ConversationId: messageApp.ConversationID,
		Seq:            messageApp.Seq,
		AttachmentId:   messageApp.AttachmentID,
		SendTime:       messageApp.SendTime,
		Status:         protocol.AckStatus(messageApp.Status),
	}

	if err != nil {
		ackEvent.Extra = err.Error()
		log.Println("处理消息失败：", err)
	}

	data, err = json.Marshal(ackEvent)

	if err != nil {
		log.Println("序列化消息确认失败：", err)
		return err
	}

	return wh.replyToClient(session, protocol.EventTypeMsgAck, data)
}

func (wh *WSHandler) replyToClient(session *realtimews.Session, op string, payload []byte) error {
	return session.PushEvent(op, payload)
}

func (wh *WSHandler) handleClientClosed(session *realtimews.Session) {
	wh.gateway.Unregister(session)
}

func (wh *WSHandler) checkOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))

	// 生产模式下直接返回成功
	if wh.config.App.Env == "development" {
		return true
	}

	if origin == "" {
		return false
	}

	for _, allowed := range strings.Split(wh.config.Security.AllowedOrigins, ",") {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}

	return false
}

func (wh *WSHandler) Handler(c *gin.Context) {
	userId := strings.TrimSpace(c.GetString("userId"))
	ctx, cancel := context.WithCancel(context.Background())

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		cancel()
		return
	}

	deviceID := c.Query("device_id")
	if deviceID == "" {
		deviceID = c.GetHeader("X-Device-ID")
	}

	platform := c.Query("platform")
	if platform == "" {
		platform = c.GetHeader("X-Platform")
	}

	sessionId := strings.TrimSpace(c.GetString("sessionId"))
	identity, err := realtimews.NewSessionIdentity(
		userId,
		deviceID,
		platform,
		sessionId,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "网络异常，无法连接服务器"))
		cancel()
		return
	}

	if isValid := wh.checkOrigin(c.Request); !isValid {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "非法请求"))
		cancel()
		return
	}

	// 服务升级
	conn, err := wh.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "网络异常，无法连接服务器"))
		cancel()
		return
	}

	batchConfig := realtimews.MessageBatchConfig{
		MaxMessages:    wh.config.WebSocket.BatchMaxMessages,
		MaxBytes:       wh.config.WebSocket.BatchMaxBytes,
		Linger:         time.Duration(wh.config.WebSocket.BatchLingerMilliseconds) * time.Millisecond,
		ReadyQueueSize: wh.config.WebSocket.BatchReadyQueueSize,
	}

	session := realtimews.NewSession(
		ctx,
		cancel,
		conn,
		identity,
		wh.config.WebSocket.MaxMessageSendBufferSize,
		wh.idGenerator,
		batchConfig,
		true,
	)
	session.SetReadLimit(int64(wh.config.WebSocket.MaxMessageSize))

	if err := wh.gateway.RegisterAndStart(
		session,
		wh.config.WebSocket.PongWaitSeconds,
		wh.config.WebSocket.PingPeriodSeconds,
		wh.config.WebSocket.WriteWaitSeconds,
		wh.dispatcher.Dispatch,
		wh.handleClientClosed,
	); err != nil {
		session.ForceClose()
		return
	}
	roomIDs, err := wh.app.ListActiveRoomIDs(userId)
	if err != nil {
		log.Printf("加载用户在线房间索引失败：用户=%s 错误=%v", userId, err)
	} else {
		// 绑定当前会话，所有活跃的房间
		wh.gateway.BindOnlineRooms(session, roomIDs)
	}
}
