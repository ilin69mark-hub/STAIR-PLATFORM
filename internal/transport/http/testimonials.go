package http

import (
	"context"
	"errors"
	"net/http"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/testimonial"
)

// TestimonialService — прикладной интерфейс отзывов для транспортного слоя.
type TestimonialService interface {
	Create(ctx context.Context, tenantID, author, text string, rating int) (*testimonial.Testimonial, error)
	ListAll(ctx context.Context, tenantID string) ([]*testimonial.Testimonial, error)
	ListPublished(ctx context.Context, tenantID string) ([]*testimonial.Testimonial, error)
	Update(ctx context.Context, tenantID, id, author, text string, rating int, published bool) (*testimonial.Testimonial, error)
	Delete(ctx context.Context, tenantID, id string) error
}

// testimonialDTO — представление отзыва в API.
type testimonialDTO struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	Rating    int    `json:"rating"`
	Published bool   `json:"published"`
	CreatedAt string `json:"created_at"`
}

func toTestimonialDTO(t *testimonial.Testimonial) testimonialDTO {
	return testimonialDTO{
		ID:        t.ID,
		Author:    t.Author,
		Text:      t.Text,
		Rating:    t.Rating,
		Published: t.Published,
		CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// handlePublicListTestimonials — GET /api/v1/public/testimonials.
// Опубликованные отзывы для лендинга (store), без аутентификации.
// Привязывается к дефолтному tenant (как регистрация и консультации).
//
//	200 — список отзывов; 500 — нет дефолтного tenant.
func handlePublicListTestimonials(svc TestimonialService, authSvc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant, err := authSvc.DefaultTenant(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		items, err := svc.ListPublished(r.Context(), tenant.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		out := make([]testimonialDTO, 0, len(items))
		for _, t := range items {
			out = append(out, toTestimonialDTO(t))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// handleAdminListTestimonials — GET /api/v1/admin/testimonials (auth+admin).
// Все отзывы tenant для менеджера (включая черновики).
func handleAdminListTestimonials(svc TestimonialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionTestimonialsList) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора")
			return
		}
		items, err := svc.ListAll(r.Context(), tenantID(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		out := make([]testimonialDTO, 0, len(items))
		for _, t := range items {
			out = append(out, toTestimonialDTO(t))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type upsertTestimonialRequest struct {
	Author    string `json:"author"`
	Text      string `json:"text"`
	Rating    int    `json:"rating"`
	Published bool   `json:"published"`
}

// handleAdminCreateTestimonial — POST /api/v1/admin/testimonials (auth+admin).
// Создаёт отзыв (по умолчанию неопубликованный черновик).
//
//	201 — отзыв создан; 400 — битый JSON; 403 — нет права; 422 — невалидный вход.
func handleAdminCreateTestimonial(svc TestimonialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionTestimonialsManage) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора")
			return
		}
		var req upsertTestimonialRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		t, err := svc.Create(r.Context(), tenantID(r.Context()), req.Author, req.Text, req.Rating)
		if err != nil {
			if errors.Is(err, testimonial.ErrInvalid) {
				writeInputError(w, "invalid_input", err)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusCreated, toTestimonialDTO(t))
	}
}

// handleAdminUpdateTestimonial — PATCH /api/v1/admin/testimonials/{id}
// (auth+admin). Обновляет текст, рейтинг и флаг публикации.
//
//	200 — отзыв обновлён; 403 — нет права; 404 — не найден; 422 — невалидный вход.
func handleAdminUpdateTestimonial(svc TestimonialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionTestimonialsManage) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора")
			return
		}
		var req upsertTestimonialRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		t, err := svc.Update(r.Context(), tenantID(r.Context()), r.PathValue("id"),
			req.Author, req.Text, req.Rating, req.Published)
		if err != nil {
			if errors.Is(err, testimonial.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "Отзыв не найден.")
				return
			}
			if errors.Is(err, testimonial.ErrInvalid) {
				writeInputError(w, "invalid_input", err)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusOK, toTestimonialDTO(t))
	}
}

// handleAdminDeleteTestimonial — DELETE /api/v1/admin/testimonials/{id}
// (auth+admin).
//
//	204 — удалён; 403 — нет права; 404 — не найден.
func handleAdminDeleteTestimonial(svc TestimonialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionTestimonialsManage) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора")
			return
		}
		if err := svc.Delete(r.Context(), tenantID(r.Context()), r.PathValue("id")); err != nil {
			if errors.Is(err, testimonial.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "Отзыв не найден.")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
