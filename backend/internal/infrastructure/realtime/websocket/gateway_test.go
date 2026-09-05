package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newGatewayTestSession(t *testing.T, userID, sessionID string) *Session {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	identity, err := NewSessionIdentity(userID, "web-device-"+sessionID, string(PlatfromWeb), sessionID)
	if err != nil {
		cancel()
		t.Fatalf("create session identity: %v", err)
	}
	return NewSession(
		ctx,
		cancel,
		nil,
		identity,
		8,
		nil,
		MessageBatchConfig{},
		false,
	)
}

func TestGatewayDeliversToOnlineRoomMembersWithoutConversationSubscription(t *testing.T) {
	gateway := NewGateway()
	sender := newGatewayTestSession(t, "u1", "s1")
	onlineMember := newGatewayTestSession(t, "u2", "s2")
	openedMember := newGatewayTestSession(t, "u3", "s3")
	for _, session := range []*Session{sender, onlineMember, openedMember} {
		gateway.mu.Lock()
		gateway.registerLocked(session)
		gateway.mu.Unlock()
	}
	gateway.BindOnlineRooms(sender, []string{"room1"})
	gateway.BindOnlineRooms(onlineMember, []string{"room1"})
	gateway.BindOnlineRooms(openedMember, []string{"room1"})

	if err := gateway.DeliverToOnlineRoomMembers("room_msg_notice", "room1", []byte(`{"seq":2}`), "u1"); err != nil {
		t.Fatalf("deliver online room notice: %v", err)
	}
	if len(sender.outbound) != 0 {
		t.Fatalf("sender should be excluded, outbound=%d", len(sender.outbound))
	}
	if len(onlineMember.outbound) != 1 || len(openedMember.outbound) != 1 {
		t.Fatalf("all online room members should receive notice: online=%d opened=%d", len(onlineMember.outbound), len(openedMember.outbound))
	}
}

func TestGatewayUnregisterRemovesOnlineRoomMembership(t *testing.T) {
	gateway := NewGateway()
	session := newGatewayTestSession(t, "u1", "s1")
	gateway.mu.Lock()
	gateway.registerLocked(session)
	gateway.mu.Unlock()
	gateway.BindOnlineRooms(session, []string{"room1"})
	gateway.Unregister(session)

	if got := len(gateway.sessionsForRoom("room1")); got != 0 {
		t.Fatalf("unregistered session should be removed from online room index, got=%d", got)
	}
}

func TestGatewayStaleUnregisterDoesNotRemoveReplacement(t *testing.T) {
	gateway := NewGateway()
	oldSession := newGatewayTestSession(t, "u1", "s1")
	newSession := newGatewayTestSession(t, "u1", "s1")

	gateway.mu.Lock()
	gateway.registerLocked(oldSession)
	gateway.registerLocked(newSession)
	gateway.mu.Unlock()
	gateway.BindOnlineRooms(newSession, []string{"room1"})

	// 旧连接晚于新连接退出时，不能清理新连接的索引。
	gateway.Unregister(oldSession)

	gateway.mu.RLock()
	current := gateway.sessions["s1"]
	gateway.mu.RUnlock()
	if current != newSession {
		t.Fatalf("stale session should not unregister replacement, current=%p replacement=%p", current, newSession)
	}
	if got := len(gateway.sessionsForRoom("room1")); got != 1 {
		t.Fatalf("replacement session should remain in online room index, got=%d", got)
	}
}

func TestGatewayRedisDeliveryWaitsForSubscription(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	gateway := NewGatewayWithRedis(context.Background(), client)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = gateway.Close(ctx)
	})

	session := newGatewayTestSession(t, "u1", "s1")
	gateway.mu.Lock()
	gateway.registerLocked(session)
	gateway.mu.Unlock()

	if err := gateway.DeliverToUser("msg", "u1", []byte(`{"message":"payload"}`)); err != nil {
		t.Fatalf("redis delivery failed: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for len(session.outbound) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(session.outbound) != 1 {
		t.Fatalf("redis delivery was not received locally, outbound=%d", len(session.outbound))
	}
}
