package websocket

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var ErrGatewayClosed = errors.New("实时通道网关已关闭")

type Gateway struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	userSessions map[string]map[string]struct{} //  个用户当前有哪些在线连接
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

func (g *Gateway) Register(session *Session) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing {
		return ErrGatewayClosed
	}

	g.sessions[session.SessionID()] = session
	ids := g.userSessions[session.UserID()]
	if ids == nil {
		ids = make(map[string]struct{})
		g.userSessions[session.UserID()] = ids
	}
	ids[session.SessionID()] = struct{}{}
	return nil
}

func (g *Gateway) Unregister(session *Session) {
	g.mu.Lock()
	defer g.mu.Unlock()
	sessionID := session.SessionID()
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
	encoded, err := EncodePayload(eventType, payload)
	if err != nil {
		return err
	}
	for _, session := range g.sessionsForUser(userID) {
		if err := session.Enqueue(
			Message{Op: eventType, Data: encoded},
			AppendPolicyBatch,
		); err != nil {
			if errors.Is(err, ErrOutboundQueueFull) {
				log.Printf("断开处理缓慢的实时通道会话：用户=%s 会话=%s", session.UserID(), session.SessionID())
				session.Close()
			}
		}
	}
	return nil
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
	encoded, err := EncodePayload(eventType, payload)
	if err != nil {
		return err
	}
	for _, session := range g.sessionsForRoom(roomID) {
		if excludeUserID != "" && session.UserID() == excludeUserID {
			continue
		}
		if err := session.Enqueue(Message{Op: eventType, Data: encoded}, AppendPolicyBatch); err != nil {
			if errors.Is(err, ErrOutboundQueueFull) {
				log.Printf("断开处理缓慢的实时通道会话：用户=%s 会话=%s", session.UserID(), session.SessionID())
				session.Close()
			}
		}
	}
	return nil
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
