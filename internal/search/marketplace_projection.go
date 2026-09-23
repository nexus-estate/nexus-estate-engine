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
// id, never the property id. The v1 query path, cache keys, pagination and the
// nexusestate.search.v1 contract are untouched; nothing on the current request
// path calls this mapping yet. Its legacy numeric response cannot represent an
// absent area, so nil is mapped to zero; the adapter is intentionally not a
// lossless decoder for the nullable marketplace schema.
func MarketplaceDocumentToPropertySearchItem(doc marketplace.MarketplaceListingDocument) PropertySearchItem {
	item := PropertySearchItem{
		ID:          doc.ListingID,
		Title:       doc.Title,
		Slug:        doc.Slug,
		Description: doc.Description,
		Type:        doc.Type,
		Purpose:     doc.Purpose,
		City:        doc.City,
		District:    doc.District,
		Ward:        doc.Ward,
		Address:     doc.Address,
		Images:      slices.Clone(doc.Media.Images),
	}
	if doc.Price != nil {
		item.Price = *doc.Price
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
