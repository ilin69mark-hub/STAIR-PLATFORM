package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appast "stairplatform/internal/application/assistant"
	"stairplatform/internal/application/stair"
)

// fakeAssistant — управляемая реализация AssistantService для транспортных
// тестов.
type fakeAssistant struct {
	res  *appast.Result
	err  error
	kind appast.Kind
}

func (f *fakeAssistant) Ask(_ context.Context, tenantID, _ string, kind appast.Kind, _ appast.Request) (*appast.Result, error) {
	f.kind = kind
	if f.err != nil {
		return nil, f.err
	}
	return f.res, nil
}

func assistantTestRouter(a AuthService, ast AssistantService) http.Handler {
	cfg := DefaultConfig()
	cfg.Assistant = ast
	return NewRouter(stair.NewService(), nil, a, cfg)
}

// assistantAuthedRequest — POST на assistant-роут с session+csrf cookie и
// заголовком CSRF (S-144: маршрут переведён на authMutating); Content-Type
// application/json (S-144: decodeJSON требует его для тел).
func assistantAuthedRequest(path, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.AddCookie(testCookie(sessionCookieName, "token-1"))
	r.AddCookie(testCookie(csrfCookieName, "csrf-1"))
	r.Header.Set(csrfHeader, "csrf-1")
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestAssistantDesignHandler(t *testing.T) {
	ast := &fakeAssistant{res: &appast.Result{
		Kind: appast.KindDesign,
		Response: appast.Response{
			Recommendation: "Рекомендуется прямой марш",
			Rating:         0.9,
		},
		Commentary: "Прямой марш — самый дешёвый.",
	}}
	router := assistantTestRouter(newFakeAuth(), ast)

	body := `{"width_mm":900,"height_mm":2700,"flight":"straight","priority":"price"}`
	req := assistantAuthedRequest("/api/v1/assistant/design", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ast.kind != appast.KindDesign {
		t.Fatalf("kind = %q, want design", ast.kind)
	}
	want := `"recommendation":"Рекомендуется прямой марш"`
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body should contain %s, got %s", want, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"kind":"design"`) {
		t.Fatalf("body should contain kind=design, got %s", rec.Body.String())
	}
}

func TestAssistantEngineeringHandler(t *testing.T) {
	ast := &fakeAssistant{res: &appast.Result{
		Kind: appast.KindEngineering,
		Response: appast.Response{
			Recommendation: "Конфигурация удовлетворяет инженерным нормативам (STANDARD).",
			Rating:         1,
		},
		Commentary: "Параметры в норме.",
	}}
	router := assistantTestRouter(newFakeAuth(), ast)

	body := `{"width_mm":900,"height_mm":2700,"flight":"straight"}`
	req := assistantAuthedRequest("/api/v1/assistant/engineering", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ast.kind != appast.KindEngineering {
		t.Fatalf("kind = %q, want engineering", ast.kind)
	}
	if !strings.Contains(rec.Body.String(), `"kind":"engineering"`) {
		t.Fatalf("body should contain kind=engineering, got %s", rec.Body.String())
	}
}

func TestAssistantManufacturingHandler(t *testing.T) {
	ast := &fakeAssistant{res: &appast.Result{
		Kind: appast.KindManufacturing,
		Response: appast.Response{
			Recommendation: "Конфигурация готова к производству",
			Rating:         1,
		},
		Commentary: "Раскрой полноценный.",
	}}
	router := assistantTestRouter(newFakeAuth(), ast)

	body := `{"width_mm":900,"height_mm":2700,"flight":"straight"}`
	req := assistantAuthedRequest("/api/v1/assistant/manufacturing", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ast.kind != appast.KindManufacturing {
		t.Fatalf("kind = %q, want manufacturing", ast.kind)
	}
}

func TestAssistantPricingHandler(t *testing.T) {
	ast := &fakeAssistant{res: &appast.Result{
		Kind: appast.KindPricing,
		Response: appast.Response{
			Recommendation: "Цена корректна и сбалансирована",
			Rating:         1,
		},
		Commentary: "Структура цены в норме.",
	}}
	router := assistantTestRouter(newFakeAuth(), ast)

	body := `{"width_mm":900,"height_mm":2700,"flight":"straight"}`
	req := assistantAuthedRequest("/api/v1/assistant/pricing", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ast.kind != appast.KindPricing {
		t.Fatalf("kind = %q, want pricing", ast.kind)
	}
}

func TestAssistantUnauthorized(t *testing.T) {
	router := assistantTestRouter(newFakeAuth(), &fakeAssistant{res: &appast.Result{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/design",
		strings.NewReader(`{"width_mm":900,"height_mm":2700}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestAssistantUnknownKind(t *testing.T) {
	router := assistantTestRouter(newFakeAuth(), &fakeAssistant{res: &appast.Result{}})
	req := assistantAuthedRequest("/api/v1/assistant/teleport", `{"width_mm":900,"height_mm":2700}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown kind, got %d", rec.Code)
	}
}

func TestAssistantServiceError(t *testing.T) {
	ast := &fakeAssistant{err: fmt.Errorf("%w: no feasible config", appast.ErrNoFeasible)}
	router := assistantTestRouter(newFakeAuth(), ast)
	req := assistantAuthedRequest("/api/v1/assistant/design", `{"width_mm":900,"height_mm":2700}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on ErrNoFeasible, got %d", rec.Code)
	}
}

func TestAssistantForbiddenProject(t *testing.T) {
	// S-142 (IDOR-фикс S-141 №1): project_id из тела, но вызывающий не
	// член проекта → 403, а не данные чужой conversation-memory.
	ast := &fakeAssistant{err: fmt.Errorf("%w: not a member of project", appast.ErrForbidden)}
	router := assistantTestRouter(newFakeAuth(), ast)
	req := assistantAuthedRequest("/api/v1/assistant/design",
		`{"width_mm":900,"height_mm":2700,"project_id":"p-other"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on non-member project, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"forbidden"`) {
		t.Fatalf("body should carry forbidden code, got %s", rec.Body.String())
	}
}

func TestAssistantInternalError(t *testing.T) {
	ast := &fakeAssistant{err: errors.New("model backend down")}
	router := assistantTestRouter(newFakeAuth(), ast)
	req := assistantAuthedRequest("/api/v1/assistant/design", `{"width_mm":900,"height_mm":2700}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on internal error, got %d", rec.Code)
	}
}

// TestAssistantInternalErrorGenericBody — S-144 (S-141 №8): сбой модели не
// должен отдавать клиенту внутреннюю цепочку (CWE-209): ни "assistant:",
// ни URL провайдера в теле; 500 с фиксированным текстом.
func TestAssistantInternalErrorGenericBody(t *testing.T) {
	ast := &fakeAssistant{err: errors.New("model backend down: dial openrouter.ai:443 timeouts")}
	router := assistantTestRouter(newFakeAuth(), ast)
	req := assistantAuthedRequest("/api/v1/assistant/design", `{"width_mm":900,"height_mm":2700}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on model failure, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, leak := range []string{"assistant:", "openrouter", "dial"} {
		if strings.Contains(body, leak) {
			t.Errorf("internal error details leaked to client (%q): %s", leak, body)
		}
	}
	if !strings.Contains(body, "Внутренняя ошибка. Попробуйте позже.") {
		t.Errorf("body should carry generic message, got %s", body)
	}
}

// TestAssistantTextPlainNoCSRF — вектор атаки S-141 №7: браузерная форма
// text/plain без preflight не несёт X-CSRF-Token (атакующая страница не
// может прочитать csrf-cookie жертвы) → requireCSRF отклоняет 403 до
// decodeJSON. Атака закрыта на уровне CSRF.
func TestAssistantTextPlainNoCSRF(t *testing.T) {
	ast := &fakeAssistant{res: &appast.Result{}}
	router := assistantTestRouter(newFakeAuth(), ast)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/design",
		strings.NewReader(`{"width_mm":900,"height_mm":2700}`))
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
	req.Header.Set("Content-Type", "text/plain") // форма без preflight
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Фактический код: 403 (требуется CSRF-токен) — CSRF-мидлвара срабатывает
	// раньше decodeJSON.
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 (CSRF) for text/plain without CSRF token, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"csrf"`) {
		t.Fatalf("body should carry csrf code, got %s", rec.Body.String())
	}
}

// TestAssistantTextPlainValidCSRF — сценарий с валидными session+csrf
// (double-submit пройден) и Content-Type text/plain: фактический код — 400
// (invalid_json от decodeJSON) — задокументировано (S-144); сам запрос
// выполнен не будет.
func TestAssistantTextPlainValidCSRF(t *testing.T) {
	ast := &fakeAssistant{res: &appast.Result{}}
	router := assistantTestRouter(newFakeAuth(), ast)

	req := assistantAuthedRequest("/api/v1/assistant/design", `{"width_mm":900,"height_mm":2700}`)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Фактический код: 400 invalid_json (Content-Type text/plain отклонён
	// decodeJSON). Зафиксировано тестом, чтобы поведение не изменилось
	// незаметно.
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (invalid_json) for text/plain with valid CSRF, got %d: %s", rec.Code, rec.Body.String())
	}
	if ast.kind != "" {
		t.Fatalf("assistant must not be called for text/plain body, got kind %q", ast.kind)
	}
}

func TestAssistantInvalidJSON(t *testing.T) {
	router := assistantTestRouter(newFakeAuth(), &fakeAssistant{res: &appast.Result{}})
	req := assistantAuthedRequest("/api/v1/assistant/design", `{broken`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad json, got %d", rec.Code)
	}
}
