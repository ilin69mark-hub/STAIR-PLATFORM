package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/order"
)

// OrderService — прикладной интерфейс заказов для транспортного слоя.
type OrderService interface {
	Create(ctx context.Context, tenantID, userID string, c order.Contact, config, price json.RawMessage) (*order.Order, error)
	CreateConsultation(ctx context.Context, tenantID string, c order.Contact, question string) (*order.Order, error)
	ListByUser(ctx context.Context, tenantID, userID string) ([]*order.Order, error)
	ListAll(ctx context.Context, tenantID string) ([]*order.Order, error)
	UpdateStatus(ctx context.Context, tenantID, id string, s order.Status) error
	Get(ctx context.Context, tenantID, id string) (*order.Order, error)
}

// orderContactDTO — контакты клиента из формы заказа.
type orderContactDTO struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone,omitempty"`
}

// orderDTO — представление заказа в API.
type orderDTO struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Status     string          `json:"status"`
	Contact    orderContactDTO `json:"contact"`
	ConfigJSON json.RawMessage `json:"config"`
	PriceJSON  json.RawMessage `json:"price"`
	ProjectID  string          `json:"project_id,omitempty"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

func toOrderDTO(o *order.Order) orderDTO {
	out := orderDTO{
		ID:         o.ID,
		Kind:       string(o.Kind),
		Status:     string(o.Status),
		Contact:    orderContactDTO{Name: o.Contact.Name, Email: o.Contact.Email, Phone: o.Contact.Phone},
		ConfigJSON: o.ConfigJSON,
		ProjectID:  o.ProjectID,
		CreatedAt:  o.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  o.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if len(o.PriceJSON) > 0 {
		out.PriceJSON = o.PriceJSON
	}
	return out
}

type createOrderRequest struct {
	Contact orderContactDTO `json:"contact"`
	Config  json.RawMessage `json:"config"`
	Price   json.RawMessage `json:"price"`
}

// validRequest проверяет обязательные поля создания заказа.
func (r *createOrderRequest) valid() error {
	if strings.TrimSpace(r.Contact.Name) == "" {
		return fmt.Errorf("%w: contact.name required", order.ErrInvalid)
	}
	if strings.TrimSpace(r.Contact.Email) == "" {
		return fmt.Errorf("%w: contact.email required", order.ErrInvalid)
	}
	if len(r.Config) == 0 || string(r.Config) == "null" {
		return fmt.Errorf("%w: config required", order.ErrInvalid)
	}
	if len(r.Price) == 0 || string(r.Price) == "null" {
		return fmt.Errorf("%w: price required", order.ErrInvalid)
	}
	return nil
}

// handleCreateOrder — POST /api/v1/orders (auth+CSRF).
// Клиент отправляет конфигурацию + снимок цены из публичного расчёта.
// 201 — заказ создан; 400 — битый JSON; 422 — невалидный вход; 401 — нет входа.
func handleCreateOrder(svc OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := userID(r.Context())
		if uid == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
			return
		}
		var req createOrderRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		if err := req.valid(); err != nil {
			writeInputError(w, "invalid_input", err)
			return
		}
		o, err := svc.Create(r.Context(), tenantID(r.Context()), uid,
			order.Contact{Name: req.Contact.Name, Email: req.Contact.Email, Phone: req.Contact.Phone},
			req.Config, req.Price)
		if err != nil {
			if errors.Is(err, order.ErrInvalid) {
				writeInputError(w, "invalid_input", err)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusCreated, toOrderDTO(o))
	}
}

// handleListMyOrders — GET /api/v1/orders (auth).
// Мои заказы (личный кабинет).
func handleListMyOrders(svc OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := userID(r.Context())
		if uid == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
			return
		}
		orders, err := svc.ListByUser(r.Context(), tenantID(r.Context()), uid)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		out := make([]orderDTO, 0, len(orders))
		for _, o := range orders {
			out = append(out, toOrderDTO(o))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// handleAdminListOrders — GET /api/v1/admin/orders (auth+admin).
// Все заказы tenant для менеджера.
func handleAdminListOrders(svc OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionOrdersList) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора")
			return
		}
		orders, err := svc.ListAll(r.Context(), tenantID(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		out := make([]orderDTO, 0, len(orders))
		for _, o := range orders {
			out = append(out, toOrderDTO(o))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type updateOrderStatusRequest struct {
	Status string `json:"status"`
}

// handleAdminUpdateOrderStatus — PATCH /api/v1/admin/orders/{id}/status
// (auth+admin). Смена статуса заказа менеджером.
// 200 — заказ обновлён; 403 — нет права; 404 — заказ не найден; 422 — невалидный статус.
func handleAdminUpdateOrderStatus(svc OrderService, auditSvc AuditService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionOrdersManage) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора")
			return
		}
		id := r.PathValue("id")
		var req updateOrderStatusRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		st := order.Status(strings.TrimSpace(req.Status))
		if !st.IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_status", fmt.Sprintf("Неизвестный статус %q", req.Status))
			return
		}
		if err := svc.UpdateStatus(r.Context(), tenantID(r.Context()), id, st); err != nil {
			if errors.Is(err, order.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "Заказ не найден")
				return
			}
			if errors.Is(err, order.ErrInvalid) {
				writeInputError(w, "invalid_status", err)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		// Audit: order status changed
		if auditSvc != nil {
			_ = auditSvc.Record(r.Context(), &audit.Event{
				ActorID:    userID(r.Context()),
				TenantID:   tenantID(r.Context()),
				Action:     audit.ActionOrderStatusChanged,
				ResourceID: id,
				Result:     audit.ResultOK,
				Detail:     fmt.Sprintf("status=%s", req.Status),
			})
		}
		o, err := svc.Get(r.Context(), tenantID(r.Context()), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusOK, toOrderDTO(o))
	}
}

type createConsultationRequest struct {
	Contact  orderContactDTO `json:"contact"`
	Question string          `json:"question"`
}

// valid проверяет обязательные поля консультации.
func (r *createConsultationRequest) valid() error {
	if strings.TrimSpace(r.Contact.Name) == "" {
		return fmt.Errorf("%w: contact.name required", order.ErrInvalid)
	}
	if strings.TrimSpace(r.Contact.Email) == "" {
		return fmt.Errorf("%w: contact.email required", order.ErrInvalid)
	}
	if strings.TrimSpace(r.Question) == "" {
		return fmt.Errorf("%w: question required", order.ErrInvalid)
	}
	return nil
}

// handleCreateConsultation — POST /api/v1/public/orders (rate-limited).
// Анонимная консультация с клиентского сайта: контакты + вопрос.
// Привязывается к дефолтному tenant (как регистрация). Заказ-лид создаётся
// без пользователя и снимка цены (kind=consultation).
//
//	201 — консультация принята; 400 — битый JSON; 422 — невалидный вход;
//	429 — превышен rate-limit; 500 — нет дефолтного tenant.
func handleCreateConsultation(svc OrderService, authSvc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createConsultationRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		if err := req.valid(); err != nil {
			writeInputError(w, "invalid_input", err)
			return
		}
		tenant, err := authSvc.DefaultTenant(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		o, err := svc.CreateConsultation(r.Context(), tenant.ID,
			order.Contact{Name: req.Contact.Name, Email: req.Contact.Email, Phone: req.Contact.Phone},
			req.Question)
		if err != nil {
			if errors.Is(err, order.ErrInvalid) {
				writeInputError(w, "invalid_input", err)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusCreated, toOrderDTO(o))
	}
}
