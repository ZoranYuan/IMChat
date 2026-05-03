package user

import "github.com/redis/go-redis/v9"

type UserCache struct {
	client *redis.Client
}
