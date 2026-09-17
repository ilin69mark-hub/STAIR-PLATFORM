package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePagination(t *testing.T) {
	r := httptest.NewRequest("GET", "/api?page=2&per_page=10", nil)
	p := ParsePagination(r)
	if p.Page != 2 || p.PerPage != 10 {
		t.Fatalf("got %+v", p)
	}
	r2 := httptest.NewRequest("GET", "/api?page=-1&per_page=200", nil)
	p2 := ParsePagination(r2)
	if p2.Page != 1 || p2.PerPage != 100 {
		t.Fatalf("clamped %+v", p2)
	}
	r3 := httptest.NewRequest("GET", "/api", nil)
	p3 := ParsePagination(r3)
	if p3.Page != 1 || p3.PerPage != 20 {
		t.Fatalf("default %+v", p3)
	}
}

func TestPaginationOffset(t *testing.T) {
	p := PaginationParams{Page: 3, PerPage: 20}
	if p.Offset() != 40 {
		t.Fatalf("want 40 got %d", p.Offset())
	}
}

func TestNewPaginatedResponse(t *testing.T) {
	p := PaginationParams{Page: 1, PerPage: 20}
	resp := NewPaginatedResponse([]string{"a"}, p, 25)
	if resp.TotalPages != 2 {
		t.Fatalf("want 2 got %d", resp.TotalPages)
	}
	resp2 := NewPaginatedResponse(nil, p, 40)
	if resp2.TotalPages != 2 {
		t.Fatalf("want 2 got %d", resp2.TotalPages)
	}
}

func TestWritePaginatedJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WritePaginatedJSON(w, []string{"a"}, PaginationParams{Page: 1, PerPage: 20}, 1)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}
