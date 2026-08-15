// Package storage реализует прикладной сервис объектного хранилища
// (EDR-0026, Phase E E5): сохранение и загрузка каркасных документов
// (экспорт CAD) с tenant-скоупом ключей. Мета-уровень зависит от
// ObjectStore (инфраструктура), но не от HTTP/БД (ADR-0006).
package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ObjectStore — порт инфраструктуры (реализует
// internal/infrastructure/storage.ObjectStore).
type ObjectStore interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// Service — прикладной сервис хранилища (EDR-0026 §3.4).
type Service struct {
	store ObjectStore
	now   func() time.Time
}

// NewService создаёт сервис хранилища.
func NewService(store ObjectStore) *Service {
	return &Service{store: store, now: time.Now}
}

// ExportRef — результат сохранения экспорта (EDR-0026 §3.5).
type ExportRef struct {
	Key         string `json:"key"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

// objectKey строит иерархический ключ {tenant}/{category}/{filename}
// (EDR-0026 §3.4); filename нормализуется (без ".." и слэшей).
func objectKey(tenantID, category, filename string) (string, error) {
	if tenantID == "" || category == "" || filename == "" {
		return "", fmt.Errorf("storage: key parts required")
	}
	if strings.Contains(filename, "/") || filename == ".." {
		return "", fmt.Errorf("storage: invalid filename %q", filename)
	}
	return tenantID + "/" + category + "/" + filename, nil
}

// SaveExport сохраняет документ экспорта и возвращает ссылку
// (EDR-0026 §3.4). Ключ tenant-скоупed: клиент не может записать в чужой
// tenant.
func (s *Service) SaveExport(ctx context.Context, tenantID, category, filename string, data []byte, contentType string) (*ExportRef, error) {
	key, err := objectKey(tenantID, category, filename)
	if err != nil {
		return nil, err
	}
	if err := s.store.Put(ctx, key, data, contentType); err != nil {
		return nil, fmt.Errorf("storage: save: %w", err)
	}
	return &ExportRef{Key: key, ContentType: contentType, Size: len(data)}, nil
}

// Load возвращает данные и content-type по ключу. Ключ должен принадлежать
// tenantID (префикс), иначе — ошибка скоупа.
func (s *Service) Load(ctx context.Context, tenantID, key string) ([]byte, string, error) {
	if !strings.HasPrefix(key, tenantID+"/") {
		return nil, "", fmt.Errorf("storage: object outside tenant scope")
	}
	data, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, "", err
	}
	return data, "", nil
}

// Delete удаляет объект tenant'а по ключу (скоуп как у Load).
func (s *Service) Delete(ctx context.Context, tenantID, key string) error {
	if !strings.HasPrefix(key, tenantID+"/") {
		return fmt.Errorf("storage: object outside tenant scope")
	}
	return s.store.Delete(ctx, key)
}
