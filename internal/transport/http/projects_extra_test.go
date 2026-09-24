package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/project"
)

// preAuthRequest — запрос с аутентифицированным контекстом напрямую в handler
// (минуя middleware), чтобы проверить ветки authUser==nil и др.
func preAuthRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json") // S-144: decodeJSON требует его
	return r.WithContext(withAuthUser(r.Context(), &auth.User{ID: "u-1", TenantID: "t-1"}))
}

func TestCreateProjectUnauthorized(t *testing.T) {
	h := handleCreateProject(newFakeProjectService())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json") // S-144: decodeJSON требует его
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestCreateProjectServiceError(t *testing.T) {
	svc := newFakeProjectService()
	svc.createErr = errors.New("boom")
	h := handleCreateProject(svc)
	rec := httptest.NewRecorder()
	h(rec, preAuthRequest(http.MethodPost, "/", `{"name":"x"}`))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", rec.Code)
	}
}

func TestGetProjectForbiddenAndInternal(t *testing.T) {
	for _, tc := range []struct {
		name string
		svc  *fakeProjectService
		want int
	}{
		{"forbidden", mustFakeProject(getForbidden()), http.StatusForbidden},
		{"internal", mustFakeProject(getProjectErr(errors.New("db down"))), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.svc.projects["p-1"] = &project.Project{ID: "p-1"}
			rec := httptest.NewRecorder()
			testRouterWithProjects(tc.svc).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/projects/p-1", ""))
			if rec.Code != tc.want {
				t.Fatalf("want %d, got %d: %s", tc.want, rec.Code, rec.Body.String())
			}
		})
	}
}

type fakeOption func(*fakeProjectService)

func mustFakeProject(opts ...fakeOption) *fakeProjectService {
	f := newFakeProjectService()
	for _, o := range opts {
		o(f)
	}
	return f
}

func getForbidden() fakeOption { return func(f *fakeProjectService) { f.getForbidden = true } }
func getProjectErr(err error) fakeOption {
	return func(f *fakeProjectService) { f.getProjectErr = err }
}
func membersListErr(err error) fakeOption {
	return func(f *fakeProjectService) { f.membersListErr = err }
}

func TestListProjectsErrorAndPagination(t *testing.T) {
	t.Run("internal", func(t *testing.T) {
		svc := newFakeProjectService()
		svc.listErr = errors.New("boom")
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/projects", ""))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d", rec.Code)
		}
	})
	t.Run("offset beyond total", func(t *testing.T) {
		svc := newFakeProjectService()
		svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А"}
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/projects?page=2&per_page=1", ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", rec.Code)
		}
		var resp PaginatedResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if list, ok := resp.Data.([]interface{}); !ok || len(list) != 0 {
			t.Fatalf("want empty list, got %+v", resp.Data)
		}
	})
}

func TestListMembersErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		svc  *fakeProjectService
		want int
	}{
		{"forbidden", mustFakeProject(membersListErr(project.ErrForbidden)), http.StatusForbidden},
		{"internal", mustFakeProject(membersListErr(errors.New("boom"))), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.svc.projects["p-1"] = &project.Project{ID: "p-1"}
			rec := httptest.NewRecorder()
			testRouterWithProjects(tc.svc).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/projects/p-1/members", ""))
			if rec.Code != tc.want {
				t.Fatalf("want %d, got %d", tc.want, rec.Code)
			}
		})
	}
}

func TestAddMemberErrors(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1"}
	base := "/api/v1/projects/p-1/members"

	t.Run("not found", func(t *testing.T) {
		svc.membershipErr = project.ErrNotFound
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, `{"user_id":"u-2","role":"editor"}`))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d", rec.Code)
		}
	})
	t.Run("forbidden", func(t *testing.T) {
		svc.membershipErr = project.ErrForbidden
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, `{"user_id":"u-2","role":"editor"}`))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d", rec.Code)
		}
	})
	t.Run("conflict", func(t *testing.T) {
		svc.membershipErr = errors.New("already member")
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, `{"user_id":"u-2","role":"editor"}`))
		if rec.Code != http.StatusConflict {
			t.Fatalf("want 409, got %d", rec.Code)
		}
	})
	t.Run("invalid json", func(t *testing.T) {
		svc.membershipErr = nil
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, "{bad"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})
}

func TestUpdateMemberRoleErrors(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1"}
	base := "/api/v1/projects/p-1/members/u-2"

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPatch, base, "{bad"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})
	t.Run("invalid role", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPatch, base, `{"role":"boss"}`))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d", rec.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		svc.membershipErr = project.ErrNotFound
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPatch, base, `{"role":"viewer"}`))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d", rec.Code)
		}
	})
	t.Run("forbidden", func(t *testing.T) {
		svc.membershipErr = project.ErrForbidden
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPatch, base, `{"role":"viewer"}`))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d", rec.Code)
		}
	})
	t.Run("conflict", func(t *testing.T) {
		svc.membershipErr = errors.New("noop")
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPatch, base, `{"role":"viewer"}`))
		if rec.Code != http.StatusConflict {
			t.Fatalf("want 409, got %d", rec.Code)
		}
	})
}

func TestRemoveMemberErrors(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1"}
	base := "/api/v1/projects/p-1/members/u-2"
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"not found", project.ErrNotFound, http.StatusNotFound},
		{"forbidden", project.ErrForbidden, http.StatusForbidden},
		{"conflict", errors.New("noop"), http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc.membershipErr = tc.err
			rec := httptest.NewRecorder()
			testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodDelete, base, ""))
			if rec.Code != tc.want {
				t.Fatalf("want %d, got %d", tc.want, rec.Code)
			}
		})
	}
}

func TestAddCommentBranches(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1"}
	base := "/api/v1/projects/p-1/comments"

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, "{bad"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})
	t.Run("empty body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, `{"body":""}`))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d", rec.Code)
		}
	})
	t.Run("too long", func(t *testing.T) {
		rec := httptest.NewRecorder()
		long := `{"body":"` + strings.Repeat("x", 10001) + `"}`
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, long))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d", rec.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		svc2 := newFakeProjectService()
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc2).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-404/comments", `{"body":"x"}`))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d", rec.Code)
		}
	})
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"forbidden", project.ErrForbidden, http.StatusForbidden},
		{"conflict", project.ErrConflict, http.StatusUnprocessableEntity},
		{"internal", errors.New("boom"), http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc.addCommentErr = tc.err
			rec := httptest.NewRecorder()
			testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, `{"body":"x"}`))
			if rec.Code != tc.want {
				t.Fatalf("want %d, got %d", tc.want, rec.Code)
			}
			svc.addCommentErr = nil
		})
	}
}

func TestDeleteCommentBranches(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1"}
	base := "/api/v1/projects/p-1/comments/c-1"

	t.Run("not found", func(t *testing.T) {
		svc2 := newFakeProjectService()
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc2).ServeHTTP(rec, authedRequest(http.MethodDelete, "/api/v1/projects/p-404/comments/c-1", ""))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d", rec.Code)
		}
	})
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"forbidden", project.ErrForbidden, http.StatusForbidden},
		{"conflict", errors.New("noop"), http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc.deleteCommentErr = tc.err
			rec := httptest.NewRecorder()
			testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodDelete, base, ""))
			if rec.Code != tc.want {
				t.Fatalf("want %d, got %d", tc.want, rec.Code)
			}
			svc.deleteCommentErr = nil
		})
	}
}

func TestRequestReviewBranches(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Status: "draft"}
	base := "/api/v1/projects/p-1/review"

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, "{bad"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		svc2 := newFakeProjectService()
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc2).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-404/review", `{"comment":"x"}`))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d", rec.Code)
		}
	})
	t.Run("internal", func(t *testing.T) {
		svc.reviewErr = errors.New("boom")
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, `{"comment":"x"}`))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d", rec.Code)
		}
	})
}

func TestReviewDecisionBranches(t *testing.T) {
	for _, route := range []string{"/sign-off", "/changes"} {
		svc := newFakeProjectService()
		svc.projects["p-1"] = &project.Project{ID: "p-1", Status: "in_review"}
		base := "/api/v1/projects/p-1/reviews/rv-1" + route

		t.Run(route+"/invalid json", func(t *testing.T) {
			rec := httptest.NewRecorder()
			testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, "{bad"))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d", rec.Code)
			}
		})
		for _, tc := range []struct {
			name string
			err  error
			want int
		}{
			{"not found", project.ErrNotFound, http.StatusNotFound},
			{"forbidden", project.ErrForbidden, http.StatusForbidden},
			{"internal", errors.New("boom"), http.StatusInternalServerError},
		} {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				svc.reviewErr = tc.err
				rec := httptest.NewRecorder()
				testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, `{"comment":"x"}`))
				if rec.Code != tc.want {
					t.Fatalf("want %d, got %d", tc.want, rec.Code)
				}
			})
		}
	}
}

func TestApproveConfigurationBranches(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Status: "approved"}
	base := "/api/v1/projects/p-1/configurations/cfg-9/approve"

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, "{bad"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		svc2 := newFakeProjectService()
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc2).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-404/configurations/cfg-9/approve", `{}`))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d", rec.Code)
		}
	})
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"forbidden", project.ErrForbidden, http.StatusForbidden},
		{"internal", errors.New("boom"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc.reviewErr = tc.err
			rec := httptest.NewRecorder()
			testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, base, `{"comment":"x"}`))
			if rec.Code != tc.want {
				t.Fatalf("want %d, got %d", tc.want, rec.Code)
			}
		})
	}
}

func TestGetConfigurationApprovalBranches(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1"}
	base := "/api/v1/projects/p-1/configurations/cfg-9/approval"

	t.Run("not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodGet, base, ""))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d", rec.Code)
		}
	})
	t.Run("forbidden", func(t *testing.T) {
		svc2 := newFakeProjectService()
		svc2.projects["p-1"] = &project.Project{ID: "p-1"}
		svc2.reviewErr = project.ErrForbidden
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc2).ServeHTTP(rec, authedRequest(http.MethodGet, base, ""))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d", rec.Code)
		}
	})
}

func TestListConfigurationsForbiddenAndError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"forbidden", project.ErrForbidden, http.StatusForbidden},
		{"internal", errors.New("boom"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newFakeProjectService()
			svc.projects["p-1"] = &project.Project{ID: "p-1"}
			svc.reviewErr = tc.err
			rec := httptest.NewRecorder()
			testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/projects/p-1/configurations", ""))
			if rec.Code != tc.want {
				t.Fatalf("want %d, got %d", tc.want, rec.Code)
			}
		})
	}
}

func TestPreviewProject(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		svc := newFakeProjectService()
		svc.projects["p-1"] = &project.Project{ID: "p-1"}
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/preview", referenceJSON))
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var c calculationDTO
		if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if c.ProjectID != "p-1" || len(c.Result) == 0 {
			t.Fatalf("unexpected preview: %+v", c)
		}
	})
	t.Run("not found", func(t *testing.T) {
		svc := newFakeProjectService()
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-404/preview", referenceJSON))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d", rec.Code)
		}
	})
	t.Run("forbidden", func(t *testing.T) {
		svc := newFakeProjectService()
		svc.projects["p-1"] = &project.Project{ID: "p-1"}
		svc.calculateErr = project.ErrForbidden
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/preview", referenceJSON))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d", rec.Code)
		}
	})
}
