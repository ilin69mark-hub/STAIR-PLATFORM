package http

import (
	"errors"
	"net/http"
	"time"

	"stairplatform/internal/application/payments"
)

// serviceCheckoutDTO — ответ на оплату услуги с витрины (этап 4).
type serviceCheckoutDTO struct {
	PaymentID   string `json:"payment_id"`
	TierID      string `json:"tier_id"`
	Title       string `json:"title"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	// CheckoutURL — адрес страницы оплаты PSP, куда уводим клиента.
	CheckoutURL string `json:"checkout_url"`
}

// myPaymentDTO — покупка клиента в личном кабинете.
type myPaymentDTO struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	TierID      string  `json:"tier_id,omitempty"`
	AmountMinor int64   `json:"amount_minor"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	PaidAt      *string `json:"paid_at,omitempty"`
}

// handleServiceCheckout — POST /api/v1/public/services/checkout (auth).
// Создаёт платёжный интент за услугу из серверного каталога (S-150) и
// возвращает URL страницы оплаты PSP. Проект не требуется: услугу покупает
// клиент сайта, а не участник инженерной панели.
//
// Клиент присылает только tier_id — сумму подменить нельзя.
func handleServiceCheckout(svc PaymentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req checkoutRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		if req.TierID == "" {
			writeInputError(w, "invalid_input", errors.New("tier_id is required"))
			return
		}
		intent, err := svc.CreateServiceCheckout(r.Context(), tenantID(r.Context()), userID(r.Context()), req.TierID)
		if errors.Is(err, payments.ErrInvalid) {
			// Неизвестный/пустой tier_id — ошибка ввода (422), а не сбой (500).
			writeInputError(w, "invalid_input", err)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusCreated, serviceCheckoutDTO{
			PaymentID:   intent.ID,
			TierID:      intent.TierID,
			Title:       tierTitle(svc, intent.TierID),
			AmountMinor: intent.AmountMinor,
			Currency:    intent.Currency,
			CheckoutURL: intent.CheckoutURL,
		})
	}
}

// handleListMyPayments — GET /api/v1/payments/mine (auth).
// Покупки и оплаты текущего клиента для личного кабинета.
func handleListMyPayments(svc PaymentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		intents, err := svc.ListByUser(r.Context(), tenantID(r.Context()), userID(r.Context()))
		if errors.Is(err, payments.ErrInvalid) {
			writeInputError(w, "invalid_input", err)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		out := make([]myPaymentDTO, 0, len(intents))
		for _, p := range intents {
			title := "Оплата проекта"
			switch {
			case p.TierID != "" && tierTitle(svc, p.TierID) != p.TierID:
				title = tierTitle(svc, p.TierID)
			case p.TierID != "":
				title = p.TierID
			}
			dto := myPaymentDTO{
				ID:          p.ID,
				Title:       title,
				TierID:      p.TierID,
				AmountMinor: p.AmountMinor,
				Currency:    p.Currency,
				Status:      string(p.Status),
				CreatedAt:   p.CreatedAt.UTC().Format(time.RFC3339),
			}
			if p.PaidAt != nil {
				paid := p.PaidAt.UTC().Format(time.RFC3339)
				dto.PaidAt = &paid
			}
			out = append(out, dto)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// tierTitle возвращает подпись услуги из каталога; без подписи — сам код
// (витрина покажет его как есть, а не пустую строку).
func tierTitle(svc PaymentService, tierID string) string {
	for _, t := range svc.ListTiers() {
		if t.ID == tierID && t.Title != "" {
			return t.Title
		}
	}
	return tierID
}
