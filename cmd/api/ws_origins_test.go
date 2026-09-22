package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ws "stairplatform/internal/transport/websocket"
)

// testWSReq строит HTTP-запрос с заданным Origin (браузерный WS-handshake:
// Origin шлёт браузер всегда; Host — хост, куда идёт запрос).
func testWSReq(origin string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	r.Host = "app.example.com"
	return r
}

// corsFromEnv повторяет чтение main.go — как corsOrigins получается в проде
// (envStringSlice с dev-дефолтом localhost:3000).
func corsFromEnv(t *testing.T) []string {
	t.Helper()
	return envStringSlice("STAIR_CORS_ORIGINS", []string{"http://localhost:3000"})
}

// TestWSAllowedOrigins — S-115: пустые STAIR_WS_ORIGINS/STAIR_CORS_ORIGINS
// должны давать nil (same-origin-only), а НЕ dev-дефолт localhost:3000.
func TestWSAllowedOrigins(t *testing.T) {
	t.Run("both unset means same-origin-only", func(t *testing.T) {
		t.Setenv("STAIR_WS_ORIGINS", "")
		t.Setenv("STAIR_CORS_ORIGINS", "")
		if got := wsAllowedOrigins(corsFromEnv(t)); len(got) != 0 {
			t.Fatalf("empty envs must mean same-origin-only (nil list), got %v", got)
		}
	})

	t.Run("explicit CORS is WS fallback (S-112)", func(t *testing.T) {
		t.Setenv("STAIR_WS_ORIGINS", "")
		t.Setenv("STAIR_CORS_ORIGINS", "https://api.example.com")
		got := wsAllowedOrigins(corsFromEnv(t))
		if len(got) != 1 || got[0] != "https://api.example.com" {
			t.Fatalf("explicit CORS must be the WS fallback, got %v", got)
		}
	})

	t.Run("explicit WS list wins", func(t *testing.T) {
		t.Setenv("STAIR_WS_ORIGINS", "https://ws.example.com,*.example.com")
		t.Setenv("STAIR_CORS_ORIGINS", "https://cors.example.com")
		got := wsAllowedOrigins(corsFromEnv(t))
		if len(got) != 2 || got[0] != "https://ws.example.com" || got[1] != "*.example.com" {
			t.Fatalf("explicit WS list must win, got %v", got)
		}
	})

	t.Run("effectively-empty WS list means same-origin-only", func(t *testing.T) {
		// Явно задан, но после split'а пуст (запятые/пробелы) — пустой
		// список, OriginChecker трактует его как same-origin-only.
		t.Setenv("STAIR_WS_ORIGINS", " , ")
		t.Setenv("STAIR_CORS_ORIGINS", "")
		if got := wsAllowedOrigins(corsFromEnv(t)); len(got) != 0 {
			t.Fatalf("effectively-empty WS list must mean same-origin-only, got %v", got)
		}
	})
}

// TestWSOriginsEnvToOriginPolicy — S-115 end-to-end: пустые env через
// OriginChecker разрешают браузерный same-origin и отклоняют кросс-ориджин;
// заданный env — exact/wildcard работают как раньше (S-112).
func TestWSOriginsEnvToOriginPolicy(t *testing.T) {
	t.Run("empty envs: same-origin allowed, cross-origin rejected", func(t *testing.T) {
		t.Setenv("STAIR_WS_ORIGINS", "")
		t.Setenv("STAIR_CORS_ORIGINS", "")
		check := ws.OriginChecker(wsAllowedOrigins(corsFromEnv(t)))

		if !check(testWSReq("http://app.example.com")) {
			t.Fatal("browser same-origin WS must be allowed with empty envs")
		}
		if check(testWSReq("http://evil.com")) {
			t.Fatal("cross-origin WS must be rejected with empty envs")
		}
		// Порт — часть same-origin.
		if check(testWSReq("http://app.example.com:8080")) {
			t.Fatal("origin port mismatch must be rejected")
		}
	})

	t.Run("explicit env: exact and wildcard work as before", func(t *testing.T) {
		t.Setenv("STAIR_WS_ORIGINS", "https://app.example.com,*.example.com")
		t.Setenv("STAIR_CORS_ORIGINS", "")
		check := ws.OriginChecker(wsAllowedOrigins(corsFromEnv(t)))

		if !check(testWSReq("https://app.example.com")) {
			t.Fatal("exact allowed origin must be accepted")
		}
		if !check(testWSReq("http://store.example.com")) {
			t.Fatal("wildcard *.example.com must be accepted")
		}
		if check(testWSReq("https://evil.com")) {
			t.Fatal("non-listed origin must be rejected")
		}
	})

	t.Run("non-browser client (no Origin) allowed with empty envs", func(t *testing.T) {
		t.Setenv("STAIR_WS_ORIGINS", "")
		t.Setenv("STAIR_CORS_ORIGINS", "")
		check := ws.OriginChecker(wsAllowedOrigins(corsFromEnv(t)))

		if !check(testWSReq("")) {
			t.Fatal("request without Origin (server/CLI client) must be allowed")
		}
	})
}
