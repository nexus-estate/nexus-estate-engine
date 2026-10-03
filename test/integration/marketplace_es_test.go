package integration

// Shared Elasticsearch test support for the marketplace gates: the gated client
// fixture, test-owned index lifecycle, seeding, the search helpers and the case
// runners. Each gate keeps its own corpus and cases in its own file.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/nexus-estate/nexus-estate-engine/internal/marketplace"
	"github.com/nexus-estate/nexus-estate-engine/internal/platform/config"
	esinfra "github.com/nexus-estate/nexus-estate-engine/internal/platform/elasticsearch"
)

// listingIndexFixture connects to the gated Elasticsearch, recreates one
// test-owned index from the checked-in definition and returns it with a request
// context. The index is deleted again after the test, so runs stay deterministic.
//
// It skips when MARKETPLACE_INTEGRATION_ES_ADDR is unset and fails when it is set
// to something that is not a URL: the operator asked for the gate to run, and the
// client would otherwise report an opaque scheme error.
func listingIndexFixture(t *testing.T, index string) (*es.Client, context.Context) {
	t.Helper()

	address := strings.TrimSpace(os.Getenv("MARKETPLACE_INTEGRATION_ES_ADDR"))
	if address == "" {
		t.Skip("set MARKETPLACE_INTEGRATION_ES_ADDR (for example http://localhost:9200) to test a running Elasticsearch")
	}
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		t.Fatalf("MARKETPLACE_INTEGRATION_ES_ADDR %q must start with http:// or https://, for example http://localhost:9200", address)
	}

	client, closeClient, err := esinfra.NewClient(config.ElasticsearchConfig{Addresses: []string{address}})
	if err != nil {
		t.Fatalf("create elasticsearch client: %v", err)
	}
	t.Cleanup(closeClient)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	if err := esinfra.Ping(ctx, client); err != nil {
		t.Fatalf("elasticsearch unreachable at %s: %v", address, err)
	}

	if err := deleteListingIndex(ctx, client, index); err != nil {
		t.Fatalf("delete stale index %q before the test: %v", index, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := deleteListingIndex(cleanupCtx, client, index); err != nil {
			t.Errorf("delete index %q after the test: %v", index, err)
		}
	})

	if err := createListingIndex(ctx, client, index); err != nil {
		t.Fatalf("create index %q from checked-in definition: %v", index, err)
	}

	return client, ctx
}

func createListingIndex(ctx context.Context, client *es.Client, index string) error {
	res, err := client.Indices.Create(
		index,
		client.Indices.Create.WithContext(ctx),
		client.Indices.Create.WithBody(strings.NewReader(marketplace.ListingIndexDefinition)),
	)
	if err != nil {
		return err
	}
	defer closeResponse(res)
	if res.IsError() {
		return responseError("create index", res)
	}
	return nil
}

func deleteListingIndex(ctx context.Context, client *es.Client, index string) error {
	res, err := client.Indices.Delete(
		[]string{index},
		client.Indices.Delete.WithContext(ctx),
		client.Indices.Delete.WithIgnoreUnavailable(true),
	)
	if err != nil {
		return err
	}
	defer closeResponse(res)
	if res.IsError() {
		return responseError("delete index", res)
	}
	return nil
}

// seedListingDocuments indexes validated canonical documents with listing id as
// the document id, exactly as the write layer must.
func seedListingDocuments(ctx context.Context, t *testing.T, client *es.Client, index string, documents ...marketplace.MarketplaceListingDocument) {
	t.Helper()

	for _, document := range documents {
		if err := marketplace.ValidateMarketplaceListingDocument(document); err != nil {
			t.Fatalf("fixture %q must be a valid document: %v", document.ListingID, err)
		}
		payload, err := json.Marshal(document)
		if err != nil {
			t.Fatalf("marshal document %q: %v", document.ListingID, err)
		}

		res, err := client.Index(
			index,
			strings.NewReader(string(payload)),
			client.Index.WithContext(ctx),
			client.Index.WithDocumentID(document.ListingID),
			client.Index.WithRefresh("true"),
		)
		if err != nil {
			t.Fatalf("index document %q: %v", document.ListingID, err)
		}
		if res.IsError() {
			t.Fatalf("index document %q: %v", document.ListingID, responseError("index document", res))
		}
		closeResponse(res)
	}
}

// searchListingIDs runs a raw query and returns the exact match count with the
// matched listing ids sorted, so a case can assert both recall and precision
// without depending on result order.
func searchListingIDs(ctx context.Context, t *testing.T, client *es.Client, index, query string) (int64, []string) {
	t.Helper()

	total, ids := searchHits(ctx, t, client, index, query)
	slices.Sort(ids)

	return total, ids
}

// searchHits runs a raw search body and returns the exact match count with the
// matched listing ids in result order, so an ordering case can assert a sort
// contract. The body is complete rather than query-only because ordering needs the
// sort and size clauses alongside the query.
func searchHits(ctx context.Context, t *testing.T, client *es.Client, index, body string) (int64, []string) {
	t.Helper()

	res, err := client.Search(
		client.Search.WithContext(ctx),
		client.Search.WithIndex(index),
		client.Search.WithBody(strings.NewReader(body)),
	)
	if err != nil {
		t.Fatalf("search %s: %v", body, err)
	}
	defer closeResponse(res)
	if res.IsError() {
		t.Fatalf("search %s: %v", body, responseError("search", res))
	}

	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID string `json:"_id"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		t.Fatalf("decode search response for %s: %v", body, err)
	}

	ids := make([]string, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		ids = append(ids, hit.ID)
	}

	return result.Hits.Total.Value, ids
}

// searchCase asserts an exact match set, so recall and precision are pinned
// together, independent of result order.
type searchCase struct {
	name string
	// query is the query clause; the runner wraps it in a body.
	query   string
	wantIDs []string
}

func runSearchCases(t *testing.T, ctx context.Context, client *es.Client, index string, cases []searchCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			total, ids := searchListingIDs(ctx, t, client, index, `{"query":`+tc.query+`}`)

			want := slices.Clone(tc.wantIDs)
			slices.Sort(want)
			if total != int64(len(want)) || !slices.Equal(ids, want) {
				t.Fatalf("query %s matched total=%d ids=%v, want %v", tc.query, total, ids, want)
			}
		})
	}
}

// orderedSearchCase asserts the exact result order, so a sort contract cannot
// silently degrade into an arbitrary order.
type orderedSearchCase struct {
	name string
	// body is the complete search request, because ordering needs query, sort,
	// from and size together.
	body string
	// wantTotal is the full match count, which stays independent of from and size,
	// while wantIDs is the exact order of the returned page.
	wantTotal int64
	wantIDs   []string
}

func runOrderedSearchCases(t *testing.T, ctx context.Context, client *es.Client, index string, cases []orderedSearchCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			total, ids := searchHits(ctx, t, client, index, tc.body)

			if total != tc.wantTotal || !slices.Equal(ids, tc.wantIDs) {
				t.Fatalf("search %s matched total=%d ids=%v, want total=%d order %v", tc.body, total, ids, tc.wantTotal, tc.wantIDs)
			}
		})
	}
}

// listingDocument builds a valid canonical document so a corpus only has to state
// what it varies. Fixtures are validated when they are seeded.
func listingDocument(listingID string, apply func(*marketplace.MarketplaceListingDocument)) marketplace.MarketplaceListingDocument {
	published := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	latitude, longitude := 10.78, 106.7

	document := marketplace.MarketplaceListingDocument{
		ListingID:  listingID,
		PropertyID: "property-" + listingID,

		Title:       "Nhà phố",
		Description: "Nhà phố mặt tiền",
		Type:        marketplace.EstateTypeHouse,
		Purpose:     marketplace.EstatePurposeSale,

		Price: 1_000_000_000,
		Area:  float64Pointer(100),

		ProvinceID:   "30000000-0000-4000-8000-000000000001",
		ProvinceName: "Hồ Chí Minh",
		WardID:       "30000000-0000-4000-8000-000000000002",
		WardName:     "Bến Nghé",
		Address:      "12 Lê Lợi",

		Location: &marketplace.GeoPoint{Lat: &latitude, Lon: &longitude},

		Media: &marketplace.MediaSummary{Images: []string{"a.jpg"}},

		PublishedAt: &published,
		UpdatedAt:   updated,
	}
	apply(&document)
	return document
}

func float64Pointer(value float64) *float64 { return &value }

func responseError(operation string, res *esapi.Response) error {
	body, _ := io.ReadAll(res.Body)
	return fmt.Errorf("%s: %s: %s", operation, res.Status(), strings.TrimSpace(string(body)))
}

func closeResponse(res *esapi.Response) {
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
}
