package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	es "github.com/elastic/go-elasticsearch/v8"
)

type ElasticsearchRepository struct {
	client *es.Client
	index  string
}

func NewElasticsearchRepository(client *es.Client, index string) Repository {
	return &ElasticsearchRepository{
		client: client,
		index:  index,
	}
}

func (r *ElasticsearchRepository) SearchProperties(ctx context.Context, req PropertySearchRequest) (*PropertySearchResponse, error) {
	page := req.Page
	limit := req.Limit

	if page <= 0 {
		page = 1
	}

	if limit <= 0 {
		limit = 20
	}

	from := (page - 1) * limit

	query := buildPropertySearchQuery(req, from, limit)

	body, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	res, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithIndex(r.index),
		r.client.Search.WithBody(bytes.NewReader(body)),
		r.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		data, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("elasticsearch search error: %s", string(data))
	}

	var parsed elasticSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	items := make([]PropertySearchItem, 0, len(parsed.Hits.Hits))
	for _, hit := range parsed.Hits.Hits {
		items = append(items, hit.Source)
	}

	total := parsed.Hits.Total.Value
	totalPages := total / int64(limit)
	if total%int64(limit) > 0 {
		totalPages++
	}

	return &PropertySearchResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func buildPropertySearchQuery(req PropertySearchRequest, from int, size int) map[string]any {
	must := make([]any, 0)
	filter := make([]any, 0)

	if req.Keyword != "" {
		must = append(must, map[string]any{
			"multi_match": map[string]any{
				"query": req.Keyword,
				"fields": []string{
					"title^3",
					"description",
					"city",
					"district",
					"ward",
					"address",
				},
			},
		})
	}

	if req.City != "" {
		filter = append(filter, map[string]any{
			"term": map[string]any{
				"city.keyword": req.City,
			},
		})
	}

	if req.District != "" {
		filter = append(filter, map[string]any{
			"term": map[string]any{
				"district.keyword": req.District,
			},
		})
	}

	priceRange := map[string]any{}
	if req.MinPrice != nil {
		priceRange["gte"] = *req.MinPrice
	}
	if req.MaxPrice != nil {
		priceRange["lte"] = *req.MaxPrice
	}
	if len(priceRange) > 0 {
		filter = append(filter, map[string]any{
			"range": map[string]any{
				"price": priceRange,
			},
		})
	}

	areaRange := map[string]any{}
	if req.MinArea != nil {
		areaRange["gte"] = *req.MinArea
	}
	if req.MaxArea != nil {
		areaRange["lte"] = *req.MaxArea
	}
	if len(areaRange) > 0 {
		filter = append(filter, map[string]any{
			"range": map[string]any{
				"area": areaRange,
			},
		})
	}

	if req.Latitude != nil && req.Longitude != nil && req.RadiusKm != nil {
		filter = append(filter, map[string]any{
			"geo_distance": map[string]any{
				"distance": fmt.Sprintf("%fkm", *req.RadiusKm),
				"location": map[string]any{
					"lat": *req.Latitude,
					"lon": *req.Longitude,
				},
			},
		})
	}

	if len(must) == 0 {
		must = append(must, map[string]any{
			"match_all": map[string]any{},
		})
	}

	return map[string]any{
		"from": from,
		"size": size,
		"query": map[string]any{
			"bool": map[string]any{
				"must":   must,
				"filter": filter,
			},
		},
		"sort": []any{
			map[string]any{
				"publishedAt": map[string]any{
					"order": "desc",
				},
			},
		},
	}
}

type elasticSearchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source PropertySearchItem `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}
