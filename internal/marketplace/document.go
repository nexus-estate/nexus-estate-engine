// Package marketplace owns the canonical Listing-centric projection model that the
// Engine materializes for indexing, reindex and Search v2.
//
// API/PostgreSQL remains the source of truth; the Engine only owns derived state.
// A MarketplaceListingDocument is such derived state: a projection may only be
// created for a listing the API has already confirmed public and searchable, so
// this model carries no lifecycle status field and never re-derives the
// publication decision from status. Materializing the document *is* the public
// assertion.
//
// Document identity is the listing id, never the property id: a property is a
// supply asset and a listing is a marketplace publication of it, and one property
// can be published more than once.
package marketplace

import "time"

// MediaSummary is a bounded projection of listing media so a search result does
// not need the full media aggregate. Images keeps source order; CoverImage is the
// listing's chosen cover and may be empty.
type MediaSummary struct {
	Images     []string `json:"images,omitempty"`
	CoverImage string   `json:"cover_image,omitempty"`
}

// GeoPoint is the canonical location representation and matches the
// Elasticsearch geo_point object form {"lat": <lat>, "lon": <lon>} that the
// geo_distance query already uses.
//
// Both coordinates are optional pointers so a half-supplied pair is detectable by
// ValidateMarketplaceListingDocument as incomplete instead of silently defaulting
// to the valid coordinate 0.
type GeoPoint struct {
	Lat *float64 `json:"lat"`
	Lon *float64 `json:"lon"`
}

// MarketplaceListingDocument is the canonical search projection of one
// marketplace publication. It is immutable by convention: helpers and adapters
// must not mutate a document a caller still owns.
//
// JSON names are snake_case and are the canonical wire/index field contract. The
// checked-in index definition, reindex and Search v2 must reuse these names; they
// are intentionally independent from the Search v1 item contract, which keeps its
// own camelCase Elasticsearch fields.
type MarketplaceListingDocument struct {
	// ListingID is the document identity. PropertyID describes the underlying
	// supply asset and never substitutes for the listing identity.
	ListingID  string `json:"listing_id"`
	PropertyID string `json:"property_id"`

	// Searchable content.
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Purpose     string `json:"purpose"`

	// Price and Area are range filters. They are non-pointer so the zero value is
	// a valid "unknown/not provided" price or area rather than an invalid one.
	Price float64 `json:"price"`
	Area  float64 `json:"area"`

	City     string `json:"city"`
	District string `json:"district"`
	Ward     string `json:"ward"`
	Address  string `json:"address"`

	// Location is absent for listings without a geocoded address.
	Location *GeoPoint `json:"location,omitempty"`

	Media MediaSummary `json:"media"`

	// PublishedAt is when the listing became public and is the projection's
	// ordering timestamp. UpdatedAt is the source revision time kept for audit.
	// Neither is a publication decision the Engine may revise. Callers must
	// supply UTC instants so the same listing and version always serialize
	// identically.
	PublishedAt *time.Time `json:"published_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`

	// AggregateVersion is the monotonic source revision of the listing aggregate
	// this document was projected from. It orders projection writes; it is not an
	// event id and must be greater than zero. See IsStaleVersion for the
	// supersession rule.
	AggregateVersion int64 `json:"aggregate_version"`
}
