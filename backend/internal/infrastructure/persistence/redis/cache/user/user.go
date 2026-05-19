package user

import (
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"

	"github.com/redis/go-redis/v9"
)

type UserCache struct {
	store *shared.Store
}

func NewUserCache(client *redis.Client) *UserCache {
	return &UserCache{store: shared.NewStore(client)}
}
