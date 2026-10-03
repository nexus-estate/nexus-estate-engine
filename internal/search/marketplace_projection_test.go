package search

import (
	"testing"
	"time"

	"github.com/nexus-estate/nexus-estate-engine/internal/marketplace"
)

func TestMarketplaceDocumentToPropertySearchItem(t *testing.T) {
	published := time.Date(2026, 2, 3, 4, 5, 6, 123_456_789, time.FixedZone("ICT", 7*60*60))
	updated := time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC)
	lat, lon := 10.75, 106.66
	area := 220.0
	doc := marketplace.MarketplaceListingDocument{
		ListingID:    "listing-1",
		PropertyID:   "property-1",
		Title:        "Sunny villa",
		Description:  "Three bedroom villa",
		Type:         marketplace.EstateTypeVilla,
		Purpose:      marketplace.EstatePurposeSale,
		Price:        1250,
		Area:         &area,
		ProvinceID:   "30000000-0000-4000-8000-000000000001",
		ProvinceName: "Hồ Chí Minh",
		WardID:       "30000000-0000-4000-8000-000000000002",
		WardName:     "Bến Nghé",
		Address:      "12 Lê Lợi",
		Location:     &marketplace.GeoPoint{Lat: &lat, Lon: &lon},
		Media:        &marketplace.MediaSummary{Images: []string{"a.jpg", "b.jpg"}},
		PublishedAt:  &published,
		UpdatedAt:    updated,
	}

	item := MarketplaceDocumentToPropertySearchItem(doc)
	if item.ID != doc.ListingID || item.ID == doc.PropertyID {
		t.Fatalf("legacy item identity = %q; want listing id %q", item.ID, doc.ListingID)
	}
	if item.Title != doc.Title || item.Description != doc.Description || item.Type != string(doc.Type) || item.Purpose != string(doc.Purpose) {
		t.Fatalf("text fields not mapped: %+v", item)
	}
	if item.Price != float64(doc.Price) || item.Area != *doc.Area {
		t.Fatalf("legacy numeric fields not mapped: %+v", item)
	}
	if item.City != doc.ProvinceName || item.Ward != doc.WardName || item.District != "" || item.Slug != "" || item.Address != doc.Address {
		t.Fatalf("lossy legacy location mapping is wrong: %+v", item)
	}
	if item.Latitude != lat || item.Longitude != lon {
		t.Fatalf("geo not mapped: %+v", item)
	}
	if len(item.Images) != 2 || item.Images[0] != "a.jpg" || item.Images[1] != "b.jpg" {
		t.Fatalf("media not mapped: %+v", item)
	}
	if want := published.UTC().Format(time.RFC3339Nano); item.PublishedAt != want {
		t.Fatalf("publishedAt = %q, want %q", item.PublishedAt, want)
	}
	if item.PublishedAt != "2026-02-02T21:05:06.123456789Z" {
		t.Fatalf("fractional timestamp precision was not preserved: %q", item.PublishedAt)
	}
}

func TestMarketplaceDocumentToPropertySearchItemMapsMissingAreaToLegacyZero(t *testing.T) {
	doc := marketplace.MarketplaceListingDocument{
		ListingID:    "listing-2",
		PropertyID:   "property-2",
		ProvinceName: "Hồ Chí Minh",
		WardName:     "Bến Nghé",
		Price:        0,
	}

	item := MarketplaceDocumentToPropertySearchItem(doc)
	if item.ID != "listing-2" || item.Area != 0 || item.Price != 0 {
		t.Fatalf("legacy zero representation is unexpected: %+v", item)
	}
	if item.Latitude != 0 || item.Longitude != 0 || item.Images != nil || item.PublishedAt != "" {
		t.Fatalf("absent optional data must stay empty: %+v", item)
	}
}

func TestMarketplaceDocumentToPropertySearchItemCopiesImages(t *testing.T) {
	doc := marketplace.MarketplaceListingDocument{
		ListingID: "listing-images",
		Media:     &marketplace.MediaSummary{Images: []string{"original.jpg"}},
	}
	item := MarketplaceDocumentToPropertySearchItem(doc)
	item.Images[0] = "mutated.jpg"
	if got := doc.Media.Images[0]; got != "original.jpg" {
		t.Fatalf("adapter mutation changed source document image to %q", got)
	}
}

func TestMarketplaceDocumentToPropertySearchItemLegacyPricePrecisionLimit(t *testing.T) {
	const exactPrice int64 = 9_007_199_254_740_993
	doc := marketplace.MarketplaceListingDocument{ListingID: "listing-large-price", Price: exactPrice}
	item := MarketplaceDocumentToPropertySearchItem(doc)
	if item.Price != float64(exactPrice) {
		t.Fatalf("legacy price = %.0f, want float64 conversion %.0f", item.Price, float64(exactPrice))
	}
	if int64(item.Price) == exactPrice {
		t.Fatal("legacy float64 response unexpectedly preserved an integer above 2^53")
	}
}
