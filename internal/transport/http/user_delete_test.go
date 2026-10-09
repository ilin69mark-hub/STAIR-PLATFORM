// Удаление сотрудника: контракт HTTP.
//
// Проверяется то, на что смотрит клиент и на что смотрит владелец в журнале:
// коды ответов и то, что получатель проектов действительно доезжает до
// сервиса (без него ручка молча удалила бы человека вместе с его проектами).
package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/stair"
)

// newFakeAuthWith готовит фейк с владельцем: роль owner даёт все права
// (EffectiveGrants отдаёт шаблон роли), иначе staff.delete не прошёл бы гейт.
func newFakeAuthWith(f *fakeAuth) *fakeAuth {
	if f == nil {
		f = &fakeAuth{}
	}
	f.tokens = map[string]*auth.User{
		"token-1": {ID: "u-1", TenantID: "t-1", Email: "a@b.co", Role: auth.RoleOwner},
	}
	return f
}

func TestDeleteUserRequiresTransferTo(t *testing.T) {
	fa := newFakeAuthWith(nil)
	router := NewRouter(stair.NewService(), nil, fa, DefaultConfig(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodDelete, "/api/v1/admin/users/u-2", ""))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("без transfer_to: код %d (%s), ожидался 422", rec.Code, rec.Body.String())
	}
	if fa.deletedID != "" {
		t.Error("сервис вызван без получателя проектов — это потеря проектов")
	}
}

func TestDeleteUserPassesRecipientToService(t *testing.T) {
	fa := newFakeAuthWith(nil)
	router := NewRouter(stair.NewService(), nil, fa, DefaultConfig(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodDelete, "/api/v1/admin/users/u-2?transfer_to=u-3", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d (%s), ожидался 200", rec.Code, rec.Body.String())
	}
	if fa.deletedID != "u-2" || fa.transferTo != "u-3" {
		t.Errorf("сервис получил %s→%s, ожидалось u-2→u-3", fa.deletedID, fa.transferTo)
	}
}

func TestDeleteUserErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"последний владелец", auth.ErrLastOwner, http.StatusConflict, "last_owner"},
		{"неподходящий получатель", auth.ErrInvalidRecipient, http.StatusUnprocessableEntity, "invalid_recipient"},
		{"себя", auth.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"нет сотрудника", auth.ErrNotFound, http.StatusNotFound, "not_found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fa := newFakeAuthWith(&fakeAuth{deleteUserErr: c.err})
			router := NewRouter(stair.NewService(), nil, fa, DefaultConfig(), nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, authedRequest(http.MethodDelete, "/api/v1/admin/users/u-2?transfer_to=u-3", ""))

			if rec.Code != c.status {
				t.Fatalf("код %d (%s), ожидался %d", rec.Code, rec.Body.String(), c.status)
			}
			if body := rec.Body.String(); !strings.Contains(body, c.code) {
				t.Errorf("код ошибки %q не в теле: %s", c.code, body)
			}
		})
	}
}

func TestUserDeletionPreviewReturnsCounts(t *testing.T) {
	fa := newFakeAuthWith(nil)
	router := NewRouter(stair.NewService(), nil, fa, DefaultConfig(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/admin/users/u-2/deletion-preview", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d (%s), ожидался 200", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"projects":3`, `"calculations":12`, `"orders":2`} {
		if !strings.Contains(body, want) {
			t.Errorf("в предпросмотре нет %s: %s", want, body)
		}
	}
}

func TestUserDeletionPreviewNotFound(t *testing.T) {
	fa := newFakeAuthWith(&fakeAuth{deletionPreviewErr: auth.ErrNotFound})
	router := NewRouter(stair.NewService(), nil, fa, DefaultConfig(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/admin/users/u-2/deletion-preview", ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("код %d, ожидался 404", rec.Code)
	}
}

// staffDeleteProbeAuth — сотрудник без staff.delete. Отдельный тип, а не
// noGrantsAuth, потому что нужно СОСЧИТАТЬ вызов сервиса: гейт доказателен
// только если сервис не был вызван вовсе.
type staffDeleteProbeAuth struct {
	testAuth
	deletedID string
}

func (a *staffDeleteProbeAuth) DeleteUser(_ context.Context, _, _, userID, _ string) error {
	a.deletedID = userID
	return nil
}

func TestDeleteUserNeedsCapability(t *testing.T) {
	// Право staff.delete не выдано: удаление недоступно. Сервис не должен быть
	// вызван вообще — иначе гейт только рисует 403, а удаляет всё равно.
	fa := &staffDeleteProbeAuth{}
	router := NewRouter(stair.NewService(), nil, fa, DefaultConfig(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodDelete, "/api/v1/admin/users/u-2?transfer_to=u-3", ""))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("код %d (%s), ожидался 403", rec.Code, rec.Body.String())
	}
	if fa.deletedID != "" {
		t.Errorf("сервис вызван без права staff.delete (удалили бы %s)", fa.deletedID)
	}
}
