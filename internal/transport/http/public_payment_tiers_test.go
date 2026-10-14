package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPublicPaymentTiers — витрина этапа 4 показывает серверный прайс услуг:
// id, подпись, описание и сумма из каталога (S-150). Клиент сумму не присылает
// и не может её подменить.
func TestPublicPaymentTiers(t *testing.T) {
	rec := httptest.NewRecorder()
	// Прайс появляется, когда подключён платёжный сервис (роутер регистрирует
	// маршрут вместе с остальными платёжными).
	router := testRouterWithPayments(nil, newFakePaymentService())
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/public/payment-tiers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got []paymentTierDTO
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 услуги, got %d: %+v", len(got), got)
	}
	byID := map[string]paymentTierDTO{}
	for _, tier := range got {
		byID[tier.ID] = tier
	}
	basic, ok := byID["basic"]
	if !ok {
		t.Fatalf("нет тарифа basic: %+v", got)
	}
	if basic.AmountMinor != 90_000 || basic.Currency != "RUB" || basic.AmountRub != 900 {
		t.Fatalf("цена basic изменилась или неверна: %+v", basic)
	}
	if basic.Title == "" || basic.Description == "" {
		t.Errorf("у тарифа basic нет подписей для витрины: %+v", basic)
	}
	pro, ok := byID["pro"]
	if !ok {
		t.Fatalf("нет тарифа pro: %+v", got)
	}
	if pro.AmountRub != 1800 {
		t.Errorf("цена pro: %+v", pro)
	}
	if rec.Header().Get("Cache-Control") == "" {
		t.Error("витринный прайс должен кэшироваться")
	}
}
