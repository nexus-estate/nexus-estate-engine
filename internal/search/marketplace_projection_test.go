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
	price := 1250.0
	area := 220.0
	doc := marketplace.MarketplaceListingDocument{
		ListingID:  "listing-1",
		PropertyID: "property-1",

		Title:       "Sunny villa",
		Slug:        "sunny-villa",
		Description: "Three bedroom villa",
		Type:        "villa",
		Purpose:     "sale",

		Price: &price,
		Area:  &area,

		City:     "HCM",
		District: "1",
		Ward:     "Ben Nghe",
		Address:  "12 Le Loi",

		Location: &marketplace.GeoPoint{Lat: &lat, Lon: &lon},

		Media:       marketplace.MediaSummary{Images: []string{"a.jpg", "b.jpg"}, CoverImage: "a.jpg"},
		PublishedAt: &published,
		UpdatedAt:   updated,
	}

	item := MarketplaceDocumentToPropertySearchItem(doc)

	// Document identity stays the listing id; the property id must never leak in.
	if item.ID != doc.ListingID {
		t.Fatalf("id = %q, want listing id %q", item.ID, doc.ListingID)
	}
	if item.ID == doc.PropertyID {
		t.Fatal("property id must not become the search item id")
	}
	if item.Title != doc.Title || item.Slug != doc.Slug || item.Description != doc.Description ||
		item.Type != doc.Type || item.Purpose != doc.Purpose {
		t.Fatalf("text fields not mapped: %+v", item)
	}
	if item.Price != *doc.Price || item.Area != *doc.Area {
		t.Fatalf("numeric fields not mapped: %+v", item)
	}
	if item.City != doc.City || item.District != doc.District || item.Ward != doc.Ward || item.Address != doc.Address {
		t.Fatalf("location text fields not mapped: %+v", item)
	}
	if item.Latitude != lat || item.Longitude != lon {
		t.Fatalf("geo not mapped: %+v", item)
	}
	if len(item.Images) != 2 || item.Images[0] != "a.jpg" || item.Images[1] != "b.jpg" {
		t.Fatalf("media not mapped: %+v", item)
	}
	// Publication time is normalized to UTC with its fractional precision intact.
	if want := published.UTC().Format(time.RFC3339Nano); item.PublishedAt != want {
		t.Fatalf("publishedAt = %q, want %q", item.PublishedAt, want)
	}
	if item.PublishedAt[len(item.PublishedAt)-1] != 'Z' {
		t.Fatalf("publishedAt = %q, want a UTC timestamp", item.PublishedAt)
	}
	if item.PublishedAt != "2026-02-02T21:05:06.123456789Z" {
		t.Fatalf("fractional timestamp precision was not preserved: %q", item.PublishedAt)
	}
}

func TestMarketplaceDocumentToPropertySearchItemWithoutOptionalData(t *testing.T) {
	doc := marketplace.MarketplaceListingDocument{ListingID: "listing-2", PropertyID: "property-2", Title: "Land"}

	item := MarketplaceDocumentToPropertySearchItem(doc)

	if item.ID != "listing-2" {
		t.Fatalf("id = %q, want listing-2", item.ID)
	}
	if item.Latitude != 0 || item.Longitude != 0 || item.Images != nil || item.PublishedAt != "" {
		t.Fatalf("absent optional data must stay empty: %+v", item)
	}
}

func TestMarketplaceDocumentToPropertySearchItemCopiesImages(t *testing.T) {
	doc := marketplace.MarketplaceListingDocument{
		ListingID:  "listing-images",
		PropertyID: "property-images",
		Media:      marketplace.MediaSummary{Images: []string{"original.jpg"}},
	}

	item := MarketplaceDocumentToPropertySearchItem(doc)
	item.Images[0] = "mutated.jpg"

	if got := doc.Media.Images[0]; got != "original.jpg" {
		t.Fatalf("adapter mutation changed source document image to %q", got)
	}
}
