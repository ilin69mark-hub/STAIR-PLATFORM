package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/stair"
	storeapp "stairplatform/internal/application/store"
)

// newHTTPFakeStoreRepo — репозиторий настроек магазина в памяти (tenant "t-1",
// как у testAuth).
func newHTTPFakeStoreRepo() storeapp.Repository {
	return &httpFakeStoreRepo{settings: map[string]storeapp.Settings{}, prices: map[string]map[string]int64{}}
}

type httpFakeStoreRepo struct {
	settings map[string]storeapp.Settings
	prices   map[string]map[string]int64
}

func (f *httpFakeStoreRepo) GetSettings(_ context.Context, tenantID string) (storeapp.Settings, error) {
	s, ok := f.settings[tenantID]
	if !ok {
		return storeapp.Settings{}, storeapp.ErrNotFound
	}
	return s, nil
}

func (f *httpFakeStoreRepo) SaveSettings(_ context.Context, tenantID string, s storeapp.Settings, updatedBy string) error {
	s.UpdatedBy = updatedBy
	f.settings[tenantID] = s
	return nil
}

func (f *httpFakeStoreRepo) ListMaterialPrices(_ context.Context, tenantID string) ([]storeapp.MaterialPrice, error) {
	out := make([]storeapp.MaterialPrice, 0, len(f.prices[tenantID]))
	for code, price := range f.prices[tenantID] {
		out = append(out, storeapp.MaterialPrice{Code: code, PricePerKgRub: price, Overridden: true})
	}
	return out, nil
}

func (f *httpFakeStoreRepo) SetMaterialPrice(_ context.Context, tenantID, code string, price int64, _ string) error {
	if f.prices[tenantID] == nil {
		f.prices[tenantID] = map[string]int64{}
	}
	f.prices[tenantID][code] = price
	return nil
}

func (f *httpFakeStoreRepo) DeleteMaterialPrice(_ context.Context, tenantID, code string) error {
	delete(f.prices[tenantID], code)
	return nil
}

// publicJSONRequest — анонимный запрос с JSON-телом: decodeJSON требует
// Content-Type: application/json (как у витрины).
func publicJSONRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

// jsonHas — проверка наличия фрагмента в JSON-ответе.
func jsonHas(body []byte, fragment string) bool {
	return strings.Contains(string(body), fragment)
}

func testRouterWithStore(s StoreService, admin bool) http.Handler {
	cfg := DefaultConfig()
	cfg.Store = s
	var auth AuthService = testAuth{}
	if admin {
		auth = adminAuth{}
	}
	return NewRouter(stair.NewService(), nil, auth, cfg)
}

// TestAdminStoreSettingsRequiresAdmin — прайс и настройки магазина доступны
// только администратору (волна 0).
func TestAdminStoreSettingsRequiresAdmin(t *testing.T) {
	svc := storeapp.NewService(newHTTPFakeStoreRepo())
	for _, path := range []string{"/api/v1/admin/store/settings", "/api/v1/admin/store/prices"} {
		rec := httptest.NewRecorder()
		testRouterWithStore(svc, false).ServeHTTP(rec, authedRequest(http.MethodGet, path, ""))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: want 403 для user, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}
}

// TestAdminUpdateStoreSettings — PUT сохраняет контакты и параметры расчёта.
func TestAdminUpdateStoreSettings(t *testing.T) {
	svc := storeapp.NewService(newHTTPFakeStoreRepo())
	router := testRouterWithStore(svc, true)

	body := `{"contacts":{"phone":"+7 900 123-45-67","email":"shop@example.com"},
	          "rates":{"margin_percent":12,"tax_percent":20}}`
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPut, "/api/v1/admin/store/settings", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got storeapp.Settings
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Contacts.Phone != "+7 900 123-45-67" || got.Rates.MarginPercent != 12 {
		t.Fatalf("настройки не применены: %+v", got)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/admin/store/settings", ""))
	if rec.Code != http.StatusOK || !jsonHas(rec.Body.Bytes(), `"margin_percent":12`) {
		t.Fatalf("GET settings: %d %s", rec.Code, rec.Body.String())
	}
}

// TestAdminUpdateStoreSettingsRejectsInvalid — недопустимые проценты → 422.
func TestAdminUpdateStoreSettingsRejectsInvalid(t *testing.T) {
	svc := storeapp.NewService(newHTTPFakeStoreRepo())
	rec := httptest.NewRecorder()
	testRouterWithStore(svc, true).ServeHTTP(rec,
		authedRequest(http.MethodPut, "/api/v1/admin/store/settings", `{"rates":{"margin_percent":300}}`))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminMaterialPrices — прайс материалов: сохранение, чтение и сброс к
// встроенной ставке движка.
func TestAdminMaterialPrices(t *testing.T) {
	svc := storeapp.NewService(newHTTPFakeStoreRepo())
	router := testRouterWithStore(svc, true)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPut, "/api/v1/admin/store/prices", `{"code":"WOOD-OAK","price_per_kg_rub":4200}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("set price: %d %s", rec.Code, rec.Body.String())
	}
	if !jsonHas(rec.Body.Bytes(), `"overridden":true`) {
		t.Fatalf("цена не помечена как заданная магазином: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/admin/store/prices", ""))
	if rec.Code != http.StatusOK || !jsonHas(rec.Body.Bytes(), `"price_per_kg_rub":4200`) {
		t.Fatalf("list prices: %d %s", rec.Code, rec.Body.String())
	}

	// Неизвестный материал нельзя добавить в прайс.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPut, "/api/v1/admin/store/prices", `{"code":"GOLD-24K","price_per_kg_rub":1}`))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown material: want 422, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodDelete, "/api/v1/admin/store/prices/WOOD-OAK", ""))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete price: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/admin/store/prices", ""))
	if jsonHas(rec.Body.Bytes(), `"overridden":true`) {
		t.Fatalf("после удаления цена магазина осталась: %s", rec.Body.String())
	}
}

// TestPublicStoreSettings — витрина получает контакты/SEO/счётчики без ставок.
func TestPublicStoreSettings(t *testing.T) {
	svc := storeapp.NewService(newHTTPFakeStoreRepo())
	router := testRouterWithStore(svc, true)

	body := `{"contacts":{"phone":"+7 900 000-11-22"},"counters":{"yandex_metrika_id":"12345"}}`
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPut, "/api/v1/admin/store/settings", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("put settings: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/public/store-settings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("public settings: %d %s", rec.Code, rec.Body.String())
	}
	if !jsonHas(rec.Body.Bytes(), `"yandex_metrika_id":"12345"`) {
		t.Fatalf("код счётчика не отдан: %s", rec.Body.String())
	}
	for _, forbidden := range []string{`"rates"`, "margin_percent", "machine_per_hour_rub"} {
		if jsonHas(rec.Body.Bytes(), forbidden) {
			t.Fatalf("в публичных настройках утекла ставка %s: %s", forbidden, rec.Body.String())
		}
	}
}

// TestPublicQuoteRejectsClientRates — анонимный клиент не может задать ставки и
// занизить предварительную цену (S-150, волна 0).
func TestPublicQuoteRejectsClientRates(t *testing.T) {
	svc := storeapp.NewService(newHTTPFakeStoreRepo())
	router := testRouterWithStore(svc, false)

	body := `{"width_mm":900,"height_mm":2800,"flight":"straight","step_height_mm":175,
	          "stringer_thickness_mm":40,"step_thickness_mm":40,"riser":true,
	          "clearance_mm":2200,"railing_height_mm":900,
	          "rates":{"material_per_kg_rub":{"STEEL-S235":0.01},"margin_percent":0}}`
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, publicJSONRequest(http.MethodPost, "/api/v1/public/stairs:quote", body))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if !jsonHas(rec.Body.Bytes(), `"code":"rates_not_allowed"`) {
		t.Fatalf("ожидался код rates_not_allowed: %s", rec.Body.String())
	}
}

// TestPublicQuoteUsesStoreRates — цена публичного расчёта считается по прайсу
// магазина, а не по ставкам запроса.
func TestPublicQuoteUsesStoreRates(t *testing.T) {
	repo := newHTTPFakeStoreRepo()
	svc := storeapp.NewService(repo)
	router := testRouterWithStore(svc, false)
	quoteBody := `{"width_mm":900,"height_mm":2800,"flight":"straight","step_height_mm":175,
	                 "stringer_thickness_mm":40,"step_thickness_mm":40,"riser":true,
	                 "clearance_mm":2200,"railing_height_mm":900,"material":"WOOD-OAK"}`

	quote := func() float64 {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, publicJSONRequest(http.MethodPost, "/api/v1/public/stairs:quote", quoteBody))
		if rec.Code != http.StatusOK {
			t.Fatalf("quote: %d %s", rec.Code, rec.Body.String())
		}
		var out publicQuoteDTO
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.Pricing == nil {
			t.Fatal("в ответе нет цены")
		}
		return out.Pricing.MaterialRub
	}

	before := quote()
	if err := svc.SetMaterialPrice(context.Background(), "t-1", "WOOD-OAK", 9000, "u-1"); err != nil {
		t.Fatalf("SetMaterialPrice: %v", err)
	}
	after := quote()
	if after <= before {
		t.Fatalf("прайс магазина не поднял стоимость материала: было %.2f, стало %.2f", before, after)
	}
}

// TestPublicMaterialsUseStorePrices — каталог витрины отдаёт цену магазина, а
// правка прайса сбрасывает кэш публичных ответов (иначе старая цена держалась
// бы до TTL 5 минут).
func TestPublicMaterialsUseStorePrices(t *testing.T) {
	svc := storeapp.NewService(newHTTPFakeStoreRepo())
	router := testRouterWithStore(svc, true)

	// Прогреваем кэш каталога.
	before := materialPriceFromResponse(t, router)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPut, "/api/v1/admin/store/prices", `{"code":"WOOD-OAK","price_per_kg_rub":7777}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("set price: %d %s", rec.Code, rec.Body.String())
	}

	after, cacheState := materialPriceFromResponseWithCache(t, router)
	if after != 7777 {
		t.Fatalf("каталог не отдал цену магазина: было %.0f, стало %.0f", before, after)
	}
	if cacheState == "HIT" {
		t.Fatal("кэш публичного каталога не сброшен после правки прайса")
	}
}

func materialPriceFromResponse(t *testing.T, router http.Handler) float64 {
	t.Helper()
	price, _ := materialPriceFromResponseWithCache(t, router)
	return price
}

func materialPriceFromResponseWithCache(t *testing.T, router http.Handler) (float64, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/public/materials", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("materials: %d %s", rec.Code, rec.Body.String())
	}
	var out []materialDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, m := range out {
		if m.Code == "WOOD-OAK" {
			return m.PricePerKgRub, rec.Header().Get("X-Cache")
		}
	}
	t.Fatal("материал WOOD-OAK не найден в каталоге")
	return 0, ""
}

// TestStoreRoutesAbsentWithoutService — без StoreService маршруты не
// регистрируются, публичный расчёт работает на ставках движка.
func TestStoreRoutesAbsentWithoutService(t *testing.T) {
	router := NewRouter(stair.NewService(), nil, adminAuth{}, DefaultConfig())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/admin/store/settings", ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 без store-сервиса, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/public/store-settings", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("public store-settings: want 404, got %d", rec.Code)
	}
}
