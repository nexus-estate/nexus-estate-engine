package app

import (
	"context"
	"go.uber.org/zap"
	"net"

	"github.com/nexus-estate/nexus-estate-engine/internal/platform/config"
	"github.com/nexus-estate/nexus-estate-engine/internal/platform/grpcserver"
	"github.com/nexus-estate/nexus-estate-engine/internal/platform/logging"
)

// RunCore hosts only the synchronous runtime foundation and standard gRPC health.
func RunCore(ctx context.Context) error {
	cfg, err := config.Load("core")
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
	logger.Info("starting grpc core", zap.String("address", listener.Addr().String()))
	return grpcserver.Serve(ctx, listener, server, health, "", nil)
}
