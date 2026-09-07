package main

import (
	"context"
	"log"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	searchv1 "github.com/nexus-estate/nexus-estate-platform-engine/gen/search/v1"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/config"
	esinfra "github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/elasticsearch"
	redisinfra "github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/redis"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/search"
)

func main() {
	ctx := context.Background()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	esClient, err := esinfra.NewClient(cfg.Elasticsearch)
	if err != nil {
		logger.Fatal("failed to create elasticsearch client", zap.Error(err))
	}

	redisClient, err := redisinfra.NewClient(ctx, cfg.Redis)
	if err != nil {
		logger.Warn("failed to connect redis, service will run without cache", zap.Error(err))
	}

	searchRepo := search.NewElasticsearchRepository(esClient, cfg.Elasticsearch.PropertyIndex)
	searchService := search.NewService(searchRepo, redisClient, cfg.Redis.SearchTTLSeconds)
	searchServer := search.NewSearchServer(searchService)

	listener, err := net.Listen("tcp", ":"+cfg.App.GRPCPort)
	if err != nil {
		logger.Fatal("failed to listen grpc port", zap.Error(err))
	}

	grpcSrv := grpc.NewServer()

	searchv1.RegisterSearchServiceServer(grpcSrv, searchServer)

	if cfg.App.GRPCReflectionEnabled {
		reflection.Register(grpcSrv)
	}

	logger.Info("starting grpc search service",
		zap.String("service", cfg.App.Name),
		zap.String("env", cfg.App.Env),
		zap.String("grpc_port", cfg.App.GRPCPort),
		zap.Bool("grpc_reflection", cfg.App.GRPCReflectionEnabled),
	)

	if err := grpcSrv.Serve(listener); err != nil {
		logger.Fatal("failed to serve grpc", zap.Error(err))
	}
}
