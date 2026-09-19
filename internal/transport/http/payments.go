package http

import (
	"context"
	"errors"
	"io"
	"net/http"

	"stairplatform/internal/application/payments"
	"stairplatform/internal/application/project"
	"stairplatform/internal/infrastructure/integrations"
)

// PaymentService — прикладной интерфейс платежей (EDR-0027 §3.3), ожидаемый
// транспортным слоем.
type PaymentService interface {
	CreateCheckout(ctx context.Context, tenantID, projectID, userID string, amountMinor int64, currency string) (*payments.PaymentIntent, error)
	ListByProject(ctx context.Context, tenantID, projectID string) ([]*payments.PaymentIntent, error)
	Get(ctx context.Context, tenantID, id string) (*payments.PaymentIntent, error)
	HandleWebhook(ctx context.Context, secret, tsUnix, sigValue string, body []byte) (*payments.PaymentEvent, error)
}

// StripeWebhookService — интерфейс для Stripe-специфичной обработки webhook.
type StripeWebhookService interface {
	HandleStripeWebhook(ctx context.Context, payload []byte, signature string) error
}

// ---- DTO ----

type paymentIntentDTO struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id,omitempty"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	Provider    string `json:"provider"`
	CheckoutURL string `json:"checkout_url,omitempty"`
	CreatedAt   string `json:"created_at"`
	PaidAt      string `json:"paid_at,omitempty"`
}

func toPaymentIntentDTO(p *payments.PaymentIntent) paymentIntentDTO {
	d := paymentIntentDTO{
		ID:          p.ID,
		ProjectID:   p.ProjectID,
		AmountMinor: p.AmountMinor,
		Currency:    p.Currency,
		Status:      string(p.Status),
		Provider:    p.Provider,
		CheckoutURL: p.CheckoutURL,
		CreatedAt:   p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if p.PaidAt != nil {
		d.PaidAt = p.PaidAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return d
}

type checkoutRequest struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

// ---- handlers ----

// handleCheckout — POST /api/v1/projects/{id}/checkout (auth, член проекта).
// Создаёт платёжный интент через PSP и возвращает checkout URL.
// 201 — создано; 400 — невалидный JSON; 403 — нет прав; 404 — нет проекта;
// 422 — невалидная сумма/валюта.
func handleCheckout(projects ProjectService, svc PaymentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		ctx := r.Context()
		user := userID(ctx)
		tenant := tenantID(ctx)

		if _, err := projects.GetProject(ctx, tenant, user, projectID); err != nil {
			if errors.Is(err, project.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "Проект не найден.")
				return
			}
			if errors.Is(err, project.ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}

		var req checkoutRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}

		p, err := svc.CreateCheckout(ctx, tenant, projectID, user, req.AmountMinor, req.Currency)
		if errors.Is(err, payments.ErrInvalid) {
			writeInputError(w, "invalid_input", err)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusCreated, toPaymentIntentDTO(p))
	}
}

// handleListPayments — GET /api/v1/projects/{id}/payments (auth, член проекта).
// 200 — список платежей проекта; 403 — нет прав; 404 — нет проекта.
func handleListPayments(projects ProjectService, svc PaymentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		ctx := r.Context()
		user := userID(ctx)
		tenant := tenantID(ctx)

		if _, err := projects.GetProject(ctx, tenant, user, projectID); err != nil {
			if errors.Is(err, project.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "Проект не найден.")
				return
			}
			if errors.Is(err, project.ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}

		list, err := svc.ListByProject(ctx, tenant, projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		out := make([]paymentIntentDTO, 0, len(list))
		for _, p := range list {
			out = append(out, toPaymentIntentDTO(p))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// handleGetPayment — GET /api/v1/payments/{id} (auth, tenant-скоуп).
// 200 — статус интента; 404 — не найден.
func handleGetPayment(svc PaymentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := svc.Get(r.Context(), tenantID(r.Context()), r.PathValue("id"))
		if errors.Is(err, payments.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Платёж не найден.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusOK, toPaymentIntentDTO(p))
	}
}

// handlePaymentWebhook — POST /api/v1/payments/webhook (публичный; безопасность
// за счёт HMAC-подписи). Верифицирует подпись PSP, переводит статус интента.
// 200 — принято; 401 — невалидная подпись; 404 — нет интента;
// 409 — расхождение суммы/валюты; 422 — невалидное тело/статус.
// secret — секрет верификации подписи (инстанс-зависимый, P2-11).
func handlePaymentWebhook(svc PaymentService, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Не удалось прочитать тело запроса.")
			return
		}
		_, err = svc.HandleWebhook(r.Context(),
			secret,
			r.Header.Get(integrations.HeaderTimestamp),
			r.Header.Get(integrations.HeaderSignature),
			body)
		switch {
		case errors.Is(err, payments.ErrInvalidSignature):
			writeError(w, http.StatusUnauthorized, "invalid_signature", "Не удалось проверить подпись webhook.")
		case errors.Is(err, payments.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Платёж не найден.")
		case errors.Is(err, payments.ErrInvalid):
			writeError(w, http.StatusConflict, "invalid_input", userInputMessage(err))
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
		default:
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		}
	}
}

// handleStripeWebhook — POST /api/v1/payments/stripe/webhook (публичный).
// Обрабатывает Stripe-специфичные webhook events с форматом подписи t=timestamp,v1=signature.
// 200 — принято; 401 — невалидная подпись; 500 — внутренняя ошибка.
func handleStripeWebhook(svc StripeWebhookService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Не удалось прочитать тело запроса.")
			return
		}

		signature := r.Header.Get("Stripe-Signature")
		if signature == "" {
			writeError(w, http.StatusBadRequest, "missing_signature", "Отсутствует заголовок Stripe-Signature.")
			return
		}

		err = svc.HandleStripeWebhook(r.Context(), body, signature)
		switch {
		case errors.Is(err, payments.ErrInvalidSignature):
			writeError(w, http.StatusUnauthorized, "invalid_signature", "Не удалось проверить подпись Stripe webhook.")
		case errors.Is(err, payments.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Платёж не найден.")
		case errors.Is(err, payments.ErrInvalid):
			writeError(w, http.StatusConflict, "invalid_input", userInputMessage(err))
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
		default:
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		}
	}
}
