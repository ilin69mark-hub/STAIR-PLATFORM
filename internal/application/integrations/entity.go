// Package integrations реализует прикладной сервис интеграций (EDR-0023,
// Phase E E2): регистрация webhook-эндпоинтов и постановка событий
// доставки (ERP quote и др.) в TaskQueue (EDR-0020). Мета-уровень зависит
// от queue и Repository, но не от HTTP/БД (ADR-0006). Payload событий —
// канонические документы (напр. Snapshot проекта, EDR-0022 §3.1).
package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Kind — тип интеграционного эндпоинта (EDR-0023 §3.3).
type Kind string

const (
	KindERP     Kind = "erp"
	KindCRM     Kind = "crm"
	KindMES     Kind = "mes"
	KindPayment Kind = "payment"
)

// Валидные виды эндпоинтов.
var validKinds = map[Kind]bool{KindERP: true, KindCRM: true, KindMES: true, KindPayment: true}

// Event types (EDR-0023 §3.3).
const (
	EventTypeQuoteSend = "erp.quote_send"
)

// Status — статус события доставки (EDR-0023 §3.3).
type Status string

const (
	StatusPending   Status = "pending"
	StatusDelivered Status = "delivered"
	StatusFailed    Status = "failed"
	StatusDLQ       Status = "dlq"
)

// Endpoint — webhook-эндпоинт tenant'а.
type Endpoint struct {
	ID        string
	TenantID  string
	Name      string
	Kind      Kind
	URL       string
	SecretEnc string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Delivery — запись события доставки (статусной машины).
type Delivery struct {
	ID          string
	TenantID    string
	EndpointID  string
	ProjectID   string
	EventType   string
	Payload     json.RawMessage
	Status      Status
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	DeliveredAt *time.Time
}

// ErrNotFound — сущность не найдена.
var ErrNotFound = errors.New("integrations: not found")

// ErrInvalid — некорректные входные данные (URL, kind, пустой секрет).
var ErrInvalid = errors.New("integrations: invalid input")

// ErrForbidden — недостаточно прав (проверяется в транспорте и частично тут).
var ErrForbidden = errors.New("integrations: forbidden")

// ErrNoEndpoint — нет активного эндпоинта нужного kind.
var ErrNoEndpoint = errors.New("integrations: no endpoint")

// Repository — хранилище эндпоинтов и событий (инверсия зависимостей).
type Repository interface {
	// CreateEndpoint сохраняет новый эндпоинт; ID присваивается внутри.
	CreateEndpoint(ctx context.Context, e *Endpoint) error
	// ListEndpoints возвращает эндпоинты tenant в порядке создания.
	ListEndpoints(ctx context.Context, tenantID string) ([]*Endpoint, error)
	// GetEndpoint возвращает эндпоинт по ID внутри tenant.
	GetEndpoint(ctx context.Context, tenantID, id string) (*Endpoint, error)
	// DeleteEndpoint удаляет эндпоинт (каскадно события).
	DeleteEndpoint(ctx context.Context, tenantID, id string) error
	// FindEndpointByKind возвращает единственный активный эндпоинт kind
	// (для quote-send). ErrNoEndpoint — нет подходящего.
	FindEndpointByKind(ctx context.Context, tenantID string, kind Kind) (*Endpoint, error)

	// CreateDelivery создаёт событие доставки (status=pending).
	CreateDelivery(ctx context.Context, d *Delivery) error
	// UpdateDeliveryStatus обновляет status/attempts/last_error/delivered_at.
	UpdateDeliveryStatus(ctx context.Context, tenantID, id string, s Status, attempts int, lastErr string, deliveredAt *time.Time) error
	// GetDelivery возвращает событие по ID.
	GetDelivery(ctx context.Context, tenantID, id string) (*Delivery, error)
}
