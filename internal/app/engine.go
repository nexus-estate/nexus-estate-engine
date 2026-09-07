package app

import (
	"context"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/config"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/grpcserver"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/logging"
	"go.uber.org/zap"
	"net"
)

func RunEngine(ctx context.Context) error {
	cfg, err := config.Load("engine")
	if err != nil {
		return err
	}
	logger, err := logging.New(cfg.App.Name, cfg.App.Env)
	if err != nil {
		return err
	}
	defer func() { _ = logger.Sync() }()
	server, health := grpcserver.New(cfg.App.GRPCReflectionEnabled)
	listener, err := net.Listen("tcp", ":"+cfg.App.GRPCPort)
	if err != nil {
		return err
	}
	logger.Info("starting grpc engine", zap.String("address", listener.Addr().String()))
	return grpcserver.Serve(ctx, listener, server, health, "", nil)
}
