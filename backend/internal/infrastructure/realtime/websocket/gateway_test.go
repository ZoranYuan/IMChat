package websocket

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGatewayDeliversToRegisteredUserSessions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &Session{
		ctx:       ctx,
		cancel:    cancel,
		userID:    "u1",
		sessionID: "s1",
		outbound:  make(chan OutboundItem, 1),
		accepting: true,
	}
	gateway := NewGateway()
	if err := gateway.Register(session); err != nil {
		t.Fatalf("register session: %v", err)
	}

	if err := gateway.DeliverToUser("custom", "u1", []byte("payload")); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	item := <-session.outbound
	if item.Policy != AppendPolicyBatch ||
		item.Message.Op != "custom" ||
		string(item.Message.Data) != "payload" {
		t.Fatalf("unexpected outbound item: %+v", item)
	}

	gateway.Unregister(session)
	if err := gateway.DeliverToUser("custom", "u1", []byte("ignored")); err != nil {
		t.Fatalf("deliver after unregister: %v", err)
	}
	select {
	case item := <-session.outbound:
		t.Fatalf("unexpected message after unregister: %+v", item)
	default:
	}
}

func TestGatewayRejectsRegistrationAfterShutdownStarts(t *testing.T) {
	gateway := NewGateway()
	if err := gateway.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown gateway: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &Session{
		ctx:       ctx,
		cancel:    cancel,
		userID:    "u1",
		sessionID: "late-session",
		outbound:  make(chan OutboundItem, 1),
		accepting: true,
	}

	if err := gateway.Register(session); !errors.Is(err, ErrGatewayClosed) {
		t.Fatalf("expected gateway closed error, got %v", err)
	}
}

func TestGatewayShutdownForceClosesSessionOnTimeout(t *testing.T) {
	gateway := NewGateway()
	ctx, cancel := context.WithCancel(context.Background())
	session := &Session{
		ctx:       ctx,
		cancel:    cancel,
		userID:    "u1",
		sessionID: "stuck-session",
		outbound:  make(chan OutboundItem, 1),
		writeDone: make(chan struct{}),
		accepting: true,
	}
	if err := gateway.Register(session); err != nil {
		t.Fatalf("register session: %v", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer shutdownCancel()
	if err := gateway.Shutdown(shutdownCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected shutdown deadline, got %v", err)
	}
	if ctx.Err() == nil {
		t.Fatal("expected session context to be cancelled")
	}
	if err := session.Enqueue(Message{Op: "msg"}, AppendPolicyBatch); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("expected closed session error, got %v", err)
	}
}
