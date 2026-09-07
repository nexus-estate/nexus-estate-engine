package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/nexus-estate/nexus-estate-engine/internal/platform/config"
)

// Cache operations must not consume the Search request deadline.
const cacheTimeout = 500 * time.Millisecond

func NewClient(ctx context.Context, cfg config.RedisConfig) (*goredis.Client, error) {
	client := newClient(cfg)

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}

// Cache adapts Redis without exposing driver types to bounded modules.
type Cache struct{ client *goredis.Client }

func NewCache(client *goredis.Client) *Cache { return &Cache{client: client} }
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, cacheTimeout)
	defer cancel()
	return c.client.Get(ctx, key).Result()
}
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, cacheTimeout)
	defer cancel()
	return c.client.Set(ctx, key, value, ttl).Err()
}

func newClient(cfg config.RedisConfig) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:                  cfg.Addr,
		Password:              cfg.Password,
		DB:                    cfg.DB,
		MaxRetries:            -1,
		DialerRetries:         1,
		DialTimeout:           cacheTimeout,
		ReadTimeout:           cacheTimeout,
		WriteTimeout:          cacheTimeout,
		PoolTimeout:           cacheTimeout,
		ContextTimeoutEnabled: true,
	})
}
