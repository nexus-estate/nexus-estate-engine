package integration

import (
	"testing"

	"github.com/nexus-estate/nexus-estate-engine/internal/marketplace"
)

// The relevance gates use one test-owned index per corpus so every expectation is
// asserted against a known corpus rather than whatever another test happened to
// write. Both indexes are deleted after the run.
const (
	relevanceIndexName      = "nexus_estate_marketplace_relevance_integration"
	relevanceMixedIndexName = "nexus_estate_marketplace_relevance_mixed_integration"
)

// TestMarketplaceListingSearchRelevance pins the search behavior the listing_text
// analyzer promises on real Vietnamese listing data: syllable-tokenized compound
// words, diacritic-insensitive matching in both directions, and exact (never
// folded) keyword filtering.
//
// The expectations also record where folding is not enough. The standard tokenizer
// splits Vietnamese into syllables, so `match` defaults to OR over syllables and is
// therefore order-insensitive, and a single syllable of a multi-syllable place name
// ("quan" in every "Quận <n>") is not selective at all: multi-syllable filters need
// an explicit AND operator or match_phrase. Keyword fields and their .keyword
// subfields stay exact, so the write layer must send source values for term filters.
//
// Run against a running local Elasticsearch; see deploy/README.md.
func TestMarketplaceListingSearchRelevance(t *testing.T) {
	client, ctx := listingIndexFixture(t, relevanceIndexName)

	seedListingDocuments(ctx, t, client, relevanceIndexName, relevanceVietnameseListings()...)

	runSearchCases(t, ctx, client, relevanceIndexName, []searchCase{
		{
			name:    "unaccented compound word in title",
			query:   `{"match":{"title":"nha pho"}}`,
			wantIDs: []string{"listing-rel-1"},
		},
		{
			name:    "accented compound word in title",
			query:   `{"match":{"title":"Nhà phố"}}`,
			wantIDs: []string{"listing-rel-1"},
		},
		{
			name:    "mixed accented and unaccented tokens",
			query:   `{"match":{"title":"nha phố"}}`,
			wantIDs: []string{"listing-rel-1"},
		},
		{
			name:    "hyphenated compound matches spaced text",
			query:   `{"match":{"title":"chung-cư"}}`,
			wantIDs: []string{"listing-rel-2"},
		},
		{
			name:    "unaccented compound in description",
			query:   `{"match":{"description":"can ho chung cu"}}`,
			wantIDs: []string{"listing-rel-2"},
		},
		{
			name:    "unaccented place name in ward",
			query:   `{"match":{"ward":"ben nghe"}}`,
			wantIDs: []string{"listing-rel-1"},
		},
		{
			name:    "accented place name in ward",
			query:   `{"match":{"ward":"Bến Nghé"}}`,
			wantIDs: []string{"listing-rel-1"},
		},
		{
			name:    "d stroke and accented place name fold",
			query:   `{"match":{"ward":"thao dien"}}`,
			wantIDs: []string{"listing-rel-3"},
		},
		{
			name:    "unaccented street name in address",
			query:   `{"match":{"address":"le loi"}}`,
			wantIDs: []string{"listing-rel-1"},
		},
		{
			// Every district starts with the syllable "quan", so the default OR
			// operator matches all three. Recorded as a hazard: a single syllable of
			// a multi-syllable Vietnamese place name is not selective.
			name:    "unaccented district single match token over-matches shared syllables",
			query:   `{"match":{"district":"quan 7"}}`,
			wantIDs: []string{"listing-rel-1", "listing-rel-2", "listing-rel-3"},
		},
		{
			name:    "unaccented district with and operator isolates the listing",
			query:   `{"match":{"district":{"query":"quan 7","operator":"and"}}}`,
			wantIDs: []string{"listing-rel-2"},
		},
		{
			name:    "unaccented district phrase isolates the listing",
			query:   `{"match_phrase":{"district":"quan 7"}}`,
			wantIDs: []string{"listing-rel-2"},
		},
		{
			name:    "unaccented city matches every listing",
			query:   `{"match":{"city":"ho chi minh"}}`,
			wantIDs: []string{"listing-rel-1", "listing-rel-2", "listing-rel-3"},
		},
		{
			name:    "match phrase keeps token order",
			query:   `{"match_phrase":{"title":"nha pho ben nghe"}}`,
			wantIDs: []string{"listing-rel-1"},
		},
		{
			name:    "match phrase rejects reordered tokens",
			query:   `{"match_phrase":{"title":"pho nha"}}`,
			wantIDs: nil,
		},
		{
			// Syllable tokenization means `match` is order-insensitive; only
			// match_phrase is order-sensitive. Recorded so relevance changes are
			// deliberate.
			name:    "match ignores token order",
			query:   `{"match":{"title":"pho nha"}}`,
			wantIDs: []string{"listing-rel-1"},
		},
		{
			name:    "keyword subfield does not fold",
			query:   `{"term":{"city.keyword":"ho chi minh"}}`,
			wantIDs: nil,
		},
		{
			name:    "keyword subfield matches the source value",
			query:   `{"term":{"city.keyword":"Hồ Chí Minh"}}`,
			wantIDs: []string{"listing-rel-1", "listing-rel-2", "listing-rel-3"},
		},
		{
			name:    "keyword field stays exact",
			query:   `{"term":{"type":"villa"}}`,
			wantIDs: []string{"listing-rel-3"},
		},
	})
}

// TestMarketplaceListingSearchMixedLanguage extends the relevance set with
// English/ASCII listings to pin cross-language behavior: ASCII text is searchable
// through the same analyzer, ASCII and accented spellings of the same place name
// collide by design after folding, keyword filters keep language and case exactness
// per source value, and the shared-syllable over-match hazard is language-independent.
//
// Run against a running local Elasticsearch; see deploy/README.md.
func TestMarketplaceListingSearchMixedLanguage(t *testing.T) {
	client, ctx := listingIndexFixture(t, relevanceMixedIndexName)

	documents := append(relevanceVietnameseListings(), relevanceEnglishListings()...)
	seedListingDocuments(ctx, t, client, relevanceMixedIndexName, documents...)

	runSearchCases(t, ctx, client, relevanceMixedIndexName, []searchCase{
		{
			name:    "ascii term finds the ascii listing",
			query:   `{"match":{"title":"apartment"}}`,
			wantIDs: []string{"listing-en-2"},
		},
		{
			name:    "ascii query does not recall vietnamese listings",
			query:   `{"match":{"title":"modern"}}`,
			wantIDs: []string{"listing-en-1"},
		},
		{
			name:    "vietnamese query does not recall ascii listings",
			query:   `{"match":{"title":"nha pho"}}`,
			wantIDs: []string{"listing-rel-1"},
		},
		{
			// The bilingual listing carries both spellings, so an ASCII term finds
			// it and an ASCII listing alongside it.
			name:    "ascii token in a bilingual title",
			query:   `{"match":{"title":"villa"}}`,
			wantIDs: []string{"listing-en-1", "listing-mixed-1"},
		},
		{
			name:    "folded vietnamese title is searchable without accents",
			query:   `{"match":{"title":"biet thu"}}`,
			wantIDs: []string{"listing-rel-3"},
		},
		{
			// Folding makes the Vietnamese and ASCII spellings of one place name the
			// same tokens on purpose: the ward filter recalls all of them.
			name:    "accented and ascii ward spellings collide after folding",
			query:   `{"match":{"ward":"thao dien"}}`,
			wantIDs: []string{"listing-en-1", "listing-mixed-1", "listing-rel-3"},
		},
		{
			name:    "accented and ascii ward spellings collide in the second listing",
			query:   `{"match":{"ward":"tan phu"}}`,
			wantIDs: []string{"listing-en-2", "listing-rel-2"},
		},
		{
			name:    "folded city recalls both languages",
			query:   `{"match":{"city":"ho chi minh"}}`,
			wantIDs: []string{"listing-en-1", "listing-en-2", "listing-mixed-1", "listing-rel-1", "listing-rel-2", "listing-rel-3"},
		},
		{
			// Each spelling is a distinct source value, so only the exact one matches.
			// The bilingual listing is stored with the accented city, so it groups with
			// the Vietnamese listings even though its title mixes languages: term
			// filtering follows the source value, not the language of the text.
			name:    "keyword city distinguishes the accented source value",
			query:   `{"term":{"city.keyword":"Hồ Chí Minh"}}`,
			wantIDs: []string{"listing-mixed-1", "listing-rel-1", "listing-rel-2", "listing-rel-3"},
		},
		{
			name:    "keyword city distinguishes the ascii source value",
			query:   `{"term":{"city.keyword":"Ho Chi Minh"}}`,
			wantIDs: []string{"listing-en-1", "listing-en-2"},
		},
		{
			// The same OR hazard as "quan": every district in this corpus contains
			// the shared token, so the default operator over-matches.
			name:    "shared ascii district token over-matches",
			query:   `{"match":{"district":"district 7"}}`,
			wantIDs: []string{"listing-en-1", "listing-en-2", "listing-rel-2"},
		},
		{
			name:    "and operator isolates the ascii district",
			query:   `{"match":{"district":{"query":"district 7","operator":"and"}}}`,
			wantIDs: []string{"listing-en-2"},
		},
		{
			// Units glued to a number stay one token, so the bilingual listing is
			// findable by its area text as written.
			name:    "number glued to a unit stays one token",
			query:   `{"match":{"title":"220m2"}}`,
			wantIDs: []string{"listing-mixed-1"},
		},
	})
}

// relevanceVietnameseListings is the base Vietnamese corpus. The mixed-language
// test reuses it so both suites search the same Vietnamese data.
func relevanceVietnameseListings() []marketplace.MarketplaceListingDocument {
	return []marketplace.MarketplaceListingDocument{
		listingDocument("listing-rel-1", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Nhà phố Bến Nghé"
			document.Description = "Nhà phố mặt tiền đường Lê Lợi"
			document.Type = "house"
			document.Purpose = "sale"
			document.District = "Quận 1"
			document.Ward = "Bến Nghé"
			document.Address = "12 Lê Lợi"
		}),
		listingDocument("listing-rel-2", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Căn hộ chung cư Quận 7"
			document.Description = "Căn hộ chung cư gần trung tâm"
			document.Type = "apartment"
			document.Purpose = "rent"
			document.District = "Quận 7"
			document.Ward = "Tân Phú"
			document.Address = "45 Nguyễn Thị Thập"
		}),
		listingDocument("listing-rel-3", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Biệt thự Thảo Điền"
			document.Description = "Biệt thự sân vườn"
			document.Type = "villa"
			document.Purpose = "sale"
			document.District = "Quận 2"
			document.Ward = "Thảo Điền"
			document.Address = "9 Nguyễn Văn Hưởng"
		}),
	}
}

// relevanceEnglishListings adds ASCII-only listings plus one bilingual listing, so
// the same place names appear with and without diacritics and the same city appears
// under two distinct source values.
func relevanceEnglishListings() []marketplace.MarketplaceListingDocument {
	return []marketplace.MarketplaceListingDocument{
		listingDocument("listing-en-1", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Modern villa in Thao Dien"
			document.Description = "Modern villa with a pool"
			document.Type = "villa"
			document.Purpose = "sale"
			document.City = "Ho Chi Minh"
			document.District = "District 2"
			document.Ward = "Thao Dien"
			document.Address = "25 Nguyen Van Huong"
		}),
		listingDocument("listing-en-2", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Apartment for rent in District 7"
			document.Description = "Apartment near the city centre"
			document.Type = "apartment"
			document.Purpose = "rent"
			document.City = "Ho Chi Minh"
			document.District = "District 7"
			document.Ward = "Tan Phu"
			document.Address = "88 Nguyen Thi Thap"
		}),
		listingDocument("listing-mixed-1", func(document *marketplace.MarketplaceListingDocument) {
			document.Title = "Villa Thảo Điền 220m2"
			document.Description = "Villa sân vườn"
			document.Type = "villa"
			document.Purpose = "sale"
			document.City = "Hồ Chí Minh"
			document.District = "Quận 2"
			document.Ward = "Thảo Điền"
			document.Address = "9 Nguyễn Văn Hưởng"
		}),
	}
}
