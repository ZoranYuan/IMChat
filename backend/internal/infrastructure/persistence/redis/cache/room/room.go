package room

import (
	roomentity "IM_backend/internal/domain/room/entity"
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
)

type RoomCache struct {
	rb *redis.Client
}

func NewRoomCache(rb *redis.Client) *RoomCache {
	return &RoomCache{
		rb: rb,
	}
}

func (rc *RoomCache) randCode(count int) (string, error) {
	letters := "0123456789"

	maxL := big.NewInt(int64(len(letters)))

	b := make([]byte, count)
	for i := 0; i < count; i++ {
		num, err := rand.Int(rand.Reader, maxL)
		if err != nil {
			return "", err
		}

		b[i] = letters[num.Int64()]
	}

	return string(b), nil
}

func (rc *RoomCache) UpdateInviteCode(
	ctx context.Context,
	roomId string,
	ttl int,
) (string, error) {

	roomKey := RoomInviteKey(roomId)
	ttlDur := time.Duration(ttl) * time.Minute

	oldCode, err := rc.rb.Get(ctx, roomKey).Result()

	if err != nil && !errors.Is(err, redis.Nil) {
		return "", err
	}

	if err == nil {
		_ = rc.rb.Expire(ctx, roomKey, ttlDur)
		_ = rc.rb.Expire(ctx, InviteKey(oldCode), ttlDur)
		return oldCode, nil
	}

	for i := 0; i < 9; i++ {

		code, err := rc.randCode(9)
		if err != nil {
			continue
		}

		ok, err := rc.rb.SetArgs(
			ctx,
			InviteKey(code),
			roomId,
			redis.SetArgs{
				Mode: "NX",
				TTL:  ttlDur,
			},
		).Result()

		if err != nil {
			continue
		}

		if ok != "OK" {
			continue
		}

		err = rc.rb.Set(
			ctx,
			roomKey,
			code,
			ttlDur,
		).Err()

		if err != nil {
			rc.rb.Del(ctx, InviteKey(code))
			continue
		}

		return code, nil
	}

	return "", ErrInviteCodeGenerationFailed
}

func (rc *RoomCache) GetInviteCode(ctx context.Context, roomId string) (string, error) {
	inviteCode, err := rc.rb.Get(ctx, RoomInviteKey(roomId)).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", roomentity.ErrRoomNotFound
		}

		return "", err
	}

	return inviteCode, err
}

func (rc *RoomCache) GetRoomIDByCode(ctx context.Context, code string) (string, error) {
	roomId, err := rc.rb.Get(ctx, InviteKey(code)).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", roomentity.ErrInviteCodeExpired
		}
	}

	return roomId, nil
}
