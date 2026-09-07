package search

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

type fakeRepo struct {
	calls   int
	request PropertySearchRequest
	result  *PropertySearchResponse
	err     error
}

func (r *fakeRepo) SearchProperties(_ context.Context, req PropertySearchRequest) (*PropertySearchResponse, error) {
	r.calls++
	r.request = req
	return r.result, r.err
}

type fakeCache struct {
	value  string
	getErr error
	setErr error
	key    string
	data   []byte
	ttl    time.Duration
	writes int
}

func (c *fakeCache) Get(_ context.Context, key string) (string, error) {
	c.key = key
	return c.value, c.getErr
}
func (c *fakeCache) Set(_ context.Context, key string, data []byte, ttl time.Duration) error {
	c.key = key
	c.data = data
	c.ttl = ttl
	c.writes++
	return c.setErr
}

func TestServiceCacheBehavior(t *testing.T) {
	req := PropertySearchRequest{Keyword: "villa", Page: 2, Limit: 10}
	expected := &PropertySearchResponse{Items: []PropertySearchItem{{ID: "property-1"}}, Total: 21, Page: 2, Limit: 10, TotalPages: 3}
	cached, _ := json.Marshal(expected)
	for _, tc := range []struct {
		name      string
		cache     *fakeCache
		repoCalls int
	}{
		{"disabled", nil, 1},
		{"miss", &fakeCache{getErr: errors.New("cache miss")}, 1},
		{"hit", &fakeCache{value: string(cached)}, 0},
		{"invalid JSON", &fakeCache{value: "{"}, 1},
		{"empty", &fakeCache{}, 1},
		{"unavailable", &fakeCache{getErr: errors.New("connection refused"), setErr: errors.New("connection refused")}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{result: expected}
			var cache Cache
			if tc.cache != nil {
				cache = tc.cache
			}
			actual, err := NewService(repo, cache, 60).SearchProperties(context.Background(), req)
			if err != nil || !reflect.DeepEqual(actual, expected) {
				t.Fatalf("result=%+v err=%v", actual, err)
			}
			if repo.calls != tc.repoCalls {
				t.Fatalf("repository called %d times", repo.calls)
			}
			if repo.calls > 0 && !reflect.DeepEqual(repo.request, req) {
				t.Fatal("request changed")
			}
			if tc.cache != nil {
				if tc.cache.key != buildSearchCacheKey(req) {
					t.Fatal("cache key changed")
				}
				if tc.cache.writes != tc.repoCalls {
					t.Fatalf("cache writes=%d", tc.cache.writes)
				}
				if tc.cache.writes > 0 && (string(tc.cache.data) != string(cached) || tc.cache.ttl != time.Minute) {
					t.Fatal("cache payload or TTL changed")
				}
			}
		})
	}
}
func TestServiceRepositoryError(t *testing.T) {
	want := errors.New("repository down")
	repo := &fakeRepo{err: want}
	cache := &fakeCache{}
	_, err := NewService(repo, cache, 60).SearchProperties(context.Background(), PropertySearchRequest{})
	if !errors.Is(err, want) || cache.writes != 0 {
		t.Fatalf("err=%v writes=%d", err, cache.writes)
	}
}
func TestCacheKeyDeterminism(t *testing.T) {
	a, b := 12.5, 12.5
	req := PropertySearchRequest{Keyword: "villa", MinPrice: &a, Page: 1, Limit: 20}
	copy := req
	copy.MinPrice = &b
	if buildSearchCacheKey(req) != buildSearchCacheKey(copy) {
		t.Fatal("equal values with different pointers must share cache")
	}
	copy.Page = 2
	if buildSearchCacheKey(req) == buildSearchCacheKey(copy) {
		t.Fatal("pages must not share cache")
	}
	copy = req
	copy.MinPrice = nil
	if buildSearchCacheKey(req) == buildSearchCacheKey(copy) {
		t.Fatal("optional filters must affect cache")
	}
	// Golden value captures the original serialized request/key contract.
	if got := buildSearchCacheKey(PropertySearchRequest{}); got != "nexus-estate:search:properties:9f29b118d60e80f6b27e11214d60dde4f64d199ed3e55a067f7464016e41476b" {
		t.Fatalf("empty request cache key changed: %s", got)
	}
}
