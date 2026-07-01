package search

type PropertySearchRequest struct {
	Keyword  string
	City     string
	District string

	MinPrice *float64
	MaxPrice *float64

	MinArea *float64
	MaxArea *float64

	Latitude  *float64
	Longitude *float64
	RadiusKm  *float64

	Page  int
	Limit int
}

type PropertySearchResponse struct {
	Items      []PropertySearchItem
	Total      int64
	Page       int
	Limit      int
	TotalPages int64
}

type PropertySearchItem struct {
	ID          string
	Title       string
	Slug        string
	Description string
	Type        string
	Purpose     string
	Price       float64
	Area        float64
	City        string
	District    string
	Ward        string
	Address     string
	Latitude    float64
	Longitude   float64
	Images      []string
	PublishedAt string
}
