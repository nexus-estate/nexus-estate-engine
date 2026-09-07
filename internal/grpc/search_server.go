package grpc

import (
	"context"

	searchv1 "github.com/nexus-estate/nexus-estate-platform-engine/gen/search/v1"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/search"
)

type SearchServer struct {
	searchv1.UnimplementedSearchServiceServer
	service search.Service
}

func NewSearchServer(service search.Service) *SearchServer {
	return &SearchServer{
		service: service,
	}
}

func (s *SearchServer) SearchProperties(
	ctx context.Context,
	req *searchv1.SearchPropertiesRequest,
) (*searchv1.SearchPropertiesResponse, error) {
	domainReq := search.PropertySearchRequest{
		Keyword:  req.GetKeyword(),
		City:     req.GetCity(),
		District: req.GetDistrict(),
		Page:     int(req.GetPage()),
		Limit:    int(req.GetLimit()),
	}

	if req.MinPrice != nil {
		value := req.GetMinPrice()
		domainReq.MinPrice = &value
	}

	if req.MaxPrice != nil {
		value := req.GetMaxPrice()
		domainReq.MaxPrice = &value
	}

	if req.MinArea != nil {
		value := req.GetMinArea()
		domainReq.MinArea = &value
	}

	if req.MaxArea != nil {
		value := req.GetMaxArea()
		domainReq.MaxArea = &value
	}

	if req.Latitude != nil {
		value := req.GetLatitude()
		domainReq.Latitude = &value
	}

	if req.Longitude != nil {
		value := req.GetLongitude()
		domainReq.Longitude = &value
	}

	if req.RadiusKm != nil {
		value := req.GetRadiusKm()
		domainReq.RadiusKm = &value
	}

	result, err := s.service.SearchProperties(ctx, domainReq)
	if err != nil {
		return nil, err
	}

	items := make([]*searchv1.PropertySearchItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, &searchv1.PropertySearchItem{
			Id:          item.ID,
			Title:       item.Title,
			Slug:        item.Slug,
			Description: item.Description,
			Type:        item.Type,
			Purpose:     item.Purpose,
			Price:       item.Price,
			Area:        item.Area,
			City:        item.City,
			District:    item.District,
			Ward:        item.Ward,
			Address:     item.Address,
			Latitude:    item.Latitude,
			Longitude:   item.Longitude,
			Images:      item.Images,
			PublishedAt: item.PublishedAt,
		})
	}

	return &searchv1.SearchPropertiesResponse{
		Items:      items,
		Total:      result.Total,
		Page:       int32(result.Page),
		Limit:      int32(result.Limit),
		TotalPages: result.TotalPages,
	}, nil
}
