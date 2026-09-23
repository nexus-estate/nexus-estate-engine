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
	ErrMissingCity        = errors.New("city is required")
	ErrMissingWard        = errors.New("ward is required")
	ErrMissingAddress     = errors.New("address is required")
	ErrMissingPublishedAt = errors.New("published at is required")
	ErrMissingPrice       = errors.New("price is required")
	ErrInvalidPrice       = errors.New("price must be a finite, non-negative number")
	ErrInvalidArea        = errors.New("area must be a finite, positive number when present")
	ErrIncompleteGeo      = errors.New("location must set both lat and lon")
	ErrInvalidLatitude    = errors.New("latitude must be a finite number within [-90, 90]")
	ErrInvalidLongitude   = errors.New("longitude must be a finite number within [-180, 180]")
	ErrMissingUpdatedAt   = errors.New("updated at is required")
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
// description, slug, district, media and coordinates may be absent. A zero price
// is valid; an absent area is represented by nil.
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
	if doc.Purpose == "" {
		return invalid("purpose", ErrMissingPurpose)
	}
	if doc.City == "" {
		return invalid("city", ErrMissingCity)
	}
	if doc.Ward == "" {
		return invalid("ward", ErrMissingWard)
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
	if doc.Price == nil {
		return invalid("price", ErrMissingPrice)
	}
	if !finiteNonNegative(*doc.Price) {
		return invalid("price", ErrInvalidPrice)
	}
	if doc.Area != nil && (!finite(*doc.Area) || *doc.Area <= 0) {
		return invalid("area", ErrInvalidArea)
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

func finiteNonNegative(v float64) bool {
	return finite(v) && v >= 0
}
