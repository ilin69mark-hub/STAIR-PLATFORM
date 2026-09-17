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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/design", strings.NewReader(body))
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/engineering", strings.NewReader(body))
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/manufacturing", strings.NewReader(body))
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/pricing", strings.NewReader(body))
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/teleport",
		strings.NewReader(`{"width_mm":900,"height_mm":2700}`))
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown kind, got %d", rec.Code)
	}
}

func TestAssistantServiceError(t *testing.T) {
	ast := &fakeAssistant{err: fmt.Errorf("%w: no feasible config", appast.ErrNoFeasible)}
	router := assistantTestRouter(newFakeAuth(), ast)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/design",
		strings.NewReader(`{"width_mm":900,"height_mm":2700}`))
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on ErrNoFeasible, got %d", rec.Code)
	}
}

func TestAssistantInternalError(t *testing.T) {
	ast := &fakeAssistant{err: errors.New("model backend down")}
	router := assistantTestRouter(newFakeAuth(), ast)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/design",
		strings.NewReader(`{"width_mm":900,"height_mm":2700}`))
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on internal error, got %d", rec.Code)
	}
}

func TestAssistantInvalidJSON(t *testing.T) {
	router := assistantTestRouter(newFakeAuth(), &fakeAssistant{res: &appast.Result{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/design",
		strings.NewReader(`{broken`))
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad json, got %d", rec.Code)
	}
}
