package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withTestRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func TestAppError_StatusCode(t *testing.T) {
	tests := []struct {
		err  *AppError
		want int
	}{
		{ErrNotFound, 404},
		{ErrAlreadyExists, 409},
		{ErrValidation, 422},
		{ErrUnauthorized, 401},
		{ErrForbidden, 403},
		{ErrInternal, 500},
		{ErrRateLimited, 429},
		{ErrPayloadTooLarge, 413},
		{ErrUnsupportedVersion, 400},
		{ErrConflict, 409},
		{ErrDeprecation, 410},
		{&AppError{Status: 0, Code: "zero"}, 500}, // default
	}
	for _, tt := range tests {
		if got := tt.err.StatusCode(); got != tt.want {
			t.Errorf("AppError{%s}.StatusCode() = %d, want %d", tt.err.Code, got, tt.want)
		}
	}
}

func TestAppError_Error(t *testing.T) {
	err := &AppError{Status: 404, Code: "not_found", Message: "not found"}
	if err.Error() != "not found" {
		t.Errorf("expected 'not found', got %q", err.Error())
	}

	inner := errors.New("inner error")
	errWithInner := &AppError{Status: 500, Code: "internal", Message: "something broke", Err: inner}
	if errWithInner.Error() != "inner error" {
		t.Errorf("expected 'inner error', got %q", errWithInner.Error())
	}
}

func TestAppError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	err := &AppError{Err: inner}
	if !errors.Is(err, inner) {
		t.Error("expected errors.Is to find inner error")
	}
}

func TestNotFoundError(t *testing.T) {
	err := NotFoundError("project not found", nil)
	if err.Status != 404 {
		t.Errorf("expected status 404, got %d", err.Status)
	}
	if err.Code != "not_found" {
		t.Errorf("expected code not_found, got %q", err.Code)
	}
}

func TestValidationError(t *testing.T) {
	err := NewValidationErrorAppError("bad input", nil)
	if err.Status != 422 {
		t.Errorf("expected status 422, got %d", err.Status)
	}
}

func TestConflictError(t *testing.T) {
	err := ConflictError("already exists", nil)
	if err.Status != 409 {
		t.Errorf("expected status 409, got %d", err.Status)
	}
}

func TestForbiddenError(t *testing.T) {
	err := ForbiddenError("no access", nil)
	if err.Status != 403 {
		t.Errorf("expected status 403, got %d", err.Status)
	}
}

func TestUnauthorizedError(t *testing.T) {
	err := UnauthorizedError("login required", nil)
	if err.Status != 401 {
		t.Errorf("expected status 401, got %d", err.Status)
	}
}

func TestInternalError(t *testing.T) {
	err := InternalError("oops", nil)
	if err.Status != 500 {
		t.Errorf("expected status 500, got %d", err.Status)
	}
}

func TestFromError_AppError(t *testing.T) {
	original := ErrNotFound
	wrapped := FromError(original)
	if wrapped != original {
		t.Error("expected FromError to return same *AppError")
	}
}

func TestFromError_Unknown(t *testing.T) {
	err := errors.New("something unknown")
	appErr := FromError(err)
	if appErr.Status != 500 {
		t.Errorf("expected status 500 for unknown error, got %d", appErr.Status)
	}
}

func TestFromError_Nil(t *testing.T) {
	if FromError(nil) != nil {
		t.Error("expected nil from FromError(nil)")
	}
}

func TestWriteAppError(t *testing.T) {
	w := httptest.NewRecorder()
	writeAppError(w, ErrNotFound)

	if w.Code != 404 {
		t.Errorf("expected status 404, got %d", w.Code)
	}
	body := w.Body.String()
	if !contains(body, "not_found") {
		t.Errorf("expected body to contain 'not_found', got %q", body)
	}
}

func TestWriteAppErrorWithRequestID(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(withTestRequestID(req.Context(), "test-req-123"))
	writeAppErrorWithRequestID(w, req, ErrNotFound)

	if w.Code != 404 {
		t.Errorf("expected status 404, got %d", w.Code)
	}
	body := w.Body.String()
	if !contains(body, "test-req-123") {
		t.Errorf("expected body to contain request_id, got %q", body)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestMapDomainError_AppError(t *testing.T) {
	original := ErrNotFound
	mapped := MapDomainError(original)
	if mapped != original {
		t.Error("expected MapDomainError to return same *AppError")
	}
}

func TestMapDomainError_NotFound(t *testing.T) {
	tests := []struct {
		errMsg string
		code   string
	}{
		{"project not found", "not_found"},
		{"user not found", "not_found"},
		{"stair not found", "not_found"},
		{"config not found", "not_found"},
		{"payment not found", "not_found"},
		{"order not found", "not_found"},
	}
	for _, tt := range tests {
		err := errors.New(tt.errMsg)
		mapped := MapDomainError(err)
		if mapped.Code != tt.code {
			t.Errorf("MapDomainError(%q).Code = %q, want %q", tt.errMsg, mapped.Code, tt.code)
		}
	}
}

func TestMapDomainError_AlreadyExists(t *testing.T) {
	err := errors.New("email already exists")
	mapped := MapDomainError(err)
	if mapped.Status != 409 {
		t.Errorf("expected status 409, got %d", mapped.Status)
	}
}

func TestMapDomainError_Validation(t *testing.T) {
	tests := []string{"validation failed", "invalid input"}
	for _, msg := range tests {
		err := errors.New(msg)
		mapped := MapDomainError(err)
		if mapped.Status != 422 {
			t.Errorf("MapDomainError(%q).Status = %d, want 422", msg, mapped.Status)
		}
	}
}

func TestMapDomainError_Forbidden(t *testing.T) {
	tests := []string{"forbidden", "insufficient permissions"}
	for _, msg := range tests {
		err := errors.New(msg)
		mapped := MapDomainError(err)
		if mapped.Status != 403 {
			t.Errorf("MapDomainError(%q).Status = %d, want 403", msg, mapped.Status)
		}
	}
}

func TestMapDomainError_Unknown(t *testing.T) {
	err := errors.New("something completely unknown")
	mapped := MapDomainError(err)
	if mapped.Status != 500 {
		t.Errorf("expected status 500 for unknown error, got %d", mapped.Status)
	}
}

func TestMapDomainError_Nil(t *testing.T) {
	if MapDomainError(nil) != nil {
		t.Error("expected nil from MapDomainError(nil)")
	}
}

func TestMapDomainError_CaseInsensitive(t *testing.T) {
	err := errors.New("Project Not Found")
	mapped := MapDomainError(err)
	if mapped.Status != 404 {
		t.Errorf("expected status 404 for case-insensitive match, got %d", mapped.Status)
	}
}
