package database

import (
	"context"
	"os"
	"testing"

	"stairplatform/internal/application/testimonial"
)

func newTestimonialRepo(t *testing.T) (*TestimonialRepository, *ProjectRepository) {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return NewTestimonialRepository(pool), NewProjectRepository(pool)
}

// TestTestimonialCRUD: создание, чтение и обновление отзыва.
func TestTestimonialCRUD(t *testing.T) {
	repo, pr := newTestimonialRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)

	tm := &testimonial.Testimonial{
		TenantID: tenant, Author: "Иван", Text: "Отличная лестница", Rating: 5,
	}
	if err := repo.Create(ctx, tm); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tm.ID == "" {
		t.Fatal("expected generated id")
	}
	got, err := repo.Get(ctx, tenant, tm.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Rating != 5 || got.Published {
		t.Fatalf("unexpected: %+v", got)
	}

	got.Published = true
	got.Rating = 4
	if err := repo.Update(ctx, tenant, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, err := repo.Get(ctx, tenant, tm.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if !after.Published || after.Rating != 4 {
		t.Fatalf("update not persisted: %+v", after)
	}
}

// TestTestimonialListPublished: лендинг получает только опубликованные.
func TestTestimonialListPublished(t *testing.T) {
	repo, pr := newTestimonialRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)

	draft := &testimonial.Testimonial{TenantID: tenant, Author: "А", Text: "черновик", Rating: 2}
	if err := repo.Create(ctx, draft); err != nil {
		t.Fatal(err)
	}
	pub := &testimonial.Testimonial{TenantID: tenant, Author: "Б", Text: "опубликованный", Rating: 5}
	if err := repo.Create(ctx, pub); err != nil {
		t.Fatal(err)
	}
	pub.Published = true
	if err := repo.Update(ctx, tenant, pub); err != nil {
		t.Fatal(err)
	}

	items, err := repo.ListPublished(ctx, tenant)
	if err != nil {
		t.Fatalf("ListPublished: %v", err)
	}
	found, leaked := false, false
	for _, it := range items {
		if it.Text == "опубликованный" {
			found = true
		}
		if it.Text == "черновик" {
			leaked = true
		}
	}
	if !found || leaked {
		t.Fatalf("published filter failed: found=%v leaked=%v items=%+v", found, leaked, items)
	}
}

func TestTestimonialDeleteAndScoped(t *testing.T) {
	repo, pr := newTestimonialRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)

	tm := &testimonial.Testimonial{TenantID: tenant, Author: "В", Text: "удалить", Rating: 3}
	if err := repo.Create(ctx, tm); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, tenant, tm.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, tenant, tm.ID); err == nil {
		t.Fatal("expected ErrNotFound after delete")
	}
	if _, err := repo.Get(ctx, "other-tenant", tm.ID); err == nil {
		t.Fatal("expected ErrNotFound from foreign tenant")
	}
}
