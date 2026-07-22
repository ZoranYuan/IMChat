package websocket

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"IM_backend/internal/infrastructure/id/snow"
	"IM_backend/internal/shared/protocol"
	wspb "IM_backend/internal/transport/ws/pb"

	gorilla "github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

func TestBatchSessionFlushesMessagesInOrder(t *testing.T) {
	session, client := newWebSocketTestSession(t, true)

	if err := session.Enqueue(Message{Op: "msg", Data: []byte("first")}, AppendPolicyBatch); err != nil {
		t.Fatalf("enqueue first message: %v", err)
	}
	if err := session.Enqueue(Message{Op: "msg_ack", Data: []byte("second")}, AppendPolicyFlush); err != nil {
		t.Fatalf("enqueue immediate message: %v", err)
	}

	frame := readTestFrame(t, client)
	if frame.GetOp() != protocol.EventMessageBatch {
		t.Fatalf("expected batch frame, got %q", frame.GetOp())
	}

	var batch wspb.WsBatch
	if err := proto.Unmarshal(frame.GetData(), &batch); err != nil {
		t.Fatalf("decode batch: %v", err)
	}
	if len(batch.GetFrames()) != 2 {
		t.Fatalf("expected two frames, got %d", len(batch.GetFrames()))
	}
	if batch.GetFrames()[0].GetOp() != "msg" || batch.GetFrames()[1].GetOp() != "msg_ack" {
		t.Fatalf("unexpected frame order: %+v", batch.GetFrames())
	}
}

func TestLegacySessionKeepsSingleFrameProtocol(t *testing.T) {
	session, client := newWebSocketTestSession(t, false)

	if err := session.Enqueue(Message{Op: "msg", Data: []byte("payload")}, AppendPolicyBatch); err != nil {
		t.Fatalf("enqueue message: %v", err)
	}

	frame := readTestFrame(t, client)
	if frame.GetOp() != "msg" || string(frame.GetData()) != "payload" {
		t.Fatalf("unexpected direct frame: %+v", frame)
	}
}

func TestSessionShutdownFlushesActiveBatch(t *testing.T) {
	session, client := newWebSocketTestSession(t, true)

	if err := session.Enqueue(Message{Op: "msg", Data: []byte("pending")}, AppendPolicyBatch); err != nil {
		t.Fatalf("enqueue pending message: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := session.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown session: %v", err)
	}

	frame := readTestFrame(t, client)
	if frame.GetOp() != protocol.EventMessageBatch {
		t.Fatalf("expected shutdown batch, got %q", frame.GetOp())
	}

	var batch wspb.WsBatch
	if err := proto.Unmarshal(frame.GetData(), &batch); err != nil {
		t.Fatalf("decode shutdown batch: %v", err)
	}
	if len(batch.GetFrames()) != 1 || string(batch.GetFrames()[0].GetData()) != "pending" {
		t.Fatalf("unexpected shutdown batch: %+v", batch.GetFrames())
	}
}

func TestSessionCloseIsIdempotent(t *testing.T) {
	session, _ := newWebSocketTestSession(t, true)
	done := make(chan struct{})

	go func() {
		session.Close()
		session.Close()
		if err := session.Enqueue(Message{Op: "msg"}, AppendPolicyBatch); !errors.Is(err, ErrSessionClosed) {
			t.Errorf("expected closed error, got %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("repeated close blocked")
	}
}

func newWebSocketTestSession(t *testing.T, batchEnabled bool) (*Session, *gorilla.Conn) {
	t.Helper()

	idGenerator, err := snow.NewGenerator(1)
	if err != nil {
		t.Fatalf("create id generator: %v", err)
	}

	sessionCh := make(chan *Session, 1)
	upgradeErrCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, upgradeErr := (&gorilla.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if upgradeErr != nil {
			upgradeErrCh <- upgradeErr
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		session := NewSession(
			ctx,
			cancel,
			conn,
			"user-1",
			"session-1",
			8,
			idGenerator,
			MessageBatchConfig{MaxMessages: 8, MaxBytes: 1024, Linger: time.Second, ReadyQueueSize: 2},
			batchEnabled,
		)
		session.Start(5, 4, 1, nil, nil)
		sessionCh <- session
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client, _, err := gorilla.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket: %v", err)
	}

	var session *Session
	select {
	case session = <-sessionCh:
	case upgradeErr := <-upgradeErrCh:
		client.Close()
		server.Close()
		t.Fatalf("upgrade websocket: %v", upgradeErr)
	case <-time.After(time.Second):
		client.Close()
		server.Close()
		t.Fatal("session startup timed out")
	}

	t.Cleanup(func() {
		session.Close()
		_ = client.Close()
		server.Close()
	})
	return session, client
}

func readTestFrame(t *testing.T, conn *gorilla.Conn) *wspb.WsFrame {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	messageType, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket frame: %v", err)
	}
	if messageType != gorilla.BinaryMessage {
		t.Fatalf("expected binary frame, got %d", messageType)
	}

	var frame wspb.WsFrame
	if err := proto.Unmarshal(payload, &frame); err != nil {
		t.Fatalf("decode websocket frame: %v", err)
	}
	return &frame
}
