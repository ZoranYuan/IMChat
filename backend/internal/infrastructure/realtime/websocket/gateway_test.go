package websocket

import (
	"context"
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
		if err := gateway.Register(session); err != nil {
			t.Fatalf("register session: %v", err)
		}
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
	if err := gateway.Register(session); err != nil {
		t.Fatalf("register session: %v", err)
	}
	gateway.BindOnlineRooms(session, []string{"room1"})
	gateway.Unregister(session)

	if got := len(gateway.sessionsForRoom("room1")); got != 0 {
		t.Fatalf("unregistered session should be removed from online room index, got=%d", got)
	}
}
