package config

import (
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

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	_ = viper.ReadInConfig()

	viper.SetDefault("APP_NAME", "nexus-estate-search-service")
	viper.SetDefault("APP_ENV", "local")
	viper.SetDefault("GRPC_PORT", "50052")

	viper.SetDefault("ELASTICSEARCH_ADDRESSES", "http://localhost:9200")
	viper.SetDefault("ELASTICSEARCH_PROPERTY_INDEX", "nexus_estate_properties")

	viper.SetDefault("REDIS_ADDR", "localhost:6379")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("REDIS_SEARCH_TTL_SECONDS", 60)

	viper.SetDefault("GRPC_REFLECTION_ENABLED", true)

	addresses := strings.Split(viper.GetString("ELASTICSEARCH_ADDRESSES"), ",")

	return &Config{
		App: AppConfig{
			Name:                  viper.GetString("APP_NAME"),
			Env:                   viper.GetString("APP_ENV"),
			GRPCPort:              viper.GetString("GRPC_PORT"),
			GRPCReflectionEnabled: viper.GetBool("GRPC_REFLECTION_ENABLED"),
		},
		Elasticsearch: ElasticsearchConfig{
			Addresses:     addresses,
			Username:      viper.GetString("ELASTICSEARCH_USERNAME"),
			Password:      viper.GetString("ELASTICSEARCH_PASSWORD"),
			PropertyIndex: viper.GetString("ELASTICSEARCH_PROPERTY_INDEX"),
		},
		Redis: RedisConfig{
			Addr:             viper.GetString("REDIS_ADDR"),
			Password:         viper.GetString("REDIS_PASSWORD"),
			DB:               viper.GetInt("REDIS_DB"),
			SearchTTLSeconds: viper.GetInt("REDIS_SEARCH_TTL_SECONDS"),
		},
	}, nil
}
