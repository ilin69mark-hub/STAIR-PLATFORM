package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/order"
	"stairplatform/internal/application/stair"
)

// fakeOrderService — тестовая реализация OrderService.
type fakeOrderService struct {
	created  *order.Order
	list     []*order.Order
	all      []*order.Order
	updated  *string
	updateSt order.Status
	userID   string
	// Точечная инъекция ошибок по операциям.
	consultErr error
	listAllErr error
	updateErr  error
	getErr     error
	postGetErr error // ошибка Get после успешного UpdateStatus (ветка 500)
}

func (f *fakeOrderService) Create(_ context.Context, tenantID, userID string, c order.Contact, config, price json.RawMessage) (*order.Order, error) {
	f.userID = userID
	f.created = &order.Order{
		ID: "ord-1", TenantID: tenantID, UserID: userID, Kind: order.KindOrder, Status: order.StatusNew,
		Contact: c, ConfigJSON: config, PriceJSON: price,
	}
	return f.created, nil
}
func (f *fakeOrderService) CreateConsultation(_ context.Context, tenantID string, c order.Contact, question string) (*order.Order, error) {
	if f.consultErr != nil {
		return nil, f.consultErr
	}
	f.created = &order.Order{
		ID: "ord-c", TenantID: tenantID, Kind: order.KindConsultation, Status: order.StatusNew,
		Contact: c,
		ConfigJSON: json.RawMessage(`{"type":"consultation","question":"` +
			strings.ReplaceAll(question, `"`, `\"`) + `"}`),
	}
	return f.created, nil
}
func (f *fakeOrderService) ListByUser(_ context.Context, tenantID, userID string) ([]*order.Order, error) {
	return f.list, nil
}
func (f *fakeOrderService) ListAll(_ context.Context, tenantID string) ([]*order.Order, error) {
	if f.listAllErr != nil {
		return nil, f.listAllErr
	}
	return f.all, nil
}
func (f *fakeOrderService) UpdateStatus(_ context.Context, tenantID, id string, s order.Status) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = &id
	f.updateSt = s
	return nil
}
func (f *fakeOrderService) Get(_ context.Context, tenantID, id string) (*order.Order, error) {
	if f.postGetErr != nil {
		return nil, f.postGetErr
	}
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.created, nil
}

func ordersRouter(svc OrderService) http.Handler {
	cfg := DefaultConfig()
	cfg.Orders = svc
	return NewRouter(stair.NewService(), nil, testAuth{}, cfg)
}

func ordersAdminRouter(svc OrderService) http.Handler {
	cfg := DefaultConfig()
	cfg.Orders = svc
	return NewRouter(stair.NewService(), nil, adminAuth{}, cfg)
}

func TestCreateOrder(t *testing.T) {
	fake := &fakeOrderService{}
	fake.list = []*order.Order{}
	router := ordersRouter(fake)
	body := `{"contact":{"name":"Иван","email":"i@ex.ru","phone":"+7 900"},` +
		`"config":{"width_mm":900},"price":{"final_price":123}}`
	req := authedRequest(http.MethodPost, "/api/v1/orders", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var out orderDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if out.Status != "new" {
		t.Fatalf("status = %s, want new", out.Status)
	}
	if out.Contact.Name != "Иван" {
		t.Fatalf("contact name = %q, want Иван", out.Contact.Name)
	}
}

func TestCreateOrderMissingContact(t *testing.T) {
	fake := &fakeOrderService{}
	router := ordersRouter(fake)
	body := `{"contact":{},"config":{"width_mm":900},"price":{"final_price":123}}`
	req := authedRequest(http.MethodPost, "/api/v1/orders", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateOrderRequiresAuth(t *testing.T) {
	fake := &fakeOrderService{}
	router := ordersRouter(fake)
	body := `{"contact":{"name":"И","email":"i@ex.ru"},"config":{},"price":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCreateConsultationPublic(t *testing.T) {
	fake := &fakeOrderService{}
	router := ordersRouter(fake)
	body := `{"contact":{"name":"Мария","email":"m@ex.ru","phone":"+7 000 000-00-00"},` +
		`"question":"Сколько стоит лестница на 3 метра?"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json") // S-144: decodeJSON требует его
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var out orderDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if out.Kind != "consultation" {
		t.Fatalf("kind = %s, want consultation", out.Kind)
	}
	if out.Status != "new" {
		t.Fatalf("status = %s, want new", out.Status)
	}
	if out.Contact.Name != "Мария" {
		t.Fatalf("contact name = %q, want Мария", out.Contact.Name)
	}
}

func TestCreateConsultationMissingQuestion(t *testing.T) {
	fake := &fakeOrderService{}
	router := ordersRouter(fake)
	body := `{"contact":{"name":"М","email":"m@ex.ru"},"question":"   "}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json") // S-144: decodeJSON требует его
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateOrderInvalidJSON(t *testing.T) {
	fake := &fakeOrderService{}
	router := ordersRouter(fake)
	req := authedRequest(http.MethodPost, "/api/v1/orders", "{bad")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListMyOrders(t *testing.T) {
	fake := &fakeOrderService{}
	fake.list = []*order.Order{{ID: "ord-1", Status: order.StatusNew,
		Contact: order.Contact{Name: "И", Email: "i@ex.ru"}}}
	router := ordersRouter(fake)
	req := authedRequest(http.MethodGet, "/api/v1/orders", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out []orderDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if len(out) != 1 || out[0].ID != "ord-1" {
		t.Fatalf("unexpected list: %+v", out)
	}
}

func TestAdminListOrders(t *testing.T) {
	fake := &fakeOrderService{}
	fake.all = []*order.Order{{ID: "ord-9", Status: order.StatusConfirmed,
		Contact: order.Contact{Name: "А", Email: "a@ex.ru"}}}
	router := ordersAdminRouter(fake)
	req := authedRequest(http.MethodGet, "/api/v1/admin/orders", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out []orderDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if len(out) != 1 || out[0].ID != "ord-9" {
		t.Fatalf("unexpected list: %+v", out)
	}
}

func TestAdminUpdateOrderStatus(t *testing.T) {
	fake := &fakeOrderService{}
	fake.created = &order.Order{ID: "ord-1", Status: order.StatusPriced,
		Contact: order.Contact{Name: "И", Email: "i@ex.ru"}}
	router := ordersAdminRouter(fake)
	body := `{"status":"confirmed"}`
	req := authedRequest(http.MethodPatch, "/api/v1/admin/orders/ord-1/status", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.updated == nil || *fake.updated != "ord-1" {
		t.Fatalf("update not routed: %+v", fake.updated)
	}
	if fake.updateSt != order.StatusConfirmed {
		t.Fatalf("status = %s, want confirmed", fake.updateSt)
	}
}

func TestAdminUpdateOrderStatusInvalid(t *testing.T) {
	fake := &fakeOrderService{}
	router := ordersAdminRouter(fake)
	body := `{"status":"bogus"}`
	req := authedRequest(http.MethodPatch, "/api/v1/admin/orders/ord-1/status", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminOrdersRequireAdmin(t *testing.T) {
	fake := &fakeOrderService{}
	// Обычный пользователь (testAuth, role=user) не видит admin-список и
	// не меняет статусы заказов.
	userRouter := ordersRouter(fake)
	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/v1/admin/orders", ""},
		{http.MethodPatch, "/api/v1/admin/orders/ord-1/status", `{"status":"confirmed"}`},
	} {
		req := authedRequest(tc.method, tc.path, tc.body)
		rec := httptest.NewRecorder()
		userRouter.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s: expected 403, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func ordersAdminRouterWithAudit(svc OrderService, a AuditService) http.Handler {
	cfg := DefaultConfig()
	cfg.Orders = svc
	return NewRouter(stair.NewService(), nil, adminAuth{}, cfg, a)
}

func ordersPublicRouterWithTenant(svc OrderService, a AuthService) http.Handler {
	cfg := DefaultConfig()
	cfg.Orders = svc
	return NewRouter(stair.NewService(), nil, a, cfg)
}

func TestAdminListOrdersError(t *testing.T) {
	fake := &fakeOrderService{listAllErr: errors.New("db")}
	router := ordersAdminRouter(fake)
	req := authedRequest(http.MethodGet, "/api/v1/admin/orders", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateOrderStatusInvalidJSON(t *testing.T) {
	fake := &fakeOrderService{}
	router := ordersAdminRouter(fake)
	req := authedRequest(http.MethodPatch, "/api/v1/admin/orders/ord-1/status", "NOT-JSON")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateOrderStatusNotFound(t *testing.T) {
	fake := &fakeOrderService{updateErr: order.ErrNotFound}
	router := ordersAdminRouter(fake)
	body := `{"status":"confirmed"}`
	req := authedRequest(http.MethodPatch, "/api/v1/admin/orders/missing/status", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateOrderStatusServiceInvalid(t *testing.T) {
	fake := &fakeOrderService{updateErr: order.ErrInvalid}
	router := ordersAdminRouter(fake)
	body := `{"status":"confirmed"}`
	req := authedRequest(http.MethodPatch, "/api/v1/admin/orders/ord-1/status", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateOrderStatusServiceError(t *testing.T) {
	fake := &fakeOrderService{updateErr: errors.New("db")}
	router := ordersAdminRouter(fake)
	body := `{"status":"confirmed"}`
	req := authedRequest(http.MethodPatch, "/api/v1/admin/orders/ord-1/status", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateOrderStatusAudited(t *testing.T) {
	fake := &fakeOrderService{}
	fake.created = &order.Order{ID: "ord-1", Status: order.StatusConfirmed,
		Contact: order.Contact{Name: "И", Email: "i@ex.ru"}}
	router := ordersAdminRouterWithAudit(fake, &fakeAuditService{})
	body := `{"status":"confirmed"}`
	req := authedRequest(http.MethodPatch, "/api/v1/admin/orders/ord-1/status", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateOrderStatusGetError(t *testing.T) {
	// UpdateStatus успешен, но повторная выборка заказа падает (500).
	fake := &fakeOrderService{postGetErr: errors.New("db")}
	router := ordersAdminRouter(fake)
	body := `{"status":"confirmed"}`
	req := authedRequest(http.MethodPatch, "/api/v1/admin/orders/ord-1/status", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.updated == nil {
		t.Fatalf("UpdateStatus should have been called")
	}
}

func TestCreateConsultationInvalidJSON(t *testing.T) {
	fake := &fakeOrderService{}
	router := ordersRouter(fake)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/orders", strings.NewReader("NOT-JSON"))
	req.Header.Set("Content-Type", "application/json") // S-144: decodeJSON требует его
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateConsultationTenantError(t *testing.T) {
	fake := &fakeOrderService{}
	router := ordersPublicRouterWithTenant(fake, failingTenantAuth{})
	body := `{"contact":{"name":"М","email":"m@ex.ru"},"question":"Какой срок?"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json") // S-144: decodeJSON требует его
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateConsultationServiceError(t *testing.T) {
	fake := &fakeOrderService{consultErr: errors.New("db")}
	router := ordersRouter(fake)
	body := `{"contact":{"name":"М","email":"m@ex.ru"},"question":"Какой срок?"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json") // S-144: decodeJSON требует его
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}
