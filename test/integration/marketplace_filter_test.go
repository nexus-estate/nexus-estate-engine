package integration

import (
	"testing"

	"github.com/nexus-estate/nexus-estate-engine/internal/marketplace"
)

// The filter gate gets its own test-owned index so its corpus stays separate from
// the text-relevance corpora, which carry different titles and source content.
const relevanceFilterIndexName = "nexus_estate_marketplace_relevance_filter_integration"

// TestMarketplaceListingSearchFilters covers the remaining queryable fields: the
// price and area range filters and the geo_distance filter over the geo_point
// location. Every bound is asserted to be inclusive, and a listing without a geo
// point must stay valid and searchable while never matching a geo query.
//
// Run against a running local Elasticsearch; see deploy/README.md.
func TestMarketplaceListingSearchFilters(t *testing.T) {
	client, ctx := listingIndexFixture(t, relevanceFilterIndexName)

	seedListingDocuments(ctx, t, client, relevanceFilterIndexName, relevanceFilterListings()...)

	runSearchCases(t, ctx, client, relevanceFilterIndexName, []searchCase{
		{
			name:    "price lower bound is inclusive",
			query:   `{"range":{"price":{"gte":1000000000}}}`,
			wantIDs: []string{"listing-filter-1", "listing-filter-2", "listing-filter-3", "listing-filter-hanoi", "listing-filter-no-geo"},
		},
		{
			name:    "price upper bound is inclusive",
			query:   `{"range":{"price":{"lte":2500000000}}}`,
			wantIDs: []string{"listing-filter-1", "listing-filter-2", "listing-filter-zero-price", "listing-filter-area-unknown"},
		},
		{
			name:    "price between both bounds",
			query:   `{"range":{"price":{"gte":1000000000,"lte":3000000000}}}`,
			wantIDs: []string{"listing-filter-1", "listing-filter-2", "listing-filter-no-geo"},
		},
		{
			name:    "price strictly above the highest listing matches nothing",
			query:   `{"range":{"price":{"gt":5000000000}}}`,
			wantIDs: nil,
		},
		{
			name:    "zero price is a valid boundary value",
			query:   `{"range":{"price":{"gte":0,"lte":0}}}`,
			wantIDs: []string{"listing-filter-zero-price"},
		},
		{
			name:    "area bounds include fractional endpoints",
			query:   `{"range":{"area":{"gte":50,"lte":120.5}}}`,
			wantIDs: []string{"listing-filter-1", "listing-filter-2", "listing-filter-hanoi", "listing-filter-no-geo"},
		},
		{
			name:    "area range excludes unknown area rather than treating it as zero",
			query:   `{"range":{"area":{"gte":0}}}`,
			wantIDs: []string{"listing-filter-1", "listing-filter-2", "listing-filter-3", "listing-filter-hanoi", "listing-filter-zero-price", "listing-filter-no-geo"},
		},
		{
			name:    "area strictly greater excludes the fractional boundary",
			query:   `{"range":{"area":{"gt":120.5}}}`,
			wantIDs: []string{"listing-filter-3"},
		},
		{
			name:    "area below the smallest listing matches nothing",
			query:   `{"range":{"area":{"lt":25}}}`,
			wantIDs: nil,
		},
		{
			name:    "geo distance of four kilometres excludes the outer listing",
			query:   `{"geo_distance":{"distance":"4km","location":{"lat":10.7769,"lon":106.7009}}}`,
			wantIDs: []string{"listing-filter-1", "listing-filter-2"},
		},
		{
			name:    "geo distance of six kilometres includes the outer listing",
			query:   `{"geo_distance":{"distance":"6km","location":{"lat":10.7769,"lon":106.7009}}}`,
			wantIDs: []string{"listing-filter-1", "listing-filter-2", "listing-filter-3"},
		},
		{
			// Even a radius that covers the globe cannot return a listing that has no
			// location, which is how an ungeocoded listing stays out of geo search.
			name:    "document without a location is never returned by geo distance",
			query:   `{"geo_distance":{"distance":"20000km","location":{"lat":10.7769,"lon":106.7009}}}`,
			wantIDs: []string{"listing-filter-1", "listing-filter-2", "listing-filter-3", "listing-filter-hanoi"},
		},
		{
			// Real searches compose filters; the ungeocoded listing matches the price
			// clause but must still be excluded by the geo clause.
			name:    "geo distance composes with a price range",
			query:   `{"bool":{"filter":[{"geo_distance":{"distance":"6km","location":{"lat":10.7769,"lon":106.7009}}},{"range":{"price":{"lte":3000000000}}}]}}`,
			wantIDs: []string{"listing-filter-1", "listing-filter-2"},
		},
	})
}

// relevanceFilterListings is the numeric and geo corpus, spread so range bounds and
// distance radii land on both sides of every assertion. One listing deliberately
// carries no location.
func relevanceFilterListings() []marketplace.MarketplaceListingDocument {
	districtOneLatitude, districtOneLongitude := 10.7769, 106.7009
	thaoDienLatitude, thaoDienLongitude := 10.7820, 106.7300
	districtSevenLatitude, districtSevenLongitude := 10.7300, 106.7200
	hanoiLatitude, hanoiLongitude := 21.0278, 105.8342

	unknownArea := filterListing("listing-filter-area-unknown", "Căn hộ chưa rõ diện tích", marketplace.EstateTypeApartment, 500_000_000, 1, nil, nil)
	unknownArea.Area = nil
	thaoDien := filterListing("listing-filter-2", "Căn hộ Thảo Điền", marketplace.EstateTypeApartment, 2_500_000_000, 120.5, &thaoDienLatitude, &thaoDienLongitude)
	thaoDien.WardID, thaoDien.WardName = "30000000-0000-4000-8000-000000000004", "Thảo Điền"
	quanBay := filterListing("listing-filter-3", "Nhà phố Quận 7", marketplace.EstateTypeHouse, 5_000_000_000, 220, &districtSevenLatitude, &districtSevenLongitude)
	quanBay.WardID, quanBay.WardName = "30000000-0000-4000-8000-000000000003", "Tân Phú"
	hanoi := filterListing("listing-filter-hanoi", "Nhà phố Hà Nội", marketplace.EstateTypeHouse, 4_000_000_000, 70, &hanoiLatitude, &hanoiLongitude)
	hanoi.ProvinceID, hanoi.ProvinceName = "40000000-0000-4000-8000-000000000001", "Hà Nội"
	hanoi.WardID, hanoi.WardName = "40000000-0000-4000-8000-000000000002", "Phúc Tân"

	return []marketplace.MarketplaceListingDocument{
		filterListing("listing-filter-1", "Căn hộ Quận 1", marketplace.EstateTypeApartment, 1_000_000_000, 50, &districtOneLatitude, &districtOneLongitude),
		thaoDien,
		quanBay,
		hanoi,
		filterListing("listing-filter-zero-price", "Đất nền Quận 2", marketplace.EstateTypeLand, 0, 25, nil, nil),
		filterListing("listing-filter-no-geo", "Căn hộ Quận 4", marketplace.EstateTypeApartment, 3_000_000_000, 80, nil, nil),
		unknownArea,
	}
}

// filterListing builds a valid document with explicit metrics. A nil coordinate pair
// means the listing has no geo point.
func filterListing(listingID, title string, estateType marketplace.EstateType, price int64, area float64, latitude, longitude *float64) marketplace.MarketplaceListingDocument {
	return listingDocument(listingID, func(document *marketplace.MarketplaceListingDocument) {
		document.Title = title
		document.Type = estateType
		document.Price = price
		document.Area = float64Pointer(area)
		document.Location = nil
		if latitude != nil && longitude != nil {
			document.Location = &marketplace.GeoPoint{Lat: latitude, Lon: longitude}
		}
	})
}
