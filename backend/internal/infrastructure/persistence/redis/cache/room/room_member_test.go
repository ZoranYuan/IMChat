package room

import (
	"IM_backend/configs"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	roomvo "IM_backend/internal/domain/room/value_object"
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newRoomMemberCacheTest(t *testing.T) (*RoomMemberCache, *redis.Client, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return NewRoomMemberCache(client, configs.MessageConfig{
		RoomMemberStateTTLSeconds:    3600,
		RoomMemberNegativeTTLSeconds: 120,
	}), client, server
}

func memberState(version int64, status roomvo.RoomUserStatus) *roomcache.MemberState {
	return &roomcache.MemberState{
		Status:  status,
		Role:    roomvo.RegularUser,
		Version: version,
	}
}

func TestRoomMemberCacheVersionIsIdempotent(t *testing.T) {
	cache, _, _ := newRoomMemberCacheTest(t)
	ctx := context.Background()

	updated, err := cache.SetMemberIfVersionGreater(ctx, "room-1", "user-1", memberState(2, roomvo.Left))
	if err != nil || !updated {
		t.Fatalf("写入新版本失败：updated=%v err=%v", updated, err)
	}

	updated, err = cache.SetMemberIfVersionGreater(ctx, "room-1", "user-1", memberState(1, roomvo.Activate))
	if err != nil || updated {
		t.Fatalf("旧版本不应覆盖新版本：updated=%v err=%v", updated, err)
	}

	state, hit, err := cache.GetMember(ctx, "room-1", "user-1")
	if err != nil || !hit || state == nil {
		t.Fatalf("读取成员缓存失败：state=%+v hit=%v err=%v", state, hit, err)
	}
	if state.Version != 2 || state.Status != roomvo.Left {
		t.Fatalf("缓存被旧版本覆盖：state=%+v", state)
	}
}

func TestRoomMemberCacheDeleteKeepsVersionWatermark(t *testing.T) {
	cache, client, _ := newRoomMemberCacheTest(t)
	ctx := context.Background()
	key := RoomMemberKey("room-1", "user-1")

	if _, err := cache.SetMemberIfVersionGreater(ctx, "room-1", "user-1", memberState(5, roomvo.Activate)); err != nil {
		t.Fatal(err)
	}
	if err := cache.DeleteMember(ctx, "room-1", "user-1"); err != nil {
		t.Fatal(err)
	}

	data, err := client.HGet(ctx, key, "data").Result()
	if err != redis.Nil || data != "" {
		t.Fatalf("data 字段应被删除：data=%q err=%v", data, err)
	}
	version, err := client.HGet(ctx, key, "version").Int64()
	if err != nil || version != 5 {
		t.Fatalf("version 字段应保留：version=%d err=%v", version, err)
	}

	updated, err := cache.SetMemberIfVersionGreater(ctx, "room-1", "user-1", memberState(4, roomvo.Activate))
	if err != nil || updated {
		t.Fatalf("删除后旧事件不应回写：updated=%v err=%v", updated, err)
	}
	updated, err = cache.SetMemberIfVersionGreater(ctx, "room-1", "user-1", memberState(6, roomvo.Activate))
	if err != nil || !updated {
		t.Fatalf("新事件应能回写：updated=%v err=%v", updated, err)
	}
}

func TestRoomMemberCacheNegativeEntryDoesNotResetVersion(t *testing.T) {
	cache, client, _ := newRoomMemberCacheTest(t)
	ctx := context.Background()
	key := RoomMemberKey("room-1", "user-1")

	if _, err := cache.SetMemberIfVersionGreater(ctx, "room-1", "user-1", memberState(7, roomvo.Activate)); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetMemberNotFound(ctx, "room-1", "user-1"); err != nil {
		t.Fatal(err)
	}

	version, err := client.HGet(ctx, key, "version").Int64()
	if err != nil || version != 7 {
		t.Fatalf("负缓存不应重置版本：version=%d err=%v", version, err)
	}
	state, hit, err := cache.GetMember(ctx, "room-1", "user-1")
	if err != nil || !hit || state != nil {
		t.Fatalf("应读取到负缓存：state=%+v hit=%v err=%v", state, hit, err)
	}
}

func TestRoomMemberCacheClearsLegacyStringKey(t *testing.T) {
	cache, client, server := newRoomMemberCacheTest(t)
	ctx := context.Background()
	key := RoomMemberKey("room-1", "user-1")
	legacy, err := json.Marshal(roomMemberEntry{Found: true, Status: int(roomvo.Activate)})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Set(ctx, key, legacy, 0).Err(); err != nil {
		t.Fatal(err)
	}

	state, hit, err := cache.GetMember(ctx, "room-1", "user-1")
	if err != nil || hit || state != nil {
		t.Fatalf("旧 string 缓存应视为未命中：state=%+v hit=%v err=%v", state, hit, err)
	}
	if server.Exists(key) {
		t.Fatalf("旧 string key 应被清理")
	}
}
