// Package storage реализует абстракцию объектного хранилища (EDR-0026,
// Phase E E5): ObjectStore с двумя бэкендами — локальная файловая система
// и S3-совместимое хранилище (SigV4 на чистой stdlib). Ключи иерархические
// ({tenant}/{category}/{filename}) и tenant-скоупed. Пакет зависит только
// от stdlib (DEV-0009).
package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ObjectStore — абстракция объектного хранилища (EDR-0026 §3.1).
// Ключ — иерархический путь ({tenant}/{category}/{filename}). Реализации
// отклоняют ключи с ".." и ведущим слэшем (ErrInvalid).
type ObjectStore interface {
	// Put сохраняет объект; при необходимости создаёт промежуточные пути.
	Put(ctx context.Context, key string, data []byte, contentType string) error
	// Get возвращает данные объекта.
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete удаляет объект; отсутствующий объект — ErrNotFound.
	Delete(ctx context.Context, key string) error
}

// ErrNotFound — объект не найден.
var ErrNotFound = errors.New("storage: not found")

// ErrInvalid — некорректный ключ (пустой, "..", ведущий слэш).
var ErrInvalid = errors.New("storage: invalid key")

// Options — параметры создания ObjectStore (EDR-0026 §3.1).
type Options struct {
	// FSRoot — корневой каталог filesystem-бэкенда (STAIR_STORAGE_DIR).
	FSRoot string
	// Endpoint — S3 endpoint (STAIR_S3_ENDPOINT), напр. http://localhost:9000.
	Endpoint string
	// Bucket — S3 bucket (STAIR_S3_BUCKET).
	Bucket string
	// Region — S3 region (STAIR_S3_REGION).
	Region string
	// AccessKey — S3 access key (STAIR_S3_ACCESS_KEY).
	AccessKey string
	// SecretKey — S3 secret key (STAIR_S3_SECRET_KEY).
	SecretKey string
	// PathStyle — path-style обращение (bucket в пути; MinIO-совместимо).
	PathStyle bool
}

// ValidateKey проверяет инварианты ключа (EDR-0026 §4.1): непустой,
// без "..", без ведущего/дублирующего слэша.
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: key required", ErrInvalid)
	}
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("%w: leading slash", ErrInvalid)
	}
	for _, part := range strings.Split(key, "/") {
		if part == ".." || part == "." {
			return fmt.Errorf("%w: invalid segment %q", ErrInvalid, part)
		}
		if part == "" {
			return fmt.Errorf("%w: empty segment", ErrInvalid)
		}
	}
	return nil
}
