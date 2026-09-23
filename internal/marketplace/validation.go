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
	ErrMissingPublishedAt = errors.New("published at is required")
	ErrInvalidVersion     = errors.New("aggregate version must be greater than zero")
	ErrInvalidPrice       = errors.New("price must be a finite, non-negative number")
	ErrInvalidArea        = errors.New("area must be a finite, non-negative number")
	ErrIncompleteGeo      = errors.New("location must set both lat and lon")
	ErrInvalidLatitude    = errors.New("latitude must be a finite number within [-90, 90]")
	ErrInvalidLongitude   = errors.New("longitude must be a finite number within [-180, 180]")
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
// The required fields are only the ones the search contract cannot function
// without: identity, version, a title and a publication timestamp. Optional text
// and geo fields may be empty because Elasticsearch tolerates absent fields.
func ValidateMarketplaceListingDocument(doc MarketplaceListingDocument) error {
	if doc.ListingID == "" {
		return invalid("listing_id", ErrMissingListingID)
	}
	if doc.PropertyID == "" {
		return invalid("property_id", ErrMissingPropertyID)
	}
	if doc.AggregateVersion <= 0 {
		return invalid("aggregate_version", ErrInvalidVersion)
	}
	if doc.Title == "" {
		return invalid("title", ErrMissingTitle)
	}
	if doc.PublishedAt == nil {
		return invalid("published_at", ErrMissingPublishedAt)
	}
	if !finiteNonNegative(doc.Price) {
		return invalid("price", ErrInvalidPrice)
	}
	if !finiteNonNegative(doc.Area) {
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
