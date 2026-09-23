package integration

import (
	"testing"

	"github.com/nexus-estate/nexus-estate-engine/internal/marketplace"
)

const (
	relevanceIndexName      = "nexus_estate_marketplace_relevance_integration"
	relevanceMixedIndexName = "nexus_estate_marketplace_relevance_mixed_integration"
)

// TestMarketplaceListingSearchRelevance pins analyzer behavior over API-owned
// title, description, province, ward and address text. Search uses the source
// province/ward names; it does not invent a district field.
func TestMarketplaceListingSearchRelevance(t *testing.T) {
	client, ctx := listingIndexFixture(t, relevanceIndexName)
	seedListingDocuments(ctx, t, client, relevanceIndexName, relevanceVietnameseListings()...)

	runSearchCases(t, ctx, client, relevanceIndexName, []searchCase{
		{name: "unaccented Vietnamese title", query: `{"match":{"title":"nha pho"}}`, wantIDs: []string{"listing-rel-1"}},
		{name: "mixed accented and unaccented title tokens", query: `{"match":{"title":"nha phố"}}`, wantIDs: []string{"listing-rel-1"}},
		{name: "hyphenated compound matches spaced text", query: `{"match":{"title":"chung-cư"}}`, wantIDs: []string{"listing-rel-2"}},
		{name: "unaccented compound in description", query: `{"match":{"description":"can ho chung cu"}}`, wantIDs: []string{"listing-rel-2"}},
		{name: "unaccented province name", query: `{"match":{"province_name":"ho chi minh"}}`, wantIDs: []string{"listing-rel-1", "listing-rel-2", "listing-rel-3"}},
		{name: "unaccented ward name", query: `{"match":{"ward_name":"ben nghe"}}`, wantIDs: []string{"listing-rel-1"}},
		{name: "accented ward name", query: `{"match":{"ward_name":"Bến Nghé"}}`, wantIDs: []string{"listing-rel-1"}},
		{name: "d stroke folds in ward name", query: `{"match":{"ward_name":"thao dien"}}`, wantIDs: []string{"listing-rel-3"}},
		{name: "unaccented street name in address", query: `{"match":{"address":"le loi"}}`, wantIDs: []string{"listing-rel-1"}},
		{
			name:    "shared title syllable over-matches with default OR",
			query:   `{"match":{"title":"quan 7"}}`,
			wantIDs: []string{"listing-rel-1", "listing-rel-2", "listing-rel-3"},
		},
		{
			name:    "AND on title isolates all query tokens",
			query:   `{"match":{"title":{"query":"quan 7","operator":"and"}}}`,
			wantIDs: []string{"listing-rel-2"},
		},
		{
			name:    "title phrase isolates ordered tokens",
			query:   `{"match_phrase":{"title":"quan 7"}}`,
			wantIDs: []string{"listing-rel-2"},
		},
		{name: "match phrase keeps token order", query: `{"match_phrase":{"title":"nha pho ben nghe"}}`, wantIDs: []string{"listing-rel-1"}},
		{name: "match phrase rejects reordered tokens", query: `{"match_phrase":{"title":"pho nha"}}`, wantIDs: nil},
		{name: "match ignores token order", query: `{"match":{"title":"pho nha"}}`, wantIDs: []string{"listing-rel-1"}},
		{name: "province keyword filter is exact", query: `{"term":{"province_name.keyword":"Hồ Chí Minh"}}`, wantIDs: []string{"listing-rel-1", "listing-rel-2", "listing-rel-3"}},
		{name: "province keyword filter does not fold", query: `{"term":{"province_name.keyword":"ho chi minh"}}`, wantIDs: nil},
		{name: "ward keyword filter is exact", query: `{"term":{"ward_name.keyword":"Bến Nghé"}}`, wantIDs: []string{"listing-rel-1"}},
		{name: "ward keyword filter does not fold", query: `{"term":{"ward_name.keyword":"ben nghe"}}`, wantIDs: nil},
		{name: "uppercase API type term matches", query: `{"term":{"type":"VILLA"}}`, wantIDs: []string{"listing-rel-3"}},
		{name: "lowercase type term does not match API values", query: `{"term":{"type":"villa"}}`, wantIDs: nil},
		{name: "uppercase API purpose term matches", query: `{"term":{"purpose":"SALE"}}`, wantIDs: []string{"listing-rel-1", "listing-rel-3"}},
		{name: "lowercase purpose term does not match API values", query: `{"term":{"purpose":"sale"}}`, wantIDs: nil},
	})
}

// TestMarketplaceListingSearchMixedLanguage keeps ASCII/Vietnamese search
// coverage while every geographic source field stays in the API's Vietnamese
// administrative names.
func TestMarketplaceListingSearchMixedLanguage(t *testing.T) {
	client, ctx := listingIndexFixture(t, relevanceMixedIndexName)
	documents := append(relevanceVietnameseListings(), relevanceEnglishListings()...)
	seedListingDocuments(ctx, t, client, relevanceMixedIndexName, documents...)

	runSearchCases(t, ctx, client, relevanceMixedIndexName, []searchCase{
		{name: "ASCII term finds ASCII title", query: `{"match":{"title":"apartment"}}`, wantIDs: []string{"listing-en-2"}},
		{name: "ASCII query recalls matching English title only", query: `{"match":{"title":"modern"}}`, wantIDs: []string{"listing-en-1"}},
		{name: "Vietnamese query finds Vietnamese title", query: `{"match":{"title":"nha pho"}}`, wantIDs: []string{"listing-rel-1"}},
		{name: "English token in bilingual title", query: `{"match":{"title":"villa"}}`, wantIDs: []string{"listing-en-1", "listing-mixed-1"}},
		{name: "unaccented Vietnamese title finds accented source", query: `{"match":{"title":"biet thu"}}`, wantIDs: []string{"listing-rel-3"}},
		{name: "ASCII ward query folds Vietnamese names", query: `{"match":{"ward_name":"thao dien"}}`, wantIDs: []string{"listing-en-1", "listing-mixed-1", "listing-rel-3"}},
		{name: "ASCII ward query folds Tân Phú", query: `{"match":{"ward_name":"tan phu"}}`, wantIDs: []string{"listing-en-2", "listing-rel-2"}},
		{name: "ASCII province query folds Vietnamese value", query: `{"match":{"province_name":"ho chi minh"}}`, wantIDs: []string{"listing-en-1", "listing-en-2", "listing-mixed-1", "listing-rel-1", "listing-rel-2", "listing-rel-3"}},
		{name: "province keyword keeps exact source spelling", query: `{"term":{"province_name.keyword":"Hồ Chí Minh"}}`, wantIDs: []string{"listing-en-1", "listing-en-2", "listing-mixed-1", "listing-rel-1", "listing-rel-2", "listing-rel-3"}},
		{name: "province keyword rejects ASCII spelling", query: `{"term":{"province_name.keyword":"Ho Chi Minh"}}`, wantIDs: nil},
		{name: "ward keyword keeps exact source spelling", query: `{"term":{"ward_name.keyword":"Thảo Điền"}}`, wantIDs: []string{"listing-en-1", "listing-mixed-1", "listing-rel-3"}},
		{name: "number and unit stay searchable in mixed title", query: `{"match":{"title":"220m2"}}`, wantIDs: []string{"listing-mixed-1"}},
	})
}

func relevanceVietnameseListings() []marketplace.MarketplaceListingDocument {
	return []marketplace.MarketplaceListingDocument{
		listingDocument("listing-rel-1", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Nhà phố Bến Nghé Quận 1"
			document.Description = "Nhà phố mặt tiền đường Lê Lợi"
			document.Type = marketplace.EstateTypeHouse
			document.Purpose = marketplace.EstatePurposeSale
			document.WardName = "Bến Nghé"
			document.Address = "12 Lê Lợi"
		}),
		listingDocument("listing-rel-2", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Căn hộ chung cư Quận 7"
			document.Description = "Căn hộ chung cư gần trung tâm"
			document.Type = marketplace.EstateTypeApartment
			document.Purpose = marketplace.EstatePurposeRent
			document.WardID = "30000000-0000-4000-8000-000000000003"
			document.WardName = "Tân Phú"
			document.Address = "45 Nguyễn Thị Thập"
		}),
		listingDocument("listing-rel-3", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Biệt thự Quận 2 Thảo Điền"
			document.Description = "Biệt thự sân vườn"
			document.Type = marketplace.EstateTypeVilla
			document.Purpose = marketplace.EstatePurposeSale
			document.WardID = "30000000-0000-4000-8000-000000000004"
			document.WardName = "Thảo Điền"
			document.Address = "9 Nguyễn Văn Hưởng"
		}),
	}
}

func relevanceEnglishListings() []marketplace.MarketplaceListingDocument {
	return []marketplace.MarketplaceListingDocument{
		listingDocument("listing-en-1", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Modern villa in Thảo Điền"
			document.Description = "Modern villa with a pool"
			document.Type = marketplace.EstateTypeVilla
			document.Purpose = marketplace.EstatePurposeSale
			document.WardID = "30000000-0000-4000-8000-000000000004"
			document.WardName = "Thảo Điền"
			document.Address = "25 Nguyễn Văn Hưởng"
		}),
		listingDocument("listing-en-2", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Apartment for rent in Quận 7"
			document.Description = "Apartment near the city centre"
			document.Type = marketplace.EstateTypeApartment
			document.Purpose = marketplace.EstatePurposeRent
			document.WardID = "30000000-0000-4000-8000-000000000003"
			document.WardName = "Tân Phú"
			document.Address = "88 Nguyễn Thị Thập"
		}),
		listingDocument("listing-mixed-1", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Villa Thảo Điền 220m2"
			document.Description = "Villa sân vườn"
			document.Type = marketplace.EstateTypeVilla
			document.Purpose = marketplace.EstatePurposeSale
			document.WardID = "30000000-0000-4000-8000-000000000004"
			document.WardName = "Thảo Điền"
			document.Address = "9 Nguyễn Văn Hưởng"
		}),
	}
}
