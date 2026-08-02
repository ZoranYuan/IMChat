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

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

func newTestSession(t *testing.T, bufferSize int) *Session {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	identity, err := NewSessionIdentity(
		"user-1",
		"device-1",
		string(PlatfromWeb),
		"session-1",
	)
	if err != nil {
		cancel()
		t.Fatalf("create session identity: %v", err)
	}

	session := NewSession(
		ctx,
		cancel,
		nil,
		identity,
		bufferSize,
		nil,
		MessageBatchConfig{},
		false,
	)
	t.Cleanup(session.Close)
	return session
}

func newWebSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	serverConnCh := make(chan *websocket.Conn, 1)
	upgrader := websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConnCh <- conn
	}))
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial websocket test server: %v", err)
	}
	serverConn := <-serverConnCh
	t.Cleanup(func() { _ = clientConn.Close() })
	t.Cleanup(func() { _ = serverConn.Close() })
	return serverConn, clientConn
}

func TestSessionEnqueueReturnsQueueFull(t *testing.T) {
	session := newTestSession(t, 1)

	message := Message{Op: "msg", Data: []byte("payload")}
	if err := session.Enqueue(message, AppendPolicyBatch); err != nil {
		t.Fatalf("first enqueue failed: %v", err)
	}

	if err := session.Enqueue(message, AppendPolicyBatch); !errors.Is(err, ErrOutboundQueueFull) {
		t.Fatalf("expected outbound queue full, got %v", err)
	}
}

func TestSessionReadLoopReturnsQueueFullAndClosesSession(t *testing.T) {
	serverConn, clientConn := newWebSocketPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	identity, err := NewSessionIdentity("user-1", "device-1", string(PlatfromWeb), "session-1")
	if err != nil {
		cancel()
		t.Fatal(err)
	}

	session := NewSession(
		ctx,
		cancel,
		serverConn,
		identity,
		1,
		nil,
		MessageBatchConfig{},
		false,
	)
	t.Cleanup(session.Close)

	session.Start(10, 60, 2, func(context.Context, *Session, string, []byte) {
		time.Sleep(100 * time.Millisecond)
	}, nil)

	frame := func() []byte {
		data, marshalErr := proto.Marshal(&wspb.WsFrame{
			Op:   "msg",
			Data: []byte("payload"),
		})
		if marshalErr != nil {
			t.Fatalf("marshal test frame: %v", marshalErr)
		}
		return data
	}

	if err := clientConn.WriteMessage(websocket.BinaryMessage, frame()); err != nil {
		t.Fatalf("write first test frame: %v", err)
	}
	if err := clientConn.WriteMessage(websocket.BinaryMessage, frame()); err != nil {
		t.Fatalf("write second test frame: %v", err)
	}
	if err := clientConn.WriteMessage(websocket.BinaryMessage, frame()); err != nil {
		t.Fatalf("write third test frame: %v", err)
	}

	select {
	case <-session.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("session was not closed after inbound queue overflow")
	}
}

func TestSessionSendBatchReturnsWriteError(t *testing.T) {
	serverConn, _ := newWebSocketPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	identity, err := NewSessionIdentity("user-1", "device-1", string(PlatfromWeb), "session-1")
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	session := NewSession(ctx, cancel, serverConn, identity, 1, nil, MessageBatchConfig{}, false)
	t.Cleanup(session.Close)

	if err := serverConn.Close(); err != nil {
		t.Fatalf("close server websocket: %v", err)
	}

	batch := NewMessageBatch("batch-1", 1)
	if err := batch.AppendMessage(Message{Op: "msg", Data: []byte("payload")}, 1); err != nil {
		t.Fatalf("append batch message: %v", err)
	}
	if err := batch.Seal(FlushReasonImmediate); err != nil {
		t.Fatalf("seal batch: %v", err)
	}

	if err := session.sendBatch(batch, 1); err == nil {
		t.Fatal("expected websocket write error")
	}
	if batch.Status != BatchFailed {
		t.Fatalf("expected failed batch, got %s", batch.Status)
	}
}

func TestSessionShutdownFlushesPendingBatch(t *testing.T) {
	serverConn, clientConn := newWebSocketPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	identity, err := NewSessionIdentity("user-1", "device-1", string(PlatfromWeb), "session-1")
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	idGenerator, err := snow.NewGenerator(1)
	if err != nil {
		cancel()
		t.Fatalf("create id generator: %v", err)
	}
	session := NewSession(
		ctx,
		cancel,
		serverConn,
		identity,
		8,
		idGenerator,
		MessageBatchConfig{MaxMessages: 10, MaxBytes: 1024, Linger: time.Second},
		true,
	)
	t.Cleanup(session.Close)
	session.Start(10, 60, 2, nil, nil)

	if err := session.Enqueue(
		Message{Op: "msg", Data: []byte("payload")},
		AppendPolicyBatch,
	); err != nil {
		t.Fatalf("enqueue pending message: %v", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := session.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown session: %v", err)
	}

	_ = clientConn.SetReadDeadline(time.Now().Add(time.Second))
	messageType, data, err := clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("read flushed batch: %v", err)
	}
	if messageType != websocket.BinaryMessage {
		t.Fatalf("expected binary batch, got message type %d", messageType)
	}

	var frame wspb.WsFrame
	if err := proto.Unmarshal(data, &frame); err != nil {
		t.Fatalf("unmarshal flushed frame: %v", err)
	}
	if frame.GetOp() != protocol.EventMessageBatch {
		t.Fatalf("expected batch event, got %s", frame.GetOp())
	}

	var batch wspb.WsBatch
	if err := proto.Unmarshal(frame.GetData(), &batch); err != nil {
		t.Fatalf("unmarshal flushed batch: %v", err)
	}
	if len(batch.GetFrames()) != 1 || batch.GetFrames()[0].GetOp() != "msg" {
		t.Fatalf("unexpected flushed frames: %+v", batch.GetFrames())
	}
}

func TestGatewayPartialSessionFailureDoesNotFailDelivery(t *testing.T) {
	gateway := NewGateway()
	healthy := newGatewayTestSession(t, "user-1", "healthy")
	slow := newGatewayTestSession(t, "user-1", "slow")

	gateway.mu.Lock()
	gateway.registerLocked(healthy)
	gateway.registerLocked(slow)
	gateway.mu.Unlock()

	for i := 0; i < cap(slow.outbound); i++ {
		if err := slow.Enqueue(Message{Op: "existing"}, AppendPolicyBatch); err != nil {
			t.Fatalf("fill slow session queue: %v", err)
		}
	}

	if err := gateway.DeliverToUser("room_msg_notice", "user-1", []byte(`{"seq":1}`)); err != nil {
		t.Fatalf("partial delivery should not fail: %v", err)
	}
	if len(healthy.outbound) != 1 {
		t.Fatalf("healthy session should receive message, outbound=%d", len(healthy.outbound))
	}

	select {
	case <-slow.ctx.Done():
	default:
		t.Fatal("slow session should be closed after queue overflow")
	}
}

func TestAppendPolicyForAckFlushesImmediately(t *testing.T) {
	if policy := AppendPolicyForEvent(protocol.EventTypeMsgAck); policy != AppendPolicyFlush {
		t.Fatalf("expected ACK to flush immediately, got %v", policy)
	}

	accumulator := NewBatchMessageAccumulator(
		MessageBatchConfig{MaxMessages: 10, MaxBytes: 1024, Linger: time.Second},
		fixedIDGenerator{},
	)
	batches, err := accumulator.Append(
		Message{Op: protocol.EventTypeMsgAck, Data: []byte("ack")},
		AppendPolicyForEvent(protocol.EventTypeMsgAck),
	)
	if err != nil {
		t.Fatalf("append ACK: %v", err)
	}
	if len(batches) != 1 || batches[0].FlushReason != FlushReasonImmediate {
		t.Fatalf("expected immediate ACK flush, got %+v", batches)
	}
}
