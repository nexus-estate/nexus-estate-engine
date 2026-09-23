package marketplace

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"
)

const (
	testLatitude  = 10.75
	testLongitude = 106.66
)

func floatPtr(v float64) *float64 { return &v }

func timePtr(t time.Time) *time.Time { return &t }

// validDocument returns a fresh valid published listing. Tests mutate the copy so
// shared pointers and slices never leak between cases.
func validDocument() MarketplaceListingDocument {
	published := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	updated := time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC)
	return MarketplaceListingDocument{
		ListingID:  "listing-1",
		PropertyID: "property-1",

		Title:       "Sunny villa in district one",
		Slug:        "sunny-villa-district-one",
		Description: "Three bedroom villa",
		Type:        "villa",
		Purpose:     "sale",

		Price: floatPtr(1_250_000_000),
		Area:  floatPtr(220),

		City:     "HCM",
		District: "1",
		Ward:     "Ben Nghe",
		Address:  "12 Le Loi",

		Location: &GeoPoint{Lat: floatPtr(testLatitude), Lon: floatPtr(testLongitude)},

		Media: MediaSummary{Images: []string{"a.jpg", "b.jpg"}, CoverImage: "a.jpg"},

		PublishedAt: timePtr(published),
		UpdatedAt:   updated,
	}
}

func TestValidateMarketplaceListingDocument(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*MarketplaceListingDocument)
		want   error
		field  string
	}{
		{name: "valid published listing", mutate: func(*MarketplaceListingDocument) {}},
		{name: "valid without geo", mutate: func(d *MarketplaceListingDocument) { d.Location = nil }},
		{name: "missing listing id", mutate: func(d *MarketplaceListingDocument) { d.ListingID = "" }, want: ErrMissingListingID, field: "listing_id"},
		{name: "missing property id", mutate: func(d *MarketplaceListingDocument) { d.PropertyID = "" }, want: ErrMissingPropertyID, field: "property_id"},
		{name: "missing title", mutate: func(d *MarketplaceListingDocument) { d.Title = "" }, want: ErrMissingTitle, field: "title"},
		{name: "missing type", mutate: func(d *MarketplaceListingDocument) { d.Type = "" }, want: ErrMissingType, field: "type"},
		{name: "missing purpose", mutate: func(d *MarketplaceListingDocument) { d.Purpose = "" }, want: ErrMissingPurpose, field: "purpose"},
		{name: "missing city", mutate: func(d *MarketplaceListingDocument) { d.City = "" }, want: ErrMissingCity, field: "city"},
		{name: "missing ward", mutate: func(d *MarketplaceListingDocument) { d.Ward = "" }, want: ErrMissingWard, field: "ward"},
		{name: "missing address", mutate: func(d *MarketplaceListingDocument) { d.Address = "" }, want: ErrMissingAddress, field: "address"},
		{name: "missing published at", mutate: func(d *MarketplaceListingDocument) { d.PublishedAt = nil }, want: ErrMissingPublishedAt, field: "published_at"},
		{name: "zero published at", mutate: func(d *MarketplaceListingDocument) { d.PublishedAt = timePtr(time.Time{}) }, want: ErrMissingPublishedAt, field: "published_at"},
		{name: "missing updated at", mutate: func(d *MarketplaceListingDocument) { d.UpdatedAt = time.Time{} }, want: ErrMissingUpdatedAt, field: "updated_at"},
		{name: "missing price", mutate: func(d *MarketplaceListingDocument) { d.Price = nil }, want: ErrMissingPrice, field: "price"},
		{name: "zero price is a valid source value", mutate: func(d *MarketplaceListingDocument) { d.Price = floatPtr(0) }},
		{name: "negative price", mutate: func(d *MarketplaceListingDocument) { d.Price = floatPtr(-1) }, want: ErrInvalidPrice, field: "price"},
		{name: "nan price", mutate: func(d *MarketplaceListingDocument) { d.Price = floatPtr(math.NaN()) }, want: ErrInvalidPrice, field: "price"},
		{name: "infinite price", mutate: func(d *MarketplaceListingDocument) { d.Price = floatPtr(math.Inf(1)) }, want: ErrInvalidPrice, field: "price"},
		{name: "unknown area is absent", mutate: func(d *MarketplaceListingDocument) { d.Area = nil }},
		{name: "zero area is invalid when present", mutate: func(d *MarketplaceListingDocument) { d.Area = floatPtr(0) }, want: ErrInvalidArea, field: "area"},
		{name: "negative area", mutate: func(d *MarketplaceListingDocument) { d.Area = floatPtr(-0.5) }, want: ErrInvalidArea, field: "area"},
		{name: "nan area", mutate: func(d *MarketplaceListingDocument) { d.Area = floatPtr(math.NaN()) }, want: ErrInvalidArea, field: "area"},
		{name: "missing longitude", mutate: func(d *MarketplaceListingDocument) { d.Location.Lon = nil }, want: ErrIncompleteGeo, field: "location"},
		{name: "missing latitude", mutate: func(d *MarketplaceListingDocument) { d.Location.Lat = nil }, want: ErrIncompleteGeo, field: "location"},
		{name: "latitude above range", mutate: func(d *MarketplaceListingDocument) { d.Location.Lat = floatPtr(90.5) }, want: ErrInvalidLatitude, field: "location.lat"},
		{name: "latitude below range", mutate: func(d *MarketplaceListingDocument) { d.Location.Lat = floatPtr(-91) }, want: ErrInvalidLatitude, field: "location.lat"},
		{name: "nan latitude", mutate: func(d *MarketplaceListingDocument) { d.Location.Lat = floatPtr(math.NaN()) }, want: ErrInvalidLatitude, field: "location.lat"},
		{name: "longitude above range", mutate: func(d *MarketplaceListingDocument) { d.Location.Lon = floatPtr(180.1) }, want: ErrInvalidLongitude, field: "location.lon"},
		{name: "nan longitude", mutate: func(d *MarketplaceListingDocument) { d.Location.Lon = floatPtr(math.NaN()) }, want: ErrInvalidLongitude, field: "location.lon"},
		{
			name: "identity reported before later violations",
			mutate: func(d *MarketplaceListingDocument) {
				d.ListingID = ""
			},
			want:  ErrMissingListingID,
			field: "listing_id",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := validDocument()
			tc.mutate(&doc)

			err := ValidateMarketplaceListingDocument(doc)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("expected valid document, got %v", err)
				}
				return
			}

			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want class %v", err, tc.want)
			}

			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error %v is not a *ValidationError", err)
			}
			if validationErr.Field != tc.field {
				t.Fatalf("field = %q, want %q", validationErr.Field, tc.field)
			}
		})
	}
}

func TestValidateMarketplaceListingDocumentAcceptsBoundaryCoordinates(t *testing.T) {
	doc := validDocument()
	doc.Location = &GeoPoint{Lat: floatPtr(-90), Lon: floatPtr(180)}
	if err := ValidateMarketplaceListingDocument(doc); err != nil {
		t.Fatalf("boundary coordinates must be valid: %v", err)
	}
	doc.Location = &GeoPoint{Lat: floatPtr(0), Lon: floatPtr(0)}
	if err := ValidateMarketplaceListingDocument(doc); err != nil {
		t.Fatalf("zero coordinates must be valid: %v", err)
	}
}

func TestDocumentJSONContractRoundTrip(t *testing.T) {
	doc := validDocument()

	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The same source snapshot must serialize identically.
	again, err := json.Marshal(validDocument())
	if err != nil {
		t.Fatalf("marshal again: %v", err)
	}
	if !bytes.Equal(encoded, again) {
		t.Fatalf("same source snapshot produced different documents:\n%s\n%s", encoded, again)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal fields: %v", err)
	}
	for _, key := range []string{"listing_id", "property_id", "location", "published_at", "updated_at", "media", "price"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("canonical field %q missing from %s", key, encoded)
		}
	}
	if _, ok := fields["aggregate_version"]; ok {
		t.Fatalf("document must not claim an upstream version the API does not provide: %s", encoded)
	}

	var decoded MarketplaceListingDocument
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	if !reflect.DeepEqual(doc, decoded) {
		t.Fatalf("round trip changed the document:\ngot  %+v\nwant %+v", decoded, doc)
	}

	var location struct {
		Lat *float64 `json:"lat"`
		Lon *float64 `json:"lon"`
	}
	if err := json.Unmarshal(fields["location"], &location); err != nil {
		t.Fatalf("unmarshal location: %v", err)
	}
	if location.Lat == nil || location.Lon == nil || *location.Lat != testLatitude || *location.Lon != testLongitude {
		t.Fatalf("geo_point shape changed: %s", fields["location"])
	}

	if err := ValidateMarketplaceListingDocument(decoded); err != nil {
		t.Fatalf("decoded document must still validate: %v", err)
	}
}

func TestDocumentJSONDistinguishesZeroPriceFromUnknownArea(t *testing.T) {
	doc := validDocument()
	doc.Price = floatPtr(0)
	doc.Area = nil

	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal fields: %v", err)
	}
	if got := string(fields["price"]); got != "0" {
		t.Fatalf("zero price encoded as %q, want 0", got)
	}
	if _, ok := fields["area"]; ok {
		t.Fatalf("unknown area must be omitted, got %s", fields["area"])
	}
	if err := ValidateMarketplaceListingDocument(doc); err != nil {
		t.Fatalf("zero price with unknown area must validate: %v", err)
	}
}
