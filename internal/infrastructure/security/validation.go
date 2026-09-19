package security

import (
	"regexp"
	"strings"
)

// ValidateEmail проверяет email на валидность.
func ValidateEmail(email string) bool {
	if len(email) == 0 || len(email) > 254 {
		return false
	}

	// Простая проверка формата
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	local := parts[0]
	domain := parts[1]

	if len(local) == 0 || len(local) > 64 {
		return false
	}

	if len(domain) == 0 || len(domain) > 253 {
		return false
	}

	// Проверяем domain
	domainParts := strings.Split(domain, ".")
	if len(domainParts) < 2 {
		return false
	}

	for _, part := range domainParts {
		if len(part) == 0 || len(part) > 63 {
			return false
		}
	}

	// Простой regex для local part
	localRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+$`)
	return localRegex.MatchString(local)
}

// SanitizeString очищает строку от потенциально опасных символов.
func SanitizeString(s string) string {
	// Удаляем null bytes
	s = strings.ReplaceAll(s, "\x00", "")

	// Удаляем невидимые символы
	s = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)

	return strings.TrimSpace(s)
}

// ValidateUsername проверяет username на валидность.
func ValidateUsername(username string) bool {
	if len(username) < 3 || len(username) > 32 {
		return false
	}

	// Только буквы, цифры, подчеркивание
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	return usernameRegex.MatchString(username)
}

// ValidatePassword проверяет password на валидность.
func ValidatePassword(password string) bool {
	if len(password) < 8 || len(password) > 128 {
		return false
	}

	// Проверяем наличие хотя бы одной буквы и одной цифры
	hasLetter := false
	hasDigit := false

	for _, c := range password {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			hasLetter = true
		}
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
	}

	return hasLetter && hasDigit
}

// ValidateProjectName проверяет имя проекта.
func ValidateProjectName(name string) bool {
	if len(name) < 1 || len(name) > 255 {
		return false
	}

	// Проверяем что имя не содержит только пробелы
	if strings.TrimSpace(name) == "" {
		return false
	}

	return true
}

// ContainsSQLInjection проверяет наличие SQL injection паттернов.
func ContainsSQLInjection(input string) bool {
	// Простые паттерны SQL injection
	patterns := []string{
		"' OR '1'='1",
		"' OR 1=1 --",
		"'; DROP TABLE",
		"UNION SELECT",
		"INSERT INTO",
		"DELETE FROM",
		"UPDATE SET",
		"1' OR '1'='1",
		"admin'--",
		"' OR ''='",
	}

	lower := strings.ToLower(input)
	for _, pattern := range patterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// ValidateSortOrder проверяет порядок сортировки.
func ValidateSortOrder(order string) bool {
	return order == "asc" || order == "desc" || order == ""
}

// ValidatePagination проверяет параметры пагинации.
func ValidatePagination(limit, offset int) bool {
	return limit >= 0 && limit <= 1000 && offset >= 0
}
