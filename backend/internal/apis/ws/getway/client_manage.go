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
	return
}

func (cm *ClientManager) RemoveClient(userId, sessionId string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.sessions, sessionId)

	sessions, ok := cm.userSessions[userId]

	if !ok {
		return
	}

	delete(sessions, sessionId)

	// 防止内存泄露
	if len(sessions) == 0 {
		delete(cm.userSessions, userId)
	}
}

func (cm *ClientManager) SendToUser(userId string, msg []byte) {
	cm.mu.RLock()

	sessions, ok := cm.userSessions[userId]

	if !ok {
		// 目标用户不再当前节点
		return
	}

	// 由于写 socket 是满操作，先拷贝再写可以提升性能

	clients := make([]*Client, 0, len(sessions))
	for sessionId := range sessions {
		if c, ok := cm.sessions[sessionId]; ok {
			clients = append(clients, c)
		}
	}
	cm.mu.RUnlock()

	for client := range clients {
		// TODO 做事情
	}
}
