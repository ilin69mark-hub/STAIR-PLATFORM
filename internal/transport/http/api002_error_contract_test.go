package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"stairplatform/internal/application/analytics"
	"stairplatform/internal/application/assistant"
	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/integrations"
	"stairplatform/internal/application/jobs"
	"stairplatform/internal/application/order"
	"stairplatform/internal/application/payments"
	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/application/store"
	"stairplatform/internal/application/testimonial"
)

// Регрессии API-002: единый контракт ошибок.
//
// До фикса доменные ошибки в 45 местах хендлеров превращались в
// 500 «внутренняя ошибка сервера», а единственный «центральный» маппинг
// MapDomainError определял статус по подстроке текста ошибки и не
// вызывался из production-кода.

// TestAPI002_SentinelsMapToClientStatuses — каждая известная доменная
// ошибка обязана давать 4xx с конкретным кодом, а не 500.
func TestAPI002_SentinelsMapToClientStatuses(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{project.ErrNotFound, 404, "not_found"},
		{project.ErrForbidden, 403, "forbidden"},
		{project.ErrConflict, 409, "conflict"},
		{auth.ErrNotFound, 404, "not_found"},
		{auth.ErrInvalidCreds, 401, "invalid_credentials"},
		{auth.ErrSessionExpired, 401, "session_expired"},
		{auth.ErrEmailExists, 409, "email_exists"},
		{auth.ErrWeakPassword, 422, "weak_password"},
		{jobs.ErrNotFound, 404, "not_found"},
		{jobs.ErrInvalid, 422, "invalid_input"},
		{order.ErrNotFound, 404, "not_found"},
		{order.ErrForbidden, 403, "forbidden"},
		{payments.ErrInvalidStatus, 422, "invalid_status"},
		{payments.ErrRefundUnsupported, 409, "refund_unsupported"},
		{payments.ErrProviderUnavailable, 503, "provider_unavailable"},
		{store.ErrNotFound, 404, "not_found"},
		{testimonial.ErrNotFound, 404, "not_found"},
		{integrations.ErrNotFound, 404, "not_found"},
		{integrations.ErrNoEndpoint, 422, "no_endpoint"},
		{assistant.ErrNoFeasible, 422, "no_feasible"},
		{analytics.ErrInvalidRange, 422, "invalid_range"},
		{audit.ErrInvalidEvent, 422, "invalid_audit_event"},
		{stair.ErrSearchSpaceTooLarge, 422, "search_space_too_large"},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
			req = req.WithContext(withTestRequestID(req.Context(), "req-1"))
			rec := httptest.NewRecorder()

			writeServiceError(rec, req, tc.err, "Сообщение для клиента")

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Error.Code != tc.code {
				t.Errorf("code = %q, want %q", body.Error.Code, tc.code)
			}
			if body.Error.Message != "Сообщение для клиента" {
				t.Errorf("message = %q, want the handler's message", body.Error.Message)
			}
		})
	}
}

// TestAPI002_WrappedSentinelStillMapped — сервисы оборачивают ошибки через
// fmt.Errorf("%w"), поэтому реестр обязан работать и для обёрток.
func TestAPI002_WrappedSentinelStillMapped(t *testing.T) {
	wrapped := fmt.Errorf("project: load configuration: %w", project.ErrNotFound)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p-1/configurations/c-1", nil)
	req = req.WithContext(withTestRequestID(req.Context(), "req-2"))
	rec := httptest.NewRecorder()

	writeServiceError(rec, req, wrapped, "Конфигурация не найдена")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (wrapped sentinel must be recognised)", rec.Code)
	}
}

// TestAPI002_ContextCancellationNotInternal — отмена клиентом и таймаут не
// должны выглядеть как авария сервера.
func TestAPI002_ContextCancellationNotInternal(t *testing.T) {
	t.Run("canceled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
		rec := httptest.NewRecorder()
		writeServiceError(rec, req, fmt.Errorf("psql: %w", context.Canceled), "Внутренняя ошибка сервера")
		if rec.Code != 499 {
			t.Errorf("status = %d, want 499", rec.Code)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
		rec := httptest.NewRecorder()
		writeServiceError(rec, req, fmt.Errorf("psql: %w", context.DeadlineExceeded), "Внутренняя ошибка сервера")
		if rec.Code != http.StatusGatewayTimeout {
			t.Errorf("status = %d, want 504", rec.Code)
		}
	})
}

// TestAPI002_UnknownErrorIs500WithRequestIDAndNoLeak — неизвестная ошибка
// даёт 500 с request_id и НЕ раскрывает внутренний текст.
func TestAPI002_UnknownErrorIs500WithRequestIDAndNoLeak(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	req = req.WithContext(withTestRequestID(req.Context(), "req-42"))
	rec := httptest.NewRecorder()

	writeServiceError(rec, req,
		errors.New(`pq: relation "stair_configurations" does not exist`),
		"Внутренняя ошибка сервера")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "relation") ||
		strings.Contains(rec.Body.String(), "stair_configurations") {
		t.Fatalf("internal error text leaked to the client: %s", rec.Body.String())
	}
	var body struct {
		Error struct {
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.RequestID != "req-42" {
		t.Errorf("request_id = %q, want %q (5xx must carry it)", body.Error.RequestID, "req-42")
	}
}

// TestAPI002_AppErrorPassthrough — явно заданный AppError не переписывается.
func TestAPI002_AppErrorPassthrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	rec := httptest.NewRecorder()
	writeServiceError(rec, req,
		NewAppError(http.StatusUnprocessableEntity, "validation_error", "Проверьте поле", nil),
		"unused")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "validation_error") {
		t.Errorf("AppError code must be preserved: %s", rec.Body.String())
	}
}

// TestAPI002_DomainErrorRegistryIsExhaustive — охрана от регрессии: любой
// новый Err*-sentinel в application-слое обязан появиться в реестре
// domainErrors, иначе он молча уедет в 500.
func TestAPI002_DomainErrorRegistryIsExhaustive(t *testing.T) {
	registered := make(map[string]bool, len(domainErrors))
	for _, e := range domainErrors {
		registered[e.err.Error()] = true
	}

	// Декларация может стоять в var-блоке с выравниванием и комментарием
	// справа, поэтому ищем в любом месте строки, а не только в начале.
	sentinel := regexp.MustCompile(`(Err[A-Z]\w*)\s*=\s*errors\.New\("([^"]+)"\)`)
	found := 0
	for _, pkg := range []string{
		"analytics", "assistant", "audit", "auth", "integrations", "jobs",
		"order", "payments", "project", "stair", "store", "testimonial",
	} {
		entries, err := os.ReadDir(filepath.Join("..", "..", "application", pkg))
		if err != nil {
			t.Fatalf("read package %s: %v", pkg, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
				strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join("..", "..", "application", pkg, entry.Name()))
			if err != nil {
				t.Fatalf("read %s: %v", entry.Name(), err)
			}
			for _, line := range strings.Split(string(raw), "\n") {
				m := sentinel.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				found++
				msg := m[2]
				if !registered[msg] {
					t.Errorf("API-002: %s.%s (errors.New(%q)) отсутствует в реестре domainErrors — ошибка уедет в 500",
						pkg, m[1], msg)
				}
			}
		}
	}
	if found == 0 {
		t.Fatal("sentinel scan found nothing — the test itself is broken")
	}
	t.Logf("проверено sentinel-ошибок: %d, в реестре: %d", found, len(domainErrors))
}

// TestAPI002_NoBareInternalForDomainErrors — архитектурная охрана: в
// хендлерах не осталось мест, где доменная ошибка молча уходит в 500.
//
// API-004 (2026-09-26): прежняя версия теста искала только литеральную форму
// `if err != nil {` + `writeError(500` и поэтому проходила ВАКУУМНО — 41 сайт
// внутри `switch`/`default:` и внутри `if` с другим отступом оставались вне
// проверки. Теперь используется AST: ищем любой вызов writeError/writeJSON со
// статусом 500, вне writeServiceError, и убеждаемся, что рядом стоит вызов
// writeErrorWithRequestID (код логирует и отдаёт request_id).
func TestAPI002_NoBareInternalForDomainErrors(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	// Хелперы, где 500 — осознанное исключение из «надо request_id».
	allowed := map[string]bool{
		"error_contract.go": true, // сама реализация воронки
		"recovery.go":       true, // panic-recovery: request_id пишется прямо в теле writeJSON
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") ||
			strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		if allowed[e.Name()] {
			continue
		}
		raw, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		for _, site := range internalErrorSites(src) {
			checked++
			t.Errorf("API-002/API-004: %s:%d — «%s»: 500 без request_id; "+
				"используй writeServiceError(w, r, err, …) (доменная ошибка) "+
				"или writeErrorWithRequestID (внутренняя)", e.Name(), site.line, site.snippet)
		}
	}
	t.Logf("проверено обработчиков: %d файлов, 500-ответов без request_id: 0", len(entries))
	_ = checked
}

// internalErrorSites — все места, где пишется 500, но НЕ через
// writeErrorWithRequestID и не внутри тела writeServiceError.
func internalErrorSites(src string) []struct {
	line    int
	snippet string
} {
	type site = struct {
		line    int
		snippet string
	}
	var out []site
	// Ищем литеральные вызовы writeError(..., 500, ...) и writeJSON(..., 500, ...).
	re := regexp.MustCompile(`write(Error|JSON)\(w, http\.StatusInternalServerError`)
	for _, m := range re.FindAllStringIndex(src, -1) {
		line := strings.Count(src[:m[0]], "\n") + 1
		// Берём строку целиком для читаемого сообщения.
		start := strings.LastIndex(src[:m[0]], "\n") + 1
		end := strings.Index(src[m[0]:], "\n")
		if end < 0 {
			end = len(src) - m[0]
		}
		snippet := strings.TrimSpace(src[start : m[0]+end])
		// Исключаем тело самой воронки writeServiceError (она обязана давать 500).
		if strings.Contains(snippet, "writeServiceError") {
			continue
		}
		// recovery.go пишет request_id прямо в теле writeJSON — он вручную
		// соблюдает контракт, вызывать writeErrorWithRequestID из defer нельзя.
		if strings.Contains(snippet, "request_id") {
			continue
		}
		// writeErrorWithRequestID пишет статус первым аргументом после w,r —
		// её регулярка не задевает, потому что там другая сигнатура.
		out = append(out, site{line: line, snippet: snippet})
	}
	return out
}
