package room

import (
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	roomentity "IM_backend/internal/domain/room/entity"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	_ roomcache.RoomCache = (*RoomCache)(nil)
)

var (
	RoomProfileTTL = 24 * 60 * 60
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
	ttl time.Duration,
) (string, error) {

	roomKey := RoomInviteKey(roomId)
	if ttl <= 0 {
		return "", ErrInvalidTTL
	}

	oldCode, err := rc.store.GetRequiredString(ctx, roomKey)

	if err != nil && !errors.Is(err, redis.Nil) {
		return "", err
	}

	if err == nil {
		mappedRoomID, reverseErr := rc.store.GetRequiredString(ctx, InviteKey(oldCode))
		if reverseErr == nil && mappedRoomID == roomId {
			// 设立双向映射
			const refreshScript = `
				redis.call('PEXPIRE', KEYS[1], ARGV[1])
				redis.call('PEXPIRE', KEYS[2], ARGV[1])
				return 1
			`
			if _, err := rc.store.Eval(
				ctx,
				refreshScript,
				[]string{roomKey, InviteKey(oldCode)},
				ttl.Milliseconds(),
			); err != nil {
				return "", err
			}
			return oldCode, nil
		}
		if reverseErr != nil && !errors.Is(reverseErr, redis.Nil) {
			return "", reverseErr
		}
		if err := rc.store.Del(ctx, roomKey); err != nil {
			return "", err
		}
	}

	for range 9 {

		code, err := rc.randCode(9)
		if err != nil {
			continue
		}

		const createScript = `
			local existing = redis.call('GET', KEYS[1])
			if existing then
				return existing
			end
			if redis.call('EXISTS', KEYS[2]) == 1 then
				return ''
			end
			redis.call('PSETEX', KEYS[1], ARGV[3], ARGV[1])
			redis.call('PSETEX', KEYS[2], ARGV[3], ARGV[2])
			return ARGV[1]
		`
		result, err := rc.store.Eval(
			ctx,
			createScript,
			[]string{roomKey, InviteKey(code)},
			code,
			roomId,
			ttl.Milliseconds(),
		)
		if err != nil {
			continue
		}
		createdCode, ok := result.(string)
		if !ok || createdCode == "" {
			continue
		}

		return createdCode, nil
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
		return "", err
	}

	return roomId, nil
}

func (rc *RoomCache) DeleteInviteCode(ctx context.Context, roomId string) error {
	// 双向删除，防止误删
	const script = `
		local code = redis.call('GET', KEYS[1])
		if not code then
			return 0
		end
		redis.call('DEL', KEYS[1])
		local reverseKey = ARGV[1] .. code
		if redis.call('GET', reverseKey) == ARGV[2] then
			redis.call('DEL', reverseKey)
		end
		return 1
	`
	_, err := rc.store.Eval(
		ctx,
		script,
		[]string{RoomInviteKey(roomId)},
		InviteKeyPrefix(),
		roomId,
	)
	return err
}

func (rc *RoomCache) SetRoomProfile(ctx context.Context, room roomentity.Room, ttl time.Duration) error {
	data, err := json.Marshal(room)
	if err != nil {
		return nil
	}

	return rc.store.SetJSON(ctx, RoomProfile(room.RoomId), string(data), ttl)
}
