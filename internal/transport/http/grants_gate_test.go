// Сквозная проверка гейта по метрикам (EDR-0041).
//
// Смысл теста: раньше права проверялись по роли, и «выдать кому-то одну
// метрику» было невозможно проверить — потому что проверять было нечего.
// Теперь у каждой закрытой ручки есть ровно одна метрика и уровень, и
// таблица ниже перечисляет их. Проверяется обе стороны:
//   - без метрики — 403 (метрика не выдана, блока не должно быть);
//   - с ровно этой метрикой — не 403 (выдали — ручка открылась).
//
// Вторая сторона важнее первой: именно она отличает точечную выдачу от
// роли. Если бы 403 не наступал с «не той» метрикой, модель тихо
// превратилась бы в старую проверку роли.
package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/auth"
)

// metricCase — закрытая ручка, её метрика и требующийся уровень.
type metricCase struct {
	name       string
	method     string
	path       string
	capability auth.Capability
	level      auth.Level
	body       string
}

// adminMetricCases — все закрытые метриками admin-ручки. Новая закрытая
// ручка обязана появиться здесь: иначе она не будет проверена ни на
// отказ, ни на открытие.
var adminMetricCases = []metricCase{
	{"список сотрудников", http.MethodGet, "/api/v1/admin/users", auth.CapStaffView, auth.LevelRead, ""},
	{"права сотрудника", http.MethodGet, "/api/v1/admin/users/u-2/grants", auth.CapStaffView, auth.LevelRead, ""},

	{"настройки безопасности", http.MethodGet, "/api/v1/admin/settings", auth.CapSecurityPolicy, auth.LevelRead, ""},
	{"политика безопасности", http.MethodPut, "/api/v1/admin/settings", auth.CapSecurityPolicy, auth.LevelWrite,
		`{"min_password_length":8,"require_number":false,"require_upper":false,"session_ttl_seconds":86400,"login_rate_limit_per_min":10}`},
	{"выгрузка данных", http.MethodGet, "/api/v1/admin/export?scope=users&format=json", auth.CapDataExport, auth.LevelAction, ""},

	{"список ключей", http.MethodGet, "/api/v1/admin/api-keys", auth.CapSecurityKeys, auth.LevelRead, ""},
	{"выдача ключа", http.MethodPost, "/api/v1/admin/api-keys", auth.CapSecurityKeysIssue, auth.LevelAction, `{"name":"k","scopes":[]}`},

	{"активность", http.MethodGet, "/api/v1/admin/analytics/usage", auth.CapTrafficUsage, auth.LevelRead, ""},
	{"сводка по проектам", http.MethodGet, "/api/v1/admin/analytics/projects", auth.CapProjectReport, auth.LevelRead, ""},
	{"производство", http.MethodGet, "/api/v1/admin/analytics/manufacturing", auth.CapProductionReport, auth.LevelRead, ""},
	{"себестоимость", http.MethodGet, "/api/v1/admin/analytics/cost", auth.CapFinanceCost, auth.LevelRead, ""},
	{"воронка", http.MethodGet, "/api/v1/admin/analytics/funnel", auth.CapTrafficFunnel, auth.LevelRead, ""},

	{"заказы", http.MethodGet, "/api/v1/admin/orders", auth.CapFinanceSales, auth.LevelRead, ""},
	{"платежи", http.MethodGet, "/api/v1/admin/payments", auth.CapFinancePayments, auth.LevelRead, ""},
	{"возврат платежа", http.MethodPost, "/api/v1/admin/payments/p1/refund", auth.CapFinanceRefunds, auth.LevelAction, ""},

	{"отзывы", http.MethodGet, "/api/v1/admin/testimonials", auth.CapSiteReviews, auth.LevelRead, ""},
	{"создание отзыва", http.MethodPost, "/api/v1/admin/testimonials", auth.CapSiteReviews, auth.LevelWrite,
		`{"author":"a","text":"t","rating":5}`},

	{"журнал аудита", http.MethodGet, "/api/v1/audit", auth.CapAuditView, auth.LevelRead, ""},

	{"настройки магазина", http.MethodGet, "/api/v1/admin/store/settings", auth.CapSiteContacts, auth.LevelRead, ""},
	{"прайс материалов", http.MethodGet, "/api/v1/admin/store/prices", auth.CapPriceMaterials, auth.LevelRead, ""},
	{"смена цены", http.MethodPut, "/api/v1/admin/store/prices", auth.CapPriceMaterials, auth.LevelWrite,
		`{"code":"WOOD-OAK","price_per_kg_rub":10}`},

	{"подключения интеграций", http.MethodGet, "/api/v1/integrations/endpoints", auth.CapIntegrationEndpoints, auth.LevelRead, ""},
	{"создание подключения", http.MethodPost, "/api/v1/integrations/endpoints", auth.CapIntegrationEndpointsIssue, auth.LevelAction,
		`{"name":"erp","url":"https://erp.example.com"}`},
}

// grantAuth — сотрудник, которому выдали ровно то, что лежит в set.
// GrantsInitialized поднят: действует набор, а не шаблон роли.
type grantAuth struct {
	testAuth
	set *auth.GrantSet
}

func (f grantAuth) Authenticate(_ context.Context, token string) (*auth.User, string, error) {
	if token == "" {
		return nil, "", auth.ErrSessionExpired
	}
	return &auth.User{
		ID: "u-1", TenantID: "t-1", Email: "employee@example.com",
		Role: auth.RoleUser, GrantsInitialized: true,
	}, "", nil
}

func (f grantAuth) EffectiveGrants(_ context.Context, _ *auth.User) (*auth.GrantSet, error) {
	if f.set == nil {
		return auth.NewGrantSet(), nil
	}
	return f.set, nil
}

// grantRouter собирает роутер со всеми сервисами, чьи маршруты закрыты
// метриками: без них ветки в NewRouter просто не поднимаются и проверять
// было бы нечего (404 вместо 403).
func grantRouter(set *auth.GrantSet) http.Handler {
	cfg := DefaultConfig()
	cfg.Analytics = &fakeAnalyticsService{}
	cfg.Orders = &fakeOrderService{}
	cfg.Testimonials = &fakeTestimonialService{}
	cfg.Integrations = &fakeIntegrationService{}
	cfg.PaymentAdmin = &fakePaymentAdminService{}
	cfg.Funnel = &stubFunnel{}
	cfg.Store = &c003Store{}
	return NewRouter(nil, nil, grantAuth{set: set}, cfg, &fakeAuditService{})
}

// grantRequest собирает запрос аутентифицированной сессией: без cookie
// requireAuth отвечает 401, и проверять было бы не то.
func grantRequest(method, path, body string) *http.Request {
	// Именно authedRequest: он ставит session-cookie. Без него requireAuth
	// отвечает 401, и проверка гейта измеряла бы не то.
	return authedRequest(method, path, body)
}

// TestMetricGateDeniesWithoutGrant — без единой выдачи все закрытые ручки
// отвечают 403. Это и есть «не выдал — не отображается» на стороне сервера.
func TestMetricGateDeniesWithoutGrant(t *testing.T) {
	t.Parallel()
	router := grantRouter(nil)

	for _, tc := range adminMetricCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, grantRequest(tc.method, tc.path, tc.body))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s: без метрики ожидался 403, получено %d: %s",
					tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestMetricGateOpensOnlyThatMetric — выдали одну метрику: открылась ровно
// та ручка, а соседние остались закрыты.
func TestMetricGateOpensOnlyThatMetric(t *testing.T) {
	t.Parallel()

	for _, tc := range adminMetricCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			router := grantRouter(auth.NewGrantSet(
				auth.Grant{Capability: tc.capability, Level: auth.GrantedLevel(tc.level)},
			))

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, grantRequest(tc.method, tc.path, tc.body))
			if rec.Code == http.StatusForbidden {
				t.Fatalf("%s %s: с метрикой %s ручка осталась закрыта",
					tc.method, tc.path, tc.capability)
			}

			// Соседние ручки обязаны остаться закрытыми: это отличает
			// точечную выдачу от «выдал — и всё сразу».
			for _, other := range adminMetricCases {
				if other.capability == tc.capability {
					continue
				}
				otherRec := httptest.NewRecorder()
				router.ServeHTTP(otherRec, grantRequest(other.method, other.path, other.body))
				if otherRec.Code != http.StatusForbidden {
					t.Fatalf("%s %s открылась без своей метрики: выдавали %s",
						other.method, other.path, tc.capability)
				}
			}
		})
	}
}

// TestWriteDoesNotOpenReadGate — «менять» не должно открывать ручку, которая
// требует только чтения, если метрика выдана на уровне read: это проверка
// обратной стороны, что уровень значим.
func TestReadGrantDoesNotOpenWriteGate(t *testing.T) {
	t.Parallel()

	// «Заказы» на чтение открыты, смена статуса — нет.
	router := grantRouter(auth.NewGrantSet(
		auth.Grant{Capability: auth.CapFinanceSales, Level: auth.GrantedRead},
	))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, grantRequest(http.MethodGet, "/api/v1/admin/orders", ""))
	if rec.Code == http.StatusForbidden {
		t.Fatal("чтение заказов должно быть открыто")
	}

	patchRec := httptest.NewRecorder()
	router.ServeHTTP(patchRec, grantRequest(http.MethodPatch, "/api/v1/admin/orders/o1/status", `{"status":"confirmed"}`))
	if patchRec.Code != http.StatusForbidden {
		t.Fatalf("смена статуса открылась при выданном только чтении: %d", patchRec.Code)
	}
}

// ownerAuth — владелец компании: единственный, кто может выдавать метрики.
type ownerAuth struct{ testAuth }

func (ownerAuth) Authenticate(_ context.Context, token string) (*auth.User, string, error) {
	if token == "" {
		return nil, "", auth.ErrSessionExpired
	}
	return &auth.User{ID: "u-owner", TenantID: "t-1", Email: "owner@x.ru", Role: auth.RoleOwner}, "", nil
}

func (f ownerAuth) EffectiveGrants(_ context.Context, u *auth.User) (*auth.GrantSet, error) {
	return auth.OwnerCapabilities(), nil
}

// TestOnlyOwnerMayGrant — право выдавать проверяется по роли, а не по
// метрике: иначе цепочку «выдал → выдал» нельзя остановить.
func TestOnlyOwnerMayGrant(t *testing.T) {
	t.Parallel()
	body := `{"grants":[{"capability":"finance.sales","level":"read"}]}`

	ownerRouter := NewRouter(nil, nil, ownerAuth{}, DefaultConfig())
	rec := httptest.NewRecorder()
	ownerRouter.ServeHTTP(rec, grantRequest(http.MethodPut, "/api/v1/admin/users/u-2/grants", body))
	if rec.Code == http.StatusForbidden {
		t.Fatalf("владелец не может выдать права: %d %s", rec.Code, rec.Body.String())
	}

	// Сотрудник с любой метрикой — не может.
	employeeRouter := grantRouter(auth.NewGrantSet(
		auth.Grant{Capability: auth.CapStaffView, Level: auth.GrantedRead},
		auth.Grant{Capability: auth.CapStaffCreate, Level: auth.GrantedAction},
	))
	rec = httptest.NewRecorder()
	employeeRouter.ServeHTTP(rec, grantRequest(http.MethodPut, "/api/v1/admin/users/u-3/grants", body))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("сотрудник смог выдать права: %d %s", rec.Code, rec.Body.String())
	}
}

// TestGrantErrorsMapToReadableAnswers — домен отдаёт типизированную ошибку,
// маршрут превращает её в понятный ответ. Само правило «нельзя себе» живёт
// в auth.GrantsService и проверяется в grants_service_test.go; здесь важно,
// что ошибка не теряется по дороге и не превращается в 500.
func TestGrantErrorsMapToReadableAnswers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{"себе", auth.ErrForbidden, http.StatusForbidden, "себе"},
		{"потолок", auth.ErrGrantCeiling, http.StatusForbidden, "нет у вас"},
		{"владельца нельзя урезать", auth.ErrGrantOwnerProtected, http.StatusForbidden, "владельца"},
		{"нет сотрудника", auth.ErrNotFound, http.StatusNotFound, "не найден"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			router := NewRouter(nil, nil, erringGrantsAuth{err: tc.err}, DefaultConfig())
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, grantRequest(http.MethodPut, "/api/v1/admin/users/u-2/grants",
				`{"grants":[{"capability":"finance.sales","level":"read"}]}`))
			if rec.Code != tc.code {
				t.Fatalf("ожидался %d, получено %d: %s", tc.code, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.msg) {
				t.Errorf("текст ответа не объясняет причину: %s", rec.Body.String())
			}
		})
	}
}

// erringGrantsAuth — владелец, чья выдача всегда отказывает заданной ошибкой.
type erringGrantsAuth struct {
	ownerAuth
	err error
}

func (f erringGrantsAuth) SetUserGrants(_ context.Context, _ string, _ *auth.User, _ string, _ *auth.GrantSet) error {
	return f.err
}
