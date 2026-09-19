package http

import (
	domainemail "stairplatform/internal/domain/email"
	"strconv"
	"strings"
)

// ValidationError — ошибка валидации с полями.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors — список ошибок валидации.
type ValidationErrors []ValidationError

// Error возвращает строковое представление.
func (ve ValidationErrors) Error() string {
	msgs := make([]string, len(ve))
	for i, e := range ve {
		msgs[i] = e.Field + ": " + e.Message
	}
	return strings.Join(msgs, "; ")
}

// ValidateRequired проверяет обязательное поле.
func ValidateRequired(field, value string) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return &ValidationError{Field: field, Message: "is required"}
	}
	return nil
}

// ValidateMinLength проверяет минимальную длину.
func ValidateMinLength(field, value string, min int) *ValidationError {
	if len(value) < min {
		return &ValidationError{Field: field, Message: "must be at least " + itoa(min) + " characters"}
	}
	return nil
}

// ValidateMaxLength проверяет максимальную длину.
func ValidateMaxLength(field, value string, max int) *ValidationError {
	if len(value) > max {
		return &ValidationError{Field: field, Message: "must be at most " + itoa(max) + " characters"}
	}
	return nil
}

// ValidateEmail проверяет формат email (базовая проверка; строгий формат —
// domain/email.Valid, здесь только dto-префрагмент).
func ValidateEmail(field, value string) *ValidationError {
	if value == "" {
		return nil
	}
	if !domainemail.Valid(value) {
		return &ValidationError{Field: field, Message: "must be a valid email address"}
	}
	return nil
}

// ValidateMinValue проверяет минимальное числовое значение.
func ValidateMinValue(field string, value, min int) *ValidationError {
	if value < min {
		return &ValidationError{Field: field, Message: "must be at least " + itoa(min)}
	}
	return nil
}

// ValidateMaxValue проверяет максимальное числовое значение.
func ValidateMaxValue(field string, value, max int) *ValidationError {
	if value > max {
		return &ValidationError{Field: field, Message: "must be at most " + itoa(max)}
	}
	return nil
}

// ValidateOneOf проверяет, что значение входит в список допустимых.
func ValidateOneOf(field, value string, allowed ...string) *ValidationError {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return &ValidationError{Field: field, Message: "must be one of: " + strings.Join(allowed, ", ")}
}

// CollectErrors собирает непустые ошибки в ValidationErrors.
func CollectErrors(errs ...*ValidationError) ValidationErrors {
	var result ValidationErrors
	for _, e := range errs {
		if e != nil {
			result = append(result, *e)
		}
	}
	return result
}

// itoa — быстрое преобразование int → string.
func itoa(n int) string {
	return strconv.Itoa(n)
}
