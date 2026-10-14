package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// EventTypePaymentRefunded — событие успешного возврата платежа.
const EventTypePaymentRefunded = "payment.refunded"

var (
	// ErrInvalidStatus — возврат разрешён только для оплаченного платежа.
	ErrInvalidStatus = errors.New("payments: invalid status for refund")
	// ErrProviderMismatch — платёж создан другим платёжным провайдером.
	ErrProviderMismatch = errors.New("payments: provider mismatch")
	// ErrRefundUnsupported — активный провайдер не поддерживает возвраты.
	ErrRefundUnsupported = errors.New("payments: refund unsupported")
	// ErrProviderUnavailable — PSP не смог выполнить возврат.
	ErrProviderUnavailable = errors.New("payments: provider unavailable")
)

// RefundProvider — опциональная возможность PSP выполнить полный возврат.
// Реализация обязана идемпотентно обрабатывать повторный вызов с тем же
// idempotencyKey: это делает безопасным повтор admin-запроса после сетевой
// ошибки или падения процесса между подтверждением PSP и записью в БД.
type RefundProvider interface {
	Refund(ctx context.Context, providerCheckoutID, idempotencyKey string) (refundID string, err error)
}

// AdminRepository — отдельный порт административных операций. Он не расширяет
// Repository клиентского платёжного сервиса: возврат имеет собственную
// транзакционную семантику и не должен менять клиентские use cases.
type AdminRepository interface {
	// GetIntent возвращает интент по ID внутри tenant.
	GetIntent(ctx context.Context, tenantID, id string) (*PaymentIntent, error)
	// ListAll возвращает платежи tenant, новые первыми. Даже пустой tenantID
	// обязан оставаться SQL-фильтром, а не снимать tenant-скоуп.
	ListAll(ctx context.Context, tenantID string) ([]*PaymentIntent, error)
	// MarkRefunded атомарно переводит paid -> refunded и записывает событие.
	// Повторный вызов для refunded возвращает текущий интент без нового события.
	MarkRefunded(ctx context.Context, tenantID, intentID string, event *PaymentEvent) (*PaymentIntent, error)
}

// AdminService — прикладной сервис административных операций с платежами.
type AdminService struct {
	repo     AdminRepository
	provider Provider
	now      func() time.Time
}

// NewAdminService создаёт сервис просмотра и возврата платежей администратором.
func NewAdminService(repo AdminRepository, provider Provider) *AdminService {
	return &AdminService{repo: repo, provider: provider, now: time.Now}
}

// ListAll возвращает все платежи текущего tenant, новые первыми.
func (s *AdminService) ListAll(ctx context.Context, tenantID string) ([]*PaymentIntent, error) {
	return s.repo.ListAll(ctx, tenantID)
}

// Refund выполняет полный возврат оплаченного платежа. Вызов PSP идёт до
// изменения БД со стабильным ключом: успешный повтор после crash-window не
// создаёт вторую PSP-операцию. Только после подтверждения PSP локальная
// транзакция атомарно меняет статус и пишет финансовое событие.
func (s *AdminService) Refund(ctx context.Context, tenantID, intentID string) (*PaymentIntent, error) {
	intent, err := s.repo.GetIntent(ctx, tenantID, intentID)
	if err != nil {
		return nil, err
	}
	if intent.Status == StatusRefunded {
		return intent, nil
	}
	if intent.Status != StatusPaid {
		return nil, fmt.Errorf("%w: cannot refund payment in status %q", ErrInvalidStatus, intent.Status)
	}
	if s.provider == nil {
		return nil, fmt.Errorf("%w: provider not configured", ErrProviderUnavailable)
	}
	if !strings.EqualFold(intent.Provider, s.provider.Name()) {
		return nil, fmt.Errorf("%w: payment uses %q, active provider is %q", ErrProviderMismatch, intent.Provider, s.provider.Name())
	}
	refunder, ok := s.provider.(RefundProvider)
	if !ok {
		return nil, fmt.Errorf("%w: provider %q", ErrRefundUnsupported, s.provider.Name())
	}
	if intent.ProviderCheckoutID == "" {
		return nil, fmt.Errorf("%w: provider checkout id is missing", ErrProviderUnavailable)
	}

	idempotencyKey := "refund:" + intent.ID
	refundID, err := refunder.Refund(ctx, intent.ProviderCheckoutID, idempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("%w: refund payment %s: %v", ErrProviderUnavailable, intent.ID, err)
	}
	payload, err := json.Marshal(struct {
		Provider         string `json:"provider"`
		ProviderRefundID string `json:"provider_refund_id,omitempty"`
		IdempotencyKey   string `json:"idempotency_key"`
	}{
		Provider:         intent.Provider,
		ProviderRefundID: refundID,
		IdempotencyKey:   idempotencyKey,
	})
	if err != nil {
		return nil, fmt.Errorf("payments: encode refund event: %w", err)
	}
	now := s.now().UTC()
	event := &PaymentEvent{
		TenantID:  intent.TenantID,
		IntentID:  intent.ID,
		EventType: EventTypePaymentRefunded,
		Payload:   payload,
		CreatedAt: now,
	}

	updated, err := s.repo.MarkRefunded(ctx, tenantID, intent.ID, event)
	if errors.Is(err, ErrInvalidStatus) {
		// Параллельный повтор мог успешно зафиксировать возврат. Не делаем
		// нового вызова PSP и возвращаем уже актуальное состояние.
		current, getErr := s.repo.GetIntent(ctx, tenantID, intent.ID)
		if getErr == nil && current.Status == StatusRefunded {
			return current, nil
		}
	}
	return updated, err
}
