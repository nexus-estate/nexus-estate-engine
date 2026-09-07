package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func TestCoreHealth(t *testing.T) {
	address := os.Getenv("CORE_SMOKE_ADDR")
	if address == "" {
		t.Skip("set CORE_SMOKE_ADDR to test a running Core")
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := healthv1.NewHealthClient(conn).Check(ctx, &healthv1.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != healthv1.HealthCheckResponse_SERVING {
		t.Fatalf("core health=%s", response.Status)
	}
}
