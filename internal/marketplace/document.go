// Package marketplace owns the canonical Listing-centric projection contract for
// future indexing, reindex and marketplace search consumers. No consumer is wired
// yet; the API must supply authoritative lifecycle and source ordering data first.
//
// API/PostgreSQL remains the source of truth; the Engine only owns derived state.
// A MarketplaceListingDocument is such derived state: a projection may only be
// created for a listing the API has already confirmed public and searchable, so
// this model carries no lifecycle status field and never re-derives the
// publication decision from status. A future write may materialize the document
// only in response to that upstream decision.
//
// Document identity is the listing id, never the property id: a property is a
// supply asset and a listing is a marketplace publication of it. The API schema
// currently enforces a unique listing per estate; that database cardinality does
// not change which domain identity a publication document represents.
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

	// Title, Type and Purpose are required source Estate fields. Slug and
	// Description are optional; Engine must not derive a slug from the title.
	Title       string `json:"title"`
	Slug        string `json:"slug,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Purpose     string `json:"purpose"`

	// Price is required by the API estate contract. A non-nil zero is a real zero
	// price; nil is invalid. Area is nullable in the API; nil means unknown, while
	// a non-nil value must be positive.
	Price *float64 `json:"price"`
	Area  *float64 `json:"area,omitempty"`

	// City and Ward are API province/ward display names; Address is the source
	// Estate address. District is optional because the current API has no district
	// field. Empty values are omitted.
	City     string `json:"city,omitempty"`
	District string `json:"district,omitempty"`
	Ward     string `json:"ward,omitempty"`
	Address  string `json:"address,omitempty"`

	// Location is absent for listings without a geocoded address.
	Location *GeoPoint `json:"location,omitempty"`

	Media MediaSummary `json:"media"`

	// PublishedAt is the API Listing.publishedAt instant and the marketplace
	// recency sort field. It is present only after the API publishes the listing.
	// UpdatedAt is the API Listing.updatedAt timestamp for audit/display; it is not
	// monotonic and must never be used for event ordering. Neither timestamp is a
	// publication decision the Engine may revise.
	PublishedAt *time.Time `json:"published_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
