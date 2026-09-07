package config

import (
	"github.com/spf13/viper"
	"testing"
)

func TestRuntimeConfig(t *testing.T) {
	for _, tc := range []struct{ runtime, port string }{{"search", "50052"}, {"engine", "50051"}, {"worker", ""}} {
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
	cfg, err = load(v, "engine")
	if err != nil || cfg.App.GRPCPort != "50051" {
		t.Fatal("legacy settings leaked into engine")
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
