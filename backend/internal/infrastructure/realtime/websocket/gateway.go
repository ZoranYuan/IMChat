package websocket

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrGatewayClosed = errors.New("实时通道网关已关闭")

type Gateway struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	userSessions map[string]map[string]struct{} //  用户当前有哪些在线连接
	roomSessions map[string]map[string]struct{} //  房间当前有哪些在线成员连接
	sessionRooms map[string]map[string]struct{} //  连接所属哪些在线房间，断线时反向清理
	closing      bool
	redisClient  *redis.Client
	nodeID       string
	redisCtx     context.Context
	redisCancel  context.CancelFunc
	redisDone    chan struct{}
	redisReady   chan struct{}
	redisErr     error
}

func NewGatewayWithRedis(ctx context.Context, client *redis.Client) *Gateway {
	gateway := NewGateway()
	gateway.startRedisDelivery(ctx, client)
	return gateway
}

func NewGateway() *Gateway {
	return &Gateway{
		sessions:     make(map[string]*Session),
		userSessions: make(map[string]map[string]struct{}),
		roomSessions: make(map[string]map[string]struct{}),
		sessionRooms: make(map[string]map[string]struct{}),
	}
}

func (g *Gateway) KeepAlive(ctx context.Context, interval, pongWait int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				for _, session := range g.snapshot() {
					if now.Sub(session.LastActive()) > time.Duration(pongWait)*time.Second {
						session.Close()
					}
				}
			}
		}
	}()
}

func (g *Gateway) Shutdown(ctx context.Context) error {
	g.mu.Lock()
	g.closing = true
	sessions := make([]*Session, 0, len(g.sessions))
	for _, session := range g.sessions {
		sessions = append(sessions, session)
	}
	g.mu.Unlock()

	for _, session := range sessions {
		session.beginShutdown()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, session := range sessions {
			<-session.writeDone
		}
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		for _, session := range sessions {
			session.ForceClose()
		}
		return ctx.Err()
	}
}

// RegisterAndStart 将 Session 注册和启动绑定为一个生命周期操作，避免
// Gateway 记录了尚未启动、无法关闭 writeDone 的 Session。
func (g *Gateway) RegisterAndStart(
	session *Session,
	pongWait int,
	pingPeriod int,
	writeWait int,
	handler MessageHandler,
	onClose func(*Session),
) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing {
		return ErrGatewayClosed
	}

	g.registerLocked(session)
	session.Start(pongWait, pingPeriod, writeWait, handler, onClose)
	return nil
}

func (g *Gateway) registerLocked(session *Session) {
	g.sessions[session.SessionID()] = session
	ids := g.userSessions[session.UserID()]
	if ids == nil {
		ids = make(map[string]struct{})
		g.userSessions[session.UserID()] = ids
	}
	ids[session.SessionID()] = struct{}{}
}

func (g *Gateway) Unregister(session *Session) {
	g.mu.Lock()
	defer g.mu.Unlock()
	sessionID := session.SessionID()
	// 同一个登录 session 可能在旧连接尚未完成清理时建立新连接。如果当前索引已经指向新 Session，旧 Session 不能继续注销这个 ID，否则会误删新连接及其房间索引。
	if current := g.sessions[sessionID]; current != session {
		return
	}
	delete(g.sessions, sessionID)
	ids := g.userSessions[session.UserID()]
	delete(ids, sessionID)
	if len(ids) == 0 {
		delete(g.userSessions, session.UserID())
	}
	for roomID := range g.sessionRooms[sessionID] {
		sessions := g.roomSessions[roomID]
		delete(sessions, sessionID)
		if len(sessions) == 0 {
			delete(g.roomSessions, roomID)
		}
	}
	delete(g.sessionRooms, sessionID)
}

func (g *Gateway) DeliverToUser(eventType, userID string, payload []byte) error {
	if g.redisClient != nil {
		publishCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return g.publishRedisDelivery(publishCtx, redisRealtimeEvent{
			Kind: "user", EventType: eventType, UserID: userID, Payload: payload,
		})
	}
	err := g.deliverLocalToUser(eventType, userID, payload)
	return err
}

func (g *Gateway) deliverLocalToUser(eventType, userID string, payload []byte) error {
	var deliveryErr error
	for _, session := range g.sessionsForUser(userID) {
		if err := session.PushEvent(eventType, payload); err != nil {
			deliveryErr = errors.Join(deliveryErr, err)
			if errors.Is(err, ErrOutboundQueueFull) {
				session.Close()
			}

			log.Printf(
				"WS 用户推送失败：user=%s session=%s event=%s error=%v",
				userID,
				session.SessionID(),
				eventType,
				err,
			)
		}
	}
	return deliveryErr
}

func (g *Gateway) BindOnlineRooms(session *Session, roomIDs []string) {
	if session == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing || g.sessions[session.SessionID()] == nil {
		return
	}
	sessionID := session.SessionID()
	rooms := g.sessionRooms[sessionID]
	if rooms == nil {
		rooms = make(map[string]struct{})
		g.sessionRooms[sessionID] = rooms
	}
	for _, roomID := range roomIDs {
		if roomID == "" {
			continue
		}
		sessions := g.roomSessions[roomID]
		if sessions == nil {
			sessions = make(map[string]struct{})
			g.roomSessions[roomID] = sessions
		}
		sessions[sessionID] = struct{}{}
		rooms[roomID] = struct{}{}
	}
}

func (g *Gateway) BindUserToRoom(userID, roomID string) {
	if userID == "" || roomID == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing {
		return
	}
	for sessionID := range g.userSessions[userID] {
		if g.sessions[sessionID] == nil {
			continue
		}
		sessions := g.roomSessions[roomID]
		if sessions == nil {
			sessions = make(map[string]struct{})
			g.roomSessions[roomID] = sessions
		}
		// 将当前的 session 加入房间
		sessions[sessionID] = struct{}{}
		rooms := g.sessionRooms[sessionID]
		if rooms == nil {
			rooms = make(map[string]struct{})
			g.sessionRooms[sessionID] = rooms
		}
		rooms[roomID] = struct{}{}
	}
}

func (g *Gateway) UnbindUserFromRoom(userID, roomID string) {
	if userID == "" || roomID == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for sessionID := range g.userSessions[userID] {
		sessions := g.roomSessions[roomID]
		delete(sessions, sessionID)
		if len(sessions) == 0 {
			delete(g.roomSessions, roomID)
		}
		rooms := g.sessionRooms[sessionID]
		delete(rooms, roomID)
		if len(rooms) == 0 {
			delete(g.sessionRooms, sessionID)
		}
	}
}

func (g *Gateway) DeliverToOnlineRoomMembers(eventType, roomID string, payload []byte, excludeUserID string) error {
	if g.redisClient != nil {
		publishCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return g.publishRedisDelivery(publishCtx, redisRealtimeEvent{
			Kind: "room", EventType: eventType, RoomID: roomID, ExcludeUserID: excludeUserID, Payload: payload,
		})
	}
	err := g.deliverLocalToRoom(eventType, roomID, payload, excludeUserID)
	return err
}

func (g *Gateway) deliverLocalToRoom(eventType, roomID string, payload []byte, excludeUserID string) error {
	var deliveryErr error
	for _, session := range g.sessionsForRoom(roomID) {
		// 排除发送者自身
		if excludeUserID != "" && session.UserID() == excludeUserID {
			continue
		}

		if err := session.PushEvent(eventType, payload); err != nil {
			deliveryErr = errors.Join(deliveryErr, err)
			if errors.Is(err, ErrOutboundQueueFull) {
				log.Printf("断开处理缓慢的实时通道会话：用户=%s 会话=%s", session.UserID(), session.SessionID())
				session.Close()
			}

			log.Printf(
				"WS 房间推送失败：room=%s user=%s session=%s event=%s error=%v",
				roomID,
				session.UserID(),
				session.SessionID(),
				eventType,
				err,
			)
		}
	}
	return deliveryErr
}

func (g *Gateway) sessionsForUser(userID string) []*Session {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := g.userSessions[userID]
	result := make([]*Session, 0, len(ids))
	for id := range ids {
		if session := g.sessions[id]; session != nil {
			result = append(result, session)
		}
	}
	return result
}

func (g *Gateway) sessionsForRoom(roomID string) []*Session {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := g.roomSessions[roomID]
	result := make([]*Session, 0, len(ids))
	for id := range ids {
		if session := g.sessions[id]; session != nil {
			result = append(result, session)
		}
	}
	return result
}

func (g *Gateway) snapshot() []*Session {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*Session, 0, len(g.sessions))
	for _, session := range g.sessions {
		result = append(result, session)
	}
	return result
}
