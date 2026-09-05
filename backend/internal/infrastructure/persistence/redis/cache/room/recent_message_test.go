package room

import (
	"IM_backend/configs"
	"IM_backend/internal/shared/protocol"
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newRecentMessageCacheTest(t *testing.T) (*RoomCache, *redis.Client, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})

	return NewRoomCache(client, configs.MessageConfig{
		RoomActivityWindowSeconds:  60,
		RoomActivityBucketSeconds:  10,
		RoomActivityKeyTTLSeconds:  120,
		RoomActivityWarnMessages:   30,
		RoomActivityActiveMessages: 100,
	}), client, server
}

func TestRoomCacheRecentMessageZSetRoundTrip(t *testing.T) {
	cache, client, _ := newRecentMessageCacheTest(t)
	ctx := context.Background()

	first := protocol.MessageEvent{
		MessageId:      "message-1",
		ConversationId: "room-1",
		SenderId:       "user-1",
		SenderUsername: "alice",
		Seq:            10,
		ConvType:       protocol.RoomChat,
		CType:          1,
		Content:        "hello",
		AttachmentId:   "attachment-1",
	}
	second := first
	second.MessageId = "message-2"
	second.Seq = 11
	second.Content = "world"

	if err := cache.AppendRecentMessageSeq(ctx, "room-1", first); err != nil {
		t.Fatal(err)
	}
	if err := cache.AppendRecentMessageSeq(ctx, "room-1", second); err != nil {
		t.Fatal(err)
	}

	page, err := cache.ListMessageAfterSeq(ctx, "room-1", 10, 11)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Covered || len(page.Events) != 1 {
		t.Fatalf("近期消息分页结果错误：page=%+v", page)
	}
	if page.Events[0].MessageId != "message-2" || page.Events[0].Seq != 11 {
		t.Fatalf("近期消息内容错误：event=%+v", page.Events[0])
	}

	members, err := client.ZRange(ctx, RecentMessageSeqKey("room-1"), 0, -1).Result()
	if err != nil || len(members) != 2 {
		t.Fatalf("ZSET 内容错误：members=%v err=%v", members, err)
	}
}

func TestRoomCacheRecentMessageCoverage(t *testing.T) {
	cache, _, _ := newRecentMessageCacheTest(t)
	ctx := context.Background()

	if err := cache.AppendRecentMessageSeq(ctx, "room-1", protocol.MessageEvent{
		MessageId: "message-100",
		Seq:       100,
		ConvType:  protocol.RoomChat,
	}); err != nil {
		t.Fatal(err)
	}

	page, err := cache.ListMessageAfterSeq(ctx, "room-1", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if page.Covered {
		t.Fatalf("缓存起点晚于 afterSeq 时不应命中：page=%+v", page)
	}

	page, err = cache.ListMessageAfterSeq(ctx, "room-1", 99, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Covered || len(page.Events) != 1 || page.Events[0].Seq != 100 {
		t.Fatalf("缓存连续区间判断错误：page=%+v", page)
	}
}

func TestRoomCacheRecentMessageRejectsInternalSeqGap(t *testing.T) {
	cache, _, _ := newRecentMessageCacheTest(t)
	ctx := context.Background()

	for _, seq := range []int64{100, 102, 103} {
		if err := cache.AppendRecentMessageSeq(ctx, "room-1", protocol.MessageEvent{
			MessageId: "message",
			Seq:       seq,
			ConvType:  protocol.RoomChat,
		}); err != nil {
			t.Fatal(err)
		}
	}

	page, err := cache.ListMessageAfterSeq(ctx, "room-1", 99, 103)
	if err != nil {
		t.Fatal(err)
	}
	if page.Covered {
		t.Fatalf("存在内部 seq 缺口时不应命中缓存：page=%+v", page)
	}
}

func TestRoomCacheRecentMessageRejectsMissingLatestSeq(t *testing.T) {
	cache, _, _ := newRecentMessageCacheTest(t)
	ctx := context.Background()

	for _, seq := range []int64{100, 101, 102} {
		if err := cache.AppendRecentMessageSeq(ctx, "room-1", protocol.MessageEvent{
			MessageId: "message",
			Seq:       seq,
			ConvType:  protocol.RoomChat,
		}); err != nil {
			t.Fatal(err)
		}
	}

	page, err := cache.ListMessageAfterSeq(ctx, "room-1", 99, 103)
	if err != nil {
		t.Fatal(err)
	}
	if page.Covered {
		t.Fatalf("未覆盖 latestSeq 时不应命中缓存：page=%+v", page)
	}
}

func TestRoomCacheRecentMessageBatchWarm(t *testing.T) {
	cache, client, _ := newRecentMessageCacheTest(t)
	ctx := context.Background()

	events := []protocol.MessageEvent{
		{MessageId: "message-103", Seq: 103, ConvType: protocol.RoomChat, Content: "third"},
		{MessageId: "message-101", Seq: 101, ConvType: protocol.RoomChat, Content: "first"},
		{MessageId: "message-102", Seq: 102, ConvType: protocol.RoomChat, Content: "second"},
	}
	if err := cache.WarmRecentMessageEvents(ctx, "room-1", events); err != nil {
		t.Fatal(err)
	}

	page, err := cache.ListMessageAfterSeq(ctx, "room-1", 100, 103)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Covered || len(page.Events) != 3 {
		t.Fatalf("批量回填后缓存区间错误：page=%+v", page)
	}

	members, err := client.ZRange(ctx, RecentMessageSeqKey("room-1"), 0, -1).Result()
	if err != nil || len(members) != 3 {
		t.Fatalf("批量回填 ZSET 内容错误：members=%v err=%v", members, err)
	}
}

func TestRoomCacheRecentMessageExpiresAfterIdleTTL(t *testing.T) {
	cache, client, server := newRecentMessageCacheTest(t)
	ctx := context.Background()
	key := RecentMessageSeqKey("room-1")

	if err := cache.AppendRecentMessageSeq(ctx, "room-1", protocol.MessageEvent{
		MessageId: "message-1",
		Seq:       1,
		ConvType:  protocol.RoomChat,
	}); err != nil {
		t.Fatal(err)
	}

	server.FastForward(defaultRecentMessageTTL + time.Second)
	if exists, err := client.Exists(ctx, key).Result(); err != nil || exists != 0 {
		t.Fatalf("近期消息 key 应过期：exists=%d err=%v", exists, err)
	}
}
