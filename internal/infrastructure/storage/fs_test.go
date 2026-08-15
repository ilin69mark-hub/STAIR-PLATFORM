package storage

import (
	"context"
	"errors"
	"testing"
)

// TestFileStorePutGetDelete — полный жизненный цикл объекта.
func TestFileStorePutGetDelete(t *testing.T) {
	s, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	ctx := context.Background()

	if err := s.Put(ctx, "t-1/exports/cad/project-1.dxf", []byte("x0"), "application/dxf"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, "t-1/exports/cad/project-1.dxf")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "x0" {
		t.Fatalf("data mismatch: %q", got)
	}
	if err := s.Delete(ctx, "t-1/exports/cad/project-1.dxf"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "t-1/exports/cad/project-1.dxf"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: err = %v, want ErrNotFound", err)
	}
}

// TestFileStoreInvalidKeys — ключи с ".." и ведущим слэшем отклоняются.
func TestFileStoreInvalidKeys(t *testing.T) {
	s, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	ctx := context.Background()
	for _, key := range []string{"", "../evil", "/abs", "a//b"} {
		if err := s.Put(ctx, key, []byte("x"), "text/plain"); !errors.Is(err, ErrInvalid) {
			t.Errorf("Put(%q): err = %v, want ErrInvalid", key, err)
		}
	}
}

// TestFileStoreEscapeRoot — ключ не может выйти за root.
func TestFileStoreEscapeRoot(t *testing.T) {
	s, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if err := s.Put(context.Background(), "a/../../outside", []byte("x"), "text/plain"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("escape: err = %v, want ErrInvalid", err)
	}
}

// TestValidateKey — базовые инварианты ключа.
func TestValidateKey(t *testing.T) {
	cases := []struct {
		key string
		ok  bool
	}{
		{"t/p/f", true},
		{"a", true},
		{"", false},
		{"..", false},
		{"a/../b", false},
		{"/a", false},
		{"a//b", false},
	}
	for _, c := range cases {
		err := ValidateKey(c.key)
		if c.ok && err != nil {
			t.Errorf("ValidateKey(%q): %v", c.key, err)
		}
		if !c.ok && !errors.Is(err, ErrInvalid) {
			t.Errorf("ValidateKey(%q): err = %v, want ErrInvalid", c.key, err)
		}
	}
}
