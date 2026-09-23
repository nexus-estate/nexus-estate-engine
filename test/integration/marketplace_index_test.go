package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"

	"github.com/nexus-estate/nexus-estate-engine/internal/marketplace"
)

// marketplaceIndexName is owned by TestMarketplaceListingIndexMapping.
const marketplaceIndexName = "nexus_estate_marketplace_listing_integration"

// TestMarketplaceListingIndexMapping proves the checked-in index definition is
// accepted by a real Elasticsearch and enforces the canonical document contract:
// listing id as the document identity, geo_point coordinates, and strict rejection
// of unmapped fields.
//
// Run against a running local Elasticsearch; see deploy/README.md.
func TestMarketplaceListingIndexMapping(t *testing.T) {
	client, ctx := listingIndexFixture(t, marketplaceIndexName)

	t.Run("mapping applied", func(t *testing.T) { assertAppliedMapping(ctx, t, client) })
	t.Run("settings applied", func(t *testing.T) { assertAppliedSettings(ctx, t, client) })
	t.Run("canonical document round trip", func(t *testing.T) { assertDocumentRoundTrip(ctx, t, client) })
	t.Run("unmapped field rejected", func(t *testing.T) { assertUnmappedFieldRejected(ctx, t, client) })
	t.Run("unmapped projection state field rejected", func(t *testing.T) { assertUnmappedProjectionStateFieldRejected(ctx, t, client) })
	t.Run("out of range geo rejected", func(t *testing.T) { assertOutOfRangeGeoRejected(ctx, t, client) })
}

// assertAppliedMapping reads the mapping back from the cluster, so a mapping the
// cluster silently rejects or normalizes cannot pass as the checked-in contract.
func assertAppliedMapping(ctx context.Context, t *testing.T, client *es.Client) {
	t.Helper()

	res, err := client.Indices.GetMapping(
		client.Indices.GetMapping.WithContext(ctx),
		client.Indices.GetMapping.WithIndex(marketplaceIndexName),
	)
	if err != nil {
		t.Fatalf("get mapping: %v", err)
	}
	defer closeResponse(res)
	if res.IsError() {
		t.Fatalf("get mapping: %v", responseError("get mapping", res))
	}

	type mappedProperty struct {
		Type       string                    `json:"type"`
		Dynamic    string                    `json:"dynamic"`
		Properties map[string]mappedProperty `json:"properties"`
	}
	var parsed map[string]struct {
		Mappings struct {
			Dynamic    string                    `json:"dynamic"`
			Properties map[string]mappedProperty `json:"properties"`
		} `json:"mappings"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode mapping: %v", err)
	}
	index, ok := parsed[marketplaceIndexName]
	if !ok {
		t.Fatalf("mapping response has no %q: %+v", marketplaceIndexName, parsed)
	}
	if index.Mappings.Dynamic != "strict" {
		t.Fatalf("dynamic = %q, want strict", index.Mappings.Dynamic)
	}
	for field, wantType := range map[string]string{
		"listing_id":   "keyword",
		"property_id":  "keyword",
		"title":        "text",
		"price":        "double",
		"area":         "double",
		"location":     "geo_point",
		"published_at": "date",
		"updated_at":   "date",
	} {
		if got := index.Mappings.Properties[field].Type; got != wantType {
			t.Fatalf("field %q type = %q, want %q", field, got, wantType)
		}
	}
	projectionState := index.Mappings.Properties["projection_state"]
	// Elasticsearch omits the implicit object type from its normalized mapping
	// response, but the nested properties and dynamic mode must still be present.
	if (projectionState.Type != "" && projectionState.Type != "object") || projectionState.Dynamic != "strict" ||
		projectionState.Properties["source_revision"].Type != "long" ||
		projectionState.Properties["deleted"].Type != "boolean" {
		t.Fatalf("projection_state mapping = %+v, want strict source_revision/deleted object", projectionState)
	}
	media := index.Mappings.Properties["media"]
	if media.Properties["images"].Type != "keyword" || media.Properties["cover_image"].Type != "keyword" {
		t.Fatalf("media mapping = %+v, want keyword images and cover_image", media)
	}
}

// assertAppliedSettings reads the analyzer back from the cluster, so settings the
// cluster rejects or normalizes cannot pass as the checked-in search contract.
func assertAppliedSettings(ctx context.Context, t *testing.T, client *es.Client) {
	t.Helper()

	res, err := client.Indices.GetSettings(
		client.Indices.GetSettings.WithContext(ctx),
		client.Indices.GetSettings.WithIndex(marketplaceIndexName),
	)
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	defer closeResponse(res)
	if res.IsError() {
		t.Fatalf("get settings: %v", responseError("get settings", res))
	}

	type appliedAnalyzer struct {
		Type      string   `json:"type"`
		Tokenizer string   `json:"tokenizer"`
		Filter    []string `json:"filter"`
	}
	var parsed map[string]struct {
		Settings struct {
			Index struct {
				Analysis struct {
					Analyzer map[string]appliedAnalyzer `json:"analyzer"`
				} `json:"analysis"`
			} `json:"index"`
		} `json:"settings"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	index, ok := parsed[marketplaceIndexName]
	if !ok {
		t.Fatalf("settings response has no %q: %+v", marketplaceIndexName, parsed)
	}

	settings := index.Settings.Index
	analyzer, ok := settings.Analysis.Analyzer["listing_text"]
	if !ok {
		t.Fatalf("analyzer listing_text is not applied: %+v", settings.Analysis.Analyzer)
	}
	if analyzer.Type != "custom" || analyzer.Tokenizer != "standard" {
		t.Fatalf("analyzer listing_text = %+v, want custom standard", analyzer)
	}
	for _, filter := range []string{"lowercase", "asciifolding"} {
		if !slices.Contains(analyzer.Filter, filter) {
			t.Fatalf("analyzer listing_text filters = %v, want %q", analyzer.Filter, filter)
		}
	}
}

// assertDocumentRoundTrip indexes a validated canonical document exactly as the
// write layer will and reads it back, proving the mapping stores it unchanged and
// that listing id is the Elasticsearch _id.
func assertDocumentRoundTrip(ctx context.Context, t *testing.T, client *es.Client) {
	t.Helper()

	document := integrationListingDocument(t, "listing-integration-1", "Sunny villa in district one", "HCM")
	seedListingDocuments(ctx, t, client, marketplaceIndexName, document)

	res, err := client.Get(
		marketplaceIndexName,
		document.ListingID,
		client.Get.WithContext(ctx),
	)
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	defer closeResponse(res)
	if res.IsError() {
		t.Fatalf("get document: %v", responseError("get document", res))
	}

	var fetched struct {
		ID     string                                 `json:"_id"`
		Found  bool                                   `json:"found"`
		Source marketplace.MarketplaceListingDocument `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if !fetched.Found {
		t.Fatalf("listing %q was not indexed", document.ListingID)
	}
	if fetched.ID != document.ListingID {
		t.Fatalf("_id = %q, want listing id %q", fetched.ID, document.ListingID)
	}
	if fetched.ID == document.PropertyID {
		t.Fatal("property id must not become the document id")
	}
	if !reflect.DeepEqual(fetched.Source, document) {
		t.Fatalf("round trip changed the document:\ngot  %+v\nwant %+v", fetched.Source, document)
	}
	if err := marketplace.ValidateMarketplaceListingDocument(fetched.Source); err != nil {
		t.Fatalf("round-tripped document must still validate: %v", err)
	}
}

func assertUnmappedFieldRejected(ctx context.Context, t *testing.T, client *es.Client) {
	t.Helper()

	const documentID = "strict-mapping-check"
	payload := `{"listing_id":"` + documentID + `","property_id":"property-1","unmapped_field":"value"}`

	res, err := client.Index(
		marketplaceIndexName,
		strings.NewReader(payload),
		client.Index.WithContext(ctx),
		client.Index.WithDocumentID(documentID),
		client.Index.WithRefresh("true"),
	)
	if err != nil {
		t.Fatalf("index unmapped field: %v", err)
	}
	defer closeResponse(res)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "strict_dynamic_mapping_exception") {
		t.Fatalf("strict mapping must reject unmapped fields: status=%s body=%s", res.Status(), body)
	}
}

func assertUnmappedProjectionStateFieldRejected(ctx context.Context, t *testing.T, client *es.Client) {
	t.Helper()

	const documentID = "strict-projection-state-check"
	payload := `{"listing_id":"` + documentID + `","projection_state":{"source_revision":1,"deleted":true,"unexpected":"value"}}`

	res, err := client.Index(
		marketplaceIndexName,
		strings.NewReader(payload),
		client.Index.WithContext(ctx),
		client.Index.WithDocumentID(documentID),
		client.Index.WithRefresh("true"),
	)
	if err != nil {
		t.Fatalf("index unmapped projection state field: %v", err)
	}
	defer closeResponse(res)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "strict_dynamic_mapping_exception") {
		t.Fatalf("projection_state must reject unmapped fields: status=%s body=%s", res.Status(), body)
	}
}

func assertOutOfRangeGeoRejected(ctx context.Context, t *testing.T, client *es.Client) {
	t.Helper()

	const documentID = "geo-range-check"
	payload := `{"listing_id":"` + documentID + `","property_id":"property-1","location":{"lat":91,"lon":106.66}}`

	res, err := client.Index(
		marketplaceIndexName,
		strings.NewReader(payload),
		client.Index.WithContext(ctx),
		client.Index.WithDocumentID(documentID),
		client.Index.WithRefresh("true"),
	)
	if err != nil {
		t.Fatalf("index out of range geo: %v", err)
	}
	defer closeResponse(res)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(strings.ToLower(string(body)), "latitude") {
		t.Fatalf("geo_point must reject an out of range latitude: status=%s body=%s", res.Status(), body)
	}
}

func integrationListingDocument(t *testing.T, listingID, title, city string) marketplace.MarketplaceListingDocument {
	t.Helper()

	published := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	updated := time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC)
	latitude, longitude := 10.75, 106.66

	document := marketplace.MarketplaceListingDocument{
		ListingID:  listingID,
		PropertyID: "property-" + listingID,

		Title:       title,
		Slug:        "listing-slug",
		Description: "Three bedroom villa",
		Type:        "villa",
		Purpose:     "sale",

		Price: float64Pointer(1_250_000_000),
		Area:  float64Pointer(220),

		City:     city,
		District: "1",
		Ward:     "Ben Nghe",
		Address:  "12 Le Loi",

		Location: &marketplace.GeoPoint{Lat: &latitude, Lon: &longitude},

		Media: marketplace.MediaSummary{Images: []string{"a.jpg", "b.jpg"}, CoverImage: "a.jpg"},

		PublishedAt: &published,
		UpdatedAt:   updated,
	}
	if err := marketplace.ValidateMarketplaceListingDocument(document); err != nil {
		t.Fatalf("integration fixture must be a valid document: %v", err)
	}
	return document
}
