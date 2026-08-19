// Package order реализует прикладной слой розничных заказов (клиентского
// сайта, Store). Заказ — лид розничного клиента: конфигурация лестницы +
// контакты + предварительная цена из публичного расчёта. Application зависит
// от порта Repository (BE-0005) и оркестрирует сохранение/списки/статусы;
// не зависит от транспорта и БД (инверсия зависимостей, DOM-0008).
package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status — статус заказа-лида.
type Status string

const (
	StatusNew        Status = "new"         // поступивший заказ
	StatusPriced     Status = "priced"      // цена расcчитана менеджером
	StatusConfirmed  Status = "confirmed"   // клиент подтвердил заказ
	StatusInProgress Status = "in_progress" // в производстве
	StatusCompleted  Status = "completed"   // выполнен
	StatusCancelled  Status = "cancelled"   // отменён
)

// Kind — тип заказа-лида на клиентском сайте.
type Kind string

const (
	// KindOrder — заказ из конструктора (auth, config+price из расчёта).
	KindOrder Kind = "order"
	// KindConsultation — консультация: запрос обратной связи без расчёта
	// (анонимная, без снимка цены).
	KindConsultation Kind = "consultation"
)

// IsValid возвращает true, если тип заказа допустим.
func (k Kind) IsValid() bool {
	return k == KindOrder || k == KindConsultation
}

// AllStatuses — допустимые статусы для валидации переходов.
var AllStatuses = []Status{StatusNew, StatusPriced, StatusConfirmed, StatusInProgress, StatusCompleted, StatusCancelled}

// IsValid возвращает true, если статус допустим.
func (s Status) IsValid() bool {
	for _, st := range AllStatuses {
		if st == s {
			return true
		}
	}
	return false
}

// Contact — контакты розничного клиента из формы заказа.
type Contact struct {
	Name  string
	Email string
	Phone string
}

// Order — розничный заказ (лид). ConfigJSON — сериализованная входная
// конфигурация (calculateRequest); PriceJSON — снимок цены публичного
// расчёта (pricing результат). ProjectID — опциональная связь с проектом,
// созданным менеджером в инженерной панели. Kind — тип заказа: обычный
// (order) или консультация (consultation, без user_id и price).
type Order struct {
	ID         string
	TenantID   string
	UserID     string
	Kind       Kind
	Status     Status
	Contact    Contact
	ConfigJSON json.RawMessage
	PriceJSON  json.RawMessage
	ProjectID  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	// ErrNotFound — заказ не найден (или вне tenant).
	ErrNotFound = errors.New("order: not found")
	// ErrInvalid — некорректные входные данные.
	ErrInvalid = errors.New("order: invalid input")
	// ErrForbidden — недостаточно прав (чужой заказ клиенту).
	ErrForbidden = errors.New("order: forbidden")
)

// Repository — порт доступа к данным заказов.
type Repository interface {
	// Create сохраняет новый заказ; ID присваивается внутри.
	Create(ctx context.Context, o *Order) error
	// Get возвращает заказ по ID в tenant.
	Get(ctx context.Context, tenantID, id string) (*Order, error)
	// ListByUser возвращает заказы пользователя (личный кабинет).
	ListByUser(ctx context.Context, tenantID, userID string) ([]*Order, error)
	// ListAll возвращает все заказы tenant (админ-раздел).
	ListAll(ctx context.Context, tenantID string) ([]*Order, error)
	// UpdateStatus меняет статус заказа (tenant-скоуп).
	UpdateStatus(ctx context.Context, tenantID, id string, s Status) error
}

// Service — прикладной сервис заказов.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService создаёт сервис заказов.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// contactValid проверяет обязательные поля контакта.
func contactValid(c Contact) error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("%w: name required", ErrInvalid)
	}
	if strings.TrimSpace(c.Email) == "" {
		return fmt.Errorf("%w: email required", ErrInvalid)
	}
	return nil
}

// Create создаёт заказ-лид от розничного клиента. userID — инициатор
// (зарегистрированный пользователь). config/price — JSON-снимки расчёта.
// Возвращает созданный заказ.
func (s *Service) Create(ctx context.Context, tenantID, userID string, c Contact, config, price json.RawMessage) (*Order, error) {
	if tenantID == "" || userID == "" {
		return nil, fmt.Errorf("%w: authenticated user required", ErrInvalid)
	}
	if err := contactValid(c); err != nil {
		return nil, err
	}
	if len(config) == 0 || len(price) == 0 {
		return nil, fmt.Errorf("%w: config and price snapshots required", ErrInvalid)
	}
	now := s.now().UTC()
	o := &Order{
		TenantID:   tenantID,
		UserID:     userID,
		Kind:       KindOrder,
		Status:     StatusNew,
		Contact:    c,
		ConfigJSON: append(json.RawMessage(nil), config...),
		PriceJSON:  append(json.RawMessage(nil), price...),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.Create(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// CreateConsultation создаёт анонимную консультацию (без расчёта цены):
// контакты + вопрос, зафиксированный в config (type: consultation).
// userID пустой; price отсутствует.
func (s *Service) CreateConsultation(ctx context.Context, tenantID string, c Contact, question string) (*Order, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenant required", ErrInvalid)
	}
	if err := contactValid(c); err != nil {
		return nil, err
	}
	if strings.TrimSpace(question) == "" {
		return nil, fmt.Errorf("%w: question required", ErrInvalid)
	}
	cfg, err := json.Marshal(map[string]string{"type": "consultation", "question": strings.TrimSpace(question)})
	if err != nil {
		return nil, fmt.Errorf("%w: marshal consultation config", ErrInvalid)
	}
	now := s.now().UTC()
	o := &Order{
		TenantID:   tenantID,
		Kind:       KindConsultation,
		Status:     StatusNew,
		Contact:    c,
		ConfigJSON: cfg,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.Create(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// ListByUser возвращает заказы пользователя (личный кабинет).
func (s *Service) ListByUser(ctx context.Context, tenantID, userID string) ([]*Order, error) {
	return s.repo.ListByUser(ctx, tenantID, userID)
}

// ListAll возвращает все заказы tenant (админ-раздел).
func (s *Service) ListAll(ctx context.Context, tenantID string) ([]*Order, error) {
	return s.repo.ListAll(ctx, tenantID)
}

// UpdateStatus меняет статус заказа. Статус должен быть допустимым.
func (s *Service) UpdateStatus(ctx context.Context, tenantID, id string, st Status) error {
	if !st.IsValid() {
		return fmt.Errorf("%w: invalid status %q", ErrInvalid, st)
	}
	return s.repo.UpdateStatus(ctx, tenantID, id, st)
}

// Get возвращает заказ по ID в tenant.
func (s *Service) Get(ctx context.Context, tenantID, id string) (*Order, error) {
	return s.repo.Get(ctx, tenantID, id)
}
