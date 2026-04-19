package ws

import (
	"log"
	"sync"
	"time"
)

// 管理用户 和 session
type Gateway struct {
	// session 和 client 的映射
	sessions map[string]*Client

	// 用户和 session 的关系   user ->
	userSessions map[string]map[string]struct{}

	mu sync.RWMutex
}

func NewGateway() *Gateway {
	return &Gateway{
		sessions:     make(map[string]*Client),
		userSessions: make(map[string]map[string]struct{}),
	}
}

// 监听协程
func (g *Gateway) KeepAlive(interval int, pongWait int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)

	go func() {
		for range ticker.C {
			now := time.Now()
			g.mu.RLock()
			clients := make([]*Client, 0, len(g.sessions))
			for _, c := range g.sessions {
				clients = append(clients, c)
			}
			g.mu.RUnlock()

			var toRemove []*Client
			for _, client := range clients {
				client.mu.RLock()
				idle := client.idle
				client.mu.RUnlock()

				if now.Sub(time.Unix(idle, 0)) > time.Duration(pongWait)*time.Second {
					toRemove = append(toRemove, client)
				}
			}

			for _, client := range toRemove {
				log.Println("超时，准备踢出", client.userId)

				client.Close()
				g.RemoveClient(client)
			}
		}
	}()
}

func (g *Gateway) AddClient(client *Client) {
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

func (g *Gateway) RemoveClient(c *Client) {
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

func (g *Gateway) getClients(recvId string) []*Client {
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

func (g *Gateway) SendToClient(op string, targetId string, payload []byte) error {
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
