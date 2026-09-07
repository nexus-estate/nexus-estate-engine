package grpcserver

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func TestReadinessAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ready func(context.Context) error
		want  healthv1.HealthCheckResponse_ServingStatus
	}{
		{"engine", nil, healthv1.HealthCheckResponse_SERVING},
		{"search ES reachable", func(context.Context) error { return nil }, healthv1.HealthCheckResponse_SERVING},
		{"search ES down", func(context.Context) error { return errors.New("unavailable") }, healthv1.HealthCheckResponse_NOT_SERVING},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			listener := bufconn.Listen(1024 * 1024)
			server, h := New(false)
			done := make(chan error, 1)
			go func() { done <- Serve(ctx, listener, server, h, "test-service", tc.ready) }()
			conn, err := grpc.NewClient("passthrough:///test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.Close() }()
			probeCtx, stop := context.WithTimeout(ctx, 3*time.Second)
			defer stop()
			client := healthv1.NewHealthClient(conn)
			// Watch waits for the monitor's first result without sleep-based polling.
			stream, err := client.Watch(probeCtx, &healthv1.HealthCheckRequest{Service: "test-service"})
			if err != nil {
				t.Fatal(err)
			}
			for {
				response, err := stream.Recv()
				if err != nil {
					t.Fatal(err)
				}
				if response.Status == tc.want {
					break
				}
			}
			stop()
			response, err := client.Check(ctx, &healthv1.HealthCheckRequest{})
			if err != nil || response.Status != tc.want {
				t.Fatalf("global health=%v err=%v", response, err)
			}
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("server did not stop")
			}
		})
	}
}
