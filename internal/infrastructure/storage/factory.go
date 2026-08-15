package storage

import "fmt"

// Backend names (EDR-0026 §3.1).
const (
	BackendFilesystem = "filesystem"
	BackendS3         = "s3"
)

// NewObjectStore выбирает бэкенд по backend (STAIR_STORAGE_BACKEND):
// "filesystem" (по умолчанию) или "s3" (EDR-0026 §3.1).
func NewObjectStore(backend string, o Options) (ObjectStore, error) {
	switch backend {
	case "", BackendFilesystem:
		return NewFileStore(o.FSRoot)
	case BackendS3:
		return NewS3Store(o)
	default:
		return nil, fmt.Errorf("storage: unknown backend %q", backend)
	}
}
