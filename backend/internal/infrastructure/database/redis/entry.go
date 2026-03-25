package redis

import (
	"log"

	"github.com/redis/go-redis/v9"
)

func InitRedis(dsn string) *redis.Client {
	opt, err := redis.ParseURL(dsn)
	if err != nil {
		log.Fatal("failed to init redis, ", err)
	}

	return redis.NewClient(opt)
}
