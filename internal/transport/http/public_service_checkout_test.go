package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestServiceCheckoutUsesServerPrice — покупка услуги с витрины (этап 4):
// клиент присылает только tier_id, сумма берётся из серверного каталога
// (S-150), ответ содержит URL страницы оплаты PSP.
func TestServiceCheckoutUsesServerPrice(t *testing.T) {
	svc := newFakePaymentService()
	router := testRouterWithPayments(nil, svc)

	req := authedRequest(http.MethodPost, "/api/v1/public/services/checkout", `{"tier_id":"pro"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var got serviceCheckoutDTO
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.TierID != "pro" || got.AmountMinor != 180_000 || got.Currency != "RUB" {
		t.Fatalf("цена/тариф из серверного каталога: %+v", got)
	}
	if got.CheckoutURL == "" {
		t.Fatal("нет URL страницы оплаты PSP")
	}
	if got.Title == "pro" {
		t.Error("ответ должен нести подпись услуги, а не код")
	}
	// Клиент не может подменить сумму: в теле её нет, а интент создан с ценой каталога.
	if len(svc.intents) != 1 || svc.intents[0].AmountMinor != 180_000 {
		t.Fatalf("интент создан с чужой суммой: %+v", svc.intents)
	}
	if svc.intents[0].TierID != "pro" {
		t.Errorf("в интенте не записан код услуги: %+v", svc.intents[0])
	}
}

func TestServiceCheckoutRejectsUnknownTier(t *testing.T) {
	router := testRouterWithPayments(nil, newFakePaymentService())
	for _, body := range []string{`{"tier_id":"enterprise"}`, `{}`} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/public/services/checkout", body))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("body %s: want 422, got %d: %s", body, rec.Code, rec.Body.String())
		}
	}
}

// TestServiceCheckoutRequiresAuth — без входа оплата не создаётся.
func TestServiceCheckoutRequiresAuth(t *testing.T) {
	router := testRouterWithPayments(nil, newFakePaymentService())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/public/services/checkout", strings.NewReader(`{"tier_id":"basic"}`)))
	if rec.Code == http.StatusCreated {
		t.Fatalf("anonymous checkout must be rejected, got %d", rec.Code)
	}
}

// TestListMyPayments — кабинет видит только свои покупки.
func TestListMyPayments(t *testing.T) {
	svc := newFakePaymentService()
	router := testRouterWithPayments(nil, svc)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/public/services/checkout", `{"tier_id":"basic"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("checkout: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/payments/mine", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []myPaymentDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 покупка, got %d: %+v", len(list), list)
	}
	if list[0].TierID != "basic" || list[0].AmountMinor != 90_000 || list[0].Status != "pending" {
		t.Fatalf("покупка в кабинете: %+v", list[0])
	}
	if list[0].Title == "basic" {
		t.Error("в кабинете нужна подпись услуги, а не код")
	}
}
