package http

// S-120: регресс-тесты rate-limit за L7-прокси (XFF только от trusted-CIDR).

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseTrustedProxies(t *testing.T) {
	if got := parseTrustedProxies(""); len(got) != 0 {
		t.Fatalf("empty csv = %d nets, want 0", len(got))
	}
	if got := parseTrustedProxies("   ,  "); len(got) != 0 {
		t.Fatalf("blank csv = %d nets, want 0", len(got))
	}

	nets := parseTrustedProxies("10.0.0.0/8, 1.2.3.4")
	if len(nets) != 2 {
		t.Fatalf("got %d nets, want 2", len(nets))
	}
	contains := func(ip string) bool {
		parsed := net.ParseIP(ip)
		if parsed == nil {
			t.Fatalf("bad test IP %q", ip)
		}
		for _, n := range nets {
			if n.Contains(parsed) {
				return true
			}
		}
		return false
	}
	for _, ip := range []string{"10.1.2.3", "10.255.255.255", "1.2.3.4"} {
		if !contains(ip) {
			t.Errorf("expected %s to be trusted", ip)
		}
	}
	for _, ip := range []string{"11.0.0.1", "1.2.3.5"} {
		if contains(ip) {
			t.Errorf("expected %s to be untrusted", ip)
		}
	}

	// Невалидные записи пропускаются, валидные сохраняются.
	nets = parseTrustedProxies("not-a-cidr, 999.999.0.0/16, 192.168.0.0/16, , 300.300.300.300")
	if len(nets) != 1 {
		t.Fatalf("got %d nets, want 1 (only 192.168.0.0/16)", len(nets))
	}

	// Одиночный IPv6 → /128.
	nets = parseTrustedProxies("::1")
	if len(nets) != 1 || !nets[0].Contains(net.ParseIP("::1")) {
		t.Fatalf("single IPv6 not parsed as /128: %v", nets)
	}
	if nets[0].Contains(net.ParseIP("::2")) {
		t.Fatal("IPv6 /128 must not contain ::2")
	}
}

func TestRateLimitIP(t *testing.T) {
	trusted := parseTrustedProxies("10.0.0.0/8, 1.2.3.4")

	newReq := func(remoteAddr, xff string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		r.RemoteAddr = remoteAddr
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}

	// Дефолт S-112: без trusted XFF игнорируется везде.
	if got := rateLimitIP(newReq("203.0.113.7:1234", "9.9.9.9"), nil); got != "203.0.113.7" {
		t.Fatalf("nil trusted: got %q, want RemoteAddr host", got)
	}

	// Пир из доверенной сети + XFF → самый левый (исходный клиент).
	if got := rateLimitIP(newReq("10.1.2.3:4567", "203.0.113.7, 10.1.2.3"), trusted); got != "203.0.113.7" {
		t.Fatalf("trusted peer: got %q, want leftmost XFF", got)
	}

	// Прямой доступ (пир не в trusted) + поддельный XFF → RemoteAddr (анти-спуфинг).
	if got := rateLimitIP(newReq("203.0.113.7:1234", "9.9.9.9"), trusted); got != "203.0.113.7" {
		t.Fatalf("untrusted peer: got %q, want RemoteAddr host", got)
	}

	// Доверенный пир без XFF → RemoteAddr.
	if got := rateLimitIP(newReq("10.1.2.3:4567", ""), trusted); got != "10.1.2.3" {
		t.Fatalf("no XFF: got %q, want RemoteAddr host", got)
	}

	// Непарсящийся XFF → fallback на RemoteAddr.
	if got := rateLimitIP(newReq("10.1.2.3:4567", "garbage"), trusted); got != "10.1.2.3" {
		t.Fatalf("bad XFF: got %q, want RemoteAddr host", got)
	}

	// Одиночный IP в trusted (без CIDR).
	if got := rateLimitIP(newReq("1.2.3.4:9999", "198.51.100.5"), trusted); got != "198.51.100.5" {
		t.Fatalf("single-IP trusted: got %q, want XFF client", got)
	}
}

// TestLimitRateTrustedProxy — сквозная проверка: два разных клиента за одним
// доверенным прокси делят НЕ общий бюджет (у каждого свой ключ), а спуфнутый
// XFF при прямом доступе не даёт чужой бюджет обойти.
func TestLimitRateTrustedProxy(t *testing.T) {
	trusted := parseTrustedProxies("10.0.0.0/8")
	limiter := newRateLimiter(context.Background(), 1, time.Minute)
	defer limiter.Stop()

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := limitRate(limiter, trusted, ok)

	do := func(remoteAddr, xff string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		r.RemoteAddr = remoteAddr
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}

	// Клиент A за прокси — первый проход, второй упирается в лимит.
	if code := do("10.0.0.1:1111", "203.0.113.10"); code != http.StatusOK {
		t.Fatalf("client A first = %d, want 200", code)
	}
	if code := do("10.0.0.1:1111", "203.0.113.10"); code != http.StatusTooManyRequests {
		t.Fatalf("client A second = %d, want 429", code)
	}
	// Клиент B за тем же прокси — свой бюджет, не задет лимитом A.
	if code := do("10.0.0.1:1111", "203.0.113.20"); code != http.StatusOK {
		t.Fatalf("client B first = %d, want 200 (per-client budget)", code)
	}
	// Прямой доступ с поддельным XFF чужого клиента — идёт по RemoteAddr,
	// а не по XFF (свой бюджет, не расходует чужой и не крадёт его).
	if code := do("198.51.100.99:2222", "203.0.113.10"); code != http.StatusOK {
		t.Fatalf("direct spoofed XFF = %d, want 200 (XFF ignored)", code)
	}
}
