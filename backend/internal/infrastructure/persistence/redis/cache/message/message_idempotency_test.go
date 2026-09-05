package message

import (
	messagecache "IM_backend/internal/application/ports/persistence/cache/message"
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestMessageDedupIsScopedBySender(t *testing.T) {
	redisServer := miniredis.RunT(t)
	cache := NewMessageCache(redis.NewClient(&redis.Options{Addr: redisServer.Addr()}))
	ctx := context.Background()

	entry := messagecache.DedupEntry{
		MessageID:      "message-1",
		RequestHash:    "hash-1",
		ConversationID: "conversation-1",
		Seq:            12,
		AttachmentID:   "attachment-1",
		SendTime:       1710000000000,
		SenderUsername: "alice",
	}
	if ok, err := cache.SetDedupEntry(ctx, "sender-1", "client-1", entry, time.Minute); err != nil || !ok {
		t.Fatalf("写入发送者 1 的去重记录失败: ok=%v err=%v", ok, err)
	}
	got, err := cache.GetDedupEntry(ctx, "sender-1", "client-1")
	if err != nil || got != entry {
		t.Fatalf("读取发送者 1 的去重记录错误: got=%q err=%v", got, err)
	}

	got, err = cache.GetDedupEntry(ctx, "sender-2", "client-1")
	if err != nil {
		t.Fatalf("读取发送者 2 的去重记录失败: %v", err)
	}
	if got.MessageID != "" {
		t.Fatalf("不同发送者不应命中同一 clientMsgId: got=%q", got)
	}
}
