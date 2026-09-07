package app

import (
	"context"
	"fmt"
	"net"
	"time"

	searchv1 "github.com/nexus-estate/nexus-estate-platform-engine/gen/search/v1"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/config"
	esinfra "github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/elasticsearch"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/grpcserver"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/logging"
	redisinfra "github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/redis"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/search"
	"go.uber.org/zap"
)

func RunSearch(ctx context.Context) error {
	cfg, err := config.Load("search")
	if err != nil {
		return err
	}
	logger, err := logging.New(cfg.App.Name, cfg.App.Env)
	if err != nil {
		return err
	}
	defer func() { _ = logger.Sync() }()
	esClient, err := esinfra.NewClient(cfg.Elasticsearch)
	if err != nil {
		return err
	}
	defer esinfra.Close(esClient)
	cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	redisClient, err := redisinfra.NewClient(cacheCtx, cfg.Redis)
	cancel()
	if err != nil {
		logger.Warn("redis unavailable; running without cache", zap.Error(err))
	}
	if redisClient != nil {
		defer func() { _ = redisClient.Close() }()
	}
	repo := search.NewElasticsearchRepository(esClient, cfg.Elasticsearch.PropertyIndex)
	var cache search.Cache
	if redisClient != nil {
		cache = redisinfra.NewCache(redisClient)
	}
	service := search.NewService(repo, cache, cfg.Redis.SearchTTLSeconds)
	server, health := grpcserver.New(cfg.App.GRPCReflectionEnabled)
	searchv1.RegisterSearchServiceServer(server, search.NewSearchServer(service))
	listener, err := net.Listen("tcp", ":"+cfg.App.GRPCPort)
	if err != nil {
		return fmt.Errorf("listen search: %w", err)
	}
	logger.Info("starting grpc search", zap.String("address", listener.Addr().String()))
	return grpcserver.Serve(ctx, listener, server, health, searchv1.SearchService_ServiceDesc.ServiceName, func(ctx context.Context) error { return esinfra.Ping(ctx, esClient) })
}
