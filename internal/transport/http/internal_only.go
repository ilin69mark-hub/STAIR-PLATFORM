package http

import (
	"net"
	"net/http"
	"strings"
)

// InternalOnlyMiddleware ограничивает доступ только с внутренних IP.
// Используется для /metrics, /debug/pprof и других внутренних endpoints.
func InternalOnlyMiddleware(next http.Handler) http.Handler {
	// Разрешенные CIDR ranges для внутренних сетей
	allowedCIDRs := []string{
		"127.0.0.0/8",     // localhost
		"10.0.0.0/8",      // private Class A
		"172.16.0.0/12",   // private Class B
		"192.168.0.0/16",  // private Class C
		"::1/128",         // IPv6 localhost
		"fc00::/7",        // IPv6 private
		"fe80::/10",       // IPv6 link-local
	}

	var allowedNets []*net.IPNet
	for _, cidr := range allowedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			allowedNets = append(allowedNets, network)
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		
		if !isInternalIP(ip, allowedNets) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractIP извлекает IP адрес из Request.
func extractIP(r *http.Request) string {
	// Проверяем X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For: client, proxy1, proxy2
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Проверяем X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Используем RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isInternalIP проверяет, является ли IP внутренним.
func isInternalIP(ip string, allowedNets []*net.IPNet) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, network := range allowedNets {
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}
