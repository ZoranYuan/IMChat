package ws

import (
	"log"
	"sync"
	"time"
)

// 管理用户 和 session
type GetWay struct {
	// session 和 client 的映射
	sessions map[string]*Client

	// 用户和 session 的关系   user ->
	userSessions map[string]map[string]struct{}

	mu sync.RWMutex
}

func NewGetWay() *GetWay {
	return &GetWay{
		sessions:     make(map[string]*Client),
		userSessions: make(map[string]map[string]struct{}),
	}
}

// 监听协程
func (g *GetWay) KeepAlive(interval int, pongWait int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)

	for range ticker.C {
		now := time.Now()

		g.mu.RLock()
		defer g.mu.RUnlock()
		for _, client := range g.sessions {

			client.mu.RLock()
			idleTime := time.Unix(client.idle, 0)
			duration := now.Sub(idleTime)
			client.mu.RUnlock()
			if duration > time.Duration(pongWait)*time.Second {
				// 超时，踢掉
				client.Close()
				g.RemoveClient(client)
			}
		}
	}
}

func (g *GetWay) AddClient(client *Client) {
	g.mu.Lock()
	defer func() {
		g.mu.Unlock()
	}()

	g.sessions[client.sessionId] = client

	value, ok := g.userSessions[client.userId]

	if !ok {
		userSession := make(map[string]struct{})
		userSession[client.sessionId] = struct{}{}

		g.userSessions[client.userId] = userSession
		return
	}

	value[client.sessionId] = struct{}{}
}

func (g *GetWay) RemoveClient(c *Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.sessions, c.sessionId)

	sessions, ok := g.userSessions[c.userId]

	if !ok {
		return
	}

	delete(sessions, c.sessionId)

	// 防止内存泄露
	if len(sessions) == 0 {
		delete(g.userSessions, c.userId)
	}
}

func (g *GetWay) getClients(recvId string) []*Client {
	g.mu.RLock()
	defer g.mu.RUnlock()
	sessions, ok := g.userSessions[recvId]

	if !ok {
		// 目标用户不再当前节点
		return nil
	}

	clients := make([]*Client, 0, len(sessions))
	for sessionId := range sessions {
		if c, ok := g.sessions[sessionId]; ok {
			clients = append(clients, c)
		}
	}

	return clients
}

// 针对接收者进行发送
func (g *GetWay) SendToClient(op string, targetId string, payload []byte) error {
	defer func() {
		if r := recover(); r != nil {
			log.Println("panic :", r)
		}
	}()

	clients := g.getClients(targetId)

	if len(clients) == 0 {
		return nil
	}

	m := WsMessage{
		Op:   op,
		Data: payload,
	}

	for _, client := range clients {
		select {
		case client.send <- m:
		default:
			// 防止阻塞（可以选择丢弃或断开）
		}
	}

	return nil
}
