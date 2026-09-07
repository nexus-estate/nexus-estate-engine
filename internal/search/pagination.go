package search

const (
	defaultPage  = 1
	defaultLimit = 20
	maxLimit     = 100
)

func normalizePagination(page, limit int) (int, int) {
	if page <= 0 {
		page = defaultPage
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return page, limit
}
