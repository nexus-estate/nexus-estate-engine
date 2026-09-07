package elasticsearch

import (
	"context"
	"fmt"
	es "github.com/elastic/go-elasticsearch/v8"
	"net/http"

	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/config"
)

func NewClient(cfg config.ElasticsearchConfig) (*es.Client, func(), error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	client, err := es.NewClient(es.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		Transport: transport,
	})
	return client, transport.CloseIdleConnections, err
}

func Ping(ctx context.Context, client *es.Client) error {
	res, err := client.Ping(client.Ping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return fmt.Errorf("elasticsearch health: %s", res.Status())
	}
	return nil
}
