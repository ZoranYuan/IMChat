package message

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"IM_backend/internal/shared/protocol"
)

type noticeTestDelivery func(string, string, []byte, string) error

func (f noticeTestDelivery) DeliverToOnlineRoomMembers(eventType, roomID string, payload []byte, exclude string) error {
	return f(eventType, roomID, payload, exclude)
}

func newNoticeTestBatcher(t *testing.T, delivery noticeTestDelivery, linger time.Duration) *largeRoomMessageBatcher {
	t.Helper()
	batcher := newLargeRoomMessageBatcher(delivery, LargeRoomMessageBatchOptions{Linger: linger, ShardCount: 1})
	t.Cleanup(func() { batcher.close(context.Background()) })
	return batcher
}

func noticeTestEvent(seq int64) protocol.MessageNotifyEvent {
	return protocol.MessageNotifyEvent{ConversationId: "room", MessageId: "message", Seq: seq}
}

func TestRoomNoticeTimerBroadcastsOnlyLatestMessage(t *testing.T) {
	delivered := make(chan protocol.MessageNotifyEvent, 4)
	batcher := newNoticeTestBatcher(t, func(eventType, roomID string, payload []byte, exclude string) error {
		if eventType != protocol.EventRoomMessageNotice || roomID != "room" || exclude != "" {
			t.Errorf("unexpected routing: event=%s room=%s exclude=%s", eventType, roomID, exclude)
		}
		var notice protocol.MessageNotifyEvent
		if err := json.Unmarshal(payload, &notice); err != nil {
			return err
		}
		delivered <- notice
		return nil
	}, 100*time.Millisecond)
	for _, seq := range []int64{100, 102, 101} {
		notice := noticeTestEvent(seq)
		if seq == 102 {
			notice.MessageId = "latest"
		}
		if err := batcher.enqueue(notice); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case notice := <-delivered:
		if notice.Seq != 102 || notice.MessageId != "latest" {
			t.Fatalf("notice=%+v", notice)
		}
	case <-time.After(time.Second):
		t.Fatal("timer did not send latest notice")
	}
	select {
	case notice := <-delivered:
		t.Fatalf("same window broadcast more than once: %+v", notice)
	case <-time.After(120 * time.Millisecond):
	}
}

func TestRoomNoticeCoalescesMessagesAndAcceptsAdditionalRooms(t *testing.T) {
	batcher := newNoticeTestBatcher(t, func(string, string, []byte, string) error { return nil }, time.Hour)
	for seq := int64(1); seq <= 2000; seq++ {
		if err := batcher.enqueue(noticeTestEvent(seq)); err != nil {
			t.Fatalf("existing room update rejected: %v", err)
		}
	}
	shard := batcher.shards[0]
	shard.mu.Lock()
	count, latest := len(shard.pending), shard.pending["room"].notice.Seq
	shard.mu.Unlock()
	if count != 1 || latest != 2000 {
		t.Fatalf("pending rooms=%d latest=%d", count, latest)
	}
	other := noticeTestEvent(1)
	other.ConversationId = "another-room"
	if err := batcher.enqueue(other); err != nil {
		t.Fatalf("additional room should be accepted: %v", err)
	}
}

func TestRoomNoticeFailureMergesWithNewerPendingNotice(t *testing.T) {
	var batcher *largeRoomMessageBatcher
	calls := 0
	var delivered protocol.MessageNotifyEvent
	batcher = newNoticeTestBatcher(t, func(_, _ string, payload []byte, _ string) error {
		calls++
		if calls == 1 {
			if err := batcher.enqueue(noticeTestEvent(200)); err != nil {
				t.Fatal(err)
			}
			return errors.New("Redis unavailable")
		}
		return json.Unmarshal(payload, &delivered)
	}, time.Hour)
	if err := batcher.enqueue(noticeTestEvent(100)); err != nil {
		t.Fatal(err)
	}
	batcher.shards[0].flush(context.Background())
	batcher.shards[0].flush(context.Background())
	if calls != 2 || delivered.Seq != 200 {
		t.Fatalf("calls=%d latest=%d", calls, delivered.Seq)
	}
}

func TestRoomNoticeCloseFlushesAndRejectsFurtherInput(t *testing.T) {
	calls := 0
	batcher := newNoticeTestBatcher(t, func(string, string, []byte, string) error { calls++; return nil }, time.Hour)
	if err := batcher.enqueue(noticeTestEvent(100)); err != nil {
		t.Fatal(err)
	}
	batcher.close(context.Background())
	if calls != 1 {
		t.Fatalf("shutdown broadcasts=%d", calls)
	}
	if err := batcher.enqueue(noticeTestEvent(101)); !errors.Is(err, ErrLargeRoomMessageBatchClosed) {
		t.Fatalf("closed enqueue=%v", err)
	}
	batcher.close(context.Background())
	if calls != 1 {
		t.Fatal("shutdown flushed twice")
	}
}
