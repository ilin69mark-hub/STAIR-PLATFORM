// Package payments реализует прикладной сервис платежей (EDR-0027, Phase E
// E6): создание платёжных интентов через внешний PSP (mock-эмулятор) и
// обработку входящего webhook с подтверждением оплаты. Мета-уровень зависит
// от Provider/WebhookVerifier/Repository, но не от HTTP/БД (ADR-0006).
package payments

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Status — статус платёжного интента (EDR-0027 §3.3).
type Status string

const (
	StatusPending  Status = "pending"
	StatusPaid     Status = "paid"
	StatusFailed   Status = "failed"
	StatusRefunded Status = "refunded"
)

// terminalStatusList — терминальные статусы интента (EDR-0027 §3.3):
// событие PSP не может перевести интент из терминального статуса в другой
// статус — разрешён только повтор того же статуса (идемпотентная повторная
// доставка одного и того же события). Защита от регрессии: поздний
// checkout.session.expired после completed перетирал paid в failed, и учёт
// расходился с деньгами (S-141 №4, CWE-20).
var terminalStatusList = []Status{StatusPaid, StatusFailed, StatusRefunded}

// TerminalStatuses возвращает терминальные статусы интента. Используется
// инфраструктурой (SQL-guard перехода) и тестами.
func TerminalStatuses() []Status {
	return terminalStatusList
}

// isTerminalStatus сообщает, является ли статус интента терминальным.
func isTerminalStatus(s Status) bool {
	for _, t := range terminalStatusList {
		if s == t {
			return true
		}
	}
	return false
}

// EventType — типы входящих событий webhook (EDR-0027 §3.4).
const (
	EventTypePaymentSucceeded = "payment.succeeded"
	EventTypePaymentFailed    = "payment.failed"
)

// PaymentIntent — платёжный интент: намерение оплатить проект.
// CheckoutURL — URL редиректа на PSP, возвращается клиенту при создании
// и НЕ сохраняется в БД (только факт+идентификатор сессии провайдера).
type PaymentIntent struct {
	ID        string
	TenantID  string
	ProjectID string
	// TierID — код купленной услуги из серверного каталога (S-150). Пусто у
	// платежей за проект инженерной панели.
	TierID             string
	UserID             string
	AmountMinor        int64
	Currency           string
	Status             Status
	Provider           string
	ProviderCheckoutID string
	CheckoutURL        string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	PaidAt             *time.Time
}

// PaymentEvent — событие из журнала webhook (аудит).
type PaymentEvent struct {
	ID        string
	TenantID  string
	IntentID  string
	EventType string
	Payload   json.RawMessage
	CreatedAt time.Time
}

// ErrNotFound — интент не найден.
var ErrNotFound = errors.New("payments: not found")

// ErrInvalid — некорректные входные данные (сумма, валюта, событие).
var ErrInvalid = errors.New("payments: invalid input")

// ErrInvalidSignature — невалидная подпись входящего webhook.
var ErrInvalidSignature = errors.New("payments: invalid webhook signature")

// ErrForbidden — недостаточно прав.
var ErrForbidden = errors.New("payments: forbidden")

// Provider — абстракция платёжного провайдера (внешний PSP или mock).
type Provider interface {
	// Name — идентификатор провайдера (напр. "mock"); хранится в интенте
	// и используется в подписи/идемпотентности.
	Name() string
	// CreateCheckout создаёт checkout-сессию на сумму amount_minor в
	// валюте currency и возвращает идентификатор сессии и URL редиректа
	// покупателя на PSP.
	CreateCheckout(ctx context.Context, amountMinor int64, currency string) (checkoutID, checkoutURL string, err error)
}

// WebhookVerifier — проверка HMAC-SHA256 подписи входящего webhook
// (EDR-0023 §3.2; Sign/Verify-трубка та же, что у интеграций).
type WebhookVerifier interface {
	// Verify возвращает ошибку при невалидных заголовках/подписи/окне.
	Verify(secret string, tsUnix, sigValue string, body []byte, maxAge time.Duration) error
}

// Repository — хранилище интентов и событий (инверсия зависимостей).
type Repository interface {
	// CreateIntent сохраняет новый интент; ID присваивается внутри.
	CreateIntent(ctx context.Context, p *PaymentIntent) error
	// GetIntent возвращает интент по ID внутри tenant.
	GetIntent(ctx context.Context, tenantID, id string) (*PaymentIntent, error)
	// GetIntentByProviderCheckout возвращает интент по
	// (provider, provider_checkout_id) для обработки webhook.
	// ErrNotFound — нет такого.
	GetIntentByProviderCheckout(ctx context.Context, provider, checkoutID string) (*PaymentIntent, error)
	// ListByProject возвращает интенты проекта в порядке создания.
	ListByProject(ctx context.Context, tenantID, projectID string) ([]*PaymentIntent, error)
	// ListByUser возвращает платежи пользователя, новые первыми (кабинет, этап 4).
	ListByUser(ctx context.Context, tenantID, userID string) ([]*PaymentIntent, error)
	// UpdateStatus обновляет статус и paid_at (nil — не менять).
	UpdateStatus(ctx context.Context, tenantID, id string, s Status, paidAt *time.Time) error
	// AppendEvent пишет событие в журнал payment_events.
	AppendEvent(ctx context.Context, e *PaymentEvent) error
}

// EventApplier — опциональная транзакционная операция «применить событие»
// (DB-002, forensic 2026-09-24): смена статуса интента и запись в журнал
// payment_events должны быть атомарны. Без неё два вызова UpdateStatus +
// AppendEvent оставляли оплаченный интент без записи в финансовом аудите,
// а повтор webhook (reconcile) уже не журналировал событие.
// Реализует инфраструктурный PaymentRepository; сервис использует её при
// наличии и деградирует к прежней последовательности для прочих реализаций.
type EventApplier interface {
	ApplyVerifiedEventTx(ctx context.Context, tenantID, intentID string, s Status, paidAt *time.Time, e *PaymentEvent) error
}
