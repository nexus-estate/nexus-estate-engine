package integration

import (
	"context"
	"os"
	"testing"
	"time"

	searchv1 "github.com/nexus-estate/nexus-estate-engine/gen/search/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

// Run against an isolated, seeded Search stack; see deploy/README.md.
func TestSearchRuntime(t *testing.T) {
	address := os.Getenv("SEARCH_INTEGRATION_ADDR")
	if address == "" {
		t.Skip("set SEARCH_INTEGRATION_ADDR to test a running Search")
	}
	keyword := os.Getenv("SEARCH_INTEGRATION_KEYWORD")
	if keyword == "" {
		keyword = "foundationfixture"
	}
	expectedID := os.Getenv("SEARCH_INTEGRATION_EXPECTED_ID")
	if expectedID == "" {
		expectedID = "foundation-1"
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	health := healthv1.NewHealthClient(conn)
	stream, err := health.Watch(ctx, &healthv1.HealthCheckRequest{Service: "nexusestate.search.v1.SearchService"}, grpc.WaitForReady(true))
	if err != nil {
		t.Fatal(err)
	}
	for {
		response, err := stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if response.Status == healthv1.HealthCheckResponse_SERVING {
			break
		}
	}
	response, err := searchv1.NewSearchServiceClient(conn).SearchProperties(ctx, &searchv1.SearchPropertiesRequest{Keyword: keyword, Page: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if response.Page != 1 || response.Limit != 10 || response.Total != 1 || response.TotalPages != 1 || len(response.Items) != 1 || response.Items[0].Id != expectedID {
		t.Fatalf("unexpected fixture result: %+v", response)
	}
}

func TestSearchReadinessDropsWhenElasticsearchStops(t *testing.T) {
	address := os.Getenv("SEARCH_INTEGRATION_ADDR")
	if address == "" || os.Getenv("SEARCH_EXPECT_NOT_SERVING") == "" {
		t.Skip("set SEARCH_INTEGRATION_ADDR and SEARCH_EXPECT_NOT_SERVING to test dependency readiness")
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	health := healthv1.NewHealthClient(conn)
	for {
		response, checkErr := health.Check(ctx, &healthv1.HealthCheckRequest{Service: "nexusestate.search.v1.SearchService"})
		if checkErr == nil && response.Status == healthv1.HealthCheckResponse_NOT_SERVING {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Search readiness did not become NOT_SERVING: %v", ctx.Err())
		case <-time.After(time.Second):
		}
	}
}
