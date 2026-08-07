package message

import (
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

	if ok, err := cache.SetDedupEntry(ctx, "sender-1", "client-1", "message-1", "hash-1", time.Minute); err != nil || !ok {
		t.Fatalf("写入发送者 1 的去重记录失败: ok=%v err=%v", ok, err)
	}
	got, err := cache.GetDedupEntry(ctx, "sender-1", "client-1")
	if err != nil || got.MessageID != "message-1" || got.RequestHash != "hash-1" {
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
