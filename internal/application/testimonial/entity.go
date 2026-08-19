// Package testimonial реализует прикладной слой отзывов клиентов
// (клиентского сайта, Store). Отзывы создаёт и публикует менеджер в
// админке; публичный эндпоинт отдаёт только опубликованные. Application
// зависит только от порта Repository (BE-0005) — инверсия зависимостей.
package testimonial

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Testimonial — отзыв клиента о заказе/лестнице.
type Testimonial struct {
	ID        string
	TenantID  string
	Author    string
	Text      string
	Rating    int
	Published bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	// ErrNotFound — отзыв не найден (или вне tenant).
	ErrNotFound = errors.New("testimonial: not found")
	// ErrInvalid — некорректные входные данные.
	ErrInvalid = errors.New("testimonial: invalid input")
)

// Repository — порт доступа к данным отзывов.
type Repository interface {
	// Create сохраняет новый отзыв; ID присваивается внутри.
	Create(ctx context.Context, t *Testimonial) error
	// Get возвращает отзыв по ID в tenant.
	Get(ctx context.Context, tenantID, id string) (*Testimonial, error)
	// ListAll возвращает все отзывы tenant (убывание created_at).
	ListAll(ctx context.Context, tenantID string) ([]*Testimonial, error)
	// ListPublished возвращает опубликованные отзывы tenant (для лендинга).
	ListPublished(ctx context.Context, tenantID string) ([]*Testimonial, error)
	// Update обновляет поля отзыва (author, text, rating) и флаг публикации.
	Update(ctx context.Context, tenantID string, t *Testimonial) error
	// Delete удаляет отзыв (tenant-скоуп).
	Delete(ctx context.Context, tenantID, id string) error
}

// Service — прикладной сервис отзывов.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService создаёт сервис отзывов.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// valid проверяет обязательные поля и диапазон рейтинга.
func valid(author, text string, rating int) error {
	if strings.TrimSpace(author) == "" {
		return fmt.Errorf("%w: author required", ErrInvalid)
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("%w: text required", ErrInvalid)
	}
	if rating < 1 || rating > 5 {
		return fmt.Errorf("%w: rating must be 1..5", ErrInvalid)
	}
	return nil
}

// Create сохраняет новый отзыв (неопубликованный).
func (s *Service) Create(ctx context.Context, tenantID, author, text string, rating int) (*Testimonial, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenant required", ErrInvalid)
	}
	if err := valid(author, text, rating); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	t := &Testimonial{
		TenantID: tenantID,
		Author:   strings.TrimSpace(author),
		Text:     strings.TrimSpace(text),
		Rating:   rating,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// ListAll возвращает все отзывы tenant (админка).
func (s *Service) ListAll(ctx context.Context, tenantID string) ([]*Testimonial, error) {
	return s.repo.ListAll(ctx, tenantID)
}

// ListPublished возвращает опубликованные отзывы (лендинг store).
func (s *Service) ListPublished(ctx context.Context, tenantID string) ([]*Testimonial, error) {
	return s.repo.ListPublished(ctx, tenantID)
}

// Update обновляет отзыв: текст, автор, рейтинг и флаг публикации.
func (s *Service) Update(ctx context.Context, tenantID, id, author, text string, rating int, published bool) (*Testimonial, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: id required", ErrInvalid)
	}
	if err := valid(author, text, rating); err != nil {
		return nil, err
	}
	t, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	t.Author = strings.TrimSpace(author)
	t.Text = strings.TrimSpace(text)
	t.Rating = rating
	t.Published = published
	t.UpdatedAt = s.now().UTC()
	if err := s.repo.Update(ctx, tenantID, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Delete удаляет отзыв.
func (s *Service) Delete(ctx context.Context, tenantID, id string) error {
	return s.repo.Delete(ctx, tenantID, id)
}