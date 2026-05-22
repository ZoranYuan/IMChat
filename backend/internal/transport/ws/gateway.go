package ws

import (
	"IM_backend/internal/shared/protocol"
	"encoding/json"
	"log"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
)

// 管理用户 和 session
type Gateway struct {
	// session 和 client 的映射
	sessions map[string]*Client

	// 用户和 session 的关系   user ->
	userSessions map[string]map[string]struct{}

	watchStates map[string]WatchVideoState

	mu      sync.RWMutex
	watchMu sync.RWMutex
}

func NewGateway() *Gateway {
	return &Gateway{
		sessions:     make(map[string]*Client),
		userSessions: make(map[string]map[string]struct{}),
		watchStates:  make(map[string]WatchVideoState),
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

				if now.Sub(time.UnixMilli(idle)) > time.Duration(pongWait)*time.Second {
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
	return g.SendToUsers(op, []string{targetId}, payload)
}

func (g *Gateway) SendToUsers(op string, targetIds []string, payload []byte) error {
	defer func() {
		if r := recover(); r != nil {
			log.Println("panic :", r)
		}
	}()

	encodedPayload, err := encodeWebSocketPayload(op, payload)
	if err != nil {
		return err
	}

	for _, targetId := range targetIds {
		clients := g.getClients(targetId)
		for _, client := range clients {
			select {
			case client.send <- WsMessage{Op: op, Data: encodedPayload}:
			default:
				// 防止阻塞（可以选择丢弃或断开）
			}
		}
	}

	return nil
}

func encodeWebSocketPayload(op string, payload []byte) ([]byte, error) {
	switch op {
	case string(protocol.EventTypeMessage):
		var event protocol.MessageEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return proto.Marshal(messageEventToPB(event))
	case string(protocol.EventTypeMsgAck):
		var event protocol.MessageAckEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return proto.Marshal(messageAckToPB(event))
	case string(protocol.EventMessageReadAck):
		var event protocol.MessageReadAckEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return proto.Marshal(messageReadAckEventToPB(event))
	default:
		return payload, nil
	}
}

func (g *Gateway) SendWatchVideoStateToUsers(op string, targetIds []string, state WatchVideoState) error {
	defer func() {
		if r := recover(); r != nil {
			log.Println("panic :", r)
		}
	}()

	for _, targetId := range targetIds {
		clients := g.getClients(targetId)
		for _, client := range clients {
			payload, err := encodeWatchVideoState(state)
			if err != nil {
				log.Println("encode watch video state failed: ", err)
				continue
			}

			select {
			case client.send <- WsMessage{Op: op, Data: payload}:
			default:
				// 防止阻塞（可以选择丢弃或断开）
			}
		}
	}

	return nil
}

func encodeWatchVideoState(state WatchVideoState) ([]byte, error) {
	return proto.Marshal(watchVideoStateToPB(state))
}

func (g *Gateway) UpsertWatchVideoState(req WatchVideoControlReq, userId string) WatchVideoState {
	g.watchMu.Lock()
	defer g.watchMu.Unlock()

	state := g.watchStates[req.RoomId]
	state.RoomId = req.RoomId
	state.Action = req.Action
	state.UpdatedBy = userId
	state.UpdatedAtMs = time.Now().UnixMilli()
	state.ClientTimeMs = req.ClientTimeMs

	if req.Action == "load" {
		state.VideoId = req.VideoId
	} else if req.VideoId != "" {
		state.VideoId = req.VideoId
	}
	if req.VideoURL != "" {
		state.VideoURL = req.VideoURL
	}
	if req.DurationMs > 0 {
		state.DurationMs = req.DurationMs
	}
	if req.PlaybackRate > 0 {
		state.PlaybackRate = req.PlaybackRate
	}
	if state.PlaybackRate == 0 {
		state.PlaybackRate = 1
	}
	if req.PositionMs >= 0 {
		state.PositionMs = req.PositionMs
	}
	state.DeltaMs = req.DeltaMs

	switch req.Action {
	case "play":
		state.IsPlaying = true
	case "pause", "ended":
		state.IsPlaying = false
	case "load":
		state.IsPlaying = false
		state.PositionMs = 0
	case "forward":
		state.PositionMs += req.DeltaMs
		if state.DurationMs > 0 && state.PositionMs > state.DurationMs {
			state.PositionMs = state.DurationMs
		}
	case "backward":
		state.PositionMs -= req.DeltaMs
		if state.PositionMs < 0 {
			state.PositionMs = 0
		}
	case "seek", "sync":
	default:
	}

	g.watchStates[req.RoomId] = state
	return state
}

func (g *Gateway) GetWatchVideoState(roomId string) (WatchVideoState, bool) {
	g.watchMu.RLock()
	defer g.watchMu.RUnlock()

	state, ok := g.watchStates[roomId]
	return state, ok
}
