package storage

import (
	"context"
	"sync"
	"testing"

	infra "stairplatform/internal/infrastructure/storage"
)

// fakeStore — in-memory ObjectStore.
type fakeStore struct {
	mu     sync.Mutex
	objs   map[string][]byte
	types  map[string]string
	putErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{objs: map[string][]byte{}, types: map[string]string{}}
}

func (f *fakeStore) Put(_ context.Context, key string, data []byte, contentType string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putErr != nil {
		return f.putErr
	}
	f.objs[key] = data
	f.types[key] = contentType
	return nil
}

func (f *fakeStore) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.objs[key]
	if !ok {
		return nil, infra.ErrNotFound
	}
	return d, nil
}

func (f *fakeStore) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.objs[key]; !ok {
		return infra.ErrNotFound
	}
	delete(f.objs, key)
	return nil
}

// TestSaveExportKey — ключ имеет tenant-префикс и категорию.
func TestSaveExportKey(t *testing.T) {
	s := NewService(newFakeStore())
	ref, err := s.SaveExport(context.Background(), "t-1", "cad", "project-1.dxf", []byte("x"), "application/dxf")
	if err != nil {
		t.Fatalf("SaveExport: %v", err)
	}
	if ref.Key != "t-1/cad/project-1.dxf" || ref.Size != 1 || ref.ContentType != "application/dxf" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

// TestSaveExportInvalidFilename — слэш в filename отклоняется.
func TestSaveExportInvalidFilename(t *testing.T) {
	s := NewService(newFakeStore())
	if _, err := s.SaveExport(context.Background(), "t-1", "cad", "a/b.dxf", []byte("x"), ""); err == nil {
		t.Fatal("SaveExport with slash filename must error")
	}
}

// TestLoadScope — запрос ключа чужого tenant отклоняется.
func TestLoadScope(t *testing.T) {
	store := newFakeStore()
	if err := store.Put(context.Background(), "t-2/cad/f.dxf", []byte("x"), ""); err != nil {
		t.Fatal(err)
	}
	s := NewService(store)
	if _, _, err := s.Load(context.Background(), "t-1", "t-2/cad/f.dxf"); err == nil {
		t.Fatal("Load of foreign tenant key must error")
	}
	if err := s.Delete(context.Background(), "t-1", "t-2/cad/f.dxf"); err == nil {
		t.Fatal("Delete of foreign tenant key must error")
	}
}
