package search

import (
	"slices"
	"time"

	"github.com/nexus-estate/nexus-estate-engine/internal/marketplace"
)

// MarketplaceDocumentToPropertySearchItem is a response-shape compatibility
// primitive for a future marketplace query path. It does not make the canonical
// marketplace index compatible with Search v1's legacy query schema.
//
// Identity is preserved deliberately: the item id is always the document listing
// id, never the property id. ProvinceName and WardName map to the legacy City and
// Ward response fields; the legacy District and Slug fields remain empty because
// the API has no authoritative values for them. The v1 query path, cache keys,
// pagination and nexusestate.search.v1 contract are untouched; nothing on the
// current request path calls this mapping yet. Price converts to float64 only at
// this compatibility boundary, so values above 2^53 may lose integer precision.
// The legacy numeric response cannot represent absent area, so nil maps to zero.
func MarketplaceDocumentToPropertySearchItem(doc marketplace.MarketplaceListingDocument) PropertySearchItem {
	item := PropertySearchItem{
		ID:          doc.ListingID,
		Title:       doc.Title,
		Description: doc.Description,
		Type:        string(doc.Type),
		Purpose:     string(doc.Purpose),
		City:        doc.ProvinceName,
		Ward:        doc.WardName,
		Address:     doc.Address,
		Price:       float64(doc.Price),
	}
	if doc.Media != nil {
		item.Images = slices.Clone(doc.Media.Images)
	}
	if doc.Area != nil {
		item.Area = *doc.Area
	}

	if doc.Location != nil && doc.Location.Lat != nil && doc.Location.Lon != nil {
		item.Latitude = *doc.Location.Lat
		item.Longitude = *doc.Location.Lon
	}

	// v1 carries the publication timestamp as an opaque string. UTC RFC3339Nano
	// preserves fractional source precision and remains valid RFC3339 when there
	// are no fractional seconds.
	if doc.PublishedAt != nil {
		item.PublishedAt = doc.PublishedAt.UTC().Format(time.RFC3339Nano)
	}

	return item
}
