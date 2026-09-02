package http

import (
	"net/http"
	"strconv"
)

const (
	// DefaultMaxBodySize — 10 MB по умолчанию.
	DefaultMaxBodySize int64 = 10 << 20
	// MaxUploadSize — 50 MB для файловых загрузок.
	MaxUploadSize int64 = 50 << 20
)

// BodySizeLimitMiddleware ограничивает размер request body.
// Возвращает 413 Request Entity Too Large при превышении.
func BodySizeLimitMiddleware(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxSize {
				writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large",
					"Request body too large: "+itoa64(r.ContentLength)+" bytes, max: "+itoa64(maxSize)+" bytes")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			next.ServeHTTP(w, r)
		})
	}
}

// BodySizeLimitDefault возвращает middleware с DefaultMaxBodySize.
func BodySizeLimitDefault() func(http.Handler) http.Handler {
	return BodySizeLimitMiddleware(DefaultMaxBodySize)
}

// BodySizeLimitUpload возвращает middleware для файловых загрузок (50MB).
func BodySizeLimitUpload() func(http.Handler) http.Handler {
	return BodySizeLimitMiddleware(MaxUploadSize)
}

// itoa64 — быстрое преобразование int64 → string.
func itoa64(n int64) string {
	return strconv.FormatInt(n, 10)
}
