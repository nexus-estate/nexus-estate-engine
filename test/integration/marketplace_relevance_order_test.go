package integration

import (
	"testing"
	"time"

	"github.com/nexus-estate/nexus-estate-engine/internal/marketplace"
)

// relevanceOrderIndexName is owned by TestMarketplaceListingSearchOrdering.
const relevanceOrderIndexName = "nexus_estate_marketplace_relevance_order_integration"

// TestMarketplaceListingSearchOrdering asserts the recency ordering contract on the
// canonical `published_at` field, including the offset paging Search v1 applies on
// top of it. v1 orders by `publishedAt` descending and pages with
// from = (page-1)*limit and size = limit (internal/search/elasticsearch_repository.go
// and internal/search/pagination.go), so the projection must keep newest first and
// page through that order without gaps or overlap.
//
// The ordering key is the tuple (published_at, listing_id), both in the requested
// direction. listing_id is the only field that is unique, required and indexed on
// every document, so it is the tiebreaker that keeps listings published at the same
// instant — and any page boundary that splits them — deterministic. Relying on the
// query rather than index.sort keeps scoring behavior untouched.
//
// Offset paging stays subject to the Elasticsearch result-window limit, exactly as
// v1 is today; that caveat is unchanged and is not asserted here.
//
// Ordering is the one assertion that cannot be expressed as a match set, so the
// cases below compare the returned order instead.
//
// Run against a running local Elasticsearch; see deploy/README.md.
func TestMarketplaceListingSearchOrdering(t *testing.T) {
	client, ctx := listingIndexFixture(t, relevanceOrderIndexName)

	seedListingDocuments(ctx, t, client, relevanceOrderIndexName, relevanceOrderListings()...)

	const (
		recencySort = `"sort":[{"published_at":{"order":"desc"}},{"listing_id":{"order":"desc"}}]`
		oldestSort  = `"sort":[{"published_at":{"order":"asc"}},{"listing_id":{"order":"asc"}}]`
	)

	runOrderedSearchCases(t, ctx, client, relevanceOrderIndexName, []orderedSearchCase{
		{
			name:      "recency sort matches the v1 publishedAt descending contract",
			body:      `{"query":{"match_all":{}},` + recencySort + `}`,
			wantTotal: 7,
			wantIDs:   []string{"listing-order-7", "listing-order-6", "listing-order-5", "listing-order-4", "listing-order-3", "listing-order-2", "listing-order-1"},
		},
		{
			name:      "ascending sort reverses the recency order",
			body:      `{"query":{"match_all":{}},` + oldestSort + `}`,
			wantTotal: 7,
			wantIDs:   []string{"listing-order-1", "listing-order-2", "listing-order-3", "listing-order-4", "listing-order-5", "listing-order-6", "listing-order-7"},
		},
		{
			name:      "recency sort applies to a filtered subset",
			body:      `{"query":{"term":{"province_name.keyword":"Hồ Chí Minh"}},` + recencySort + `}`,
			wantTotal: 5,
			wantIDs:   []string{"listing-order-7", "listing-order-6", "listing-order-4", "listing-order-2", "listing-order-1"},
		},
		{
			// A recency window is a date range on the same field the sort uses.
			name:      "published_at range with recency sort",
			body:      `{"query":{"range":{"published_at":{"gte":"2026-02-01"}}},` + recencySort + `}`,
			wantTotal: 6,
			wantIDs:   []string{"listing-order-7", "listing-order-6", "listing-order-5", "listing-order-4", "listing-order-3", "listing-order-2"},
		},
		{
			// The tied pair shares the newest publication instant and opens the first
			// page, which is where a missing tiebreaker would shuffle results.
			name:      "equal published_at is broken by listing id descending",
			body:      `{"query":{"range":{"published_at":{"gte":"2026-05-02"}}},` + recencySort + `}`,
			wantTotal: 2,
			wantIDs:   []string{"listing-order-7", "listing-order-6"},
		},
		{
			// The tiebreaker mirrors the requested direction, so an ascending request
			// reverses the tied pair too instead of leaving an arbitrary order.
			name:      "equal published_at follows the requested direction",
			body:      `{"query":{"range":{"published_at":{"gte":"2026-05-02"}}},` + oldestSort + `}`,
			wantTotal: 2,
			wantIDs:   []string{"listing-order-6", "listing-order-7"},
		},
		{
			// v1 pages with from/size after the sort, so the first page is the newest
			// listings while the total still counts every match.
			name:      "first page is the newest listings",
			body:      `{"query":{"match_all":{}},` + recencySort + `,"size":2}`,
			wantTotal: 7,
			wantIDs:   []string{"listing-order-7", "listing-order-6"},
		},
		{
			name:      "second page continues the recency order",
			body:      `{"query":{"match_all":{}},` + recencySort + `,"from":2,"size":2}`,
			wantTotal: 7,
			wantIDs:   []string{"listing-order-5", "listing-order-4"},
		},
		{
			name:      "third page continues the recency order",
			body:      `{"query":{"match_all":{}},` + recencySort + `,"from":4,"size":2}`,
			wantTotal: 7,
			wantIDs:   []string{"listing-order-3", "listing-order-2"},
		},
		{
			// The last page is partial rather than empty, which is the boundary v1
			// reports through total/total_pages.
			name:      "last page is partial",
			body:      `{"query":{"match_all":{}},` + recencySort + `,"from":6,"size":2}`,
			wantTotal: 7,
			wantIDs:   []string{"listing-order-1"},
		},
		{
			// An out-of-range page is empty but still reports how many listings
			// matched, which is what v1 needs to answer a page past the end.
			name:      "page beyond the last page returns no hits but keeps the total",
			body:      `{"query":{"match_all":{}},` + recencySort + `,"from":8,"size":2}`,
			wantTotal: 7,
			wantIDs:   nil,
		},
		{
			name:      "paging composes with a filter",
			body:      `{"query":{"term":{"province_name.keyword":"Hồ Chí Minh"}},` + recencySort + `,"from":2,"size":2}`,
			wantTotal: 5,
			wantIDs:   []string{"listing-order-4", "listing-order-2"},
		},
	})
}

// relevanceOrderListings is the ordering corpus, ordered so that a higher listing
// number is always at least as recent. It carries one deliberate published_at tie:
// listing-order-6 and listing-order-7 share an instant, and they are seeded in
// ascending id order so that insertion order contradicts the descending tiebreaker
// and a missing tiebreaker cannot pass by accident. Every other instant is distinct,
// because Elasticsearch does not guarantee a stable order for equal sort values.
func relevanceOrderListings() []marketplace.MarketplaceListingDocument {
	const (
		hoChiMinhID     = "30000000-0000-4000-8000-000000000001"
		haNoiID         = "40000000-0000-4000-8000-000000000001"
		daNangID        = "50000000-0000-4000-8000-000000000001"
		hoChiMinhWardID = "30000000-0000-4000-8000-000000000002"
		haNoiWardID     = "40000000-0000-4000-8000-000000000002"
		daNangWardID    = "50000000-0000-4000-8000-000000000002"
		hoChiMinh       = "Hồ Chí Minh"
		haNoi           = "Hà Nội"
		daNang          = "Đà Nẵng"
	)
	sharedInstant := time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC)
	return []marketplace.MarketplaceListingDocument{
		orderListing("listing-order-1", "Nhà phố tháng một", hoChiMinhID, hoChiMinh, hoChiMinhWardID, "Bến Nghé", time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC)),
		orderListing("listing-order-2", "Căn hộ tháng hai", hoChiMinhID, hoChiMinh, hoChiMinhWardID, "Bến Nghé", time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC)),
		orderListing("listing-order-3", "Biệt thự Hà Nội", haNoiID, haNoi, haNoiWardID, "Phúc Tân", time.Date(2026, 2, 28, 8, 0, 0, 0, time.UTC)),
		orderListing("listing-order-4", "Căn hộ tháng ba", hoChiMinhID, hoChiMinh, hoChiMinhWardID, "Bến Nghé", time.Date(2026, 3, 20, 8, 0, 0, 0, time.UTC)),
		orderListing("listing-order-5", "Nhà phố Đà Nẵng", daNangID, daNang, daNangWardID, "Hải Châu", time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC)),
		orderListing("listing-order-6", "Căn hộ đợt một", hoChiMinhID, hoChiMinh, hoChiMinhWardID, "Bến Nghé", sharedInstant),
		orderListing("listing-order-7", "Căn hộ đợt hai", hoChiMinhID, hoChiMinh, hoChiMinhWardID, "Bến Nghé", sharedInstant),
	}
}

// orderListing builds a valid document with an explicit publication instant.
func orderListing(listingID, title, provinceID, provinceName, wardID, wardName string, published time.Time) marketplace.MarketplaceListingDocument {
	return listingDocument(listingID, func(document *marketplace.MarketplaceListingDocument) {
		document.Title = title
		document.ProvinceID = provinceID
		document.ProvinceName = provinceName
		document.WardID = wardID
		document.WardName = wardName
		document.PublishedAt = &published
	})
}
