// Передача владения проектом: контракт HTTP.
//
// Проверяется то, на что смотрит интерфейс: код ответа, что новый владелец
// доезжает до сервиса, и что платформенное право staff.project_transfer
// доезжает вместе с ним (иначе компания не смогла бы забрать проект).
package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
)

func TestTransferProjectPassesRecipient(t *testing.T) {
	svc := newFakeProjectService()
	router := NewRouter(stair.NewService(), svc, newFakeAuthWith(nil), DefaultConfig())

	req := authedRequest(http.MethodPost, "/api/v1/projects/p1/transfer", `{"to_user_id":"u-editor"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d (%s), ожидался 200", rec.Code, rec.Body.String())
	}
	if svc.transferTo != "u-editor" {
		t.Errorf("сервис получил нового владельца %q, ожидался u-editor", svc.transferTo)
	}
}

func TestTransferProjectRequiresRecipient(t *testing.T) {
	svc := newFakeProjectService()
	router := NewRouter(stair.NewService(), svc, newFakeAuthWith(nil), DefaultConfig())

	req := authedRequest(http.MethodPost, "/api/v1/projects/p1/transfer", `{}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("код %d (%s), ожидался 422", rec.Code, rec.Body.String())
	}
	if svc.transferTo != "" {
		t.Error("сервис вызван без нового владельца")
	}
}

// Флаг byCapability означает «у актора есть платформенное право
// staff.project_transfer», а не «передача совершена по праву»: владелец проекта
// может передать его и без этого права. Решение — на сервисе.
func TestTransferProjectFlagFollowsCapability(t *testing.T) {
	t.Run("владелец компании — право есть", func(t *testing.T) {
		svc := newFakeProjectService()
		router := NewRouter(stair.NewService(), svc, newFakeAuthWith(nil), DefaultConfig())

		req := authedRequest(http.MethodPost, "/api/v1/projects/p1/transfer", `{"to_user_id":"u-editor"}`)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("код %d (%s), ожидался 200", rec.Code, rec.Body.String())
		}
		if !svc.transferByCapability {
			t.Error("у владельца компании есть staff.project_transfer — флаг должен быть true")
		}
	})

	t.Run("сотрудник без права — флага нет", func(t *testing.T) {
		svc := newFakeProjectService()
		svc.transferErr = project.ErrForbidden
		router := NewRouter(stair.NewService(), svc, &noGrantsAuth{}, DefaultConfig())

		req := authedRequest(http.MethodPost, "/api/v1/projects/p1/transfer", `{"to_user_id":"u-editor"}`)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if svc.transferByCapability {
			t.Error("без staff.project_transfer флаг не должен выставляться")
		}
		if rec.Code != http.StatusForbidden {
			t.Fatalf("код %d (%s), ожидался 403", rec.Code, rec.Body.String())
		}
	})
}

func TestTransferProjectErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"чужой проект", project.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"получатель не участник", project.ErrNotFound, http.StatusNotFound, "not_found"},
		{"себе или уже владелец", project.ErrConflict, http.StatusConflict, "conflict"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := newFakeProjectService()
			svc.transferErr = c.err
			router := NewRouter(stair.NewService(), svc, newFakeAuthWith(nil), DefaultConfig())

			req := authedRequest(http.MethodPost, "/api/v1/projects/p1/transfer", `{"to_user_id":"u-editor"}`)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != c.status {
				t.Fatalf("код %d (%s), ожидался %d", rec.Code, rec.Body.String(), c.status)
			}
			if body := rec.Body.String(); !strings.Contains(body, c.code) {
				t.Errorf("код ошибки %q не в теле: %s", c.code, body)
			}
		})
	}
}
