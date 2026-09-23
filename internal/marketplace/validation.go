package marketplace

import (
	"errors"
	"fmt"
	"math"
)

// Sentinel errors identify the validation failure class. An invalid document is a
// permanent input error: the projection write layer must reject it before any
// Elasticsearch write and must not retry it indefinitely.
//
// Use errors.Is to test the class and errors.As with *ValidationError to inspect
// the offending field.
var (
	ErrMissingListingID   = errors.New("listing id is required")
	ErrMissingPropertyID  = errors.New("property id is required")
	ErrMissingTitle       = errors.New("title is required")
	ErrMissingType        = errors.New("type is required")
	ErrMissingPurpose     = errors.New("purpose is required")
	ErrInvalidType        = errors.New("type must be an API EstateType value")
	ErrInvalidPurpose     = errors.New("purpose must be an API EstatePurpose value")
	ErrMissingProvinceID  = errors.New("province id is required")
	ErrMissingProvince    = errors.New("province name is required")
	ErrMissingWardID      = errors.New("ward id is required")
	ErrMissingWardName    = errors.New("ward name is required")
	ErrMissingAddress     = errors.New("address is required")
	ErrMissingPublishedAt = errors.New("published at is required")
	ErrInvalidPrice       = errors.New("price must be a non-negative integer")
	ErrInvalidArea        = errors.New("area must be a finite, positive number when present")
	ErrIncompleteGeo      = errors.New("location must set both lat and lon")
	ErrInvalidLatitude    = errors.New("latitude must be a finite number within [-90, 90]")
	ErrInvalidLongitude   = errors.New("longitude must be a finite number within [-180, 180]")
	ErrMissingUpdatedAt   = errors.New("updated at is required")
	ErrTooManyImages      = errors.New("media image count exceeds the marketplace projection limit")
)

// ValidationError is the typed, inspectable error returned by
// ValidateMarketplaceListingDocument. It unwraps to one of the sentinel errors
// above.
type ValidationError struct {
	// Field is the canonical document field path that failed validation.
	Field string
	err   error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid marketplace listing document field %q: %v", e.Field, e.err)
}

func (e *ValidationError) Unwrap() error { return e.err }

// ValidateMarketplaceListingDocument reports whether a projection is safe to
// index. It returns nil for a valid, public/searchable listing document, or the
// first violation in a fixed field order so the same invalid input always reports
// the same error class.
//
// Required fields follow the API source contract: identity, required Estate
// content/location and price, plus Listing publication/update timestamps. Area,
// description, media and coordinates may be absent. A zero price is valid; an
// absent area is represented by nil. Enum values must preserve the API casing.
func ValidateMarketplaceListingDocument(doc MarketplaceListingDocument) error {
	if doc.ListingID == "" {
		return invalid("listing_id", ErrMissingListingID)
	}
	if doc.PropertyID == "" {
		return invalid("property_id", ErrMissingPropertyID)
	}
	if doc.Title == "" {
		return invalid("title", ErrMissingTitle)
	}
	if doc.Type == "" {
		return invalid("type", ErrMissingType)
	}
	if !validEstateType(doc.Type) {
		return invalid("type", ErrInvalidType)
	}
	if doc.Purpose == "" {
		return invalid("purpose", ErrMissingPurpose)
	}
	if !validEstatePurpose(doc.Purpose) {
		return invalid("purpose", ErrInvalidPurpose)
	}
	if doc.ProvinceID == "" {
		return invalid("province_id", ErrMissingProvinceID)
	}
	if doc.ProvinceName == "" {
		return invalid("province_name", ErrMissingProvince)
	}
	if doc.WardID == "" {
		return invalid("ward_id", ErrMissingWardID)
	}
	if doc.WardName == "" {
		return invalid("ward_name", ErrMissingWardName)
	}
	if doc.Address == "" {
		return invalid("address", ErrMissingAddress)
	}
	if doc.PublishedAt == nil || doc.PublishedAt.IsZero() {
		return invalid("published_at", ErrMissingPublishedAt)
	}
	if doc.UpdatedAt.IsZero() {
		return invalid("updated_at", ErrMissingUpdatedAt)
	}
	if doc.Price < 0 {
		return invalid("price", ErrInvalidPrice)
	}
	if doc.Area != nil && (!finite(*doc.Area) || *doc.Area <= 0) {
		return invalid("area", ErrInvalidArea)
	}
	if doc.Media != nil && len(doc.Media.Images) > MaxMarketplaceImages {
		return invalid("media.images", ErrTooManyImages)
	}

	if doc.Location != nil {
		if doc.Location.Lat == nil || doc.Location.Lon == nil {
			return invalid("location", ErrIncompleteGeo)
		}
		if !finite(*doc.Location.Lat) || *doc.Location.Lat < -90 || *doc.Location.Lat > 90 {
			return invalid("location.lat", ErrInvalidLatitude)
		}
		if !finite(*doc.Location.Lon) || *doc.Location.Lon < -180 || *doc.Location.Lon > 180 {
			return invalid("location.lon", ErrInvalidLongitude)
		}
	}

	return nil
}

func invalid(field string, err error) error {
	return &ValidationError{Field: field, err: err}
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func validEstateType(value EstateType) bool {
	switch value {
	case EstateTypeApartment, EstateTypeHouse, EstateTypeVilla, EstateTypeTownhouse,
		EstateTypeLand, EstateTypeOffice, EstateTypeShophouse, EstateTypeWarehouse,
		EstateTypeCommercial, EstateTypeHotel, EstateTypeResort, EstateTypeFarm,
		EstateTypeOther:
		return true
	default:
		return false
	}
}

func validEstatePurpose(value EstatePurpose) bool {
	switch value {
	case EstatePurposeSale, EstatePurposeRent, EstatePurposeSaleOrRent:
		return true
	default:
		return false
	}
}
