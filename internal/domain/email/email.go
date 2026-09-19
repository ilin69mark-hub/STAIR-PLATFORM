package email

import (
	"regexp"
	"strings"
)

// Проверка формата email (EDR-NONE, единый источник правды для всех слоёв).
// Раньше дублировалась в infrastructure/validation (мёртвый пакет) и в
// application/auth (loose-регэксп); application не может зависеть от
// infrastructure (architecture_test), поэтому канон живёт в domain.
var pattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Valid проверяет email на валидность.
// Возвращает true если email валиден.
func Valid(email string) bool {
	if email == "" {
		return false
	}

	email = strings.TrimSpace(strings.ToLower(email))

	if len(email) > 254 {
		return false
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	local, domain := parts[0], parts[1]

	if local == "" || len(local) > 64 {
		return false
	}

	if domain == "" || len(domain) > 253 {
		return false
	}

	if !strings.Contains(domain, ".") {
		return false
	}

	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}

	return pattern.MatchString(email)
}

// Normalize нормализует email (trim + lowercase).
func Normalize(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
