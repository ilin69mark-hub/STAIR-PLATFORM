package sentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	sentrysdk "github.com/getsentry/sentry-go"
)

// recordingTransport — тестовый транспорт: вместо сети собирает события
// (SendEvent) в память. Позволяет проверять весь pipeline (capture → scope →
// BeforeSend → transport) НЕ трогая глобальный хаб и без сети.
type recordingTransport struct {
	mu     sync.Mutex
	events []*sentrysdk.Event
}

var _ sentrysdk.Transport = (*recordingTransport)(nil)

func (t *recordingTransport) Flush(_ time.Duration) bool              { return true }
func (t *recordingTransport) FlushWithContext(_ context.Context) bool { return true }
func (t *recordingTransport) Configure(_ sentrysdk.ClientOptions)     {}
func (t *recordingTransport) Close()                                  {}
func (t *recordingTransport) SendEvent(event *sentrysdk.Event) {
	if event == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (t *recordingTransport) all() []*sentrysdk.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*sentrysdk.Event, len(t.events))
	copy(out, t.events)
	return out
}

// newTestClient создаёт клиент с recordingTransport и нашим BeforeSend,
// не трогая глобальный хаб.
func newTestClient(t *testing.T) (*sentrysdk.Client, *recordingTransport) {
	t.Helper()
	tr := &recordingTransport{}
	client, err := sentrysdk.NewClient(sentrysdk.ClientOptions{
		Dsn:        "https://public@example.com/1",
		BeforeSend: scrubEvent,
		Transport:  tr,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client, tr
}

// TestScrubEvent — табличные кейсы скраббера (BeforeSend как чистая функция).
func TestScrubEvent(t *testing.T) {
	tests := []struct {
		name  string
		mut   func(e *sentrysdk.Event)
		check func(t *testing.T, e *sentrysdk.Event)
	}{
		{
			name: "email hashed to 12 hex, deterministic",
			mut: func(e *sentrysdk.Event) {
				e.User = sentrysdk.User{Email: "user@example.com"}
			},
			check: func(t *testing.T, e *sentrysdk.Event) {
				got := e.User.Email
				if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(got) {
					t.Fatalf("email = %q, want 12 hex chars", got)
				}
				if want := hashEmail("user@example.com"); got != want {
					t.Fatalf("email = %q, want %q", got, want)
				}
			},
		},
		{
			name: "email normalized to lowercase before hash",
			mut: func(e *sentrysdk.Event) {
				e.User = sentrysdk.User{Email: "  User@Example.COM "}
			},
			check: func(t *testing.T, e *sentrysdk.Event) {
				if got, want := e.User.Email, hashEmail("user@example.com"); got != want {
					t.Fatalf("email = %q, want %q (lowercase+trim)", got, want)
				}
				if got := e.User.Email; len(got) != 12 {
					t.Fatalf("email len = %d, want 12", len(got))
				}
			},
		},
		{
			name: "ip dropped",
			mut: func(e *sentrysdk.Event) {
				e.User = sentrysdk.User{IPAddress: "203.0.113.7"}
				e.Request = &sentrysdk.Request{
					Env: map[string]string{"REMOTE_ADDR": "203.0.113.7", "REMOTE_PORT": "54321"},
				}
			},
			check: func(t *testing.T, e *sentrysdk.Event) {
				if e.User.IPAddress != "" {
					t.Errorf("user.ip_address = %q, want empty", e.User.IPAddress)
				}
				if e.Request != nil {
					if v, ok := e.Request.Env["REMOTE_ADDR"]; ok {
						t.Errorf("env REMOTE_ADDR survived: %q", v)
					}
					if v, ok := e.Request.Env["REMOTE_PORT"]; ok {
						t.Errorf("env REMOTE_PORT survived: %q", v)
					}
				}
			},
		},
		{
			name: "username and name dropped, id kept",
			mut: func(e *sentrysdk.Event) {
				e.User = sentrysdk.User{ID: "uuid-123", Username: "alice", Name: "Alice Smith"}
			},
			check: func(t *testing.T, e *sentrysdk.Event) {
				if e.User.Username != "" || e.User.Name != "" {
					t.Errorf("username/name survived: %q %q", e.User.Username, e.User.Name)
				}
				if e.User.ID != "uuid-123" {
					t.Errorf("user.id = %q, want kept", e.User.ID)
				}
			},
		},
		{
			name: "authorization header dropped, safe headers kept",
			mut: func(e *sentrysdk.Event) {
				e.Request = &sentrysdk.Request{
					Headers: map[string]string{
						"Authorization":     "Bearer sekrit",
						"Cookie":            "session=abc",
						"X-CSRF-Token":      "TKN",
						"X-Stair-Signature": "SIG",
						"X-Forwarded-For":   "10.0.0.1",
						"Host":              "api.example.com",
						"User-Agent":        "curl/8",
					},
				}
			},
			check: func(t *testing.T, e *sentrysdk.Event) {
				h := e.Request.Headers
				for _, drop := range []string{"Authorization", "Cookie", "X-CSRF-Token", "X-Stair-Signature", "X-Forwarded-For"} {
					if v, ok := h[drop]; ok {
						t.Errorf("header %s survived: %q", drop, v)
					}
				}
				if h["Host"] != "api.example.com" || h["User-Agent"] != "curl/8" {
					t.Errorf("safe headers changed: %#v", h)
				}
			},
		},
		{
			name: "request body and query secret masked",
			mut: func(e *sentrysdk.Event) {
				e.Request = &sentrysdk.Request{
					Data:        `{"password":"hunter2","email":"a@b.c"}`,
					QueryString: "token=abc123&page=2",
					// S-143: scope.SetRequest дублирует куки строкой
					// "session=SECRET; csrf=..." — она маскируется формой.
					Cookies: "session=SECRET; csrf=TKN",
				}
			},
			check: func(t *testing.T, e *sentrysdk.Event) {
				if strings.Contains(e.Request.Data, "hunter2") {
					t.Errorf("password leaked in data: %q", e.Request.Data)
				}
				if !strings.Contains(e.Request.Data, "***") {
					t.Errorf("data not masked: %q", e.Request.Data)
				}
				if strings.Contains(e.Request.QueryString, "abc123") {
					t.Errorf("token leaked in query: %q", e.Request.QueryString)
				}
				for _, leak := range []string{"SECRET", "TKN"} {
					if strings.Contains(e.Request.Cookies, leak) {
						t.Errorf("%s leaked in cookies: %q", leak, e.Request.Cookies)
					}
				}
				if !strings.Contains(e.Request.Cookies, "***") {
					t.Errorf("cookies not masked: %q", e.Request.Cookies)
				}
			},
		},
		{
			name: "message, extra and contexts scrubbed via JSON round-trip",
			mut: func(e *sentrysdk.Event) {
				e.Message = "login failed: password=hunter2"
				e.Tags = map[string]string{"client_secret": "s3cr3t", "path": "/api"}
				e.Contexts = map[string]sentrysdk.Context{
					"auth": {"token": "xyz", "ok": true},
				}
			},
			check: func(t *testing.T, e *sentrysdk.Event) {
				if strings.Contains(e.Message, "hunter2") {
					t.Errorf("password leaked in message: %q", e.Message)
				}
				if e.Tags["client_secret"] != "***" {
					t.Errorf("client_secret tag not masked: %q", e.Tags["client_secret"])
				}
				ctx, ok := e.Contexts["auth"]
				if !ok {
					t.Fatalf("contexts[auth] missing: %#v", e.Contexts)
				}
				if ctx["token"] != "***" {
					t.Errorf("context token not masked: %#v", ctx["token"])
				}
				if ctx["ok"] != true {
					t.Errorf("non-string context value changed: %#v", ctx["ok"])
				}
			},
		},
		{
			name:  "nil event dropped",
			mut:   func(e *sentrysdk.Event) {},
			check: func(t *testing.T, e *sentrysdk.Event) {},
		},
		{
			name: "emails hashed everywhere, no double-hash, no leak",
			mut: func(e *sentrysdk.Event) {
				e.User = sentrysdk.User{ID: "uuid-9", Email: "alice@example.com"}
				e.Contexts = map[string]sentrysdk.Context{
					"request_meta": {"receiver_email": "bob@example.com"},
					"auth":         {"email": "dave@example.com", "ok": true},
				}
				e.Tags = map[string]string{"user_email": "carol@example.com"}
			},
			check: func(t *testing.T, e *sentrysdk.Event) {
				if got := e.User.Email; got != hashEmail("alice@example.com") {
					t.Errorf("user.email = %q, want hash (не двойной хеш) of alice@example.com", got)
				}
				for _, got := range []string{
					e.Contexts["request_meta"]["receiver_email"].(string),
					e.Tags["user_email"],
					e.Contexts["auth"]["email"].(string),
				} {
					if len(got) != 12 || !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(got) {
						t.Errorf("email not hashed: %q", got)
					}
				}
				if e.Contexts["auth"]["ok"] != true {
					t.Errorf("non-string context value changed: %#v", e.Contexts["auth"]["ok"])
				}
				// в сериализованном виде не должно быть ни одного сырого адреса
				raw, err := json.Marshal(e)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				for _, addr := range []string{
					"alice@example.com", "bob@example.com", "carol@example.com", "dave@example.com",
				} {
					if strings.Contains(string(raw), addr) {
						t.Errorf("email %s leaked into serialized event", addr)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := &sentrysdk.Event{Message: "test"}
			if tt.name == "nil event dropped" {
				if got := scrubEvent(nil, nil); got != nil {
					t.Fatalf("scrubEvent(nil) = %#v, want nil", got)
				}
				return
			}
			tt.mut(ev)
			got := scrubEvent(ev, nil)
			if got == nil {
				t.Fatalf("scrubEvent returned nil for non-nil event")
			}
			tt.check(t, got)
		})
	}
}

// TestScrubPipeline — полный pipeline: capture → scope → BeforeSend →
// transport. Убеждаемся, что до транспорта доходит уже скрабленный event,
// а нечто с секретами наружу не уходит.
func TestScrubPipeline(t *testing.T) {
	client, tr := newTestClient(t)
	hub := sentrysdk.NewHub(client, sentrysdk.NewScope())

	hub.WithScope(func(scope *sentrysdk.Scope) {
		scope.SetUser(sentrysdk.User{
			ID:        "uuid-42",
			Email:     "alice@example.com",
			IPAddress: "203.0.113.9",
			Username:  "alice",
		})
		scope.SetTag("path", "/api/v1/quotes")
		scope.SetContext("request_meta", sentrysdk.Context{"receiver_email": "zed@example.com"})
		hub.CaptureException(errSensitive("boom: password=hunter2"))
	})
	client.Flush(2 * time.Second)

	events := tr.all()
	if len(events) != 1 {
		t.Fatalf("transport got %d events, want 1", len(events))
	}
	ev := events[0]

	if ev.User.Email == "alice@example.com" || !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(ev.User.Email) {
		t.Errorf("email not hashed: %q", ev.User.Email)
	}
	if ev.User.IPAddress != "" || ev.User.Username != "" {
		t.Errorf("PII user fields survived: %#v", ev.User)
	}
	if ev.User.ID != "uuid-42" {
		t.Errorf("user.id lost: %q", ev.User.ID)
	}
	if got, ok := ev.Contexts["request_meta"]["receiver_email"].(string); !ok || got != hashEmail("zed@example.com") {
		t.Errorf("extra email not hashed: %#v", ev.Contexts["request_meta"]["receiver_email"])
	}

	// событие сериализуется в JSON — секретов не должно быть вовсе.
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	for _, leak := range []string{"hunter2", "alice@example.com", "zed@example.com"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("%s leaked into serialized event", leak)
		}
	}
}

// errSensitive — error с секретом в сообщении (типичный кейс паники/ошибки).
type errSensitive string

func (e errSensitive) Error() string { return string(e) }

// TestCapturePanicScrubsSessionTokens — симуляция паники обработчика с
// валидными cookie/CSRF (аудит S-141 №2: раньше session=SECRET уходил в
// Sentry через scope.SetRequest). После CapturePanic → scrub пайплайна в
// сериализованном событии не должно быть ни SECRET, ни TKN, ни SIG.
func TestCapturePanicScrubsSessionTokens(t *testing.T) {
	client, tr := newTestClient(t)
	hub := sentrysdk.CurrentHub()
	prev := hub.Client()
	hub.BindClient(client)
	defer func() { hub.BindClient(prev) }()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stairs:calculate", nil)
	req.AddCookie(&http.Cookie{ //nolint:gosec // тестовая фикстура с Secure-атрибутами
		Name: "session", Value: "SECRET", Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: true,
	})
	req.AddCookie(&http.Cookie{ //nolint:gosec // тестовая фикстура с Secure-атрибутами
		Name: "csrf", Value: "TKN", Path: "/",
		HttpOnly: false, SameSite: http.SameSiteLaxMode, Secure: true,
	})
	req.Header.Set("X-CSRF-Token", "TKN")
	req.Header.Set("X-Stair-Signature", "SIG")

	CapturePanic("boom: impossible state", req, []byte("fake stack"))
	client.Flush(2 * time.Second)

	events := tr.all()
	if len(events) != 1 {
		t.Fatalf("transport got %d events, want 1", len(events))
	}
	raw, err := json.Marshal(events[0])
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	for _, leak := range []string{"SECRET", "TKN", "SIG"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("panic event leaked %s: %s", leak, string(raw))
		}
	}
}

func TestInitEmptyDSNNoop(t *testing.T) {
	// to be safe: если другой тест нечаянно инициализировал глобальный хаб,
	// этот тест должен остаться валидным — снимаем клиент.
	hub := sentrysdk.CurrentHub()
	client := hub.Client()

	shutdown, err := Init(Config{DSN: "", Environment: "test", ServiceName: "test"})
	if err != nil {
		t.Fatalf("Init(empty DSN) error: %v, want nil", err)
	}
	shutdown() // не паникует, ничего не шлёт

	if hub.Client() != client {
		t.Error("empty-DSN Init mutated the global hub")
	}

	// CapturePanic при выключенном SDK — безопасный no-op (с любым r, даже nil).
	CapturePanic("boom", nil, []byte("stack"))
	CapturePanic("boom", httptest.NewRequest(http.MethodGet, "/test", nil), []byte("stack"))
}
