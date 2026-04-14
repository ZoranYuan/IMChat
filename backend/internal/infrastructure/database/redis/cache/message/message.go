package message_cache

import (
	message_entity "IM_backend/internal/domain/message/entity"
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

func (mc *MessageCache) IncrMessageLatestSeq(ctx context.Context, convId string) (int64, error) {
	key := ConversationSeqKeys(convId)

	r, err := mc.rb.Incr(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return r, message_entity.ErrConversationNotCreated
		}

		return r, err
	}

	return r, nil
}

func (mc *MessageCache) GetMessageLatestSeq(ctx context.Context, convId string) (int64, error) {
	key := ConversationSeqKeys(convId)
	r, err := mc.rb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return r, message_entity.ErrConversationNotCreated
		}

		return r, err
	}

	return r, nil
}

func (mc *MessageCache) SetMessageSeq(ctx context.Context, convId string, seq int64) error {
	key := ConversationSeqKeys(convId)
	return mc.rb.SetArgs(ctx, key, seq, redis.SetArgs{
		Mode: "NX",
	}).Err()
}
