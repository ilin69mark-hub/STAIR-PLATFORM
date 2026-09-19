package http

import (
	"net/http"
	"strconv"
)

// PaginationParams параметры пагинации из query string.
type PaginationParams struct {
	Page    int
	PerPage int
}

// DefaultPagination дефолтные параметры пагинации.
var DefaultPagination = PaginationParams{
	Page:    1,
	PerPage: 20,
}

// MaxPerPage максимальный размер страницы.
const MaxPerPage = 100

// ParsePagination парсит параметры пагинации из query string.
func ParsePagination(r *http.Request) PaginationParams {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = DefaultPagination.PerPage
	}
	if perPage > MaxPerPage {
		perPage = MaxPerPage
	}

	return PaginationParams{
		Page:    page,
		PerPage: perPage,
	}
}

// Offset вычисляет offset для SQL запроса.
func (p PaginationParams) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// PaginatedResponse ответ с пагинацией.
type PaginatedResponse struct {
	Data       any `json:"data"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// NewPaginatedResponse создаёт ответ с пагинацией.
func NewPaginatedResponse(data any, params PaginationParams, total int) PaginatedResponse {
	totalPages := total / params.PerPage
	if total%params.PerPage > 0 {
		totalPages++
	}

	return PaginatedResponse{
		Data:       data,
		Page:       params.Page,
		PerPage:    params.PerPage,
		Total:      total,
		TotalPages: totalPages,
	}
}

// WritePaginatedJSON записывает paginated JSON ответ.
func WritePaginatedJSON(w http.ResponseWriter, data any, params PaginationParams, total int) {
	resp := NewPaginatedResponse(data, params, total)
	writeJSON(w, http.StatusOK, resp)
}
