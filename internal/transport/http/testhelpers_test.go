package http

import "net/http"

// testCookie строит cookie с безопасными атрибутами для тестовых запросов.
// Secure/HttpOnly/SameSite не влияют на Cookie-header запроса (они — атрибуты
// Set-Cookie), но удовлетворяют gosec G124.
func testCookie(name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	}
}
