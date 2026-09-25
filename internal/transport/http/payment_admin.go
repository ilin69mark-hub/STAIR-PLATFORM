package http

import (
	"context"
	"errors"
	"net/http"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/payments"
)

type PaymentAdminService interface {
	ListAll(ctx context.Context, tenantID string) ([]*payments.PaymentIntent, error)
	Refund(ctx context.Context, tenantID, id string) (*payments.PaymentIntent, error)
}

type adminPaymentDTO struct {
	ID          string `json:"id"`
	TierID      string `json:"tier_id,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	Provider    string `json:"provider"`
	CreatedAt   string `json:"created_at"`
	PaidAt      string `json:"paid_at,omitempty"`
}

func toAdminPaymentDTO(p *payments.PaymentIntent) adminPaymentDTO {
	d := adminPaymentDTO{
		ID:          p.ID,
		TierID:      p.TierID,
		ProjectID:   p.ProjectID,
		UserID:      p.UserID,
		AmountMinor: p.AmountMinor,
		Currency:    p.Currency,
		Status:      string(p.Status),
		Provider:    p.Provider,
		CreatedAt:   p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if p.PaidAt != nil {
		d.PaidAt = p.PaidAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return d
}

func handleAdminListPayments(svc PaymentAdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionPaymentsList) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора платежей")
			return
		}
		list, err := svc.ListAll(r.Context(), tenantID(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		out := make([]adminPaymentDTO, 0, len(list))
		for _, p := range list {
			out = append(out, toAdminPaymentDTO(p))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func handleAdminRefundPayment(svc PaymentAdminService, auditSvc AuditService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionPaymentsManage) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуются права управления платежами")
			return
		}
		p, err := svc.Refund(r.Context(), tenantID(r.Context()), r.PathValue("id"))
		if err != nil {
			writePaymentAdminError(w, err)
			return
		}
		if auditSvc != nil {
			_ = auditSvc.Record(r.Context(), &audit.Event{
				ActorID:      userID(r.Context()),
				TenantID:     tenantID(r.Context()),
				Action:       audit.ActionPaymentRefunded,
				ResourceType: "payment",
				ResourceID:   p.ID,
				Result:       audit.ResultOK,
				Detail:       "status=refunded",
				RequestID:    auditRequestID(r.Context()),
				IP:           clientIP(r),
			})
		}
		writeJSON(w, http.StatusOK, toAdminPaymentDTO(p))
	}
}

func writePaymentAdminError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, payments.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Платёж не найден")
	case errors.Is(err, payments.ErrInvalidStatus):
		writeError(w, http.StatusConflict, "invalid_status", "Возврат доступен только для оплаченного платежа")
	case errors.Is(err, payments.ErrProviderMismatch):
		writeError(w, http.StatusConflict, "provider_mismatch", "Платёж относится к другому платёжному провайдеру")
	case errors.Is(err, payments.ErrRefundUnsupported):
		writeError(w, http.StatusNotImplemented, "refund_unsupported", "Активный платёжный провайдер не поддерживает возвраты")
	case errors.Is(err, payments.ErrProviderUnavailable):
		writeError(w, http.StatusServiceUnavailable, "provider_unavailable", "Платёжный провайдер временно недоступен")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
	}
}
