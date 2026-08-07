package outbox

import (
	outboxport "IM_backend/internal/application/ports/outbox"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOutboxPoolProcessesMultipleEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processed atomic.Int32
	processedDone := make(chan struct{})
	var once sync.Once

	pool := NewOutboxPool(2, 4, func(context.Context, *outboxport.Entry) error {
		if processed.Add(1) == 3 {
			once.Do(func() { close(processedDone) })
		}
		return nil
	})
	pool.Start(ctx)

	for i, key := range []string{"conversation-a", "conversation-b", "conversation-c"} {
		if err := pool.Submit(ctx, &outboxport.Entry{
			ID:         string(rune('1' + i)),
			MessageKey: key,
		}); err != nil {
			t.Fatalf("提交 Outbox 事件失败：%v", err)
		}
	}

	select {
	case <-processedDone:
	case <-time.After(time.Second):
		t.Fatalf("多个 Outbox 事件未全部处理，实际处理数=%d", processed.Load())
	}

	cancel()
	pool.Wait()
}

func TestOutboxPoolCanRestartAfterWorkerExit(t *testing.T) {
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstProcessed := make(chan struct{})
	firstPool := NewOutboxPool(1, 1, func(context.Context, *outboxport.Entry) error {
		close(firstProcessed)
		return nil
	})
	firstPool.Start(firstCtx)

	if err := firstPool.Submit(firstCtx, &outboxport.Entry{ID: "first", MessageKey: "conversation-a"}); err != nil {
		t.Fatalf("第一次提交事件失败：%v", err)
	}
	select {
	case <-firstProcessed:
	case <-time.After(time.Second):
		t.Fatal("第一次 Worker 未处理事件")
	}
	cancelFirst()
	firstPool.Wait()

	secondCtx, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	secondProcessed := make(chan struct{})
	secondPool := NewOutboxPool(1, 1, func(context.Context, *outboxport.Entry) error {
		close(secondProcessed)
		return nil
	})
	secondPool.Start(secondCtx)

	if err := secondPool.Submit(secondCtx, &outboxport.Entry{ID: "reclaimed", MessageKey: "conversation-a"}); err != nil {
		t.Fatalf("重启后提交事件失败：%v", err)
	}
	select {
	case <-secondProcessed:
	case <-time.After(time.Second):
		t.Fatal("重启后的 Worker 未处理重新领取的事件")
	}

	cancelSecond()
	secondPool.Wait()
}
