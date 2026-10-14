// Package envguard — fail-closed валидация STAIR_ENVIRONMENT (S-151).
//
// Прод-гарды api/worker включаются по значению STAIR_ENVIRONMENT
// (production|prod). Опечатка или пустое значение молча выключали защиты:
// не шифровались webhook-секреты, не требовался Stripe, воркер разрешал
// loopback в SSRF-политике. Теперь неизвестное/пустое значение — фатальная
// ошибка старта: деплоить «на глаз» нельзя, дефолт-безопасность.
package envguard

import (
	"fmt"
	"strings"
)

// Known — допустимые значения STAIR_ENVIRONMENT (регистр/пробелы не важны).
var Known = []string{"development", "dev", "test", "testing", "staging", "stage", "production", "prod"}

// IsProduction — нормализованный прод-детект (production|prod).
func IsProduction(environment string) bool {
	v := strings.ToLower(strings.TrimSpace(environment))
	return v == "production" || v == "prod"
}

// Validate проверяет значение STAIR_ENVIRONMENT: пустое или неизвестное —
// ошибка со списком допустимых. Возвращает нормализованное значение.
func Validate(environment string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(environment))
	if v == "" {
		return "", fmt.Errorf("STAIR_ENVIRONMENT is required (allowed: %s)", strings.Join(Known, ", "))
	}
	for _, k := range Known {
		if v == k {
			return v, nil
		}
	}
	return "", fmt.Errorf("unknown STAIR_ENVIRONMENT %q (allowed: %s)", environment, strings.Join(Known, ", "))
}
