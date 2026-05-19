package room

import (
	roomentity "IM_backend/internal/domain/room/entity"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
)

type RoomCache struct {
	store *shared.Store
}

func NewRoomCache(rb *redis.Client) *RoomCache {
	return &RoomCache{
		store: shared.NewStore(rb),
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

	oldCode, err := rc.store.GetRequiredString(ctx, roomKey)

	if err != nil && !errors.Is(err, redis.Nil) {
		return "", err
	}

	if err == nil {
		_ = rc.store.Expire(ctx, roomKey, ttlDur)
		_ = rc.store.Expire(ctx, InviteKey(oldCode), ttlDur)
		return oldCode, nil
	}

	for i := 0; i < 9; i++ {

		code, err := rc.randCode(9)
		if err != nil {
			continue
		}

		ok, err := rc.store.SetNXString(ctx, InviteKey(code), roomId, ttlDur)

		if err != nil {
			continue
		}

		if !ok {
			continue
		}

		err = rc.store.SetString(
			ctx,
			roomKey,
			code,
			ttlDur,
		)

		if err != nil {
			_ = rc.store.Del(ctx, InviteKey(code))
			continue
		}

		return code, nil
	}

	return "", ErrInviteCodeGenerationFailed
}

func (rc *RoomCache) GetInviteCode(ctx context.Context, roomId string) (string, error) {
	inviteCode, err := rc.store.GetRequiredString(ctx, RoomInviteKey(roomId))

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", roomentity.ErrRoomNotFound
		}

		return "", err
	}

	return inviteCode, err
}

func (rc *RoomCache) GetRoomIDByCode(ctx context.Context, code string) (string, error) {
	roomId, err := rc.store.GetRequiredString(ctx, InviteKey(code))

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", roomentity.ErrInviteCodeExpired
		}
	}

	return roomId, nil
}
