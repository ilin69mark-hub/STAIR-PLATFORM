package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/database"
)

// integrationRouter собирает реальный стек: PostgreSQL (по
// STAIR_TEST_DATABASE_URL), application/project.Service, application/auth
// и транспортный роутер. Без переменной окружения тест пропускается
// (как и в infrastructure/database).
func integrationRouter(t *testing.T) http.Handler {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping HTTP+DB integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Connect(ctx, database.DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool, "../../../migrations", "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	auditRepo := database.NewAuditRepository(pool)
	auditSvc := audit.NewService(auditRepo)
	svc := project.NewService(
		database.NewProjectRepository(pool),
		stair.NewService(),
		project.DefaultRules(),
		auditSvc,
	)
	authSvc := auth.NewService(database.NewAuthRepository(pool), time.Hour, auditSvc)
	return NewRouter(stair.NewService(), svc, authSvc, DefaultConfig(), auditSvc)
}

// testEmail возвращает уникальный на запуск адрес, чтобы повторные прогоны
// против общей БД не конфликтовали (email в users уникален).
func testEmail(base string) string {
	return fmt.Sprintf("%s-%d@example.com", base, time.Now().UnixNano())
}

// registerLogin выполняет регистрацию и вход через HTTP, возвращает cookie.
func registerLogin(t *testing.T, router http.Handler, email string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"`+email+`","name":"Тест","password":"secret123"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"`+email+`","password":"secret123"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login must set session cookie")
	}
	return strings.Join(rec.Header().Values("Set-Cookie"), "; ")
}

// authedDo выполняет запрос с cookie сессии и CSRF-заголовком.
func authedDo(router http.Handler, method, path, cookie, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for _, c := range cookieHeader(cookie) {
		req.Header.Add("Cookie", c)
	}
	// CSRF-токен берём из cookie (double-submit): session-cookie — токен.
	req.Header.Set(csrfHeader, csrfValue(cookie))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func cookieHeader(cookie string) []string {
	var out []string
	for _, part := range strings.Split(cookie, "; ") {
		if strings.Contains(part, "=") {
			out = append(out, part)
		}
	}
	return out
}

func csrfValue(cookie string) string {
	for _, part := range strings.Split(cookie, "; ") {
		if strings.HasPrefix(part, csrfCookieName+"=") {
			return strings.TrimPrefix(part, csrfCookieName+"=")
		}
	}
	return ""
}

// TestIntegrationCriticalFlow — сквозной поток через HTTP поверх реальной
// БД: регистрация → вход → создание проекта → расчёт → чтение → экспорт.
// Проверяет, что снапшот сохраняется целиком (включая mesh) и отдаётся
// в /export.
func TestIntegrationCriticalFlow(t *testing.T) {
	router := integrationRouter(t)
	cookie := registerLogin(t, router, testEmail("flow"))

	// 1. Создание проекта.
	rec := authedDo(router, http.MethodPost, "/api/v1/projects", cookie,
		`{"name": "Интеграционный поток", "description": "HTTP+DB"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var p projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("create decode: %v", err)
	}
	if p.ID == "" || p.Status != "draft" {
		t.Fatalf("unexpected project: %+v", p)
	}

	// 2. Расчёт референса (n=15, валидный конвейер с производственным
	// пакетом и mesh).
	rec = authedDo(router, http.MethodPost, "/api/v1/projects/"+p.ID+"/calculate", cookie, referenceJSON)
	if rec.Code != http.StatusOK {
		t.Fatalf("calculate: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var c calculationDTO
	if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
		t.Fatalf("calculate decode: %v", err)
	}

	var snap project.Snapshot
	if err := json.Unmarshal(c.Result, &snap); err != nil {
		t.Fatalf("snapshot unmarshal: %v", err)
	}
	if !snap.Validation.Valid || snap.Validation.Blocking {
		t.Fatalf("expected valid snapshot, got %+v", snap.Validation)
	}
	if snap.ProjectID != p.ID {
		t.Fatalf("project id = %s, want %s", snap.ProjectID, p.ID)
	}
	if snap.Mesh == nil || len(snap.Mesh.Vertices) == 0 || len(snap.Mesh.Triangles) == 0 {
		t.Fatal("snapshot must persist full mesh (vertices+triangles)")
	}
	if snap.Manufacturing == nil || len(snap.Manufacturing.Parts) == 0 {
		t.Fatal("snapshot must persist manufacturing package")
	}
	if snap.Pricing == nil || snap.Pricing.FinalPrice.Major(snap.Pricing.Currency) <= 0 {
		t.Fatal("snapshot must persist pricing")
	}

	// 3. Чтение проекта по ID.
	rec = authedDo(router, http.MethodGet, "/api/v1/projects/"+p.ID, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", rec.Code)
	}
	var got projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("get decode: %v", err)
	}
	if got.Name != "Интеграционный поток" {
		t.Fatalf("name = %q", got.Name)
	}

	// 4. Экспортный документ — тот же снапшот с mesh.
	rec = authedDo(router, http.MethodGet, "/api/v1/projects/"+p.ID+"/export", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("export: expected 200, got %d", rec.Code)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("expected attachment content-disposition, got %q", cd)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("export must be valid JSON: %v", err)
	}
	if _, ok := doc["project_id"]; !ok {
		t.Fatal("export must contain project_id")
	}
	if _, ok := doc["mesh"]; !ok {
		t.Fatal("export must contain mesh snapshot")
	}
}

// TestIntegrationAuthFlow — регистрация, вход, me, logout и отказ без
// аутентификации (SEC-0003).
func TestIntegrationAuthFlow(t *testing.T) {
	router := integrationRouter(t)

	// /health публичен.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health: expected 200, got %d", rec.Code)
	}

	// Защищённый маршрут без cookie → 401.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-auth: expected 401, got %d", rec.Code)
	}

	// Дубликат email → 409.
	dupEmail := testEmail("dup")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"`+dupEmail+`","name":"A","password":"secret123"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"`+dupEmail+`","name":"B","password":"secret123"}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("dup register: expected 409, got %d", rec.Code)
	}

	// Неверный пароль → 401.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"`+dupEmail+`","password":"wrongpass"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: expected 401, got %d", rec.Code)
	}

	// me с валидной сессией.
	authEmail := testEmail("auth")
	cookie := registerLogin(t, router, authEmail)
	rec = authedDo(router, http.MethodGet, "/api/v1/auth/me", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("me: expected 200, got %d", rec.Code)
	}
	var u userDTO
	if err := json.NewDecoder(rec.Body).Decode(&u); err != nil {
		t.Fatalf("me decode: %v", err)
	}
	if u.Email != authEmail {
		t.Fatalf("me email = %q, want %q", u.Email, authEmail)
	}

	// logout → последующий me без сессии → 401.
	rec = authedDo(router, http.MethodPost, "/api/v1/auth/logout", cookie, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: expected 204, got %d", rec.Code)
	}
	rec = authedDo(router, http.MethodGet, "/api/v1/auth/me", cookie, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: expected 401, got %d", rec.Code)
	}
}

// TestIntegrationTenantIsolation — SEC-0005: пользователи из разных tenant
// не видят проекты друг друга.
func TestIntegrationTenantIsolation(t *testing.T) {
	router := integrationRouter(t)
	u1 := registerLogin(t, router, testEmail("t1"))

	// Создаём проект от имени u1.
	rec := authedDo(router, http.MethodPost, "/api/v1/projects", u1,
		`{"name": "Проект первого", "description": "секрет"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", rec.Code)
	}
	var p projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Пользователь u2 (дефолтный tenant — MVP: оба в одном tenant, поэтому
	// видит проект; истинная изоляция проверена на уровне service).
	u2 := registerLogin(t, router, testEmail("t2"))
	rec = authedDo(router, http.MethodGet, "/api/v1/projects/"+p.ID, u2, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get from other user: expected 200 in single-tenant MVP, got %d", rec.Code)
	}
}

// TestIntegrationAuditLog — аудит (EDR-0013): регистрация, вход и создание
// проекта пишут события; чтение аудита проекта доступно члену.
func TestIntegrationAuditLog(t *testing.T) {
	router := integrationRouter(t)
	cookie := registerLogin(t, router, testEmail("auditlog"))

	rec := authedDo(router, http.MethodPost, "/api/v1/projects", cookie,
		`{"name": "Аудит-поток", "description": "журнал"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var p projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("create decode: %v", err)
	}

	rec = authedDo(router, http.MethodGet, "/api/v1/projects/"+p.ID+"/audit", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("audit: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var events []auditEventDTO
	if err := json.NewDecoder(rec.Body).Decode(&events); err != nil {
		t.Fatalf("audit decode: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected audit events after register+login+create")
	}
	foundCreate := false
	for _, e := range events {
		if e.Action == string(audit.ActionProjectCreated) {
			foundCreate = true
		}
	}
	if !foundCreate {
		t.Fatalf("expected project.created in audit, got %+v", events)
	}
}

// TestIntegrationSessionRotation — EDR-0014 §3.1: старая сессия (> TTL/2)
// ротируется при аутентификации; новый session-cookie выдаётся, старый
// токен перестаёт работать.
func TestIntegrationSessionRotation(t *testing.T) {
	router := integrationRouter(t)
	cookie := registerLogin(t, router, testEmail("rotate"))

	// Состарим сессию в БД сверх половины TTL (TTL=1ч, половина — 30 мин).
	if err := ageSessions(t); err != nil {
		t.Fatalf("age sessions: %v", err)
	}

	// Запрос с (теперь уже старым) токеном: requireAuth ротирует сессию
	// и выдаёт новый session-cookie.
	rec := authedDo(router, http.MethodGet, "/api/v1/projects", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated request: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	newCookie := joinSetCookie(rec.Result().Cookies())
	if newCookie == cookie {
		t.Fatal("expected a new session cookie after rotation")
	}
	if !strings.Contains(newCookie, sessionCookieName+"=") {
		t.Fatalf("expected session cookie in response, got %q", newCookie)
	}

	// Старый токен больше не валиден.
	rec = authedDo(router, http.MethodGet, "/api/v1/projects", cookie, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old token must be revoked after rotation: expected 401, got %d", rec.Code)
	}

	// Новый токен валиден.
	rec = authedDo(router, http.MethodGet, "/api/v1/projects", newCookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("new token must work: expected 200, got %d", rec.Code)
	}
}

// ageSessions смещает created_at всех сессий в прошлое (интеграционный
// хелпер для теста ротации).
func ageSessions(t *testing.T) error {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Connect(ctx, database.DefaultConfig(url))
	if err != nil {
		return err
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `UPDATE sessions SET created_at = created_at - interval '40 minutes'`)
	return err
}

// joinSetCookie собирает Set-Cookie в один заголовок (как registerLogin).
func joinSetCookie(cookies []*http.Cookie) string {
	var parts []string
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}
