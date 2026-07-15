package websocket

import (
	"context"
	"testing"
)

func TestGatewayDeliversToRegisteredUserSessions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &Session{
		ctx:       ctx,
		cancel:    cancel,
		userID:    "u1",
		sessionID: "s1",
		outbound:  make(chan Message, 1),
	}
	gateway := NewGateway()
	gateway.Register(session)

	if err := gateway.DeliverToUser("custom", "u1", []byte("payload")); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	message := <-session.outbound
	if message.Op != "custom" || string(message.Data) != "payload" {
		t.Fatalf("unexpected message: %+v", message)
	}

	gateway.Unregister(session)
	if err := gateway.DeliverToUser("custom", "u1", []byte("ignored")); err != nil {
		t.Fatalf("deliver after unregister: %v", err)
	}
	select {
	case message := <-session.outbound:
		t.Fatalf("unexpected message after unregister: %+v", message)
	default:
	}
}
