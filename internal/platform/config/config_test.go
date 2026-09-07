package config

import (
	"github.com/spf13/viper"
	"testing"
)

func TestRuntimeConfig(t *testing.T) {
	for _, tc := range []struct{ runtime, port string }{{"search", "50052"}, {"core", "50051"}, {"worker", ""}} {
		t.Run(tc.runtime, func(t *testing.T) {
			v := viper.New()
			v.Set("APP_ENV", "production")
			cfg, err := load(v, tc.runtime)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.App.Name != "nexus-"+tc.runtime || cfg.App.GRPCPort != tc.port || cfg.App.GRPCReflectionEnabled {
				t.Fatalf("config %+v", cfg.App)
			}
		})
	}
	if _, err := load(viper.New(), "engine"); err == nil {
		t.Fatal("legacy engine runtime is still accepted")
	}
}

func TestCoreConfigUsesCanonicalSettings(t *testing.T) {
	v := viper.New()
	v.Set("APP_ENV", "staging")
	v.Set("CORE_SERVICE_NAME", "core-staging")
	v.Set("CORE_GRPC_PORT", "51051")
	v.Set("CORE_GRPC_REFLECTION_ENABLED", false)
	v.Set("ELASTICSEARCH_ADDRESSES", "http://unused:9200")
	v.Set("REDIS_ADDR", "unused:6379")

	cfg, err := load(v, "core")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Name != "core-staging" || cfg.App.GRPCPort != "51051" || cfg.App.GRPCReflectionEnabled {
		t.Fatalf("core config %+v", cfg.App)
	}
	if len(cfg.Elasticsearch.Addresses) != 0 || cfg.Redis.Addr != "" {
		t.Fatalf("core unexpectedly loaded Search dependencies: %+v %+v", cfg.Elasticsearch, cfg.Redis)
	}
}

func TestWorkerConfigHasNoGRPCOrSearchDependencies(t *testing.T) {
	v := viper.New()
	v.Set("WORKER_SERVICE_NAME", "worker-staging")
	v.Set("WORKER_GRPC_PORT", "51053")
	v.Set("ELASTICSEARCH_ADDRESSES", "http://unused:9200")
	v.Set("REDIS_ADDR", "unused:6379")

	cfg, err := load(v, "worker")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Name != "worker-staging" || cfg.App.GRPCPort != "" || cfg.App.GRPCReflectionEnabled {
		t.Fatalf("worker config %+v", cfg.App)
	}
	if len(cfg.Elasticsearch.Addresses) != 0 || cfg.Redis.Addr != "" {
		t.Fatalf("worker unexpectedly loaded Search dependencies: %+v %+v", cfg.Elasticsearch, cfg.Redis)
	}
}
func TestSearchLegacyConfig(t *testing.T) {
	v := viper.New()
	v.Set("GRPC_PORT", "51052")
	v.Set("GRPC_REFLECTION_ENABLED", false)
	v.Set("APP_NAME", "legacy")
	cfg, err := load(v, "search")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.GRPCPort != "51052" || cfg.App.Name != "legacy" || cfg.App.GRPCReflectionEnabled {
		t.Fatalf("legacy fallback %+v", cfg.App)
	}
	v.Set("SEARCH_GRPC_PORT", "52052")
	cfg, err = load(v, "search")
	if err != nil || cfg.App.GRPCPort != "52052" {
		t.Fatal("runtime setting must win")
	}
	cfg, err = load(v, "core")
	if err != nil || cfg.App.GRPCPort != "50051" {
		t.Fatal("legacy settings leaked into core")
	}
}
func TestInvalidPort(t *testing.T) {
	for _, port := range []string{"abc", "0", "65536", "-1"} {
		v := viper.New()
		v.Set("SEARCH_GRPC_PORT", port)
		if _, err := load(v, "search"); err == nil {
			t.Fatalf("accepted %q", port)
		}
	}
}
