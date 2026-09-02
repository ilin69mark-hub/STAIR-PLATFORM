package http

import (
	"errors"
	"net/http"
	"strings"
)

// AppError — структурированная ошибка приложения.
// Маппит доменные ошибки на HTTP status codes.
type AppError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// StatusCode возвращает HTTP статус код.
func (e *AppError) StatusCode() int {
	if e.Status == 0 {
		return http.StatusInternalServerError
	}
	return e.Status
}

// Known domain errors → HTTP mapping.
var (
	ErrNotFound = &AppError{
		Status:  http.StatusNotFound,
		Code:    "not_found",
		Message: "Resource not found",
	}
	ErrAlreadyExists = &AppError{
		Status:  http.StatusConflict,
		Code:    "already_exists",
		Message: "Resource already exists",
	}
	ErrValidation = &AppError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "validation_error",
		Message: "Validation failed",
	}
	ErrUnauthorized = &AppError{
		Status:  http.StatusUnauthorized,
		Code:    "unauthorized",
		Message: "Unauthorized",
	}
	ErrForbidden = &AppError{
		Status:  http.StatusForbidden,
		Code:    "forbidden",
		Message: "Forbidden",
	}
	ErrConflict = &AppError{
		Status:  http.StatusConflict,
		Code:    "conflict",
		Message: "Conflict",
	}
	ErrRateLimited = &AppError{
		Status:  http.StatusTooManyRequests,
		Code:    "rate_limited",
		Message: "Rate limit exceeded",
	}
	ErrPayloadTooLarge = &AppError{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    "payload_too_large",
		Message: "Payload too large",
	}
	ErrInternal = &AppError{
		Status:  http.StatusInternalServerError,
		Code:    "internal",
		Message: "Internal server error",
	}
	ErrUnsupportedVersion = &AppError{
		Status:  http.StatusBadRequest,
		Code:    "unsupported_version",
		Message: "API version not supported",
	}
	ErrDeprecation = &AppError{
		Status:  http.StatusGone,
		Code:    "deprecated",
		Message: "This endpoint has been deprecated",
	}
)

// NewAppError создаёт новый AppError с опциональным wrapping.
func NewAppError(status int, code, message string, err error) *AppError {
	return &AppError{
		Status:  status,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// NotFoundError создаёт ошибку 404 с опциональным wrapping.
func NotFoundError(msg string, err error) *AppError {
	if msg == "" {
		msg = ErrNotFound.Message
	}
	return &AppError{Status: 404, Code: "not_found", Message: msg, Err: err}
}

// ValidationErrorResponse создаёт ошибку 422 с опциональным wrapping.
func ValidationErrorResponse(msg string, err error) *AppError {
	if msg == "" {
		msg = ErrValidation.Message
	}
	return &AppError{Status: 422, Code: "validation_error", Message: msg, Err: err}
}

// NewValidationErrorAppError создаёт ValidationError AppError (для совместимости с tests).
func NewValidationErrorAppError(msg string, err error) *AppError {
	return ValidationErrorResponse(msg, err)
}

// ConflictError создаёт ошибку 409 с опциональным wrapping.
func ConflictError(msg string, err error) *AppError {
	if msg == "" {
		msg = ErrConflict.Message
	}
	return &AppError{Status: 409, Code: "conflict", Message: msg, Err: err}
}

// ForbiddenError создаёт ошибку 403 с опциональным wrapping.
func ForbiddenError(msg string, err error) *AppError {
	if msg == "" {
		msg = ErrForbidden.Message
	}
	return &AppError{Status: 403, Code: "forbidden", Message: msg, Err: err}
}

// UnauthorizedError создаёт ошибку 401 с опциональным wrapping.
func UnauthorizedError(msg string, err error) *AppError {
	if msg == "" {
		msg = ErrUnauthorized.Message
	}
	return &AppError{Status: 401, Code: "unauthorized", Message: msg, Err: err}
}

// InternalError создаёт ошибку 500 с опциональным wrapping.
func InternalError(msg string, err error) *AppError {
	if msg == "" {
		msg = ErrInternal.Message
	}
	return &AppError{Status: 500, Code: "internal", Message: msg, Err: err}
}

// RateLimitedError создаёт ошибку 429 с опциональным wrapping.
func RateLimitedError(msg string, err error) *AppError {
	if msg == "" {
		msg = ErrRateLimited.Message
	}
	return &AppError{Status: 429, Code: "rate_limited", Message: msg, Err: err}
}

// AlreadyExistsError создаёт ошибку 409 с опциональным wrapping.
func AlreadyExistsError(msg string, err error) *AppError {
	if msg == "" {
		msg = ErrAlreadyExists.Message
	}
	return &AppError{Status: 409, Code: "already_exists", Message: msg, Err: err}
}

// MapDomainError маппит ошибку из domain/application слоя на AppError.
// Если ошибка уже AppError — возвращает как есть.
// Если ошибка является sentinel error — маппит на соответствующий AppError.
// Иначе — InternalError.
func MapDomainError(err error) *AppError {
	if err == nil {
		return nil
	}

	// Если уже AppError
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	// Проверяем sentinel errors из domain слоя
	errMsg := err.Error()
	errLower := strings.ToLower(errMsg)

	switch {
	case strings.Contains(errLower, "not found"):
		return NotFoundError(errMsg, err)
	case strings.Contains(errLower, "already exists"):
		return AlreadyExistsError(errMsg, err)
	case strings.Contains(errLower, "conflict"):
		return ConflictError(errMsg, err)
	case strings.Contains(errLower, "validation") || strings.Contains(errLower, "invalid"):
		return ValidationErrorResponse(errMsg, err)
	case strings.Contains(errLower, "unauthorized") || strings.Contains(errLower, "authentication"):
		return UnauthorizedError(errMsg, err)
	case strings.Contains(errLower, "forbidden") || strings.Contains(errLower, "permission"):
		return ForbiddenError(errMsg, err)
	case strings.Contains(errLower, "rate limit"):
		return RateLimitedError(errMsg, err)
	default:
		return InternalError(errMsg, err)
	}
}

// FromError маппит произвольную ошибку на AppError.
func FromError(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	// Unknown error → 500
	return InternalError("", err)
}

// writeAppError отправляет структурированную ошибку.
func writeAppError(w http.ResponseWriter, err *AppError) {
	writeJSON(w, err.StatusCode(), map[string]any{
		"error": map[string]any{
			"code":    err.Code,
			"message": err.Message,
		},
	})
}

// writeAppErrorWithRequestID отправляет ошибку с request_id.
func writeAppErrorWithRequestID(w http.ResponseWriter, r *http.Request, err *AppError) {
	writeJSON(w, err.StatusCode(), map[string]any{
		"error": map[string]any{
			"code":      err.Code,
			"message":   err.Message,
			"request_id": RequestID(r.Context()),
		},
	})
}
