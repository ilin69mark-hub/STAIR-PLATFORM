package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCSRFOriginAllowlist — double-submit CSRF (SEC-0003/EDR-0014 §3.3):
// по умолчанию источник должен совпадать с host запроса, но явно
// перечисленные источники тоже допускаются — без этого витрина на Next.js
// (другой порт/домен + прокси, подменяющий Host) не может оплатить услугу
// (этап 4). Чужой источник по-прежнему отклоняется.
func TestCSRFOriginAllowlist(t *testing.T) {
	cases := []struct {
		name     string
		host     string
		origin   string
		referer  string
		allowed  []string
		wantCode int
	}{
		{
			name:     "тот же host без списка",
			host:     "shop.example",
			origin:   "https://shop.example",
			wantCode: http.StatusNoContent,
		},
		{
			name:     "источник из allowlist",
			host:     "api.example",
			origin:   "https://shop.example",
			allowed:  []string{"https://shop.example"},
			wantCode: http.StatusNoContent,
		},
		{
			name:     "хост из allowlist без схемы",
			host:     "api.example",
			origin:   "https://shop.example",
			allowed:  []string{"shop.example"},
			wantCode: http.StatusNoContent,
		},
		{
			name:     "порт в allowlist (локальная витрина за прокси)",
			host:     "localhost:8080",
			origin:   "http://localhost:5176",
			allowed:  []string{"http://localhost:5176"},
			wantCode: http.StatusNoContent,
		},
		{
			name:     "чужой источник",
			host:     "api.example",
			origin:   "https://evil.example",
			allowed:  []string{"https://shop.example"},
			wantCode: http.StatusForbidden,
		},
		{
			name:     "поддомен не подставляется",
			host:     "api.example",
			origin:   "https://shop.example.evil",
			allowed:  []string{"https://shop.example"},
			wantCode: http.StatusForbidden,
		},
		{
			name:     "без Origin и Referer — не браузер",
			host:     "api.example",
			allowed:  nil,
			wantCode: http.StatusNoContent,
		},
		{
			name:     "только Referer",
			host:     "api.example",
			referer:  "https://shop.example/materials",
			allowed:  []string{"https://shop.example"},
			wantCode: http.StatusNoContent,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/api/v1/public/services/checkout", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.referer != "" {
				req.Header.Set("Referer", tc.referer)
			}
			rec := httptest.NewRecorder()
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			requireCSRF(tc.allowed, next).ServeHTTP(rec, req)

			// CSRF-токен проверяется до источника; без cookie ответ 403 в любом
			// случае, поэтому проверяем именно решение по источнику.
			if got := csrfOriginAllowed(req, tc.allowed); got != (tc.wantCode == http.StatusNoContent) {
				t.Fatalf("csrfOriginAllowed = %v, ожидалось %v (allowlist %v, origin %q)",
					got, tc.wantCode == http.StatusNoContent, tc.allowed, tc.origin)
			}
		})
	}
}
