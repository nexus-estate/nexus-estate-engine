package shutdown

import (
	"context"
	"google.golang.org/grpc"
	"os/signal"
	"syscall"
	"time"
)

const Timeout = 10 * time.Second

func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

func GRPC(server *grpc.Server, timeout time.Duration) {
	done := make(chan struct{})
	go func() { server.GracefulStop(); close(done) }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		server.Stop()
		<-done
	}
}
