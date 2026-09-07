package shutdown

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

type blockedHealth struct {
	healthv1.UnimplementedHealthServer
	started  chan struct{}
	finished chan struct{}
}

func (s *blockedHealth) Check(ctx context.Context, _ *healthv1.HealthCheckRequest) (*healthv1.HealthCheckResponse, error) {
	close(s.started)
	<-ctx.Done()
	close(s.finished)
	return nil, ctx.Err()
}

func TestGRPCForcesStopAfterDeadline(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	handler := &blockedHealth{started: make(chan struct{}), finished: make(chan struct{})}
	healthv1.RegisterHealthServer(server, handler)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	defer server.Stop()
	conn, err := grpc.NewClient("passthrough:///test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	callDone := make(chan error, 1)
	go func() {
		_, err := healthv1.NewHealthClient(conn).Check(ctx, &healthv1.HealthCheckRequest{})
		callDone <- err
	}()
	select {
	case <-handler.started:
	case <-ctx.Done():
		t.Fatal("RPC never started")
	}
	stopDone := make(chan struct{})
	go func() { GRPC(server, 20*time.Millisecond); close(stopDone) }()
	select {
	case <-stopDone:
	case <-ctx.Done():
		t.Fatal("forced stop did not complete")
	}
	if err := <-callDone; err == nil {
		t.Fatal("blocked RPC unexpectedly succeeded")
	}
	select {
	case <-handler.finished:
	case <-ctx.Done():
		t.Fatal("handler not cancelled")
	}
	<-served
}
