package search

import (
	"context"
	"encoding/json"
	es "github.com/elastic/go-elasticsearch/v8"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestQueryConstruction(t *testing.T) {
	n := 10.0
	req := PropertySearchRequest{Keyword: "villa", City: "HCM", District: "1", MinPrice: &n, MaxPrice: &n, MinArea: &n, MaxArea: &n, Latitude: &n, Longitude: &n, RadiusKm: &n}
	got := buildPropertySearchQuery(req, 20, 10)
	expected := `{"from":20,"size":10,"query":{"bool":{"must":[{"multi_match":{"query":"villa","fields":["title^3","description","city","district","ward","address"]}}],"filter":[{"term":{"city.keyword":"HCM"}},{"term":{"district.keyword":"1"}},{"range":{"price":{"gte":10,"lte":10}}},{"range":{"area":{"gte":10,"lte":10}}},{"geo_distance":{"distance":"10.000000km","location":{"lat":10,"lon":10}}}]}},"sort":[{"publishedAt":{"order":"desc"}}]}`
	assertJSON(t, got, expected)
	assertJSON(t, buildPropertySearchQuery(PropertySearchRequest{Latitude: &n}, 0, 20), `{"from":0,"size":20,"query":{"bool":{"must":[{"match_all":{}}],"filter":[]}},"sort":[{"publishedAt":{"order":"desc"}}]}`)
}
func assertJSON(t *testing.T, got any, want string) {
	t.Helper()
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	if err = json.Unmarshal(data, &a); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(want), &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("query mismatch\ngot %s\nwant %s", data, want)
	}
}
func TestRepositoryPagination(t *testing.T) {
	for _, tc := range []struct {
		name                                   string
		page, limit, from, wantPage, wantLimit int
		total                                  int64
		pages                                  int64
	}{
		{"defaults", 0, 0, 0, 1, 20, 21, 2},
		{"negative", -1, -5, 0, 1, 20, 0, 0},
		{"second page", 2, 10, 10, 2, 10, 21, 3},
		{"exact multiple", 3, 10, 20, 3, 10, 30, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := es.NewClient(es.Config{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/nexus_estate_properties/_search" || req.URL.Query().Get("track_total_hits") != "true" {
					t.Fatalf("unexpected request: %s", req.URL)
				}
				var body struct {
					From int
					Size int
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.From != tc.from || body.Size != tc.wantLimit {
					t.Fatalf("pagination %+v", body)
				}
				payload, _ := json.Marshal(map[string]any{"hits": map[string]any{"total": map[string]any{"value": tc.total}, "hits": []any{map[string]any{"_source": map[string]any{"id": "p1", "publishedAt": "2026-01-01"}}}}})
				return &http.Response{StatusCode: 200, Header: http.Header{"X-Elastic-Product": []string{"Elasticsearch"}}, Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			got, err := NewElasticsearchRepository(client, "nexus_estate_properties").SearchProperties(context.Background(), PropertySearchRequest{Page: tc.page, Limit: tc.limit})
			if err != nil {
				t.Fatal(err)
			}
			if got.Page != tc.wantPage || got.Limit != tc.wantLimit || got.Total != tc.total || got.TotalPages != tc.pages || len(got.Items) != 1 || got.Items[0].ID != "p1" {
				t.Fatalf("result: %+v", got)
			}
		})
	}
}
