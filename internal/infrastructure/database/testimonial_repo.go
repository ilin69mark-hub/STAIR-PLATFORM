package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/testimonial"
)

// TestimonialRepository — реализация порта testimonial.Repository на
// PostgreSQL. Отзывы клиентов (store): CRUD для админки + опубликованные
// для публичного лендинга.
type TestimonialRepository struct {
	pool *pgxpool.Pool
}

// NewTestimonialRepository создаёт репозиторий отзывов.
func NewTestimonialRepository(pool *pgxpool.Pool) *TestimonialRepository {
	return &TestimonialRepository{pool: pool}
}

var _ testimonial.Repository = (*TestimonialRepository)(nil)

// scanTestimonial читает строку отзыва.
func scanTestimonial(row pgx.Row) (*testimonial.Testimonial, error) {
	var t testimonial.Testimonial
	if err := row.Scan(&t.ID, &t.TenantID, &t.Author, &t.Text, &t.Rating,
		&t.Published, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

// Create сохраняет новый отзыв.
func (r *TestimonialRepository) Create(ctx context.Context, t *testimonial.Testimonial) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO testimonials (tenant_id, author, text, rating, published)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, updated_at`,
		t.TenantID, t.Author, t.Text, t.Rating, t.Published,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("testimonial: create: %w", err)
	}
	return nil
}

// Get возвращает отзыв по ID в tenant.
func (r *TestimonialRepository) Get(ctx context.Context, tenantID, id string) (*testimonial.Testimonial, error) {
	t, err := scanTestimonial(r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, author, text, rating, published, created_at, updated_at
		 FROM testimonials WHERE tenant_id = $1 AND id = $2`, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, testimonial.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("testimonial: get: %w", err)
	}
	return t, nil
}

// ListAll возвращает все отзывы tenant (сначала новые).
func (r *TestimonialRepository) ListAll(ctx context.Context, tenantID string) ([]*testimonial.Testimonial, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, author, text, rating, published, created_at, updated_at
		 FROM testimonials WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("testimonial: list all: %w", err)
	}
	defer rows.Close()
	return r.scanAll(rows)
}

// ListPublished возвращает опубликованные отзывы tenant (для лендинга).
func (r *TestimonialRepository) ListPublished(ctx context.Context, tenantID string) ([]*testimonial.Testimonial, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, author, text, rating, published, created_at, updated_at
		 FROM testimonials WHERE tenant_id = $1 AND published ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("testimonial: list published: %w", err)
	}
	defer rows.Close()
	return r.scanAll(rows)
}

func (r *TestimonialRepository) scanAll(rows pgx.Rows) ([]*testimonial.Testimonial, error) {
	var out []*testimonial.Testimonial
	for rows.Next() {
		var t testimonial.Testimonial
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Author, &t.Text, &t.Rating,
			&t.Published, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("testimonial: scan: %w", err)
		}
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("testimonial: rows: %w", err)
	}
	return out, nil
}

// Update обновляет отзыв (tenant-скоуп).
func (r *TestimonialRepository) Update(ctx context.Context, tenantID string, t *testimonial.Testimonial) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE testimonials SET author = $1, text = $2, rating = $3, published = $4, updated_at = now()
		 WHERE tenant_id = $5 AND id = $6`,
		t.Author, t.Text, t.Rating, t.Published, tenantID, t.ID)
	if err != nil {
		return fmt.Errorf("testimonial: update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return testimonial.ErrNotFound
	}
	return nil
}

// Delete удаляет отзыв (tenant-скоуп).
func (r *TestimonialRepository) Delete(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM testimonials WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("testimonial: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return testimonial.ErrNotFound
	}
	return nil
}