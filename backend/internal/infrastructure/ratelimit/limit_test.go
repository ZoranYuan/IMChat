package ratelimit

import (
	shared_ratelimit "IM_backend/internal/shared/ratelimit"
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisLimitTokenBucket(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	limiter := NewRedisLimit(client, "test:rate:")
	policy := shared_ratelimit.Policy{Rate: 1, Burst: 2}
	for i := 0; i < 2; i++ {
		decision, err := limiter.Allow(context.Background(), "user", policy)
		if err != nil || !decision.Allowed {
			t.Fatalf("request %d should be allowed, allowed=%v err=%v", i, decision.Allowed, err)
		}
	}
	decision, err := limiter.Allow(context.Background(), "user", policy)
	if err != nil {
		t.Fatalf("third request returned error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("third request should be rate limited")
	}
}
