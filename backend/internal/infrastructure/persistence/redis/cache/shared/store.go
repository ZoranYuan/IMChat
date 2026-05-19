package shared

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
}

func NewStore(client *redis.Client) *Store {
	return &Store{client: client}
}

func (s *Store) Client() *redis.Client {
	return s.client
}

func (s *Store) SetString(ctx context.Context, key string, value string, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *Store) GetString(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return value, err
}

func (s *Store) GetRequiredString(ctx context.Context, key string) (string, error) {
	return s.client.Get(ctx, key).Result()
}

func (s *Store) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, data, ttl).Err()
}

func (s *Store) GetJSON(ctx context.Context, key string, target any) (bool, error) {
	data, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(data, target)
}

func (s *Store) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.client.Del(ctx, keys...).Err()
}

func (s *Store) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return s.client.Expire(ctx, key, ttl).Err()
}

func (s *Store) SetNXString(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	result, err := s.client.SetArgs(ctx, key, value, redis.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Result()
	if err != nil {
		return false, err
	}
	return result == "OK", nil
}

func (s *Store) SetNXInt64(ctx context.Context, key string, value int64) error {
	return s.client.SetArgs(ctx, key, value, redis.SetArgs{
		Mode: "NX",
	}).Err()
}

func (s *Store) GetInt64(ctx context.Context, key string) (int64, error) {
	return s.client.Get(ctx, key).Int64()
}

func (s *Store) Incr(ctx context.Context, key string) (int64, error) {
	return s.client.Incr(ctx, key).Result()
}

func (s *Store) SIsMember(ctx context.Context, key string, member string) (bool, error) {
	return s.client.SIsMember(ctx, key, member).Result()
}

func (s *Store) SAddStrings(ctx context.Context, key string, values ...string) error {
	if len(values) == 0 {
		return nil
	}

	members := make([]interface{}, 0, len(values))
	for _, value := range values {
		members = append(members, value)
	}

	return s.client.SAdd(ctx, key, members...).Err()
}

func (s *Store) SMembers(ctx context.Context, key string) ([]string, error) {
	return s.client.SMembers(ctx, key).Result()
}

func (s *Store) SAddPair(ctx context.Context, key1, member1, key2, member2 string) error {
	pipe := s.client.Pipeline()
	pipe.SAdd(ctx, key1, member1)
	pipe.SAdd(ctx, key2, member2)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) SRemPair(ctx context.Context, key1, member1, key2, member2 string) error {
	pipe := s.client.Pipeline()
	pipe.SRem(ctx, key1, member1)
	pipe.SRem(ctx, key2, member2)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return s.client.Eval(ctx, script, keys, args...).Result()
}
