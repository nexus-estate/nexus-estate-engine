package search

import (
	"time"

	"github.com/nexus-estate/nexus-estate-engine/internal/marketplace"
)

// MarketplaceDocumentToPropertySearchItem maps the canonical listing projection
// onto the legacy Search v1 item so a future listing-centric index can still
// serve the current response shape.
//
// Identity is preserved deliberately: the item id is always the document listing
// id, never the property id. The v1 query path, cache keys, pagination and the
// nexusestate.search.v1 contract are untouched; nothing on the current request
// path calls this mapping yet.
func MarketplaceDocumentToPropertySearchItem(doc marketplace.MarketplaceListingDocument) PropertySearchItem {
	item := PropertySearchItem{
		ID:          doc.ListingID,
		Title:       doc.Title,
		Slug:        doc.Slug,
		Description: doc.Description,
		Type:        doc.Type,
		Purpose:     doc.Purpose,
		Price:       doc.Price,
		Area:        doc.Area,
		City:        doc.City,
		District:    doc.District,
		Ward:        doc.Ward,
		Address:     doc.Address,
		Images:      doc.Media.Images,
	}

	if doc.Location != nil && doc.Location.Lat != nil && doc.Location.Lon != nil {
		item.Latitude = *doc.Location.Lat
		item.Longitude = *doc.Location.Lon
	}

	// v1 carries the publication timestamp as an opaque string. UTC RFC3339 keeps
	// the mapping deterministic for the same instant.
	if doc.PublishedAt != nil {
		item.PublishedAt = doc.PublishedAt.UTC().Format(time.RFC3339)
	}

	return item
}
