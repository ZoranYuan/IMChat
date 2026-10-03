package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"IM_backend/internal/shared/diagnostics"
	"IM_backend/internal/shared/protocol"
	wspb "IM_backend/internal/transport/ws/pb"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

const redisRealtimeChannel = "im:realtime:deliver"

const (
	roomOnlineSessionLease           = 90 * time.Second
	roomOnlineSessionRefreshInterval = 30 * time.Second
)

type redisRealtimeEvent struct {
	NodeID        string `json:"nodeId"`
	Kind          string `json:"kind"`
	EventType     string `json:"eventType"`
	UserID        string `json:"userId,omitempty"`
	RoomID        string `json:"roomId,omitempty"`
	ExcludeUserID string `json:"excludeUserId,omitempty"`
	Payload       []byte `json:"payload"`
	PublishedAt   int64  `json:"publishedAt,omitempty"`
}

func (g *Gateway) startRedisDelivery(ctx context.Context, client *redis.Client) {
	if client == nil {
		return
	}
	g.redisClient = client
	g.nodeID = uuid.NewString()
	g.redisCtx, g.redisCancel = context.WithCancel(ctx)
	g.redisDone = make(chan struct{})
	g.presenceDone = make(chan struct{})
	g.redisReady = make(chan struct{})
	go g.consumeRedisDelivery()
	go g.refreshRoomOnlinePresenceLoop()
}

func (g *Gateway) consumeRedisDelivery() {
	defer close(g.redisDone)
	subscription := g.redisClient.Subscribe(g.redisCtx, redisRealtimeChannel)
	defer subscription.Close()
	if _, err := subscription.Receive(g.redisCtx); err != nil {
		g.mu.Lock()
		g.redisErr = err
		close(g.redisReady)
		g.mu.Unlock()
		if !errors.Is(err, context.Canceled) {
			log.Printf("订阅跨实例实时推送失败：%v", err)
		}
		return
	}
	g.mu.Lock()
	close(g.redisReady)
	g.mu.Unlock()
	for {
		select {
		case <-g.redisCtx.Done():
			return
		case message, ok := <-subscription.Channel():
			if !ok {
				return
			}
			var event redisRealtimeEvent
			if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
				log.Printf("解析跨实例实时推送失败：%v", err)
				continue
			}
			receivedAt := time.Now()
			queueDelay := time.Duration(0)
			if event.PublishedAt > 0 {
				queueDelay = receivedAt.Sub(time.Unix(0, event.PublishedAt))
			}
			localDeliveryStartedAt := time.Now()
			switch event.Kind {
			case "user":
				g.deliverLocalToUser(event.EventType, event.UserID, event.Payload)
			case "room":
				g.deliverLocalToRoom(event.EventType, event.RoomID, event.Payload, event.ExcludeUserID)
			}
			if diagnostics.Enabled() {
				messageID, seq := realtimeMessageIdentity(event.EventType, event.Payload)
				if messageID != "" || seq > 0 {
					diagnostics.Logf("stage=redis_local_delivery event_type=%s room_id=%s message_id=%s seq=%d pubsub_queue_us=%d local_delivery_us=%d outcome=dispatched",
						event.EventType, event.RoomID, messageID, seq, queueDelay.Microseconds(), time.Since(localDeliveryStartedAt).Microseconds())
				}
			}
		}
	}
}

func (g *Gateway) refreshRoomOnlinePresenceLoop() {
	defer close(g.presenceDone)
	ticker := time.NewTicker(roomOnlineSessionRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-g.redisCtx.Done():
			return
		case <-ticker.C:
			g.refreshRoomOnlinePresence()
		}
	}
}

func roomOnlineSessionKey(roomID string) string {
	return "im:realtime:room-online:" + roomID
}

func (g *Gateway) roomOnlineSessionMember(session *Session) string {
	return g.nodeID + ":" + session.SessionID()
}

func (g *Gateway) updateOnlineRoomPresence(roomIDs []string, session *Session, online bool) {
	if g == nil || g.redisClient == nil || len(roomIDs) == 0 || session == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	member := g.roomOnlineSessionMember(session)
	_, err := g.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, roomID := range roomIDs {
			key := roomOnlineSessionKey(roomID)
			if online {
				pipe.ZAdd(ctx, key, redis.Z{
					Score:  float64(time.Now().Add(roomOnlineSessionLease).UnixMilli()),
					Member: member,
				})
				pipe.Expire(ctx, key, roomOnlineSessionLease)
			} else {
				pipe.ZRem(ctx, key, member)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("更新房间在线连接缓存失败：rooms=%d session=%s online=%t err=%v", len(roomIDs), session.SessionID(), online, err)
	}
}

// OnlineRoomSessionCount 返回所有网关节点上当前有效的房间连接数。
func (g *Gateway) OnlineRoomSessionCount(ctx context.Context, roomID string) (int, error) {
	if g == nil || roomID == "" {
		return 0, errors.New("房间在线连接查询参数无效")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if g.redisClient == nil {
		return len(g.sessionsForRoom(roomID)), nil
	}
	select {
	case <-g.redisReady:
		g.mu.RLock()
		readyErr := g.redisErr
		g.mu.RUnlock()
		if readyErr != nil {
			return 0, readyErr
		}
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	const script = `
		redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
		local count = redis.call('ZCARD', KEYS[1])
		if count == 0 then redis.call('DEL', KEYS[1]) end
		return count
	`
	count, err := g.redisClient.Eval(ctx, script, []string{roomOnlineSessionKey(roomID)}, time.Now().UnixMilli()).Int()
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (g *Gateway) refreshRoomOnlinePresence() {
	if g == nil || g.redisClient == nil {
		return
	}
	roomMembers := make(map[string][]redis.Z)
	expiresAt := float64(time.Now().Add(roomOnlineSessionLease).UnixMilli())
	g.mu.RLock()
	for roomID, sessionIDs := range g.roomSessions {
		for sessionID := range sessionIDs {
			session := g.sessions[sessionID]
			if session == nil {
				continue
			}
			roomMembers[roomID] = append(roomMembers[roomID], redis.Z{
				Score:  expiresAt,
				Member: g.roomOnlineSessionMember(session),
			})
		}
	}
	g.mu.RUnlock()
	if len(roomMembers) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(g.redisCtx, 2*time.Second)
	defer cancel()
	_, err := g.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for roomID, members := range roomMembers {
			key := roomOnlineSessionKey(roomID)
			pipe.ZAdd(ctx, key, members...)
			pipe.Expire(ctx, key, roomOnlineSessionLease)
		}
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("续期房间在线连接缓存失败：rooms=%d err=%v", len(roomMembers), err)
	}
}

func (g *Gateway) publishRedisDelivery(ctx context.Context, event redisRealtimeEvent) error {
	if g.redisClient == nil {
		return nil
	}
	select {
	case <-g.redisReady:
		g.mu.RLock()
		err := g.redisErr
		g.mu.RUnlock()
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	startedAt := time.Now()
	event.PublishedAt = startedAt.UnixNano()
	event.NodeID = g.nodeID
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	err = g.redisClient.Publish(ctx, redisRealtimeChannel, payload).Err()
	if diagnostics.Enabled() {
		messageID, seq := realtimeMessageIdentity(event.EventType, event.Payload)
		if messageID != "" || seq > 0 {
			outcome := "published"
			if err != nil {
				outcome = "error"
			}
			diagnostics.Logf("stage=redis_publish event_type=%s room_id=%s message_id=%s seq=%d publish_us=%d outcome=%s",
				event.EventType, event.RoomID, messageID, seq, time.Since(startedAt).Microseconds(), outcome)
		}
	}
	return err
}

func realtimeMessageIdentity(eventType string, payload []byte) (string, int64) {
	switch eventType {
	case string(protocol.EventTypeSendMessage):
		var event wspb.MessageEvent
		if err := proto.Unmarshal(payload, &event); err == nil {
			return event.GetMessageId(), event.GetSeq()
		}
		var eventJSON protocol.MessageEvent
		if err := json.Unmarshal(payload, &eventJSON); err == nil {
			return eventJSON.MessageId, eventJSON.Seq
		}
	case protocol.EventRoomMessageNotice:
		var event wspb.RoomMessageNotice
		if err := proto.Unmarshal(payload, &event); err == nil {
			return event.GetMessageId(), event.GetSeq()
		}
		var eventJSON protocol.MessageNotifyEvent
		if err := json.Unmarshal(payload, &eventJSON); err == nil {
			return eventJSON.MessageId, eventJSON.Seq
		}
	}
	return "", 0
}

func (g *Gateway) Close(ctx context.Context) error {
	if g.redisCancel == nil {
		return nil
	}
	g.redisCancel()
	select {
	case <-g.redisDone:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-g.presenceDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
