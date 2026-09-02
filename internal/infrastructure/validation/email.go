package validation

import (
	"regexp"
	"strings"
)

// Email pattern для базовой валидации email.
var emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// ValidateEmail проверяет email на валидность.
// Возвращает true если email валиден.
func ValidateEmail(email string) bool {
	if email == "" {
		return false
	}

	// Нормализуем email
	email = strings.TrimSpace(strings.ToLower(email))

	// Базовые проверки
	if len(email) > 254 {
		return false
	}

	// Разделяем на local и domain parts
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	local, domain := parts[0], parts[1]

	// Проверяем local part
	if local == "" || len(local) > 64 {
		return false
	}

	// Проверяем domain
	if domain == "" || len(domain) > 253 {
		return false
	}

	// Domain должен содержать точку
	if !strings.Contains(domain, ".") {
		return false
	}

	// Domain не должен начинаться или заканчиваться точкой
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}

	// Проверяем регулярным выражением
	return emailPattern.MatchString(email)
}

// NormalizeEmail нормализует email (trim + lowercase).
func NormalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
