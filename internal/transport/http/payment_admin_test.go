package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/payments"
	"stairplatform/internal/application/stair"
)

type fakePaymentAdminService struct {
	list      []*payments.PaymentIntent
	refunded  *payments.PaymentIntent
	tenantID  string
	refundID  string
	listErr   error
	refundErr error
}

func (f *fakePaymentAdminService) ListAll(_ context.Context, tenantID string) ([]*payments.PaymentIntent, error) {
	f.tenantID = tenantID
	return f.list, f.listErr
}

func (f *fakePaymentAdminService) Refund(_ context.Context, tenantID, id string) (*payments.PaymentIntent, error) {
	f.tenantID = tenantID
	f.refundID = id
	return f.refunded, f.refundErr
}

func paymentAdminRouter(svc PaymentAdminService, admin bool) http.Handler {
	cfg := DefaultConfig()
	cfg.PaymentAdmin = svc
	if admin {
		return NewRouter(stair.NewService(), nil, adminAuth{}, cfg)
	}
	return NewRouter(stair.NewService(), nil, testAuth{}, cfg)
}

func TestAdminListPayments(t *testing.T) {
	svc := &fakePaymentAdminService{list: []*payments.PaymentIntent{{
		ID: "pay-1", TenantID: "t-1", TierID: "design", AmountMinor: 500000,
		Currency: "RUB", Status: payments.StatusPaid, Provider: "mock",
		CreatedAt: timeForPayment(),
	}}}
	req := authedRequest(http.MethodGet, "/api/v1/admin/payments", "")
	rec := httptest.NewRecorder()
	paymentAdminRouter(svc, true).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.tenantID != "t-1" || !strings.Contains(rec.Body.String(), `"tier_id":"design"`) {
		t.Fatalf("tenant or DTO missing: tenant=%q body=%s", svc.tenantID, rec.Body.String())
	}
}

func TestAdminListPaymentsRequiresPermission(t *testing.T) {
	req := authedRequest(http.MethodGet, "/api/v1/admin/payments", "")
	rec := httptest.NewRecorder()
	paymentAdminRouter(&fakePaymentAdminService{}, false).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRefundPayment(t *testing.T) {
	svc := &fakePaymentAdminService{refunded: &payments.PaymentIntent{
		ID: "pay-1", TenantID: "t-1", AmountMinor: 500000, Currency: "RUB",
		Status: payments.StatusRefunded, Provider: "mock", CreatedAt: timeForPayment(),
	}}
	req := authedRequest(http.MethodPost, "/api/v1/admin/payments/pay-1/refund", "")
	rec := httptest.NewRecorder()
	paymentAdminRouter(svc, true).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.tenantID != "t-1" || svc.refundID != "pay-1" {
		t.Fatalf("wrong refund scope: tenant=%q id=%q", svc.tenantID, svc.refundID)
	}
	var got adminPaymentDTO
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Status != string(payments.StatusRefunded) {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestAdminRefundPaymentErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "not found", err: payments.ErrNotFound, want: http.StatusNotFound},
		{name: "invalid status", err: payments.ErrInvalidStatus, want: http.StatusConflict},
		{name: "provider unavailable", err: payments.ErrProviderUnavailable, want: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakePaymentAdminService{refundErr: tt.err}
			req := authedRequest(http.MethodPost, "/api/v1/admin/payments/pay-1/refund", "")
			rec := httptest.NewRecorder()
			paymentAdminRouter(svc, true).ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("want %d, got %d: %s", tt.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminPaymentRoutesRequireService(t *testing.T) {
	router := NewRouter(stair.NewService(), nil, adminAuth{}, DefaultConfig())
	for _, req := range []*http.Request{
		authedRequest(http.MethodGet, "/api/v1/admin/payments", ""),
		authedRequest(http.MethodPost, "/api/v1/admin/payments/pay-1/refund", ""),
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		want := http.StatusNotFound
		if req.Method == http.MethodPost {
			want = http.StatusMethodNotAllowed
		}
		if rec.Code != want {
			t.Fatalf("want %d without PaymentAdmin, got %d", want, rec.Code)
		}
	}
}

func timeForPayment() time.Time {
	return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
}
