package ratelimit

import (
	shared_ratelimit "IM_backend/internal/shared/ratelimit"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed limit.lua
var limitScript string

// 使用 redis 实现分布式限流
type RedisLimit struct {
	client *redis.Client

	perfix string
}

func NewRedisLimit(client *redis.Client, perfix string) *RedisLimit {
	if perfix == "" {
		perfix = "rate_limit:"
	}

	return &RedisLimit{
		client,
		perfix,
	}
}

func (r *RedisLimit) Allow(ctx context.Context, key string, policy shared_ratelimit.Policy) (shared_ratelimit.Decision, error) {
	if r.client == nil {
		return shared_ratelimit.Decision{}, errors.New("限流器缓存客户端不能为空")
	}

	if key == "" {
		return shared_ratelimit.Decision{}, errors.New("限流键不能为空")
	}

	if policy.Rate <= 0 || policy.Burst <= 0 {
		return shared_ratelimit.Decision{}, errors.New("限流策略参数无效")
	}

	// 执行 lua 脚本
	result, err := r.client.Eval(
		ctx,
		limitScript,
		[]string{r.perfix + key},
		strconv.FormatFloat(
			policy.Rate,
			'f',
			-1,
			64,
		),
		policy.Burst,
	).Result()

	if err != nil {
		return shared_ratelimit.Decision{}, err
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 3 {
		return shared_ratelimit.Decision{},
			fmt.Errorf(
				"限流结果格式异常：%T",
				result,
			)
	}

	allowed, err := redisInt64(values[0])
	if err != nil {
		return shared_ratelimit.Decision{}, err
	}

	retryAfterMs, err := redisInt64(values[2])
	if err != nil {
		return shared_ratelimit.Decision{}, err
	}

	return shared_ratelimit.Decision{
		Allowed: allowed == 1,
		RetryAfter: time.Duration(retryAfterMs) *
			time.Millisecond,
	}, nil
}

func redisInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil

	case string:
		return strconv.ParseInt(v, 10, 64)

	case []byte:
		return strconv.ParseInt(
			string(v),
			10,
			64,
		)

	default:
		return 0, fmt.Errorf(
			"缓存整数类型异常：%T",
			value,
		)
	}
}
