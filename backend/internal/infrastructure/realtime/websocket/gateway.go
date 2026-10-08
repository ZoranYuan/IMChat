package websocket

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"IM_backend/internal/shared/diagnostics"
)

var ErrGatewayClosed = errors.New("实时通道网关已关闭")

type Gateway struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	userSessions map[string]map[string]struct{} //  用户当前有哪些在线连接
	roomSessions map[string]map[string]struct{} //  房间当前有哪些在线成员连接
	sessionRooms map[string]map[string]struct{} //  连接所属哪些在线房间，断线时反向清理
	closing      bool
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
	sessionID := session.SessionID()
	// 同一个登录 session 可能在旧连接尚未完成清理时建立新连接。如果当前索引已经指向新 Session，旧 Session 不能继续注销这个 ID，否则会误删新连接及其房间索引。
	if current := g.sessions[sessionID]; current != session {
		g.mu.Unlock()
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
		if _, exists := sessions[sessionID]; exists {
			delete(sessions, sessionID)
		}
		if len(sessions) == 0 {
			delete(g.roomSessions, roomID)
		}
	}
	delete(g.sessionRooms, sessionID)
	g.mu.Unlock()
}

func (g *Gateway) DeliverToUser(eventType, userID string, payload []byte) error {
	for _, session := range g.sessionsForUser(userID) {
		if err := session.PushEvent(eventType, payload); err != nil {
			// 当前 Session 投递失败，主动断开，然后让前端重新连接并同步消息。
			// 单个 Session 的失败不能影响同一用户的其他连接。
			session.Close()

			log.Printf(
				"WS 用户推送失败：user=%s session=%s event=%s error=%v",
				userID,
				session.SessionID(),
				eventType,
				err,
			)
		}
	}
	return nil
}

func (g *Gateway) BindOnlineRooms(session *Session, roomIDs []string) {
	if session == nil {
		return
	}
	g.mu.Lock()
	if g.closing || g.sessions[session.SessionID()] == nil {
		g.mu.Unlock()
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
		if _, exists := sessions[sessionID]; !exists {
			sessions[sessionID] = struct{}{}
		}
		rooms[roomID] = struct{}{}
	}
	g.mu.Unlock()
}

func (g *Gateway) BindUserToRoom(userID, roomID string) {
	if userID == "" || roomID == "" {
		return
	}
	g.mu.Lock()
	if g.closing {
		g.mu.Unlock()
		return
	}
	for sessionID := range g.userSessions[userID] {
		session := g.sessions[sessionID]
		if session == nil {
			continue
		}
		sessions := g.roomSessions[roomID]
		if sessions == nil {
			sessions = make(map[string]struct{})
			g.roomSessions[roomID] = sessions
		}
		// 将当前的 session 加入房间
		if _, exists := sessions[sessionID]; !exists {
			sessions[sessionID] = struct{}{}
		}
		rooms := g.sessionRooms[sessionID]
		if rooms == nil {
			rooms = make(map[string]struct{})
			g.sessionRooms[sessionID] = rooms
		}
		rooms[roomID] = struct{}{}
	}
	g.mu.Unlock()
}

func (g *Gateway) UnbindUserFromRoom(userID, roomID string) {
	if userID == "" || roomID == "" {
		return
	}
	g.mu.Lock()
	for sessionID := range g.userSessions[userID] {
		sessions := g.roomSessions[roomID]
		if _, exists := sessions[sessionID]; exists {
			delete(sessions, sessionID)
		}
		if len(sessions) == 0 {
			delete(g.roomSessions, roomID)
		}
		rooms := g.sessionRooms[sessionID]
		delete(rooms, roomID)
		if len(rooms) == 0 {
			delete(g.sessionRooms, sessionID)
		}
	}
	g.mu.Unlock()
}

func (g *Gateway) DeliverToOnlineRoomMembers(eventType, roomID string, payload []byte, excludeUserID string) error {
	diagnosticEnabled := diagnostics.Enabled()
	localDelivered := 0
	localFailed := 0
	for _, session := range g.sessionsForRoom(roomID) {
		// 排除发送者自身
		if excludeUserID != "" && session.UserID() == excludeUserID {
			continue
		}

		if err := session.PushEvent(eventType, payload); err != nil {
			if diagnosticEnabled {
				localFailed++
			}
			// 当前 Session 投递失败，主动断开，然后让前端重新连接并同步消息。
			session.Close()

			log.Printf(
				"WS 房间推送失败：room=%s user=%s session=%s event=%s error=%v",
				roomID,
				session.UserID(),
				session.SessionID(),
				eventType,
				err,
			)
			continue
		}
		if diagnosticEnabled {
			localDelivered++
		}
	}
	if diagnosticEnabled {
		messageID, seq := realtimeMessageIdentity(eventType, payload)
		diagnostics.Logf("stage=ws_room_enqueue room_id=%s event_type=%s message_id=%s seq=%d delivered_sessions=%d failed_sessions=%d outcome=completed",
			roomID, eventType, messageID, seq, localDelivered, localFailed)
	}
	return nil
}

// OnlineRoomSessionCount 直接读取当前节点的房间连接索引。
func (g *Gateway) OnlineRoomSessionCount(ctx context.Context, roomID string) (int, error) {
	if g == nil || roomID == "" {
		return 0, errors.New("房间在线连接查询参数无效")
	}
	if ctx != nil && ctx.Err() != nil {
		return 0, ctx.Err()
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.roomSessions[roomID]), nil
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
