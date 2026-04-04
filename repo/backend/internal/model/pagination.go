package model

// Pagination carries the offset/limit parameters for list queries.
type Pagination struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

// Offset returns the SQL OFFSET for this page.
func (p Pagination) Offset() int {
	if p.Page < 1 {
		p.Page = 1
	}
	return (p.Page - 1) * p.PerPage
}

// Limit returns the SQL LIMIT, clamped to a maximum of 200.
func (p Pagination) Limit() int {
	if p.PerPage < 1 {
		return 20
	}
	if p.PerPage > 200 {
		return 200
	}
	return p.PerPage
}

// DefaultPagination returns a Pagination with sensible defaults.
func DefaultPagination() Pagination {
	return Pagination{Page: 1, PerPage: 20}
}

// PageResult wraps a list result with total-count metadata.
type PageResult[T any] struct {
	Items      []T `json:"items"`
	Total      int `json:"total"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages"`
}

// NewPageResult constructs a PageResult from items, total count, and pagination params.
func NewPageResult[T any](items []T, total int, p Pagination) PageResult[T] {
	totalPages := 1
	if p.Limit() > 0 && total > 0 {
		totalPages = (total + p.Limit() - 1) / p.Limit()
	}
	return PageResult[T]{
		Items:      items,
		Total:      total,
		Page:       p.Page,
		PerPage:    p.Limit(),
		TotalPages: totalPages,
	}
}
