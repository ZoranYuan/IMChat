package mq

import (
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	"IM_backend/internal/domain/message/entity"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

type testTxManager struct{}

func (t *testTxManager) WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(&gorm.DB{})
}

type testOutboxRepo struct {
	mu        sync.Mutex
	batch     []*entity.MessageOutbox
	sentIDs   []string
	retryIDs  []string
	retryMsgs []string
}

func (r *testOutboxRepo) Create(ctx context.Context, outbox *entity.MessageOutbox) error {
	return nil
}

func (r *testOutboxRepo) ClaimPending(ctx context.Context, now time.Time, staleBefore time.Time, limit int) ([]*entity.MessageOutbox, error) {
	return r.batch, nil
}

func (r *testOutboxRepo) MarkSent(ctx context.Context, id string, sentAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sentIDs = append(r.sentIDs, id)
	return nil
}

func (r *testOutboxRepo) MarkRetry(ctx context.Context, id string, nextRetryAt time.Time, lastError string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retryIDs = append(r.retryIDs, id)
	r.retryMsgs = append(r.retryMsgs, lastError)
	return nil
}

func (r *testOutboxRepo) WithTx(tx any) messagerepo.MessageOutboxRepository {
	return r
}

type testTaskManager struct {
	mu                        sync.Mutex
	sentMessages              []protocol.MessageEvent
	publishedReadAcks         []protocol.MessageReadAckEvent
	publishedConversationSync []protocol.ConversationSyncSeqEvent
}

func (m *testTaskManager) SendMessage(ctx context.Context, topic string, key string, event protocol.MessageEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = append(m.sentMessages, event)
	return nil
}

func (m *testTaskManager) PublishMessageReadAck(ctx context.Context, topic string, key string, event protocol.MessageReadAckEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishedReadAcks = append(m.publishedReadAcks, event)
	return nil
}

func (m *testTaskManager) SendConversationSyncSeq(ctx context.Context, topic string, key string, event protocol.ConversationSyncSeqEvent) error {
	return nil
}

func TestDispatchPendingOnceDispatchesSupportedOutboxEvents(t *testing.T) {
	readAckPayload, _ := json.Marshal(protocol.MessageReadAckEvent{UserId: "u2", ConversationId: "conv-1", LastReadSeq: 12, SenderId: "u1"})
	messagePayload, _ := json.Marshal(protocol.MessageEvent{MessageId: "m1", ConversationId: "conv-1", SendId: "u1", RecvId: "u2", ClientMsgId: "client-1"})

	repo := &testOutboxRepo{batch: []*entity.MessageOutbox{
		{ID: "o1", EventType: string(protocol.EventTypeMessage), Topic: string(protocol.EventTypeMessage), MessageKey: "conv-1", Payload: messagePayload},
		{ID: "o2", EventType: string(protocol.EventMessageReadAck), Topic: string(protocol.EventMessageReadAck), MessageKey: "u2:conv-1", Payload: readAckPayload},
	}}
	taskManager := &testTaskManager{}
	worker := &ReadAckOutboxWorker{
		txManager:     &testTxManager{},
		outboxRepo:    repo,
		taskManager:   taskManager,
		batchSize:     10,
		interval:      time.Second,
		staleAfter:    30 * time.Second,
		baseRetryWait: time.Second,
	}

	if err := worker.dispatchPendingOnce(context.Background()); err != nil {
		t.Fatalf("dispatchPendingOnce() error = %v", err)
	}

	taskManager.mu.Lock()
	defer taskManager.mu.Unlock()
	if len(taskManager.sentMessages) != 1 {
		t.Fatalf("expected 1 message event, got %d", len(taskManager.sentMessages))
	}
	if len(taskManager.publishedReadAcks) != 1 {
		t.Fatalf("expected 1 read ack event, got %d", len(taskManager.publishedReadAcks))
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.sentIDs) != 2 {
		t.Fatalf("expected 2 sent outboxes, got %d", len(repo.sentIDs))
	}
}
