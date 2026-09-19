package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"stairplatform/internal/engine/solver"
)

func TestUserInputMessage(t *testing.T) {
	// InputError passthrough
	inp := &solver.InputError{Message: "bad input"}
	if m := userInputMessage(inp); m != "bad input" {
		t.Fatalf("want bad input got %q", m)
	}
	cases := map[string]string{ //nolint:gosec // переводы сообщений, не учётные данные
		"invalid rate":           "Некорректная ставка",
		"invalid email":          "Некорректный email",
		"password too weak":      "Пароль слишком слабый",
		"invalid credentials":    "Неверный email",
		"already registered":     "уже зарегистрирован",
		"contact.name required":  "имя контактного",
		"width must be positive": "Ширина марша",
		"unknown xyz":            "Некорректный запрос",
	}
	for msg, want := range cases {
		got := userInputMessage(errors.New(msg))
		if want != "" && !containsExtra(got, want) {
			t.Fatalf("msg %q got %q want contain %q", msg, got, want)
		}
	}
}

func containsExtra(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

func TestWriteInputError(t *testing.T) {
	w := httptest.NewRecorder()
	writeInputError(w, "validation", errors.New("invalid email"))
	if w.Code != 422 {
		t.Fatalf("want 422 got %d", w.Code)
	}
	_ = http.StatusUnprocessableEntity
}
