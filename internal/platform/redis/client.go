package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"

	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/config"
)

func NewClient(ctx context.Context, cfg config.RedisConfig) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
