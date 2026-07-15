package websocket

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	realtime "IM_backend/internal/infrastructure/realtime"
)

var _ realtime.Gateway = (*Gateway)(nil)

// Gateway is the local long-connection gateway, equivalent to goim comet or
// OpenIM msggateway. It owns only session indexing and realtime delivery.
type Gateway struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	userSessions map[string]map[string]struct{}
}

func NewGateway() *Gateway {
	return &Gateway{
		sessions:     make(map[string]*Session),
		userSessions: make(map[string]map[string]struct{}),
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

func (g *Gateway) Register(session *Session) {
	g.mu.Lock()
	defer g.mu.Unlock()
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
	delete(g.sessions, session.SessionID())
	ids := g.userSessions[session.UserID()]
	delete(ids, session.SessionID())
	if len(ids) == 0 {
		delete(g.userSessions, session.UserID())
	}
}

func (g *Gateway) DeliverToUser(eventType, userID string, payload []byte) error {
	encoded, err := EncodePayload(eventType, payload)
	if err != nil {
		return err
	}
	for _, session := range g.sessionsForUser(userID) {
		if err := session.Enqueue(Message{Op: eventType, Data: encoded}); err != nil {
			if errors.Is(err, ErrOutboundQueueFull) {
				log.Printf("disconnect slow websocket session: user=%s session=%s", session.UserID(), session.SessionID())
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

func (g *Gateway) snapshot() []*Session {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*Session, 0, len(g.sessions))
	for _, session := range g.sessions {
		result = append(result, session)
	}
	return result
}
