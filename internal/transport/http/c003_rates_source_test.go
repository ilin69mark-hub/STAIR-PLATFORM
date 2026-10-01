package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/application/store"
	domprc "stairplatform/internal/domain/pricing"
	engprc "stairplatform/internal/engine/pricing"
)

// Регрессия CRITICAL-03 (2026-09-27): расхождение цен между маршрутами.
//
// ДО фикса политика выбора источника ставок жила в транспорте: публичная
// витрина подкладывала ставки магазина, а девять авторизованных маршрутов —
// нет. С одними и теми же входными данными витрина и расчёт давали РАЗНЫЕ
// цены, причём расхождение зависело от маршрута, а не от данных.
//
// Тест строит роутер с подключённым store-сервисом и заведомо НЕ дефолтными
// ставками магазина, затем сравнивает цену авторизованного расчёта с ценой
// публичной витрины.

// c003Store — заглушка StoreService, отдающая фиксированные ставки.
type c003Store struct {
	rates *engprc.Rates
	err   error
	calls int
}

func (s *c003Store) Settings(context.Context, string) (store.Settings, error) {
	return store.Settings{}, s.err
}
func (s *c003Store) UpdateSettings(context.Context, string, store.Settings) (store.Settings, error) {
	return store.Settings{}, s.err
}
func (s *c003Store) PublicSettings(context.Context, string) (store.PublicSettings, error) {
	return store.PublicSettings{}, s.err
}
func (s *c003Store) MaterialPrices(context.Context, string) ([]store.MaterialPrice, error) {
	return nil, s.err
}
func (s *c003Store) SetMaterialPrice(context.Context, string, string, int64, string) error {
	return s.err
}
func (s *c003Store) DeleteMaterialPrice(context.Context, string, string) error { return s.err }
func (s *c003Store) ResolveRates(context.Context, string) (*engprc.Rates, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.rates, nil
}

// c003Auth — sec1Auth, но с настоящим дефолтным tenant'ом: витрина берёт
// ставки именно его магазина, а sec1Auth отдаёт (nil, nil).
type c003Auth struct {
	sec1Auth
	t *auth.Tenant
}

func (a *c003Auth) DefaultTenant(context.Context) (*auth.Tenant, error) { return a.t, nil }

func c003RouterWithStore(t *testing.T, st *c003Store) http.Handler {
	t.Helper()
	u := &auth.User{ID: "u1", TenantID: "t1", Email: "a@b.c", Role: auth.RoleAdmin}
	cfg := DefaultConfig()
	cfg.SecurityConfig = &SecurityConfig{AllowedOrigins: []string{"http://x"}}
	cfg.Jobs = sec1Jobs{}
	cfg.Assistant = sec1Assistant{}
	cfg.Store = st
	authStub := &c003Auth{
		sec1Auth: sec1Auth{u: u},
		t:        &auth.Tenant{ID: "shop-1", Name: "Магазин", Slug: "default"},
	}
	return NewRouter(&sec1Stair{stair.NewServiceWithRates(st)}, &sec1Projects{n: 25},
		authStub, cfg, nil)
}

func c003StoreRates() *engprc.Rates {
	rs := engprc.DefaultRates()
	// Заметно отличающиеся ставки: дефолт движка даёт другую цену.
	margin, err := domprc.NewRate(35)
	if err != nil {
		panic(err)
	}
	rs.MarginPercent = margin
	rs.Material["STEEL-S235"] = domprc.NewMoney(90000) // 900 ₽/кг
	return &rs
}

// c003Price достаёт итоговую цену из ответа любого из двух видов.
func c003Price(t *testing.T, body []byte) (float64, bool) {
	t.Helper()
	var calc struct {
		Pricing *struct {
			FinalPriceRub float64 `json:"final_price_rub"`
		} `json:"pricing"`
	}
	if err := json.Unmarshal(body, &calc); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, body)
	}
	if calc.Pricing == nil {
		return 0, false
	}
	return calc.Pricing.FinalPriceRub, true
}

// TestCRIT003_AuthorizedAndPublicRoutesUseSameRates — ядро дефекта: одна
// лестница, один набор ставок магазина, два маршрута — цена обязана совпасть.
func TestCRIT003_AuthorizedAndPublicRoutesUseSameRates(t *testing.T) {
	st := &c003Store{rates: c003StoreRates()}
	h := c003RouterWithStore(t, st)

	authRec := sec1Post(t, h, "/api/v1/stairs:calculate", sec1Body)
	if authRec.Code != http.StatusOK {
		t.Fatalf("authorized calculate: status=%d body=%s", authRec.Code, authRec.Body.String())
	}
	authPrice, ok := c003Price(t, authRec.Body.Bytes())
	if !ok {
		t.Fatalf("authorized response has no pricing: %s", authRec.Body.String())
	}

	pubRec := sec1Post(t, h, "/api/v1/public/stairs:quote", sec1Body)
	if pubRec.Code != http.StatusOK {
		t.Fatalf("public quote: status=%d body=%s", pubRec.Code, pubRec.Body.String())
	}
	pubPrice, ok := c003Price(t, pubRec.Body.Bytes())
	if !ok {
		t.Fatalf("public response has no pricing: %s", pubRec.Body.String())
	}

	if authPrice != pubPrice {
		t.Fatalf("CRITICAL-03: authorized price %v != public price %v — routes disagree on rates",
			authPrice, pubPrice)
	}
	if st.calls == 0 {
		t.Error("store rates resolver was never called — rates source is not wired")
	}
	t.Logf("authorized=%v public=%v resolver calls=%d", authPrice, pubPrice, st.calls)
}

// TestCRIT003_AuthorizedRouteActuallyUsesStoreRates — витрина и расчёт могли бы
// совпасть и потому, что оба молча уехали во встроенные ставки. Тест требует
// подключённого резолвера: без вызова store-сервиса цена = дефолт движка.
func TestCRIT003_AuthorizedRouteActuallyUsesStoreRates(t *testing.T) {
	withStore := &c003Store{rates: c003StoreRates()}
	hStore := c003RouterWithStore(t, withStore)
	storeRec := sec1Post(t, hStore, "/api/v1/stairs:calculate", sec1Body)
	storePrice, ok := c003Price(t, storeRec.Body.Bytes())
	if !ok {
		t.Fatalf("no pricing: %s", storeRec.Body.String())
	}

	// Тот же расчёт без store-сервиса: обязаны получить встроенные ставки.
	hEngine := sec1Router()
	engineRec := sec1Post(t, hEngine, "/api/v1/stairs:calculate", sec1Body)
	enginePrice, ok := c003Price(t, engineRec.Body.Bytes())
	if !ok {
		t.Fatalf("no pricing: %s", engineRec.Body.String())
	}

	if storePrice == enginePrice {
		t.Fatalf("store rates had no effect (%v) — resolver is not wired into the authorized route",
			storePrice)
	}
	t.Logf("store rates=%v engine defaults=%v", storePrice, enginePrice)
}

// TestCRIT003_StoreFailureDoesNotFailCalculation — сбой прайса магазина не
// должен превращать расчёт в 500: витрина и расчёт откатываются на встроенные
// ставки движка.
func TestCRIT003_StoreFailureDoesNotFailCalculation(t *testing.T) {
	st := &c003Store{err: c003Err("store settings unavailable")}
	h := c003RouterWithStore(t, st)

	for _, path := range []string{"/api/v1/stairs:calculate", "/api/v1/public/stairs:quote"} {
		rec := sec1Post(t, h, path, sec1Body)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status=%d (want 200) body=%s", path, rec.Code, rec.Body.String())
		}
		price, ok := c003Price(t, rec.Body.Bytes())
		if !ok {
			t.Fatalf("%s: no pricing in response: %s", path, rec.Body.String())
		}
		if price <= 0 {
			t.Errorf("%s: price = %v, want > 0", path, price)
		}
	}
}

type c003Err string

func (e c003Err) Error() string { return string(e) }

// TestCRIT003_OptionsOrRejectStampsTenant — optionsOrReject обязан проставлять
// tenant-контекст: без него девять авторизованных маршрутов остались бы на
// встроенных ставках. Точка одна — и именно поэтому тест проверяет её
// напрямую, а не поведение каждого маршрута по отдельности.
func TestCRIT003_OptionsOrRejectStampsTenant(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stairs:calculate", strings.NewReader("{}"))
	req = req.WithContext(withAuthUser(req.Context(), &auth.User{
		ID: "u", TenantID: "tenant-42", Email: "a@b.c", Role: auth.RoleAdmin,
	}))

	opts, ok := optionsOrReject(req, httptest.NewRecorder(), calculateRequest{})
	if !ok {
		t.Fatal("optionsOrReject rejected a request without client rates")
	}
	if opts.TenantID != "tenant-42" {
		t.Errorf("Options.TenantID = %q, want tenant-42", opts.TenantID)
	}
	if opts.Rates != nil {
		t.Errorf("transport must never set Rates, got %+v", opts.Rates)
	}
}
