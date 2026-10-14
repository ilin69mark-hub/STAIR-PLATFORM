package http

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
)

// defaultInternalCIDRs — разрешённые CIDR по умолчанию (RFC1918+loopback).
// ВНИМАНИЕ (S-141 №11): за L7-балансировщиком/nodePort RemoteAddr — это IP
// балансировщика/ноды (внутри RFC1918), и дефолтный список фактически
// открывает /metrics,/swagger,/debug/pprof всем, кто достучался до пода.
// В проде за LB обязательно сузьте список через STAIR_INTERNAL_CIDRS
// (например, только pod-CIDR кластера) + закройте путь SG/NetworkPolicy
// (runbook: REPORTS/builder-S149.md).
var defaultInternalCIDRs = []string{
	"127.0.0.0/8",    // localhost
	"10.0.0.0/8",     // private Class A
	"172.16.0.0/12",  // private Class B
	"192.168.0.0/16", // private Class C
	"::1/128",        // IPv6 localhost
	"fc00::/7",       // IPv6 private
	"fe80::/10",      // IPv6 link-local
}

// internalNets собирает разрешённые сети (S-149, S-141 №11): STAIR_INTERNAL_CIDRS
// (comma-separated) переопределяет дефолт. Невалидные записи пропускаются с
// warn; если из env не собралось НИЧЕГО валидного — откат к дефолту с error
// (опечатка в env не должна молча открывать/закрывать всё). Парсинг на каждый
// запрос осознанно: middleware висит на редких путях (/metrics,/swagger),
// зато env перечитывается без рестарта и тесты не страдают от кеша.
func internalNets() []*net.IPNet {
	raw, set := os.LookupEnv("STAIR_INTERNAL_CIDRS")
	cidrs := defaultInternalCIDRs
	if set {
		cidrs = strings.Split(raw, ",")
	}
	var nets []*net.IPNet
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			slog.Warn("internal-only: skip invalid CIDR", "cidr", cidr, "error", err)
			continue
		}
		nets = append(nets, network)
	}
	if set && len(nets) == 0 {
		slog.Error("internal-only: STAIR_INTERNAL_CIDRS has no valid CIDRs, falling back to defaults")
		return internalNetsDefault()
	}
	return nets
}

// internalNetsDefault парсит дефолтный список (всегда валиден).
func internalNetsDefault() []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range defaultInternalCIDRs {
		if _, network, err := net.ParseCIDR(cidr); err == nil {
			nets = append(nets, network)
		}
	}
	return nets
}

// InternalOnlyMiddleware ограничивает доступ только с внутренних IP.
// Используется для /metrics, /debug/pprof и других внутренних endpoints.
func InternalOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)

		if !isInternalIP(ip, internalNets()) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractIP извлекает IP адрес из Request.
// Для внутренних endpoints используется только RemoteAddr (TCP source),
// чтобы X-Forwarded-For не мог быть подделан атакующим.
func extractIP(r *http.Request) string {
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
