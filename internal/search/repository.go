package search

import "context"

type Repository interface {
	SearchProperties(context.Context, PropertySearchRequest) (*PropertySearchResponse, error)
}
