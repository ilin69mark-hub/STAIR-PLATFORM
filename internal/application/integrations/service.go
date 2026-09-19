package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"stairplatform/internal/infrastructure/queue"
)

// SecretCrypter — защита webhook-секретов at rest (S1-2). Реализация —
// infrastructure/secrets.Box; при отсутствии криптера секрет хранится
// открыто (legacy-поведение, dev-режим).
type SecretCrypter interface {
	Seal(plain string) (string, error)
	Open(sealed string) (string, error)
}

// Service — прикладной сервис интеграций (EDR-0023 §3.4). Оркеструет
// Repository и TaskQueue: регистрирует эндпоинты и ставит события доставки
// в очередь, откуда их выполняет воркер (cmd/worker).
type Service struct {
	repo    Repository
	queue   queue.JobQueue
	now     func() time.Time
	crypter SecretCrypter
}

// NewService создаёт сервис интеграций.
func NewService(repo Repository, q queue.JobQueue) *Service {
	return &Service{repo: repo, queue: q, now: time.Now}
}

// WithSecretCrypter подключает шифрование секретов at rest (S1-2).
func (s *Service) WithSecretCrypter(c SecretCrypter) *Service {
	s.crypter = c
	return s
}

// RegisterEndpoint регистрирует webhook-эндпоинт tenant'а. Валидирует kind,
// URL (https вне localhost — проверка в infrastructure/integrations на
// отправке; здесь базовые проверки) и непустой секрет.
func (s *Service) RegisterEndpoint(ctx context.Context, tenantID, name, kindStr, endpointURL, secret string) (*Endpoint, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenant required", ErrInvalid)
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name required", ErrInvalid)
	}
	kind := Kind(kindStr)
	if !validKinds[kind] {
		return nil, fmt.Errorf("%w: unsupported kind %q", ErrInvalid, kindStr)
	}
	u, err := url.Parse(endpointURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%w: invalid url %q", ErrInvalid, endpointURL)
	}
	if strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("%w: secret required", ErrInvalid)
	}
	secretEnc := secret
	if s.crypter != nil {
		sealed, err := s.crypter.Seal(secret)
		if err != nil {
			return nil, fmt.Errorf("%w: seal secret: %v", ErrInvalid, err)
		}
		secretEnc = sealed
	}
	e := &Endpoint{
		TenantID:  tenantID,
		Name:      strings.TrimSpace(name),
		Kind:      kind,
		URL:       endpointURL,
		SecretEnc: secretEnc,
		CreatedAt: s.now().UTC(),
		UpdatedAt: s.now().UTC(),
	}
	if err := s.repo.CreateEndpoint(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// ListEndpoints возвращает эндпоинты tenant.
func (s *Service) ListEndpoints(ctx context.Context, tenantID string) ([]*Endpoint, error) {
	return s.repo.ListEndpoints(ctx, tenantID)
}

// GetEndpoint возвращает эндпоинт по ID.
func (s *Service) GetEndpoint(ctx context.Context, tenantID, id string) (*Endpoint, error) {
	return s.repo.GetEndpoint(ctx, tenantID, id)
}

// DeleteEndpoint удаляет эндпоинт (каскадно события доставки).
func (s *Service) DeleteEndpoint(ctx context.Context, tenantID, id string) error {
	return s.repo.DeleteEndpoint(ctx, tenantID, id)
}

// SendQuote ставит задание erp.quote_send для проекта (EDR-0023 §3.4):
// создаёт событие доставки (status=pending) и enqueue задания в TaskQueue.
// payload — канонический документ quote (Snapshot проекта).
func (s *Service) SendQuote(ctx context.Context, tenantID, projectID string, payload []byte) (*Delivery, error) {
	return s.enqueue(ctx, tenantID, projectID, KindERP, EventTypeQuoteSend, payload)
}

// SyncProject ставит задание crm.project_sync для проекта (EDR-0024 §3.2):
// создаёт событие доставки (status=pending) и enqueue задания в TaskQueue.
// payload — канонический документ проекта (EDR-0024 §3.3).
func (s *Service) SyncProject(ctx context.Context, tenantID, projectID string, payload []byte) (*Delivery, error) {
	return s.enqueue(ctx, tenantID, projectID, KindCRM, EventTypeProjectSync, payload)
}

// SendManufacturingOrder ставит задание mes.order_send для проекта
// (EDR-0025 §3.2): создаёт событие доставки (status=pending) и enqueue
// задания в TaskQueue. payload — канонический документ производственного
// заказа (EDR-0025 §3.3).
func (s *Service) SendManufacturingOrder(ctx context.Context, tenantID, projectID string, payload []byte) (*Delivery, error) {
	return s.enqueue(ctx, tenantID, projectID, KindMES, EventTypeOrderSend, payload)
}

// enqueue — общая постановка события доставки в очередь (EDR-0024 §3.2):
// находит единственный активный эндпоинт нужного kind, создаёт событие
// (pending) и ставит задание {event_id, endpoint_id, tenant_id}.
func (s *Service) enqueue(ctx context.Context, tenantID, projectID string, kind Kind, eventType string, payload []byte) (*Delivery, error) {
	if s.queue == nil {
		return nil, fmt.Errorf("integrations: queue not configured")
	}
	endpoint, err := s.repo.FindEndpointByKind(ctx, tenantID, kind)
	if err != nil {
		return nil, err
	}

	d := &Delivery{
		TenantID:   tenantID,
		EndpointID: endpoint.ID,
		ProjectID:  projectID,
		EventType:  eventType,
		Payload:    append(json.RawMessage(nil), payload...),
		Status:     StatusPending,
		CreatedAt:  s.now().UTC(),
	}
	if err := s.repo.CreateDelivery(ctx, d); err != nil {
		return nil, err
	}

	jobPayload, err := json.Marshal(struct {
		EventID    string `json:"event_id"`
		EndpointID string `json:"endpoint_id"`
		TenantID   string `json:"tenant_id"`
	}{
		EventID:    d.ID,
		EndpointID: endpoint.ID,
		TenantID:   tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("integrations: marshal job: %w", err)
	}
	job, err := queue.NewJob(eventType, json.RawMessage(jobPayload))
	if err != nil {
		return nil, err
	}
	if err := s.queue.Enqueue(ctx, job); err != nil {
		return nil, fmt.Errorf("integrations: enqueue %s: %w", eventType, err)
	}
	return d, nil
}

// SecretOf возвращает plaintext-секрет эндпоинта для доставки webhook (S1-2).
// Без криптера (legacy/dev) — секрет как есть.
func (s *Service) SecretOf(ep *Endpoint) (string, error) {
	if s.crypter == nil {
		return ep.SecretEnc, nil
	}
	return s.crypter.Open(ep.SecretEnc)
}

// MarkDelivered фиксирует успешную доставку (воркер).
func (s *Service) MarkDelivered(ctx context.Context, tenantID, id string) error {
	deliveredAt := s.now().UTC()
	return s.repo.UpdateDeliveryStatus(ctx, tenantID, id, StatusDelivered, 0, "", &deliveredAt)
}

// MarkFailed фиксирует неуспех (воркер, перед ретраем/DLQ).
func (s *Service) MarkFailed(ctx context.Context, tenantID, id string, attempts int, lastErr string) error {
	s2 := StatusFailed
	if attempts >= queue.DefaultMaxAttempts {
		s2 = StatusDLQ
	}
	return s.repo.UpdateDeliveryStatus(ctx, tenantID, id, s2, attempts, lastErr, nil)
}
