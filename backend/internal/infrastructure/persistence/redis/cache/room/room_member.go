package room

import (
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	roomMemberTTL         = 6 * time.Hour
	roomMemberNegativeTTL = 2 * time.Minute
	roomMemberIDsTTL      = 6 * time.Hour
	roomMembersLoaded     = "__members_cache_loaded__"
)

var _ roomcache.RoomMemberCache = (*RoomMemberCache)(nil)

type RoomMemberCache struct {
	store *shared.Store
}

func NewRoomMemberCache(client *redis.Client) *RoomMemberCache {
	return &RoomMemberCache{store: shared.NewStore(client)}
}

type roomMemberEntry struct {
	Found     bool   `json:"found"`
	Status    int    `json:"status,omitempty"`
	Role      int    `json:"role,omitempty"`
	MuteUntil *int64 `json:"muteUntil,omitempty"`
}

func (c *RoomMemberCache) GetMember(
	ctx context.Context,
	roomID, userID string,
) (*roomcache.MemberState, bool, error) {
	key := RoomMemberKey(roomID, userID)
	data, err := c.store.Client().Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var entry roomMemberEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		_ = c.store.Del(ctx, key)
		return nil, false, fmt.Errorf("解析房间成员缓存失败：%w", err)
	}
	if !entry.Found {
		return nil, true, nil
	}
	return &roomcache.MemberState{
		Status:    roomvo.RoomUserStatus(entry.Status),
		Role:      roomvo.Role(entry.Role),
		MuteUntil: entry.MuteUntil,
	}, true, nil
}

func (c *RoomMemberCache) SetMember(
	ctx context.Context,
	roomID, userID string,
	state *roomcache.MemberState,
) error {
	if state == nil {
		return fmt.Errorf("房间成员状态不能为空")
	}
	return c.setMemberEntry(ctx, roomID, userID, roomMemberEntry{
		Found:     true,
		Status:    int(state.Status),
		Role:      int(state.Role),
		MuteUntil: state.MuteUntil,
	}, roomMemberTTL)
}

func (c *RoomMemberCache) SetMemberNotFound(ctx context.Context, roomID, userID string) error {
	return c.setMemberEntry(ctx, roomID, userID, roomMemberEntry{}, roomMemberNegativeTTL)
}

func (c *RoomMemberCache) DeleteMember(ctx context.Context, roomID, userID string) error {
	return c.store.Del(ctx, RoomMemberKey(roomID, userID))
}

func (c *RoomMemberCache) setMemberEntry(
	ctx context.Context,
	roomID, userID string,
	entry roomMemberEntry,
	ttl time.Duration,
) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return c.store.Client().Set(ctx, RoomMemberKey(roomID, userID), data, ttl).Err()
}

func (c *RoomMemberCache) SetMemberIDs(ctx context.Context, roomID string, userIDs []string) error {
	args := make([]any, 0, len(userIDs)+2)
	args = append(args, roomMembersLoaded)
	for _, userID := range userIDs {
		args = append(args, userID)
	}
	args = append(args, int64(roomMemberIDsTTL/time.Second))

	const script = `
		redis.call('DEL', KEYS[1])
		for i = 1, #ARGV - 1 do
			redis.call('SADD', KEYS[1], ARGV[i])
		end
		redis.call('EXPIRE', KEYS[1], ARGV[#ARGV])
		return 1
	`
	_, err := c.store.Eval(ctx, script, []string{RoomMembersKey(roomID)}, args...)
	return err
}

func (c *RoomMemberCache) GetMemberIDs(ctx context.Context, roomID string) ([]string, bool, error) {
	const script = `
		if redis.call('EXISTS', KEYS[1]) == 0 then
			return {0, {}}
		end
		return {1, redis.call('SMEMBERS', KEYS[1])}
	`
	result, err := c.store.Eval(ctx, script, []string{RoomMembersKey(roomID)})
	if err != nil {
		return nil, false, err
	}
	data, ok := result.([]any)
	if !ok || len(data) != 2 {
		return nil, false, fmt.Errorf("房间成员列表缓存结果无效")
	}
	cached, ok := data[0].(int64)
	if !ok {
		return nil, false, fmt.Errorf("房间成员列表缓存标记无效")
	}
	rawMembers, ok := data[1].([]any)
	if !ok {
		return nil, false, fmt.Errorf("房间成员列表缓存值无效")
	}

	members := make([]string, 0, len(rawMembers))
	for _, raw := range rawMembers {
		var member string
		switch value := raw.(type) {
		case string:
			member = value
		case []byte:
			member = string(value)
		default:
			return nil, false, fmt.Errorf("房间成员标识类型无效：%T", raw)
		}
		if member != roomMembersLoaded {
			members = append(members, member)
		}
	}
	return members, cached == 1, nil
}

func (c *RoomMemberCache) DeleteMemberIDs(ctx context.Context, roomID string) error {
	return c.store.Del(ctx, RoomMembersKey(roomID))
}
