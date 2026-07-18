package outbox

import (
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
}

func (stub *outboxRepositoryStub) Create(context.Context, *outboxport.Entry) error {
	return nil
}
func (stub *outboxRepositoryStub) ClaimPending(context.Context, time.Time, time.Time, int) ([]*outboxport.Entry, error) {
	return nil, nil
}
func (stub *outboxRepositoryStub) MarkSent(context.Context, string, time.Time) error {
	stub.markedSent = true
	return nil
}
func (stub *outboxRepositoryStub) MarkRetry(context.Context, string, time.Time, string) error {
	stub.retried = true
	return nil
}
func (stub *outboxRepositoryStub) MarkDead(context.Context, string, string) error {
	stub.markedDead = true
	return nil
}
func (stub *outboxRepositoryStub) WithTx(any) outboxport.Repository {
	return stub
}

func TestWorkerPublishesStoredPayload(t *testing.T) {
	repository := &outboxRepositoryStub{}
	publisher := &publisherStub{}
	worker := NewWorker(nil, repository, publisher)
	item := &outboxport.Entry{
		ID:         "o1",
		EventType:  "msg",
		MessageKey: "c1",
		Payload:    []byte("wire-payload"),
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
}

func TestWorkerMarksExhaustedItemDead(t *testing.T) {
	repository := &outboxRepositoryStub{}
	worker := NewWorker(nil, repository, &publisherStub{err: errors.New("Kafka 不可用")})
	item := &outboxport.Entry{ID: "o1", RetryCount: worker.maxRetries}

	if err := worker.dispatchOne(context.Background(), item); err != nil {
		t.Fatalf("标记死信失败：%v", err)
	}
	if !repository.markedDead || repository.retried {
		t.Fatalf("超过重试次数后状态错误：%+v", repository)
	}
}
