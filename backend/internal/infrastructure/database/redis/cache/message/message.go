package message_cache

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

type MessageCache struct {
	rb *redis.Client
}

func NewMessageCache(rb *redis.Client) *MessageCache {
	return &MessageCache{
		rb: rb,
	}
}

func (mc *MessageCache) GetConvLatestSeq(ctx context.Context, convId string) (int64, error) {
	key := ConversationSeqKeys(convId)

	seq, err := mc.rb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 当前会话不存在, 创建会话
			initSeq := int64(0)

			ok, err := mc.rb.SetArgs(ctx, key, initSeq, redis.SetArgs{
				Mode: "NX",
			}).Result()

			if err != nil {
				return 0, err
			}

			if ok == "OK" {
				return initSeq, nil
			}

			// double read，key 已经被创建了
			seq, err := mc.rb.Get(ctx, key).Int64()

			if err != nil {
				return 0, err
			}

			return seq, nil
		}

		return 0, err
	}

	return seq, nil
}
