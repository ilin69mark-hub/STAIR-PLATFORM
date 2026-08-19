package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/application/testimonial"
)

// fakeTestimonialService — тестовая реализация TestimonialService.
type fakeTestimonialService struct {
	list      []*testimonial.Testimonial
	published []*testimonial.Testimonial
	created   *testimonial.Testimonial
	updated   *testimonial.Testimonial
	deleted   *string
	err       error
}

func (f *fakeTestimonialService) Create(_ context.Context, tenantID, author, text string, rating int) (*testimonial.Testimonial, error) {
	if f.err != nil {
		return nil, f.err
	}
	if author == "" || text == "" || rating < 1 || rating > 5 {
		return nil, testimonial.ErrInvalid
	}
	f.created = &testimonial.Testimonial{ID: "t-1", TenantID: tenantID, Author: author, Text: text, Rating: rating}
	return f.created, nil
}
func (f *fakeTestimonialService) ListAll(_ context.Context, _ string) ([]*testimonial.Testimonial, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}
func (f *fakeTestimonialService) ListPublished(_ context.Context, _ string) ([]*testimonial.Testimonial, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.published, nil
}
func (f *fakeTestimonialService) Update(_ context.Context, tenantID, id, author, text string, rating int, published bool) (*testimonial.Testimonial, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.updated = &testimonial.Testimonial{ID: id, TenantID: tenantID, Author: author, Text: text, Rating: rating, Published: published}
	return f.updated, nil
}
func (f *fakeTestimonialService) Delete(_ context.Context, _ string, id string) error {
	f.deleted = &id
	return f.err
}

func testimonialsPublicRouter(svc TestimonialService) http.Handler {
	cfg := DefaultConfig()
	cfg.Testimonials = svc
	return NewRouter(stair.NewService(), nil, testAuth{}, cfg)
}

func testimonialsAdminRouter(svc TestimonialService) http.Handler {
	cfg := DefaultConfig()
	cfg.Testimonials = svc
	return NewRouter(stair.NewService(), nil, adminAuth{}, cfg)
}

func TestPublicListTestimonialsPublished(t *testing.T) {
	fake := &fakeTestimonialService{published: []*testimonial.Testimonial{
		{ID: "t-1", Author: "Иван", Text: "Отличная лестница", Rating: 5, Published: true},
	}}
	router := testimonialsPublicRouter(fake)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/testimonials", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"author":"Иван"`) || !strings.Contains(body, `"rating":5`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestPublicListTestimonialsFiltersDrafts(t *testing.T) {
	// Публичный метод сервиса возвращает только опубликованные; проверим,
	// что эндпоинт вызывает именно ListPublished (не ListAll).
	fake := &fakeTestimonialService{
		list: []*testimonial.Testimonial{
			{ID: "t-draft", Author: "Скрытый", Text: "черновик", Rating: 1, Published: false},
		},
		published: []*testimonial.Testimonial{},
	}
	router := testimonialsPublicRouter(fake)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/testimonials", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "черновик") {
		t.Fatalf("draft leaked to public: %s", rec.Body.String())
	}
}

func TestAdminCreateTestimonial(t *testing.T) {
	fake := &fakeTestimonialService{}
	router := testimonialsAdminRouter(fake)
	body := `{"author":"Мария","text":"Монтаж вовремя","rating":5,"published":false}`
	req := authedRequest(http.MethodPost, "/api/v1/admin/testimonials", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.created == nil || fake.created.Rating != 5 {
		t.Fatalf("create not routed: %+v", fake.created)
	}
}

func TestAdminCreateTestimonialInvalid(t *testing.T) {
	fake := &fakeTestimonialService{}
	router := testimonialsAdminRouter(fake)
	body := `{"author":"","text":"пустой автор","rating":0}`
	req := authedRequest(http.MethodPost, "/api/v1/admin/testimonials", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateTestimonial(t *testing.T) {
	fake := &fakeTestimonialService{}
	router := testimonialsAdminRouter(fake)
	body := `{"author":"Мария","text":"обновлённый","rating":4,"published":true}`
	req := authedRequest(http.MethodPatch, "/api/v1/admin/testimonials/t-1", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.updated == nil || !fake.updated.Published || fake.updated.Rating != 4 {
		t.Fatalf("unexpected update: %+v", fake.updated)
	}
}

func TestAdminDeleteTestimonial(t *testing.T) {
	fake := &fakeTestimonialService{}
	router := testimonialsAdminRouter(fake)
	req := authedRequest(http.MethodDelete, "/api/v1/admin/testimonials/t-1", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.deleted == nil || *fake.deleted != "t-1" {
		t.Fatalf("delete not routed: %+v", fake.deleted)
	}
}

func TestAdminTestimonialsRequireAdmin(t *testing.T) {
	fake := &fakeTestimonialService{}
	userRouter := testimonialsPublicRouter(fake)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/testimonials", ""},
		{http.MethodPost, "/api/v1/admin/testimonials", `{"author":"А","text":"т","rating":5}`},
		{http.MethodDelete, "/api/v1/admin/testimonials/t-1", ""},
	} {
		req := authedRequest(tc.method, tc.path, tc.body)
		rec := httptest.NewRecorder()
		userRouter.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s: expected 403, got %d", tc.method, tc.path, rec.Code)
		}
	}
}
