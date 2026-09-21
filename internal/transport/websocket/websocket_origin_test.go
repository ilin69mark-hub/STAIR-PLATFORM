package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// testOriginRequest строит запрос с заданным Origin и Host.
func testOriginRequest(origin, host string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if host != "" {
		req.Host = host
	}
	return req
}

// TestOriginCheckerNonBrowserAllowed — отсутствие Origin разрешено
// (не-браузерные клиенты); браузеры всегда шлют Origin на WS-handshake.
func TestOriginCheckerNonBrowserAllowed(t *testing.T) {
	check := OriginChecker(nil)
	if !check(testOriginRequest("", "example.com")) {
		t.Fatal("request without Origin must be allowed (non-browser client)")
	}
	if !check(testOriginRequest("", "")) {
		t.Fatal("request without Origin and Host must be allowed")
	}
}

// TestOriginCheckerEmptyListSameOrigin — пустой список из конфига:
// безопасный дефолт — только same-origin (S-112).
func TestOriginCheckerEmptyListSameOrigin(t *testing.T) {
	check := OriginChecker(nil)

	if !check(testOriginRequest("http://app.example.com", "app.example.com")) {
		t.Fatal("same-origin must be allowed with empty allowlist")
	}
	if check(testOriginRequest("http://app.example.com", "evil.example.com")) {
		t.Fatal("different host must be rejected with empty allowlist (same-origin default)")
	}
	// Порт учитывается (Host браузера включает порт).
	if check(testOriginRequest("http://app.example.com:8080", "app.example.com")) {
		t.Fatal("origin port mismatch must be rejected")
	}
	// Непарсибельный Origin — отклоняется.
	if check(testOriginRequest("://bad-origin", "app.example.com")) {
		t.Fatal("malformed origin must be rejected")
	}
}

// TestOriginCheckerExactMatch — exact-матч из конфига.
func TestOriginCheckerExactMatch(t *testing.T) {
	check := OriginChecker([]string{"http://localhost:3000", "https://stairs.example.com"})

	if !check(testOriginRequest("http://localhost:3000", "stairs.example.com")) {
		t.Fatal("exact allowed origin must be accepted")
	}
	if !check(testOriginRequest("https://stairs.example.com", "stairs.example.com")) {
		t.Fatal("exact allowed https origin must be accepted")
	}
	if check(testOriginRequest("http://localhost:5174", "stairs.example.com")) {
		t.Fatal("non-listed origin must be rejected")
	}
	if check(testOriginRequest("http://evil.com", "stairs.example.com")) {
		t.Fatal("evil origin must be rejected")
	}
}

// TestOriginCheckerWildcard — "*" и "*.domain" (симметрично CORS-политике).
func TestOriginCheckerWildcard(t *testing.T) {
	if !OriginChecker([]string{"*"})(testOriginRequest("http://anything.example.com", "stairs.example.com")) {
		t.Fatal("'*' must allow any origin")
	}
	check := OriginChecker([]string{"*.example.com"})
	if !check(testOriginRequest("http://store.example.com", "stairs.example.com")) {
		t.Fatal("wildcard *.example.com must match store.example.com")
	}
	if !check(testOriginRequest("http://admin.example.com", "stairs.example.com")) {
		t.Fatal("wildcard *.example.com must match admin.example.com")
	}
	if check(testOriginRequest("http://evil.com", "stairs.example.com")) {
		t.Fatal("wildcard *.example.com must not match evil.com")
	}
	// Суффикс не должен матчиться по произвольному домену.
	if check(testOriginRequest("http://notexample.com", "stairs.example.com")) {
		t.Fatal("wildcard *.example.com must not match notexample.com")
	}
}

// TestHandleWebSocketSameOriginDefault — интеграция: nil-список = same-origin
// дефолт; Origin сервера проходит upgrade, чужой Origin — отклонение handshake.
func TestHandleWebSocketSameOriginDefault(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, w, r, "", nil)
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "/ws"
	host := strings.TrimPrefix(srv.URL, "http://")

	// Same-origin: Origin host == Host запроса (127.0.0.1:port) → успех.
	hdr := http.Header{}
	hdr.Set("Origin", "http://"+host)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		t.Fatalf("same-origin dial must succeed: %v", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	_ = conn.Close()
	time.Sleep(20 * time.Millisecond)

	// Cross-origin: Origin не совпадает с Host запроса → handshake отклонён.
	hdr2 := http.Header{}
	hdr2.Set("Origin", "http://evil.com")
	_, resp2, err := websocket.DefaultDialer.Dial(wsURL, hdr2)
	if resp2 != nil {
		_ = resp2.Body.Close()
	}
	if err == nil {
		t.Fatal("cross-origin dial must fail with same-origin default")
	}
}
