package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// SEC-PRICING-PUB: публичный витринный расчёт не аутентифицирован, поэтому
// структура себестоимости в нём быть не должна.
//
// Изначально маршрут отдавал полную pricingDTO: материалы, обработка, работа,
// накладные, себестоимость, МАРЖА, скидка, НДС и построчные lines. Любой
// посетитель получал это одним curl — то есть наценку продавца и его
// себестоимость. Комментарий над publicQuoteDTO при этом обещал «только
// предварительную цену»: реализация комментарию не соответствовала, и
// расхождение было неочевидно, потому что тип назывался так же, как
// внутренний.
//
// Тест ловит именно возврат утечки: если кто-то снова подставит в
// publicQuoteDTO полный pricingDTO, тест упадёт на перечислении ключей.

func TestPublicQuoteLeaksNoCostStructure(t *testing.T) {
	req := publicQuoteRequest(referenceJSON)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Разбираем ответ как сырую карту, а не как publicQuoteDTO: цель — увидеть
	// все JSON-ключи, которые реально ушли клиенту, включая те, которых нет в
	// типе ответа.
	var raw struct {
		Pricing map[string]any `json:"pricing"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if raw.Pricing == nil {
		t.Fatal("pricing is absent: покупатель должен видеть хотя бы предварительную цену")
	}

	// Внутренние статьи, которые не должны покидать сервер.
	forbidden := []string{
		"material_rub",
		"machine_rub",
		"labor_rub",
		"overhead_rub",
		"production_cost_rub",
		"margin_rub",
		"discount_rub",
		"pre_tax_rub",
		"tax_rub",
		"lines",
	}
	for _, key := range forbidden {
		if _, ok := raw.Pricing[key]; ok {
			t.Errorf("pricing.%s утекает в анонимный публичный расчёт", key)
		}
	}

	// Разрешено ровно два поля: валюта и итог.
	allowed := map[string]bool{"currency": true, "final_price_rub": true}
	for key := range raw.Pricing {
		if !allowed[key] {
			t.Errorf("pricing.%s — неожиданное поле в публичном ответе", key)
		}
	}
	if _, ok := raw.Pricing["final_price_rub"]; !ok {
		t.Error("pricing.final_price_rub обязателен: без него витрина не покажет цену")
	}
	if v, ok := raw.Pricing["final_price_rub"].(float64); !ok || v <= 0 {
		t.Errorf("final_price_rub должен быть положительным числом, получено %v", raw.Pricing["final_price_rub"])
	}
}

// TestPublicQuoteBodyHasNoCostKeys — та же гарантия на уровне СЫРОГО тела:
// даже если в DTO добавят поле с omitempty или забудут про omitempty, в
// теле ответа этих ключей быть не должно.
func TestPublicQuoteBodyHasNoCostKeys(t *testing.T) {
	req := publicQuoteRequest(referenceJSON)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, key := range []string{
		"margin_rub",
		"overhead_rub",
		"production_cost_rub",
		"material_rub",
		"machine_rub",
		"labor_rub",
		"pre_tax_rub",
		"tax_rub",
		"discount_rub",
	} {
		if strings.Contains(body, `"`+key+`"`) {
			t.Errorf("тело публичного ответа содержит %q — утечка внутренних данных", key)
		}
	}
	// Итоговая цена при этом обязана остаться: расчёт без цены бессмыслен.
	if !strings.Contains(body, `"final_price_rub"`) {
		t.Error("в публичном ответе нет final_price_rub")
	}
}
