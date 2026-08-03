package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const redisRealtimeChannel = "im:realtime:deliver"

type redisRealtimeEvent struct {
	NodeID        string `json:"nodeId"`
	Kind          string `json:"kind"`
	EventType     string `json:"eventType"`
	UserID        string `json:"userId,omitempty"`
	RoomID        string `json:"roomId,omitempty"`
	ExcludeUserID string `json:"excludeUserId,omitempty"`
	Payload       []byte `json:"payload"`
}

func (g *Gateway) startRedisDelivery(ctx context.Context, client *redis.Client) {
	if client == nil {
		return
	}
	g.redisClient = client
	g.nodeID = uuid.NewString()
	g.redisCtx, g.redisCancel = context.WithCancel(ctx)
	g.redisDone = make(chan struct{})
	g.redisReady = make(chan struct{})
	go g.consumeRedisDelivery()
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
			var err error
			switch event.Kind {
			case "user":
				err = g.deliverLocalToUser(event.EventType, event.UserID, event.Payload)
			case "room":
				err = g.deliverLocalToRoom(event.EventType, event.RoomID, event.Payload, event.ExcludeUserID)
			}
			if err != nil {
				log.Printf("跨实例实时推送本地投递失败：%v", err)
			}
		}
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
	event.NodeID = g.nodeID
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return g.redisClient.Publish(ctx, redisRealtimeChannel, payload).Err()
}

func (g *Gateway) Close(ctx context.Context) error {
	if g.redisCancel == nil {
		return nil
	}
	g.redisCancel()
	select {
	case <-g.redisDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
