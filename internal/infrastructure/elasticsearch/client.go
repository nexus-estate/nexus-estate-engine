package elasticsearch

import (
	es "github.com/elastic/go-elasticsearch/v8"

	"github.com/NexusEstate/nexus-estate-search-service/internal/config"
)

func NewClient(cfg config.ElasticsearchConfig) (*es.Client, error) {
	return es.NewClient(es.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
	})
}
