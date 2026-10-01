package http

import "net/http"

// paymentTierDTO — публичная карточка платной услуги для витрины (этап 4).
// Источник истины — серверный каталог тарифов (S-150): сумма приходит
// только оттуда, клиент её не влияет и не подменяет.
type paymentTierDTO struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	// AmountMinor — сумма в копейках (ADR-0008), точная для расчётов.
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	// AmountRub — та же сумма в рублях для показа: витрине не нужно знать
	// про minor-единицы, а округление на бэкенде гарантирует одинаковую цифру
	// на всех страницах.
	AmountRub float64 `json:"amount_rub"`
}

// handlePublicPaymentTiers — GET /api/v1/public/payment-tiers.
// Прайс платных услуг для витрины. Ответ детерминирован, поэтому кэшируется
// на edge: цены меняются редко и только через конфигурацию.
func handlePublicPaymentTiers(svc PaymentService) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if svc == nil {
			writeError(w, http.StatusServiceUnavailable, "catalog_unavailable", "Каталог услуг временно недоступен")
			return
		}
		tiers := svc.ListTiers()
		out := make([]paymentTierDTO, 0, len(tiers))
		for _, t := range tiers {
			out = append(out, paymentTierDTO{
				ID:          t.ID,
				Title:       t.Title,
				Description: t.Description,
				AmountMinor: t.AmountMinor,
				Currency:    t.Currency,
				AmountRub:   float64(t.AmountMinor) / 100,
			})
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, http.StatusOK, out)
	}
}
