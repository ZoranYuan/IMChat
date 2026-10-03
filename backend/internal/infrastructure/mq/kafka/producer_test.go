package kafka

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"IM_backend/configs"
	eventbus "IM_backend/internal/application/ports/eventbus"
	outboxport "IM_backend/internal/application/ports/outbox"

	"github.com/IBM/sarama"
)

type producerTestOutboxRepository struct {
	markedBatches [][]string
	markTokens    []string
	retried       []string
	dead          []string
}

func (r *producerTestOutboxRepository) Create(context.Context, *outboxport.Entry) error {
	return nil
}

func (r *producerTestOutboxRepository) CreateBatch(context.Context, []*outboxport.Entry) error {
	return nil
}

func (r *producerTestOutboxRepository) ClaimPending(context.Context, time.Time, time.Time, int) ([]*outboxport.Entry, error) {
	return nil, nil
}

func (r *producerTestOutboxRepository) MarkSentBatch(_ context.Context, ids []string, lockToken string, _ time.Time) error {
	r.markedBatches = append(r.markedBatches, append([]string(nil), ids...))
	r.markTokens = append(r.markTokens, lockToken)
	return nil
}

func (r *producerTestOutboxRepository) MarkRetry(_ context.Context, id, _ string, _ time.Time, _ string) error {
	r.retried = append(r.retried, id)
	return nil
}

func (r *producerTestOutboxRepository) MarkDead(_ context.Context, id, _ string, _ string) error {
	r.dead = append(r.dead, id)
	return nil
}

func (r *producerTestOutboxRepository) WithTx(any) outboxport.OutboxRepository { return r }

type producerTestSyncProducer struct {
	values []string
	failOn string
}

func TestProduceRequiresEventID(t *testing.T) {
	producer := &Producer{}
	if err := producer.Produce(context.Background(), eventbus.IntegrationEvent{Name: "message"}); err == nil {
		t.Fatal("Produce should reject events without event_id")
	}
}

func (p *producerTestSyncProducer) SendMessage(message *sarama.ProducerMessage) (int32, int64, error) {
	encoded, err := message.Value.Encode()
	if err != nil {
		return 0, 0, err
	}
	value := string(encoded)
	p.values = append(p.values, value)
	if value == p.failOn {
		return 0, 0, errors.New("test Kafka publish failure")
	}
	return 0, int64(len(p.values)), nil
}

func (*producerTestSyncProducer) SendMessages([]*sarama.ProducerMessage) error { return nil }
func (*producerTestSyncProducer) Close() error                                 { return nil }
func (*producerTestSyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag      { return 0 }
func (*producerTestSyncProducer) IsTransactional() bool                        { return false }
func (*producerTestSyncProducer) BeginTxn() error                              { return nil }
func (*producerTestSyncProducer) CommitTxn() error                             { return nil }
func (*producerTestSyncProducer) AbortTxn() error                              { return nil }
func (*producerTestSyncProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}
func (*producerTestSyncProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}

func TestDispatchBatchMarksSuccessfulPublishesTogether(t *testing.T) {
	repository := &producerTestOutboxRepository{}
	syncProducer := &producerTestSyncProducer{}
	router, err := NewTopicRouter(map[string]string{"message": "im.message"})
	if err != nil {
		t.Fatal(err)
	}
	producer := &Producer{
		client:     &Client{Producer: syncProducer},
		router:     router,
		repository: repository,
		options:    normalizeProducerConfig(configs.KafkaProducerConfig{}),
	}

	items := []*outboxport.Entry{
		{ID: "event-1", EventType: "message", MessageKey: "room-1", Payload: []byte("one"), LockToken: "lease-a"},
		{ID: "event-2", EventType: "message", MessageKey: "room-1", Payload: []byte("two"), LockToken: "lease-a"},
		{ID: "event-3", EventType: "message", MessageKey: "room-2", Payload: []byte("three"), LockToken: "lease-b"},
	}
	tasks := make([]producerTask, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, producerTask{item: item, submittedAt: time.Now()})
	}

	if err := producer.dispatchBatch(context.Background(), tasks); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(syncProducer.values, []string{"one", "two", "three"}) {
		t.Fatalf("Kafka publish order = %v", syncProducer.values)
	}
	if !reflect.DeepEqual(repository.markedBatches, [][]string{{"event-1", "event-2"}, {"event-3"}}) {
		t.Fatalf("MarkSentBatch calls = %v", repository.markedBatches)
	}
	if !reflect.DeepEqual(repository.markTokens, []string{"lease-a", "lease-b"}) {
		t.Fatalf("lease tokens = %v", repository.markTokens)
	}
}

func TestDispatchBatchDoesNotMarkFailedPublishSent(t *testing.T) {
	repository := &producerTestOutboxRepository{}
	syncProducer := &producerTestSyncProducer{failOn: "bad"}
	router, err := NewTopicRouter(map[string]string{"message": "im.message"})
	if err != nil {
		t.Fatal(err)
	}
	producer := &Producer{
		client:     &Client{Producer: syncProducer},
		router:     router,
		repository: repository,
		options:    normalizeProducerConfig(configs.KafkaProducerConfig{}),
	}

	tasks := []producerTask{
		{item: &outboxport.Entry{ID: "event-ok", EventType: "message", MessageKey: "room", Payload: []byte("ok"), LockToken: "lease"}},
		{item: &outboxport.Entry{ID: "event-bad", EventType: "message", MessageKey: "room", Payload: []byte("bad"), LockToken: "lease"}},
	}
	if err := producer.dispatchBatch(context.Background(), tasks); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repository.markedBatches, [][]string{{"event-ok"}}) {
		t.Fatalf("only successful event should be marked sent, got %v", repository.markedBatches)
	}
	if !reflect.DeepEqual(repository.retried, []string{"event-bad"}) {
		t.Fatalf("failed event retry updates = %v", repository.retried)
	}
	if len(repository.dead) != 0 {
		t.Fatalf("unexpected dead-letter updates: %v", repository.dead)
	}
}

var _ eventbus.Publisher = (*Producer)(nil)
var _ outboxport.OutboxRepository = (*producerTestOutboxRepository)(nil)
var _ sarama.SyncProducer = (*producerTestSyncProducer)(nil)
