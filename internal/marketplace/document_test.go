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

func validDocument() MarketplaceListingDocument {
	published := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	updated := time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC)
	return MarketplaceListingDocument{
		ListingID:    "listing-1",
		PropertyID:   "property-1",
		Title:        "Sunny villa",
		Description:  "Three bedroom villa",
		Type:         EstateTypeVilla,
		Purpose:      EstatePurposeSale,
		Price:        1_250_000_000,
		Area:         floatPtr(220),
		ProvinceID:   "30000000-0000-4000-8000-000000000001",
		ProvinceName: "Hồ Chí Minh",
		WardID:       "30000000-0000-4000-8000-000000000002",
		WardName:     "Bến Nghé",
		Address:      "12 Lê Lợi",
		Location:     &GeoPoint{Lat: floatPtr(testLatitude), Lon: floatPtr(testLongitude)},
		Media:        &MediaSummary{Images: []string{"a.jpg", "b.jpg"}},
		PublishedAt:  timePtr(published),
		UpdatedAt:    updated,
	}
}

func TestValidateMarketplaceListingDocument(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*MarketplaceListingDocument)
		want   error
		field  string
	}{
		{name: "valid API values", mutate: func(*MarketplaceListingDocument) {}},
		{name: "valid without optional geo and media", mutate: func(d *MarketplaceListingDocument) { d.Location, d.Media = nil, nil }},
		{name: "missing listing id", mutate: func(d *MarketplaceListingDocument) { d.ListingID = "" }, want: ErrMissingListingID, field: "listing_id"},
		{name: "missing property id", mutate: func(d *MarketplaceListingDocument) { d.PropertyID = "" }, want: ErrMissingPropertyID, field: "property_id"},
		{name: "missing title", mutate: func(d *MarketplaceListingDocument) { d.Title = "" }, want: ErrMissingTitle, field: "title"},
		{name: "missing type", mutate: func(d *MarketplaceListingDocument) { d.Type = "" }, want: ErrMissingType, field: "type"},
		{name: "lowercase type is not an API enum", mutate: func(d *MarketplaceListingDocument) { d.Type = "villa" }, want: ErrInvalidType, field: "type"},
		{name: "missing purpose", mutate: func(d *MarketplaceListingDocument) { d.Purpose = "" }, want: ErrMissingPurpose, field: "purpose"},
		{name: "lowercase purpose is not an API enum", mutate: func(d *MarketplaceListingDocument) { d.Purpose = "sale" }, want: ErrInvalidPurpose, field: "purpose"},
		{name: "missing province id", mutate: func(d *MarketplaceListingDocument) { d.ProvinceID = "" }, want: ErrMissingProvinceID, field: "province_id"},
		{name: "missing province name", mutate: func(d *MarketplaceListingDocument) { d.ProvinceName = "" }, want: ErrMissingProvince, field: "province_name"},
		{name: "missing ward id", mutate: func(d *MarketplaceListingDocument) { d.WardID = "" }, want: ErrMissingWardID, field: "ward_id"},
		{name: "missing ward name", mutate: func(d *MarketplaceListingDocument) { d.WardName = "" }, want: ErrMissingWardName, field: "ward_name"},
		{name: "missing address", mutate: func(d *MarketplaceListingDocument) { d.Address = "" }, want: ErrMissingAddress, field: "address"},
		{name: "missing published at", mutate: func(d *MarketplaceListingDocument) { d.PublishedAt = nil }, want: ErrMissingPublishedAt, field: "published_at"},
		{name: "zero published at", mutate: func(d *MarketplaceListingDocument) { d.PublishedAt = timePtr(time.Time{}) }, want: ErrMissingPublishedAt, field: "published_at"},
		{name: "missing updated at", mutate: func(d *MarketplaceListingDocument) { d.UpdatedAt = time.Time{} }, want: ErrMissingUpdatedAt, field: "updated_at"},
		{name: "zero price is valid", mutate: func(d *MarketplaceListingDocument) { d.Price = 0 }},
		{name: "negative price is invalid", mutate: func(d *MarketplaceListingDocument) { d.Price = -1 }, want: ErrInvalidPrice, field: "price"},
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
		{name: "too many images", mutate: func(d *MarketplaceListingDocument) { d.Media.Images = make([]string, MaxMarketplaceImages+1) }, want: ErrTooManyImages, field: "media.images"},
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
			if !errors.As(err, &validationErr) || validationErr.Field != tc.field {
				t.Fatalf("validation error = %#v, want field %q", validationErr, tc.field)
			}
		})
	}
}

func TestValidateMarketplaceListingDocumentAcceptsAPIEnumValues(t *testing.T) {
	types := []EstateType{
		EstateTypeApartment, EstateTypeHouse, EstateTypeVilla, EstateTypeTownhouse,
		EstateTypeLand, EstateTypeOffice, EstateTypeShophouse, EstateTypeWarehouse,
		EstateTypeCommercial, EstateTypeHotel, EstateTypeResort, EstateTypeFarm,
		EstateTypeOther,
	}
	purposes := []EstatePurpose{EstatePurposeSale, EstatePurposeRent, EstatePurposeSaleOrRent}
	for _, estateType := range types {
		for _, purpose := range purposes {
			doc := validDocument()
			doc.Type, doc.Purpose = estateType, purpose
			if err := ValidateMarketplaceListingDocument(doc); err != nil {
				t.Errorf("API enum values %q/%q rejected: %v", estateType, purpose, err)
			}
		}
	}
}

func TestValidateMarketplaceListingDocumentAcceptsImageLimit(t *testing.T) {
	doc := validDocument()
	doc.Media.Images = make([]string, MaxMarketplaceImages)
	if err := ValidateMarketplaceListingDocument(doc); err != nil {
		t.Fatalf("exactly %d images must validate: %v", MaxMarketplaceImages, err)
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

func TestDocumentJSONContractRoundTripPreservesLargePrice(t *testing.T) {
	const exactPrice int64 = 9_007_199_254_740_993
	doc := validDocument()
	doc.Price = exactPrice
	doc.Area = nil

	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	again, err := json.Marshal(validDocumentWithPrice(exactPrice))
	if err != nil {
		t.Fatalf("marshal same source snapshot: %v", err)
	}
	if !bytes.Equal(encoded, again) {
		t.Fatalf("same source snapshot produced different documents:\n%s\n%s", encoded, again)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal fields: %v", err)
	}
	for _, key := range []string{"listing_id", "property_id", "type", "purpose", "price", "province_id", "province_name", "ward_id", "ward_name", "address", "location", "published_at", "updated_at", "media"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("canonical field %q missing from %s", key, encoded)
		}
	}
	for _, forbidden := range []string{"city", "district", "ward", "slug", "cover_image", "aggregate_version"} {
		if _, ok := fields[forbidden]; ok {
			t.Errorf("noncanonical field %q leaked into %s", forbidden, encoded)
		}
	}
	if got := string(fields["price"]); got != "9007199254740993" {
		t.Fatalf("large price encoded as %s, want exact int64", got)
	}
	if _, ok := fields["area"]; ok {
		t.Fatalf("unknown area must be omitted, got %s", fields["area"])
	}

	var decoded MarketplaceListingDocument
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	if !reflect.DeepEqual(doc, decoded) || decoded.Price != exactPrice {
		t.Fatalf("round trip changed document or exact price:\ngot  %+v\nwant %+v", decoded, doc)
	}
	if err := ValidateMarketplaceListingDocument(decoded); err != nil {
		t.Fatalf("round-tripped document must validate: %v", err)
	}
}

func validDocumentWithPrice(price int64) MarketplaceListingDocument {
	doc := validDocument()
	doc.Price = price
	doc.Area = nil
	return doc
}

func TestDocumentJSONPreservesZeroPrice(t *testing.T) {
	doc := validDocument()
	doc.Price = 0
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
	if err := ValidateMarketplaceListingDocument(doc); err != nil {
		t.Fatalf("zero price is valid: %v", err)
	}
}
