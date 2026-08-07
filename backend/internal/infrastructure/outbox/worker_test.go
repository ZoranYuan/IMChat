package outbox

import (
	"IM_backend/configs"
	eventbus "IM_backend/internal/application/ports/eventbus"
	outboxport "IM_backend/internal/application/ports/outbox"
	"context"
	"errors"
	"testing"
	"time"
)

type publisherStub struct {
	topic   string
	key     string
	payload []byte
	err     error
}

func (stub *publisherStub) Publish(_ context.Context, event eventbus.IntegrationEvent) error {
	stub.topic = event.Name
	stub.key = event.PartitionKey
	stub.payload = event.Payload
	return stub.err
}

type outboxRepositoryStub struct {
	markedSent bool
	markedDead bool
	retried    bool
	lockToken  string
}

func (stub *outboxRepositoryStub) Create(context.Context, *outboxport.Entry) error {
	return nil
}
func (stub *outboxRepositoryStub) ClaimPending(context.Context, time.Time, time.Time, int) ([]*outboxport.Entry, error) {
	return nil, nil
}
func (stub *outboxRepositoryStub) MarkSent(_ context.Context, _ string, token string, _ time.Time) error {
	stub.markedSent = true
	stub.lockToken = token
	return nil
}
func (stub *outboxRepositoryStub) MarkRetry(_ context.Context, _ string, token string, _ time.Time, _ string) error {
	stub.retried = true
	stub.lockToken = token
	return nil
}
func (stub *outboxRepositoryStub) MarkDead(_ context.Context, _ string, token string, _ string) error {
	stub.markedDead = true
	stub.lockToken = token
	return nil
}
func (stub *outboxRepositoryStub) WithTx(any) outboxport.Repository {
	return stub
}

func TestWorkerPublishesStoredPayload(t *testing.T) {
	repository := &outboxRepositoryStub{}
	publisher := &publisherStub{}
	worker := NewWorker(nil, repository, publisher, configs.OutboxConfig{
		BatchSize:            10,
		PollIntervalSeconds:  2,
		StaleAfterSeconds:    30,
		BaseRetryWaitSeconds: 2,
		MaxRetries:           10,
		WorkerCount:          2,
		QueueSize:            4,
	})
	item := &outboxport.Entry{
		ID:         "o1",
		EventType:  "msg",
		MessageKey: "c1",
		Payload:    []byte("wire-payload"),
		LockToken:  "lease-1",
	}

	if err := worker.dispatchOne(context.Background(), item); err != nil {
		t.Fatalf("发布 Outbox 失败：%v", err)
	}
	if publisher.topic != "msg" || publisher.key != "c1" || string(publisher.payload) != "wire-payload" {
		t.Fatalf("发布参数错误：%+v", publisher)
	}
	if !repository.markedSent {
		t.Fatal("发布成功后应标记为已发送")
	}
	if repository.lockToken != "lease-1" {
		t.Fatalf("标记已发送时未传递租约 token：%s", repository.lockToken)
	}
}

func TestWorkerMarksExhaustedItemDead(t *testing.T) {
	repository := &outboxRepositoryStub{}
	worker := NewWorker(nil, repository, &publisherStub{err: errors.New("消息队列不可用")}, configs.OutboxConfig{
		BatchSize:            10,
		PollIntervalSeconds:  2,
		StaleAfterSeconds:    30,
		BaseRetryWaitSeconds: 2,
		MaxRetries:           10,
		WorkerCount:          2,
		QueueSize:            4,
	})
	item := &outboxport.Entry{ID: "o1", RetryCount: worker.options.MaxRetries, LockToken: "lease-2"}

	if err := worker.dispatchOne(context.Background(), item); err != nil {
		t.Fatalf("标记死信失败：%v", err)
	}
	if !repository.markedDead || repository.retried {
		t.Fatalf("超过重试次数后状态错误：%+v", repository)
	}
	if repository.lockToken != "lease-2" {
		t.Fatalf("标记死信时未传递租约 token：%s", repository.lockToken)
	}
}
