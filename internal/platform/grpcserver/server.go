package grpcserver

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/shutdown"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func New(reflect bool) (*grpc.Server, *health.Server) {
	server := grpc.NewServer()
	h := health.NewServer()
	h.SetServingStatus("", healthv1.HealthCheckResponse_NOT_SERVING)
	healthv1.RegisterHealthServer(server, h)
	if reflect {
		reflection.Register(server)
	}
	return server, h
}

// Serve owns the listener and joins the readiness monitor before stopping health.
// An empty dependency check means this runtime has no required dependencies yet.
func Serve(ctx context.Context, listener net.Listener, server *grpc.Server, h *health.Server, service string, ready func(context.Context) error) error {
	defer func() { _ = listener.Close() }()
	monitorCtx, cancel := context.WithCancel(ctx)
	monitored := make(chan struct{})
	go func() {
		defer close(monitored)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			status := healthv1.HealthCheckResponse_SERVING
			if ready != nil {
				probeCtx, stop := context.WithTimeout(monitorCtx, 2*time.Second)
				err := ready(probeCtx)
				stop()
				if err != nil {
					status = healthv1.HealthCheckResponse_NOT_SERVING
				}
			}
			h.SetServingStatus("", status)
			h.SetServingStatus(service, status)
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	var err error
	select {
	case err = <-served:
	case <-ctx.Done():
	}
	cancel()
	<-monitored
	h.Shutdown()
	shutdown.GRPC(server, shutdown.Timeout)
	if errors.Is(err, grpc.ErrServerStopped) {
		return nil
	}
	return err
}
