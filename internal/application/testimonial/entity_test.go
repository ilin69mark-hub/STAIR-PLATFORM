package testimonial

import (
	"context"
	"testing"
)

// fakeRepo — простейшая in-memory реализация Repository для unit-тестов.
type fakeRepo struct {
	stored []*Testimonial
	latest *Testimonial
}

func (f *fakeRepo) Create(_ context.Context, t *Testimonial) error {
	t.ID = "t-new"
	f.latest = t
	f.stored = append(f.stored, t)
	return nil
}
func (f *fakeRepo) Get(_ context.Context, tenantID, id string) (*Testimonial, error) {
	for _, t := range f.stored {
		if t.ID == id && t.TenantID == tenantID {
			return t, nil
		}
	}
	return nil, ErrNotFound
}
func (f *fakeRepo) ListAll(_ context.Context, _ string) ([]*Testimonial, error) { return f.stored, nil }
func (f *fakeRepo) ListPublished(_ context.Context, _ string) ([]*Testimonial, error) {
	var out []*Testimonial
	for _, t := range f.stored {
		if t.Published {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fakeRepo) Update(_ context.Context, tenantID string, t *Testimonial) error {
	for i, st := range f.stored {
		if st.ID == t.ID {
			f.stored[i] = t
			f.latest = t
			return nil
		}
	}
	return ErrNotFound
}
func (f *fakeRepo) Delete(_ context.Context, tenantID, id string) error {
	for i, st := range f.stored {
		if st.ID == id {
			f.stored = append(f.stored[:i], f.stored[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func TestCreateValid(t *testing.T) {
	r := &fakeRepo{}
	s := NewService(r)
	tm, err := s.Create(context.Background(), "t-1", "Иван", "Отличная работа", 5)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tm.Published {
		t.Fatal("new testimonial must be unpublished")
	}
	if tm.Rating != 5 || tm.Author != "Иван" {
		t.Fatalf("unexpected: %+v", tm)
	}
}

func TestCreateInvalidRating(t *testing.T) {
	s := NewService(&fakeRepo{})
	if _, err := s.Create(context.Background(), "t-1", "И", "текст", 6); err == nil {
		t.Fatal("expected ErrInvalid for rating 6")
	}
	if _, err := s.Create(context.Background(), "t-1", "И", "текст", 0); err == nil {
		t.Fatal("expected ErrInvalid for rating 0")
	}
}

func TestCreateMissingFields(t *testing.T) {
	s := NewService(&fakeRepo{})
	if _, err := s.Create(context.Background(), "t-1", "", "текст", 5); err == nil {
		t.Fatal("expected ErrInvalid for empty author")
	}
	if _, err := s.Create(context.Background(), "t-1", "И", "  ", 5); err == nil {
		t.Fatal("expected ErrInvalid for empty text")
	}
}

func TestUpdatePublishes(t *testing.T) {
	r := &fakeRepo{}
	s := NewService(r)
	if _, err := s.Create(context.Background(), "t-1", "И", "текст", 5); err != nil {
		t.Fatal(err)
	}
	tm, err := s.Update(context.Background(), "t-1", "t-new", "И2", "новый текст", 4, true)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !tm.Published || tm.Rating != 4 || tm.Author != "И2" {
		t.Fatalf("unexpected update: %+v", tm)
	}
}

func TestListPublishedOnly(t *testing.T) {
	r := &fakeRepo{}
	s := NewService(r)
	_, _ = s.Create(context.Background(), "t-1", "А", "черновик", 3)
	_, _ = s.Update(context.Background(), "t-1", "t-new", "А", "опубликованный", 5, true)
	pub, err := s.ListPublished(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("list published: %v", err)
	}
	if len(pub) != 1 || pub[0].Text != "опубликованный" {
		t.Fatalf("published = %+v, want only one", pub)
	}
}

func TestDeleteNotFound(t *testing.T) {
	s := NewService(&fakeRepo{})
	if err := s.Delete(context.Background(), "t-1", "missing"); err == nil {
		t.Fatal("expected ErrNotFound")
	}
}
