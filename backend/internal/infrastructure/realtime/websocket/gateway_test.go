package websocket

import (
	"bytes"
	"context"
	"errors"
	"testing"
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
		1,
		nil,
		MessageBatchConfig{},
	)
}

func TestGatewayDeliversDirectlyToAllLocalUserSessions(t *testing.T) {
	gateway := NewGateway()
	first := newGatewayTestSession(t, "u1", "first")
	second := newGatewayTestSession(t, "u1", "second")
	other := newGatewayTestSession(t, "u2", "other")
	for _, session := range []*Session{first, second, other} {
		gateway.mu.Lock()
		gateway.registerLocked(session)
		gateway.mu.Unlock()
		t.Cleanup(session.Close)
	}
	payload := []byte("local event")
	if err := gateway.DeliverToUser("test_event", "u1", payload); err != nil {
		t.Fatal(err)
	}
	for _, session := range []*Session{first, second} {
		select {
		case item := <-session.outbound:
			if item.Message.Op != "test_event" || !bytes.Equal(item.Message.Data, payload) {
				t.Fatalf("unexpected local payload: %+v", item.Message)
			}
		default:
			t.Fatal("delivery must enqueue locally before returning")
		}
	}
	if len(other.outbound) != 0 {
		t.Fatal("delivered to unrelated user")
	}
}

func TestGatewayOnlineRoomSessionCountTracksLocalMembership(t *testing.T) {
	gateway := NewGateway()
	first := newGatewayTestSession(t, "u1", "first")
	second := newGatewayTestSession(t, "u1", "second")
	other := newGatewayTestSession(t, "u2", "other")
	for _, session := range []*Session{first, second, other} {
		gateway.mu.Lock()
		gateway.registerLocked(session)
		gateway.mu.Unlock()
		t.Cleanup(session.Close)
	}
	check := func(want int) {
		t.Helper()
		count, err := gateway.OnlineRoomSessionCount(context.Background(), "room")
		if err != nil || count != want {
			t.Fatalf("local count=%d want=%d err=%v", count, want, err)
		}
	}
	check(0)
	gateway.BindOnlineRooms(first, []string{"room", "room"})
	check(1)
	gateway.BindUserToRoom("u1", "room")
	check(2)
	gateway.BindOnlineRooms(other, []string{"room"})
	check(3)
	gateway.UnbindUserFromRoom("u1", "room")
	check(1)
	gateway.Unregister(other)
	check(0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := gateway.OnlineRoomSessionCount(ctx, "room"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled query=%v", err)
	}
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
