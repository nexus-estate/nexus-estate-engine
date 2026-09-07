package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App           AppConfig
	Elasticsearch ElasticsearchConfig
	Redis         RedisConfig
}
type AppConfig struct {
	Name                  string
	Env                   string
	GRPCPort              string
	GRPCReflectionEnabled bool
}
type ElasticsearchConfig struct {
	Addresses     []string
	Username      string
	Password      string
	PropertyIndex string
}
type RedisConfig struct {
	Addr             string
	Password         string
	DB               int
	SearchTTLSeconds int
}

// Load reads a runtime-specific configuration. Legacy Search variables remain aliases.
func Load(runtime string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()
	// Environment-only deployments do not need an .env file.
	_ = v.ReadInConfig()
	return load(v, runtime)
}

func load(v *viper.Viper, runtime string) (*Config, error) {
	if runtime != "search" && runtime != "engine" && runtime != "worker" {
		return nil, fmt.Errorf("unknown runtime %q", runtime)
	}
	v.SetDefault("APP_ENV", "development")
	prefix := strings.ToUpper(runtime)
	if runtime != "worker" {
		port := "50052"
		if runtime == "engine" {
			port = "50051"
		}
		v.SetDefault(prefix+"_GRPC_PORT", port)
	}
	v.SetDefault(prefix+"_SERVICE_NAME", "nexus-"+runtime)
	v.SetDefault(prefix+"_GRPC_REFLECTION_ENABLED", v.GetString("APP_ENV") != "production")
	if runtime == "search" {
		for key, legacy := range map[string]string{"SEARCH_SERVICE_NAME": "APP_NAME", "SEARCH_GRPC_PORT": "GRPC_PORT", "SEARCH_GRPC_REFLECTION_ENABLED": "GRPC_REFLECTION_ENABLED"} {
			if !v.InConfig(key) && v.IsSet(legacy) {
				v.SetDefault(key, v.Get(legacy))
			}
		}
	}
	v.SetDefault("ELASTICSEARCH_ADDRESSES", "http://localhost:9200")
	v.SetDefault("ELASTICSEARCH_PROPERTY_INDEX", "nexus_estate_properties")
	v.SetDefault("REDIS_ADDR", "localhost:6379")
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("REDIS_SEARCH_TTL_SECONDS", 60)
	cfg := &Config{
		App:           AppConfig{Name: v.GetString(prefix + "_SERVICE_NAME"), Env: v.GetString("APP_ENV"), GRPCPort: v.GetString(prefix + "_GRPC_PORT"), GRPCReflectionEnabled: v.GetBool(prefix + "_GRPC_REFLECTION_ENABLED")},
		Elasticsearch: ElasticsearchConfig{Addresses: strings.Split(v.GetString("ELASTICSEARCH_ADDRESSES"), ","), Username: v.GetString("ELASTICSEARCH_USERNAME"), Password: v.GetString("ELASTICSEARCH_PASSWORD"), PropertyIndex: v.GetString("ELASTICSEARCH_PROPERTY_INDEX")},
		Redis:         RedisConfig{Addr: v.GetString("REDIS_ADDR"), Password: v.GetString("REDIS_PASSWORD"), DB: v.GetInt("REDIS_DB"), SearchTTLSeconds: v.GetInt("REDIS_SEARCH_TTL_SECONDS")},
	}
	if runtime != "worker" {
		n, err := strconv.Atoi(cfg.App.GRPCPort)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid %s_GRPC_PORT %q", prefix, cfg.App.GRPCPort)
		}
	}
	if runtime == "search" {
		for i := range cfg.Elasticsearch.Addresses {
			cfg.Elasticsearch.Addresses[i] = strings.TrimSpace(cfg.Elasticsearch.Addresses[i])
		}
		if _, _, err := net.SplitHostPort(cfg.Redis.Addr); err != nil {
			return nil, fmt.Errorf("invalid REDIS_ADDR: %w", err)
		}
	}
	return cfg, nil
}
