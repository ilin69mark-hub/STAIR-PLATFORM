package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileStore — ObjectStore на локальной файловой системе (EDR-0026 §3.2).
// Объекты лежат в root/<key>; ключ очищается и не может выйти за root.
type FileStore struct {
	root string
}

// NewFileStore создаёт filesystem-бэкенд с корнем root.
func NewFileStore(root string) (*FileStore, error) {
	if root == "" {
		return nil, fmt.Errorf("%w: filesystem root required", ErrInvalid)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("storage: filesystem root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create root: %w", err)
	}
	return &FileStore{root: abs}, nil
}

// keyPath нормализует ключ в абсолютный путь внутри root.
func (f *FileStore) keyPath(key string) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	p := filepath.Join(f.root, filepath.FromSlash(key))
	if !strings.HasPrefix(p, f.root) {
		return "", fmt.Errorf("%w: key escapes root", ErrInvalid)
	}
	return p, nil
}

// Put сохраняет объект (EDR-0026 §3.2).
func (f *FileStore) Put(_ context.Context, key string, data []byte, _ string) error {
	p, err := f.keyPath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("storage: mkdir: %w", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return fmt.Errorf("storage: write: %w", err)
	}
	return nil
}

// Get возвращает данные объекта (EDR-0026 §3.2).
func (f *FileStore) Get(_ context.Context, key string) ([]byte, error) {
	p, err := f.keyPath(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("storage: read: %w", err)
	}
	return data, nil
}

// Delete удаляет объект (EDR-0026 §3.2).
func (f *FileStore) Delete(_ context.Context, key string) error {
	p, err := f.keyPath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("storage: delete: %w", err)
	}
	return nil
}
