package app

import (
	"context"

	"github.com/nexus-estate/nexus-estate-engine/internal/platform/config"
	"github.com/nexus-estate/nexus-estate-engine/internal/platform/logging"
)

// RunWorker has no work sources until an event contract is implemented.
func RunWorker(ctx context.Context) error {
	cfg, err := config.Load("worker")
	if err != nil {
		return err
	}
	logger, err := logging.New(cfg.App.Name, cfg.App.Env)
	if err != nil {
		return err
	}
	defer func() { _ = logger.Sync() }()
	logger.Info("worker bootstrap started")
	<-ctx.Done()
	logger.Info("worker stopped")
	return nil
}
