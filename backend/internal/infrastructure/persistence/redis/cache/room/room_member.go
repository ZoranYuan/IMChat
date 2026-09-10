package room

import (
	"IM_backend/configs"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var _ roomcache.RoomMemberCache = (*RoomMemberCache)(nil)

type RoomMemberCache struct {
	store  *shared.Store
	config configs.MessageConfig
}

func NewRoomMemberCache(client *redis.Client, config configs.MessageConfig) *RoomMemberCache {
	return &RoomMemberCache{
		store:  shared.NewStore(client),
		config: config,
	}
}

type roomMemberTTLSettings struct {
	state    time.Duration
	negative time.Duration
}

func (c *RoomMemberCache) ttlSettings() (roomMemberTTLSettings, error) {
	if c.config.RoomMemberStateTTLSeconds <= 0 {
		return roomMemberTTLSettings{}, fmt.Errorf("成员状态缓存 TTL 必须大于 0")
	}
	if c.config.RoomMemberNegativeTTLSeconds <= 0 {
		return roomMemberTTLSettings{}, fmt.Errorf("成员负缓存 TTL 必须大于 0")
	}
	return roomMemberTTLSettings{
		state:    time.Duration(c.config.RoomMemberStateTTLSeconds) * time.Second,
		negative: time.Duration(c.config.RoomMemberNegativeTTLSeconds) * time.Second,
	}, nil
}

type roomMemberEntry struct {
	Found     bool   `json:"found"`
	Status    int    `json:"status,omitempty"`
	Role      int    `json:"role,omitempty"`
	MuteUntil *int64 `json:"muteUntil,omitempty"`
	Version   int64  `json:"version,omitempty"`
}

func (c *RoomMemberCache) GetMember(
	ctx context.Context,
	roomID, userID string,
) (*roomcache.MemberState, bool, error) {
	key := RoomMemberKey(roomID, userID)
	values, err := c.store.Client().HMGet(ctx, key, "data", "version").Result()
	if err != nil {
		if strings.Contains(err.Error(), "WRONGTYPE") {
			// 清理旧版本的 string key，下一次回源后会按 Hash 重建。
			_ = c.store.Del(ctx, key)
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(values) != 2 || values[0] == nil {
		return nil, false, nil
	}

	data, ok := values[0].(string)
	if !ok {
		if raw, isBytes := values[0].([]byte); isBytes {
			data = string(raw)
		} else {
			return nil, false, fmt.Errorf("房间成员缓存数据类型无效：%T", values[0])
		}
	}

	var entry roomMemberEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		_ = c.store.Del(ctx, key)
		return nil, false, fmt.Errorf("解析房间成员缓存失败：%w", err)
	}
	if !entry.Found {
		return nil, true, nil
	}
	version := entry.Version
	if values[1] != nil {
		versionText, ok := values[1].(string)
		if !ok {
			if raw, isBytes := values[1].([]byte); isBytes {
				versionText = string(raw)
			} else {
				return nil, false, fmt.Errorf("房间成员缓存版本类型无效：%T", values[1])
			}
		}
		version, err = strconv.ParseInt(versionText, 10, 64)
		if err != nil {
			_ = c.store.Del(ctx, key)
			return nil, false, fmt.Errorf("解析房间成员缓存版本失败：%w", err)
		}
	}
	return &roomcache.MemberState{
		Status:    roomvo.RoomUserStatus(entry.Status),
		Role:      roomvo.Role(entry.Role),
		MuteUntil: entry.MuteUntil,
		Version:   version,
	}, true, nil
}

// 幂等性更新缓存
func (c *RoomMemberCache) SetMemberIfVersionGreater(ctx context.Context, roomID, userID string, state *roomcache.MemberState) (bool, error) {
	if state == nil {
		return false, fmt.Errorf("房间成员状态不能为空")
	}
	ttls, err := c.ttlSettings()
	if err != nil {
		return false, err
	}
	data, err := json.Marshal(roomMemberEntry{
		Found: true, Status: int(state.Status), Role: int(state.Role),
		MuteUntil: state.MuteUntil, Version: state.Version,
	})
	if err != nil {
		return false, err
	}
	const script = `
		local current = redis.call('HGET', KEYS[1], 'version')
		if current and tonumber(current) >= tonumber(ARGV[1]) then
		return 0
		end
		redis.call('HSET', KEYS[1], 'data', ARGV[2], 'version', ARGV[1])
		redis.call('EXPIRE', KEYS[1], ARGV[3])
		return 1
	`
	result, err := c.store.Eval(ctx, script, []string{RoomMemberKey(roomID, userID)},
		state.Version, string(data), int64(ttls.state/time.Second))
	if err != nil {
		return false, err
	}
	value, ok := result.(int64)
	return ok && value == 1, nil
}

func (c *RoomMemberCache) SetMemberNotFound(ctx context.Context, roomID, userID string) error {
	ttls, err := c.ttlSettings()
	if err != nil {
		return err
	}
	return c.setMemberEntry(ctx, roomID, userID, roomMemberEntry{}, ttls.negative)
}

func (c *RoomMemberCache) DeleteMember(ctx context.Context, roomID, userID string) error {
	// 保留版本水位，防止旧的 Outbox 事件在失效后重新覆盖新状态。
	return c.store.Client().HDel(ctx, RoomMemberKey(roomID, userID), "data").Err()
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
	pipe := c.store.Client().TxPipeline()
	pipe.HSet(ctx, RoomMemberKey(roomID, userID), "data", data)
	pipe.Expire(ctx, RoomMemberKey(roomID, userID), ttl)
	_, err = pipe.Exec(ctx)
	return err
}
