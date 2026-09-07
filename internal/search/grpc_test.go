package search

import (
	"context"
	"reflect"
	"testing"

	searchv1 "github.com/nexus-estate/nexus-estate-platform-engine/gen/search/v1"
)

func TestGRPCMappingPreservesOptionalZeroAndResponse(t *testing.T) {
	zero := 0.0
	repo := &fakeRepo{result: &PropertySearchResponse{Items: []PropertySearchItem{{ID: "p1", Title: "villa", Images: []string{"image"}, PublishedAt: "2026-01-01", Latitude: 10, Longitude: 20}}, Total: 1, Page: 1, Limit: 20, TotalPages: 1}}
	server := NewSearchServer(NewService(repo, nil, 60))
	got, err := server.SearchProperties(context.Background(), &searchv1.SearchPropertiesRequest{Keyword: "villa", MinPrice: &zero, Latitude: &zero})
	if err != nil {
		t.Fatal(err)
	}
	if repo.request.MinPrice == nil || *repo.request.MinPrice != 0 || repo.request.MaxPrice != nil || repo.request.Latitude == nil || repo.request.Longitude != nil {
		t.Fatalf("optional fields lost: %+v", repo.request)
	}
	if got.Total != 1 || got.Page != 1 || got.Limit != 20 || got.TotalPages != 1 || len(got.Items) != 1 {
		t.Fatalf("response %+v", got)
	}
	item := got.Items[0]
	if item.Id != "p1" || item.Title != "villa" || item.PublishedAt != "2026-01-01" || item.Latitude != 10 || item.Longitude != 20 || !reflect.DeepEqual(item.Images, []string{"image"}) {
		t.Fatalf("item %+v", item)
	}
}
