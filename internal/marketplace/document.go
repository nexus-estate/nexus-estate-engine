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
// supply asset and a listing is a marketplace publication of it. Those are
// distinct domain identities, and the publication document remains keyed by
// listing_id regardless of current database cardinality constraints.
package marketplace

import "time"

// MaxMarketplaceTitleLength and MaxMarketplaceAddressLength mirror the API's
// varchar(500) source limits. Exact-value keyword mappings must index the full
// valid source range.
const (
	MaxMarketplaceTitleLength   = 500
	MaxMarketplaceAddressLength = 500
)

// MaxMarketplaceImages bounds the image URLs copied into a search projection.
// It is a projection/result-size limit and does not constrain API media storage.
const MaxMarketplaceImages = 20

// EstateType and EstatePurpose preserve the source API enum values verbatim.
type EstateType string

const (
	EstateTypeApartment  EstateType = "APARTMENT"
	EstateTypeHouse      EstateType = "HOUSE"
	EstateTypeVilla      EstateType = "VILLA"
	EstateTypeTownhouse  EstateType = "TOWNHOUSE"
	EstateTypeLand       EstateType = "LAND"
	EstateTypeOffice     EstateType = "OFFICE"
	EstateTypeShophouse  EstateType = "SHOPHOUSE"
	EstateTypeWarehouse  EstateType = "WAREHOUSE"
	EstateTypeCommercial EstateType = "COMMERCIAL"
	EstateTypeHotel      EstateType = "HOTEL"
	EstateTypeResort     EstateType = "RESORT"
	EstateTypeFarm       EstateType = "FARM"
	EstateTypeOther      EstateType = "OTHER"
)

type EstatePurpose string

const (
	EstatePurposeSale       EstatePurpose = "SALE"
	EstatePurposeRent       EstatePurpose = "RENT"
	EstatePurposeSaleOrRent EstatePurpose = "SALE_OR_RENT"
)

// MediaSummary is a bounded projection of source ordered IMAGE media. Cover
// selection is omitted because the API does not currently define one.
type MediaSummary struct {
	Images []string `json:"images,omitempty"`
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

	// Title, Type and Purpose are required source Estate fields. Description is
	// optional. Enum casing is preserved exactly as supplied by the API.
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Type        EstateType    `json:"type"`
	Purpose     EstatePurpose `json:"purpose"`

	// Price is the required PostgreSQL bigint source value; zero is a real value.
	// Area is nullable in the API; nil means unknown, while a present value must
	// be positive.
	Price int64    `json:"price"`
	Area  *float64 `json:"area,omitempty"`

	// Location preserves the API administrative identities and display names.
	// The source has province and ward, with no authoritative district field.
	ProvinceID   string `json:"province_id"`
	ProvinceName string `json:"province_name"`
	WardID       string `json:"ward_id"`
	WardName     string `json:"ward_name"`
	Address      string `json:"address"`

	// Location is absent for listings without a geocoded address.
	Location *GeoPoint `json:"location,omitempty"`

	Media *MediaSummary `json:"media,omitempty"`

	// PublishedAt is the API Listing.publishedAt instant and the marketplace
	// recency sort field. It is present only after the API publishes the listing.
	// UpdatedAt is the API Listing.updatedAt timestamp for audit/display; it is not
	// monotonic and must never be used for event ordering. Neither timestamp is a
	// publication decision the Engine may revise.
	PublishedAt *time.Time `json:"published_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
