package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Service interface {
	SearchProperties(ctx context.Context, req PropertySearchRequest) (*PropertySearchResponse, error)
}

type service struct {
	repo     Repository
	redis    *goredis.Client
	cacheTTL time.Duration
}

func NewService(repo Repository, redisClient *goredis.Client, cacheTTLSeconds int) Service {
	return &service{
		repo:     repo,
		redis:    redisClient,
		cacheTTL: time.Duration(cacheTTLSeconds) * time.Second,
	}
}

func (s *service) SearchProperties(ctx context.Context, req PropertySearchRequest) (*PropertySearchResponse, error) {
	cacheKey := buildSearchCacheKey(req)

	if s.redis != nil {
		cached, err := s.redis.Get(ctx, cacheKey).Result()
		if err == nil && cached != "" {
			var result PropertySearchResponse
			if json.Unmarshal([]byte(cached), &result) == nil {
				return &result, nil
			}
		}
	}

	result, err := s.repo.SearchProperties(ctx, req)
	if err != nil {
		return nil, err
	}

	if s.redis != nil {
		data, err := json.Marshal(result)
		if err == nil {
			_ = s.redis.Set(ctx, cacheKey, data, s.cacheTTL).Err()
		}
	}

	return result, nil
}

func buildSearchCacheKey(req PropertySearchRequest) string {
	data, _ := json.Marshal(req)
	hash := sha256.Sum256(data)

	return "nexus-estate:search:properties:" + hex.EncodeToString(hash[:])
}
