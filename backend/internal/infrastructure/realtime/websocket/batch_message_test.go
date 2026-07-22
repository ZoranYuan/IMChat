package websocket

import (
	"testing"
	"time"
)

type fixedIDGenerator struct{}

func (fixedIDGenerator) Generate() (string, error) {
	return "batch-1", nil
}

func TestAccumulatorFlushPolicySealsCurrentBatch(t *testing.T) {
	accumulator := NewBatchMessageAccumulator(
		MessageBatchConfig{
			MaxMessages: 10,
			MaxBytes:    1024,
			Linger:      time.Second,
		},
		fixedIDGenerator{},
	)

	batches, err := accumulator.Append(
		Message{Op: "msg", Data: []byte("first")},
		AppendPolicyBatch,
	)
	if err != nil {
		t.Fatalf("append batchable message: %v", err)
	}
	if len(batches) != 0 {
		t.Fatalf("batchable message flushed unexpectedly: %d", len(batches))
	}

	batches, err = accumulator.Append(
		Message{Op: "msg_ack", Data: []byte("second")},
		AppendPolicyFlush,
	)
	if err != nil {
		t.Fatalf("append immediate message: %v", err)
	}
	if len(batches) != 1 {
		t.Fatalf("expected one ready batch, got %d", len(batches))
	}

	batch := batches[0]
	if batch.FlushReason != FlushReasonImmediate {
		t.Fatalf("unexpected flush reason: %s", batch.FlushReason)
	}
	if batch.MessageCount() != 2 {
		t.Fatalf("expected two messages, got %d", batch.MessageCount())
	}
	if batch.Messages[0].Op != "msg" || batch.Messages[1].Op != "msg_ack" {
		t.Fatalf("unexpected message order: %+v", batch.Messages)
	}
}

func TestAccumulatorFlushesByMessageCount(t *testing.T) {
	accumulator := NewBatchMessageAccumulator(
		MessageBatchConfig{MaxMessages: 2, MaxBytes: 1024, Linger: time.Second},
		fixedIDGenerator{},
	)

	if batches, err := accumulator.Append(Message{Op: "first"}, AppendPolicyBatch); err != nil || len(batches) != 0 {
		t.Fatalf("append first message: batches=%d err=%v", len(batches), err)
	}
	batches, err := accumulator.Append(Message{Op: "second"}, AppendPolicyBatch)
	if err != nil {
		t.Fatalf("append second message: %v", err)
	}
	if len(batches) != 1 || batches[0].FlushReason != FlushReasonMessageCount {
		t.Fatalf("unexpected count flush: %+v", batches)
	}
}

func TestAccumulatorFlushesByBytes(t *testing.T) {
	accumulator := NewBatchMessageAccumulator(
		MessageBatchConfig{MaxMessages: 10, MaxBytes: 1, Linger: time.Second},
		fixedIDGenerator{},
	)

	batches, err := accumulator.Append(
		Message{Op: "msg", Data: []byte("payload")},
		AppendPolicyBatch,
	)
	if err != nil {
		t.Fatalf("append message: %v", err)
	}
	if len(batches) != 1 || batches[0].FlushReason != FlushReasonBytes {
		t.Fatalf("unexpected bytes flush: %+v", batches)
	}
}

func TestAccumulatorExposesLingerTimer(t *testing.T) {
	accumulator := NewBatchMessageAccumulator(
		MessageBatchConfig{MaxMessages: 10, MaxBytes: 1024, Linger: 10 * time.Millisecond},
		fixedIDGenerator{},
	)

	if _, err := accumulator.Append(Message{Op: "msg"}, AppendPolicyBatch); err != nil {
		t.Fatalf("append message: %v", err)
	}

	select {
	case <-accumulator.TimerC():
		batch, err := accumulator.Flush(FlushReasonLinger)
		if err != nil {
			t.Fatalf("flush linger batch: %v", err)
		}
		if batch == nil || batch.FlushReason != FlushReasonLinger {
			t.Fatalf("unexpected linger batch: %+v", batch)
		}
	case <-time.After(time.Second):
		t.Fatal("linger timer did not fire")
	}
}
