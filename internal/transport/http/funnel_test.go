package http

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/funnel"
)

type stubFunnel struct {
	accepted, dropped int
	err               error
	report            *funnel.FunnelReport
	gotSession        string
	gotConsent        int
	gotEvents         []funnel.Event
	gotFrom, gotTo    time.Time
}

func (s *stubFunnel) Ingest(_ context.Context, sessionID string, events []funnel.Event, consent int) (funnel.IngestResult, error) {
	s.gotSession, s.gotConsent, s.gotEvents = sessionID, consent, events
	if s.err != nil {
		return funnel.IngestResult{}, s.err
	}
	return funnel.IngestResult{Accepted: s.accepted, Dropped: s.dropped}, nil
}

func (s *stubFunnel) SetVisitorIdentity(bool) {}

func (s *stubFunnel) Report(_ context.Context, from, to time.Time) (*funnel.FunnelReport, error) {
	s.gotFrom, s.gotTo = from, to
	if s.err != nil {
		return nil, s.err
	}
	return s.report, nil
}

const okSession = "11111111-2222-3333-4444-555555555555"

// newVisitorHasherX — hasher без доверенных прокси (как в проде за nginx их
// список задаётся STAIR_TRUSTED_PROXIES; здесь — пусто, чтобы тесты не
// зависели от адресов). Тесты за прокси создают hasher явно.
func newVisitorHasherX(salt string) visitorHasher { return visitorHasher{key: []byte(salt)} }

func postEvents(t *testing.T, body string, ua string) *httptest.ResponseRecorder {
	t.Helper()
	svc := &stubFunnel{accepted: 2}
	h := handleIngestAnalytics(svc, newVisitorHasherX("salt"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/analytics:events", strings.NewReader(body))
	// Content-Type обязателен: decodeJSON отвергает тело не с application/json
	// (закрытый CSRF-вектор). Именно поэтому на фронте уход событий уходит
	// sendBeacon с Blob типа application/json — заголовки sendBeacon ставить
	// не умеет.
	req.Header.Set("Content-Type", "application/json")
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func TestIngestAnalytics_Accepted(t *testing.T) {
	body := `{"session_id":"` + okSession + `","consent_version":1,"events":[
		{"name":"funnel.constructor_open","path":"/#constructor"},
		{"name":"blocker.field_invalid","props":{"reason":"widthMM"}}]}`
	rr := postEvents(t, body, "Mozilla/5.0 (X11) Chrome/120.0 Safari/537.36")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("код %d, хочу 202: %s", rr.Code, rr.Body.String())
	}
	var res struct{ Accepted, Dropped int }
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("ответ не JSON: %v", err)
	}
	if res.Accepted != 2 {
		t.Errorf("accepted=%d, хочу 2", res.Accepted)
	}
}

func TestIngestAnalytics_BadJSON(t *testing.T) {
	rr := postEvents(t, `{"session_id":`, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("код %d, хочу 400", rr.Code)
	}
}

func TestIngestAnalytics_TooManyEvents(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"session_id":"` + okSession + `","consent_version":1,"events":[`)
	for i := 0; i < funnel.MaxBatchEvents+1; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"name":"page.view"}`)
	}
	sb.WriteString(`]}`)
	rr := postEvents(t, sb.String(), "")
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("код %d, хочу 413: %s", rr.Code, rr.Body.String())
	}
}

// Отсутствие согласия — НЕ ошибка витрины: она просто не должна была слать.
// Отвечаем 202, чтобы клиент не повторял и не засорял лог ошибками.
func TestIngestAnalytics_NoConsentAnswers202(t *testing.T) {
	svc := &stubFunnel{err: funnel.ErrConsentRequired}
	h := handleIngestAnalytics(svc, newVisitorHasherX(""))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/analytics:events",
		strings.NewReader(`{"session_id":"`+okSession+`","consent_version":0,"events":[{"name":"page.view"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("код %d, хочу 202", rr.Code)
	}
}

func TestIngestAnalytics_InvalidSessionAnswers400(t *testing.T) {
	svc := &stubFunnel{err: funnel.ErrInvalidSession}
	h := handleIngestAnalytics(svc, newVisitorHasherX(""))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/analytics:events",
		strings.NewReader(`{"session_id":"мусор","consent_version":1,"events":[{"name":"page.view"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("код %d, хочу 400", rr.Code)
	}
}

// visitor и browser считает СЕРВЕР. Значения из тела игнорируются, иначе
// клиент подмешивал бы в отчёт «где у людей ломается» то, что сам написал.
func TestIngestAnalytics_ServerDerivesVisitorAndBrowser(t *testing.T) {
	svc := &stubFunnel{accepted: 1}
	h := handleIngestAnalytics(svc, newVisitorHasherX("secret-salt"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/analytics:events",
		strings.NewReader(`{"session_id":"`+okSession+`","consent_version":1,"events":[
			{"name":"page.view","browser":"что угодно","visitor":"подделка"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0) Firefox/121.0")
	req.RemoteAddr = "203.0.113.9:5555"
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	if len(svc.gotEvents) != 1 {
		t.Fatalf("событий %d, хочу 1", len(svc.gotEvents))
	}
	got := svc.gotEvents[0]
	if got.Browser != "firefox" {
		t.Errorf("browser=%q, хочу firefox (выведен из User-Agent)", got.Browser)
	}
	if got.Visitor == "" || got.Visitor == "подделка" {
		t.Errorf("visitor=%q — значение из тела подставилось или хеш пустой", got.Visitor)
	}
	if len(got.Visitor) != 16 {
		t.Errorf("visitor длиной %d, хочу 16 hex-символов", len(got.Visitor))
	}
}

// Без соли посетитель не идентифицируется вовсе — это штатный режим.
func TestIngestAnalytics_NoSaltNoVisitor(t *testing.T) {
	svc := &stubFunnel{accepted: 1}
	h := handleIngestAnalytics(svc, newVisitorHasherX(""))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/analytics:events",
		strings.NewReader(`{"session_id":"`+okSession+`","consent_version":1,"events":[{"name":"page.view"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.9:5555"
	rr := httptest.NewRecorder()
	h(rr, req)
	if svc.gotEvents[0].Visitor != "" {
		t.Errorf("visitor=%q без соли, хочу пустую строку", svc.gotEvents[0].Visitor)
	}
}

func TestVisitorHasher_StableAndDiffers(t *testing.T) {
	h := newVisitorHasherX("k")
	a := h.hash("1.1.1.1")
	b := h.hash("1.1.1.1")
	c := h.hash("2.2.2.2")
	if a != b {
		t.Error("хеш нестабилен для одинаковых входных данных")
	}
	if a == c {
		t.Error("разные IP дали одинаковый visitor")
	}
	if strings.Contains(a, "1.1.1.1") {
		t.Error("IP попал в visitor открытым текстом")
	}
}

func TestBrowserFamily(t *testing.T) {
	cases := map[string]string{
		"Mozilla/5.0 Chrome/120.0 Safari/537.36": "chrome",
		"Mozilla/5.0 Firefox/121.0":              "firefox",
		"Mozilla/5.0 Safari/605.1":               "safari",
		"Mozilla/5.0 Chrome/120 Safari Edg/120":  "edge",
		"Mozilla/5.0 Chrome OPR/106":             "opera",
		"Mozilla/5.0 YaBrowser/23":               "yandex",
		"":                                       "unknown",
		"какой-то краулер":                       "other",
	}
	for ua, want := range cases {
		if got := browserFamily(ua); got != want {
			t.Errorf("browserFamily(%q)=%q, хочу %q", ua, got, want)
		}
	}
}

func TestFunnelAnalytics_Forbidden(t *testing.T) {
	svc := &stubFunnel{}
	h := handleFunnelAnalytics(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/funnel", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("код %d, хочу 403 без правы", rr.Code)
	}
}

func TestFunnelAnalytics_Report(t *testing.T) {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	svc := &stubFunnel{report: &funnel.FunnelReport{
		From:     from,
		To:       from.AddDate(0, 0, 30),
		Sessions: 100,
		Events:   500,
		Visitors: 40, VisitorIdentityEnabled: true,
		Steps:    []funnel.FunnelStep{{Name: funnel.EventFunnelOpen, Sessions: 60, Visitors: 24, Share: 0.6, StepShare: 0.667}},
		Blockers: []funnel.Blocker{{Event: funnel.EventBlockerField, Reason: "widthMM", Count: 25}},
		Abandons: []funnel.AbandonPoint{{LastEvent: funnel.EventFunnelOpen, Sessions: 20, AvgSec: 42}},
	}}
	h := handleFunnelAnalytics(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/funnel?from=2026-09-01&to=2026-10-01", nil)
	req = req.WithContext(context.WithValue(req.Context(), authCtxKey{},
		&auth.User{Role: auth.RoleAdmin}))
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d, хочу 200: %s", rr.Code, rr.Body.String())
	}
	var d funnelReportDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &d); err != nil {
		t.Fatalf("ответ не JSON: %v", err)
	}
	if d.Sessions != 100 || len(d.Steps) != 1 || len(d.Blockers) != 1 || len(d.Abandons) != 1 {
		t.Errorf("ответ неполный: %+v", d)
	}
	if d.Visitors != 40 || !d.VisitorIdentityEnabled {
		t.Errorf("уникальные посетители не доехали: %+v", d)
	}
	if d.Steps[0].Visitors != 24 {
		t.Errorf("посетителей в шаге %d, хочу 24", d.Steps[0].Visitors)
	}
	// Доля ухода считается от ВСЕХ уходов (они же визиты), а не от
	// посетителей: единственная точка оттока = 100% уходов. Считать от
	// посетителей нельзя — на стенде выходило 200% (6 уходов у 3 людей).
	if d.Abandons[0].Share != 1 {
		t.Errorf("share точки оттока %v, хочу 1 (единственная точка оттока)", d.Abandons[0].Share)
	}
	if d.From != "2026-09-01" || d.To != "2026-10-01" {
		t.Errorf("окно %s..%s", d.From, d.To)
	}
}

func TestFunnelAnalytics_InvalidRange(t *testing.T) {
	svc := &stubFunnel{err: funnel.ErrInvalidRange}
	h := handleFunnelAnalytics(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/funnel?from=2026-10-01&to=2026-09-01", nil)
	req = req.WithContext(context.WithValue(req.Context(), authCtxKey{},
		&auth.User{Role: auth.RoleAdmin}))
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("код %d, хочу 422", rr.Code)
	}
}

// За обратным прокси RemoteAddr — это адрес прокси, а не посетителя. Без
// доверия прокси и чтения XFF все посетители сайта схлопнулись бы в одного, а
// ещё хуже — nginx ходит к api то по IPv4, то по IPv6, и ОДИН человек давал
// бы два разных visitor (на стенде уникальных «посетителей» получилось
// больше, чем визитов). Этот тест фиксирует именно вычисление адреса.
func TestVisitorIP_BehindTrustedProxy(t *testing.T) {
	// Список должен покрывать и IPv4, и IPv6 docker-сети: Docker отдаёт A и
	// AAAA для сервиса, и при IPv4-only списке подключение по IPv6 считалось
	// недоверенным (это и ломало visitor — см. TestVisitorHash_StableBehindProxy).
	_, docker4, err := net.ParseCIDR("172.16.0.0/12")
	if err != nil {
		t.Fatalf("ParseCIDR v4: %v", err)
	}
	_, docker6, err := net.ParseCIDR("fc00::/7")
	if err != nil {
		t.Fatalf("ParseCIDR v6: %v", err)
	}
	trusted := []*net.IPNet{docker4, docker6}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/analytics:events", nil)
	req.RemoteAddr = "172.18.0.5:41234"                          // пир — прокси
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 172.18.0.5") // реальный клиент + прокси
	if got := visitorIP(req, trusted); got != "203.0.113.7" {
		t.Errorf("visitorIP = %q, хочу 203.0.113.7 (адрес посетителя из XFF)", got)
	}

	// Пир НЕ в списке доверенных: XFF игнорируется, иначе его подделка
	// позволяла бы нарисовать себе любого посетителя.
	spoof := httptest.NewRequest(http.MethodPost, "/api/v1/public/analytics:events", nil)
	spoof.RemoteAddr = "198.51.100.9:5555"
	spoof.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := visitorIP(spoof, trusted); got != "198.51.100.9" {
		t.Errorf("visitorIP = %q, хочу RemoteAddr (XFF от недоверенного пира)", got)
	}

	// Без списка доверенных — тоже RemoteAddr.
	if got := visitorIP(req, nil); got != "172.18.0.5" {
		t.Errorf("visitorIP без trusted = %q, хочу RemoteAddr", got)
	}
}

// Один и тот же посетитель за прокси обязан давать один visitor: соль та же,
// адрес из XFF стабильный, UA не меняется.
func TestVisitorHash_StableBehindProxy(t *testing.T) {
	_, docker4, _ := net.ParseCIDR("172.16.0.0/12")
	_, docker6, _ := net.ParseCIDR("fc00::/7")
	trusted := []*net.IPNet{docker4, docker6}
	h := visitorHasher{key: []byte("salt"), trusted: trusted}

	mk := func(proxyAddr, xff, ua string) string {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/public/analytics:events", nil)
		r.RemoteAddr = proxyAddr
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		r.Header.Set("User-Agent", ua)
		return h.hash(visitorIP(r, trusted))
	}
	// nginx может подключиться к api по IPv4 или IPv6 — адрес ПИРА меняется,
	// адрес посетителя нет.
	a := mk("172.18.0.5:1111", "203.0.113.7, 172.18.0.5", "UA/1")
	b := mk("[fd00::5]:2222", "203.0.113.7, 172.18.0.5", "UA/1")
	if a == "" || a != b {
		t.Errorf("один посетитель дал разные visitor: %q и %q", a, b)
	}
}

// User-Agent НЕ входит в хеш. На стенде он разошёлся между fetch и beacon
// одного визита (подмена Playwright применялась не ко всем запросам), и с UA
// в хеше один человек давал двух «уникальных посетителей» — их становилось
// больше, чем визитов. UA подделывается одним заголовком, как признак
// личности он не значит ничего; семейство браузера живёт отдельно.
func TestVisitorHash_IgnoresUserAgent(t *testing.T) {
	h := newVisitorHasherX("salt")
	if h.hash("1.1.1.1") != h.hash("1.1.1.1") {
		t.Fatal("один IP обязан давать один visitor")
	}
	if h.hash("1.1.1.1") == h.hash("2.2.2.2") {
		t.Error("разные IP дали одинаковый visitor")
	}
}

// Две точки оттока — доли делят ВСЕ уходы между собой и в сумме дают 100%.
// Знаменатель — число уходов, а не визитов и не посетителей.
func TestFunnelAnalytics_AbandonSharesSumToOne(t *testing.T) {
	svc := &stubFunnel{report: &funnel.FunnelReport{
		From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
		// 10 визитов и всего 3 уникальных посетителя: если делить уходы на
		// посетителей, получится 667% и выше.
		Visitors: 3, Sessions: 10, Events: 50, VisitorIdentityEnabled: true,
		Abandons: []funnel.AbandonPoint{
			{LastEvent: "blocker.field_invalid", Sessions: 6, AvgSec: 40},
			{LastEvent: "funnel.constructor_open", Sessions: 4, AvgSec: 15},
		},
	}}
	d := toFunnelReportDTO(svc.report)
	total := 0.0
	for _, a := range d.Abandons {
		total += a.Share
	}
	if total < 0.999 || total > 1.001 {
		t.Errorf("доли уходов в сумме %v, хочу 1 (6 уходов + 4 ухода из 10)", total)
	}
	if d.Abandons[0].Share != 0.6 {
		t.Errorf("доля первой точки %v, хочу 0.6", d.Abandons[0].Share)
	}
}
