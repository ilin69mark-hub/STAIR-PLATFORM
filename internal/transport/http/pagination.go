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

// MaxPage — верхняя граница номера страницы.
//
// API-001 (аудит 2026-09-26): раньше граница была только снизу
// (`if page < 1`), поэтому `GET /api/v1/projects?page=2^62&per_page=20`
// давал Offset() = (2^62-1)*20 = переполнение int64 в ОТРИЦАТЕЛЬНОЕ
// значение (-20), и обработчик резал срез этим индексом:
// `list[-20:20]` -> panic "slice bounds out of range [-20:]" -> HTTP 500.
// Воспроизведено на 7 эндпоинтах (projects, comments, reviews, approvals,
// users, tenant audit, admin orders).
//
// MaxPage выбран так, чтобы Offset() не мог переполниться при любом
// PerPage ≤ MaxPerPage: MaxPage*MaxPerPage = 1_000_000*100 = 1e8, что на
// четыре порядка меньше math.MaxInt64. Это заведомо больше любого реального
// набора данных (миллиард строк пользователю не показывают), поэтому
// ограничение не влияет на поведение API.
const MaxPage = 1_000_000

// ParsePagination парсит параметры пагинации из query string.
//
// API-001: обе границы проверяются явно. Отсутствие верхней границы было
// причиной переполнения int64 и паники в 7 обработчиках.
func ParsePagination(r *http.Request) PaginationParams {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	if page < 1 {
		page = 1
	}
	// API-001: верхняя граница. Переполнение при (page-1)*perPage
	// начинается выше MaxInt64/MaxPerPage; MaxPage заведомо ниже.
	if page > MaxPage {
		page = MaxPage
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
//
// API-001: Offset() больше не может быть отрицательным. Раньше арифметика
// `int((Page-1) * PerPage)` переполнялась и возвращала отрицательное
// значение, которое использовалось как индекс среза. Теперь страница уже
// ограничена MaxPage в ParsePagination, а здесь стоит защита «снизу вверх»
// на случай, если PaginationParams собрана вручную (минуя ParsePagination) —
// например, в тесте или будущим вызывающим.
func (p PaginationParams) Offset() int {
	if p.Page < 1 {
		return 0
	}
	if p.PerPage < 1 {
		return 0
	}
	// Страховка от переполнения даже при ручной сборке params.
	if p.Page > MaxPage {
		p.Page = MaxPage
	}
	if p.PerPage > MaxPerPage {
		p.PerPage = MaxPerPage
	}
	off := (p.Page - 1) * p.PerPage
	if off < 0 {
		return 0
	}
	return off
}

// SliceInPage безопасно вырезает окно [offset, offset+perPage) из среза
// произвольной длины.
//
// API-001: единственное безопасное место для нарезки в памяти. Раньше каждый
// обработчик писал нарезку сам (`list[start:end]`), и отрицательный offset
// приводил к panic. Все 7 обработчиков с пагинацией в памяти переведены на
// эту функцию, поэтому отрицательный индекс больше невозможен даже если
// появится новый вызывающий.
//
// Объявлена пакетной функцией, а не методом: Go не допускает генерик-методов
// у негенерикного типа.
//
// Контракт:
//   - start всегда в [0, len(all)] (Offset() клампит);
//   - PerPage < 1 (вырожденные параметры, собранные вручную мимо
//     ParsePagination) -> возвращается хвост от start, то есть все
//     оставшиеся элементы; это же поведение давал прежний код на первом
//     непустом Offset();
//   - иначе — ровно min(PerPage, len(all)-start) элементов.
func SliceInPage[T any](p PaginationParams, all []T) []T {
	start := p.Offset()
	if start >= len(all) {
		return []T{}
	}
	if p.PerPage < 1 {
		return all[start:]
	}
	end := start + p.PerPage
	if end > len(all) {
		end = len(all)
	}
	return all[start:end]
}

// TotalPages — число страниц для постраничного ответа.
func (p PaginationParams) TotalPages(total int) int {
	if p.PerPage < 1 {
		return 0
	}
	if total <= 0 {
		return 0
	}
	tp := total / p.PerPage
	if total%p.PerPage > 0 {
		tp++
	}
	return tp
}

// PaginatedResponse — ответ с пагинацией.
type PaginatedResponse struct {
	Data       any `json:"data"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// NewPaginatedResponse — конструктор ответа с пагинацией.
func NewPaginatedResponse(data any, params PaginationParams, total int) PaginatedResponse {
	return PaginatedResponse{
		Data:       data,
		Page:       params.Page,
		PerPage:    params.PerPage,
		Total:      total,
		TotalPages: params.TotalPages(total),
	}
}

// WritePaginatedJSON — сериализует ответ с пагинацией.
func WritePaginatedJSON(w http.ResponseWriter, data any, params PaginationParams, total int) {
	writeJSON(w, http.StatusOK, NewPaginatedResponse(data, params, total))
}
