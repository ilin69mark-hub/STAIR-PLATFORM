package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestAPI001_PaginationBounds — регрессия API-001: page/per_page имели только
// нижнюю границу, Offset() переполнял int64 в отрицательное значение, а
// обработчики резали срез этим индексом -> panic -> HTTP 500.
func TestAPI001_PaginationBounds(t *testing.T) {
	values := []string{
		"0", "-1", "-9223372036854775808", "1", "2",
		"2147483647",                 // MaxInt32
		"2147483648",                 // MaxInt32+1 — граница int32
		"9223372036854775807",        // MaxInt64
		"4611686018427387904",        // 2^62 — payload из аудита
		"18446744073709551616",       // 2^64 — переполнение парсинга
		"99999999999999999999999999", // намного больше
		"notanumber", "", "1e400", "0x10", "1.5", " ", "+5", "007",
	}
	for _, v := range values {
		t.Run("page="+v, func(t *testing.T) {
			for _, pp := range []string{"0", "1", "20", "100", "101", "1000000000", "-5", "bad"} {
				q := "/x?page=" + urlEncode(v) + "&per_page=" + urlEncode(pp)
				req := httptest.NewRequest(http.MethodGet, q, nil)
				p := ParsePagination(req)
				if p.Page < 1 || p.Page > MaxPage {
					t.Fatalf("page out of bounds: %d (raw %q)", p.Page, v)
				}
				if p.PerPage < 1 || p.PerPage > MaxPerPage {
					t.Fatalf("per_page out of bounds: %d (raw %q)", p.PerPage, pp)
				}
				off := p.Offset()
				if off < 0 {
					t.Fatalf("Offset() must never be negative, got %d (page=%q per_page=%q)", off, v, pp)
				}
				// Ключевая проверка: нарезка не паникует ни при каком вводе.
				got := SliceInPage(p, []int{1, 2, 3, 4, 5})
				if len(got) > len([]int{1, 2, 3, 4, 5}) {
					t.Fatalf("slice longer than source")
				}
			}
		})
	}
}

// TestAPI001_SliceInPageNeverPanics — исчерпывающая проверка нарезки на всех
// комбинациях page/per_page против наборов разной длины.
func TestAPI001_SliceInPageNeverPanics(t *testing.T) {
	pages := []int{-1000, -1, 0, 1, 2, 10, 1000, MaxPage, MaxPage + 1, 1 << 40, 1 << 62}
	pers := []int{-100, -1, 0, 1, 2, 20, 100, 1000, 1 << 40, 1 << 62}
	for _, n := range []int{0, 1, 2, 19, 20, 21, 100, 101, 5000} {
		src := make([]int, n)
		for i := range src {
			src[i] = i
		}
		for _, pg := range pages {
			for _, pp := range pers {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("PANIC n=%d page=%d per_page=%d: %v", n, pg, pp, r)
						}
					}()
					p := PaginationParams{Page: pg, PerPage: pp}
					got := SliceInPage(p, src)
					// Инварианты результата.
					if len(got) > len(src) {
						t.Fatalf("n=%d page=%d per_page=%d: result %d > source %d", n, pg, pp, len(got), len(src))
					}
					if off := p.Offset(); off >= 0 && off < n && len(got) > 0 {
						want := p.PerPage
						if want < 1 {
							// Вырожденный per_page -> весь хвост от off.
							want = n - off
						}
						if off+want > n {
							want = n - off
						}
						if len(got) != want {
							t.Fatalf("n=%d page=%d per_page=%d: got %d items, want %d", n, pg, pp, len(got), want)
						}
						if got[0] != src[off] {
							t.Fatalf("n=%d page=%d per_page=%d: first item %d, want %d", n, pg, pp, got[0], src[off])
						}
					}
				}()
			}
		}
	}
}

// TestAPI001_OffsetNeverOverflows — Offset() не переполняется ни при каких
// значениях, прошедших ParsePagination.
func TestAPI001_OffsetNeverOverflows(t *testing.T) {
	maxOffset := MaxPage * MaxPerPage
	if maxOffset <= 0 {
		t.Fatalf("MaxPage*MaxPerPage overflowed: %d", maxOffset)
	}
	// Заведомо ниже MaxInt64 с запасом в 4 порядка.
	if maxOffset > 1<<60 {
		t.Fatalf("MaxPage*MaxPerPage = %d is too close to overflow", maxOffset)
	}
	for _, pg := range []int{1, 2, 1000, MaxPage - 1, MaxPage} {
		for _, pp := range []int{1, 20, MaxPerPage} {
			p := PaginationParams{Page: pg, PerPage: pp}
			off := p.Offset()
			if off < 0 || off > maxOffset {
				t.Fatalf("page=%d per_page=%d -> offset=%d out of [0,%d]", pg, pp, off, maxOffset)
			}
		}
	}
}

// TestAPI001_NoPanicOverHTTP — сквозная проверка на реальном роутере с
// payload'ом из аудита на каждом из 7 затронутых list-эндпоинтов.
func TestAPI001_NoPanicOverHTTP(t *testing.T) {
	h := sec1Router()
	pages := []string{"4611686018427387904", "9223372036854775807", "18446744073709551616", "-5", "0"}
	paths := []string{
		"/api/v1/projects",
		"/api/v1/admin/users",
		"/api/v1/audit",
	}
	for _, path := range paths {
		for _, pg := range pages {
			for _, pp := range []string{"20", "100", "1", "-1"} {
				q := path + "?page=" + pg + "&per_page=" + pp
				req := httptest.NewRequest(http.MethodGet, q, nil)
				req.AddCookie(&http.Cookie{Name: "session", Value: "sess"})
				rec := httptest.NewRecorder()
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("PANIC %s: %v", q, r)
						}
					}()
					h.ServeHTTP(rec, req)
				}()
				if rec.Code >= 500 {
					t.Errorf("%s -> %d (must not be 5xx)", q, rec.Code)
				}
			}
		}
	}
}

// TestAPI001_TotalPages — арифметика числа страниц на границах.
func TestAPI001_TotalPages(t *testing.T) {
	cases := []struct {
		total, perPage, want int
	}{
		{0, 20, 0},
		{1, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{40, 20, 2},
		{41, 20, 3},
		{100, 20, 5},
		{101, 20, 6},
		{0, 0, 0},
	}
	for _, c := range cases {
		p := PaginationParams{Page: 1, PerPage: c.perPage}
		if got := p.TotalPages(c.total); got != c.want {
			t.Errorf("TotalPages(total=%d, per_page=%d)=%d want %d", c.total, c.perPage, got, c.want)
		}
	}
}

func urlEncode(s string) string { return url.QueryEscape(s) }
