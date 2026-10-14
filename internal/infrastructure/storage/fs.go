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
	if err := os.MkdirAll(abs, 0o750); err != nil {
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

// contentTypePath — путь к файлу с content-type объекта (DOM-006).
//
// Раньше FileStore игнорировал content-type, поэтому GET /storage/{key}
// всегда отдавал application/octet-stream, хотя SaveExport сохранял
// content-type и возвращал его в DTO. Метаданные лежат рядом с объектом
// под именем <key>.content-type и не попадают в выдачу List.
func contentTypePath(p string) string { return p + ".content-type" }

// Put сохраняет объект (EDR-0026 §3.2).
func (f *FileStore) Put(_ context.Context, key string, data []byte, contentType string) error {
	p, err := f.keyPath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return fmt.Errorf("storage: mkdir: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("storage: write: %w", err)
	}
	if contentType != "" {
		if err := os.WriteFile(contentTypePath(p), []byte(contentType), 0o600); err != nil {
			return fmt.Errorf("storage: write content-type: %w", err)
		}
	}
	return nil
}

// Get возвращает данные объекта и его content-type (EDR-0026 §3.2).
func (f *FileStore) Get(_ context.Context, key string) ([]byte, string, error) {
	p, err := f.keyPath(key)
	if err != nil {
		return nil, "", err
	}
	// #nosec G304 -- file store reads objects by validated key under root.
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("storage: read: %w", err)
	}
	ct, err := os.ReadFile(contentTypePath(p)) //nolint:gosec // путь выведен из проверенного ключа
	if err != nil {
		return data, "", nil // метаданных нет (legacy-объект) — не ошибка
	}
	return data, string(ct), nil
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
