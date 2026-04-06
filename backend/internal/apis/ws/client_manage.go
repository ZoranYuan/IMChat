package ws

import (
	"sync"
)

// 管理用户 和 session
type ClientManager struct {
	// session 和 client 的映射
	sessions map[string]*Client

	// 用户和 session 的关系   user ->
	userSessions map[string]map[string]struct{}

	mu sync.RWMutex
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		sessions:     make(map[string]*Client),
		userSessions: make(map[string]map[string]struct{}),
	}
}

func (cm *ClientManager) AddClient(client *Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.sessions[client.sessionId] = client

	value, ok := cm.userSessions[client.userId]

	if !ok {
		userSession := make(map[string]struct{})
		userSession[client.sessionId] = struct{}{}

		cm.userSessions[client.userId] = userSession
		return
	}

	value[client.sessionId] = struct{}{}
}

func (cm *ClientManager) RemoveClient(c *Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.sessions, c.sessionId)

	sessions, ok := cm.userSessions[c.userId]

	if !ok {
		return
	}

	delete(sessions, c.sessionId)

	// 防止内存泄露
	if len(sessions) == 0 {
		delete(cm.userSessions, c.userId)
	}
}

func (cm *ClientManager) GetClients(userId string) []*Client {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	sessions, ok := cm.userSessions[userId]

	if !ok {
		// 目标用户不再当前节点
		return nil
	}

	clients := make([]*Client, 0, len(sessions))
	for sessionId := range sessions {
		if c, ok := cm.sessions[sessionId]; ok {
			clients = append(clients, c)
		}
	}

	return clients
}
