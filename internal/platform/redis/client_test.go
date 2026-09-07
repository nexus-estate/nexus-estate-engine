package redis

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/nexus-estate/nexus-estate-engine/internal/platform/config"
	goredis "github.com/redis/go-redis/v9"
)

func TestCacheBoundsUnresponsiveRedis(t *testing.T) {
	configured := newClient(config.RedisConfig{Addr: "unused:6379"})
	opts := *configured.Options()
	_ = configured.Close()
	// A connected server that never reads or responds must not stall Search.
	opts.Dialer = func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()
		t.Cleanup(func() { _ = server.Close() })
		return client, nil
	}
	client := goredis.NewClient(&opts)
	defer func() { _ = client.Close() }()
	cache := NewCache(client)
	for _, op := range []struct {
		name string
		run  func() error
	}{
		{"get", func() error { _, err := cache.Get(context.Background(), "key"); return err }},
		{"set", func() error { return cache.Set(context.Background(), "key", []byte("value"), time.Minute) }},
	} {
		t.Run(op.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() { done <- op.run() }()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("stalled cache unexpectedly succeeded")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("cache blocked past its budget")
			}
		})
	}
}
