package http

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strings"
	"sync"

	"stairplatform/internal/infrastructure/metrics"
)

var (
	compressionRegistry = metrics.NewRegistry()

	compressionRequests = compressionRegistry.Counter(
		"compression_requests_total",
		"Total compression decisions",
		"result", // "compressed", "skipped_small", "skipped_binary", "skipped_method"
	)
	compressionRatio = compressionRegistry.Histogram(
		"compression_ratio",
		"Compression ratio (compressed/original)",
		nil,
	)
	compressionSaved = compressionRegistry.Counter(
		"compression_saved_bytes_total",
		"Total bytes saved by compression",
	)
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(nil)
	},
}

// CompressionMiddleware добавляет gzip-сжатие ответов.
// Поддерживает Accept-Encoding: gzip. Минимальный размер ответа — 1KB.
// Отключает сжатие для HEAD/OPTIONS, бинарных Content-Type и маленьких ответов.
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		cw := &compressionWriter{
			ResponseWriter: w,
			code:           http.StatusOK,
		}
		next.ServeHTTP(cw, r)
		cw.Flush()
	})
}

// compressionWriter буферизует ответ и решает сжимать ли по факту.
type compressionWriter struct {
	http.ResponseWriter
	code   int
	buf    bytes.Buffer
	closed bool
}

func (cw *compressionWriter) WriteHeader(code int) {
	cw.code = code
}

func (cw *compressionWriter) Write(b []byte) (int, error) {
	return cw.buf.Write(b)
}

func (cw *compressionWriter) Unwrap() http.ResponseWriter {
	return cw.ResponseWriter
}

// Flush записывает буфер в ResponseWriter с решением о сжатии.
func (cw *compressionWriter) Flush() {
	if cw.closed {
		return
	}
	cw.closed = true

	ct := cw.ResponseWriter.Header().Get("Content-Type")
	body := cw.buf.Bytes()
	originalSize := len(body)
	tooSmall := originalSize < 1024
	binary := shouldNotCompress(ct)

	if tooSmall || binary {
		cw.ResponseWriter.WriteHeader(cw.code)
		_, _ = cw.ResponseWriter.Write(body)
		if tooSmall {
			compressionRequests.With("skipped_small").Inc()
		} else {
			compressionRequests.With("skipped_binary").Inc()
		}
		return
	}

	// Сжимаем
	gz := gzipWriterPool.Get().(*gzip.Writer)
	defer gzipWriterPool.Put(gz)

	var compressed bytes.Buffer
	gz.Reset(&compressed)
	_, _ = gz.Write(body)
	_ = gz.Close()

	compressedData := compressed.Bytes()
	compressedSize := len(compressedData)

	cw.ResponseWriter.Header().Set("Content-Encoding", "gzip")
	cw.ResponseWriter.Header().Del("Content-Length")
	cw.ResponseWriter.WriteHeader(cw.code)
	_, _ = cw.ResponseWriter.Write(compressedData)

	// Метрики
	compressionRequests.With("compressed").Inc()
	if originalSize > 0 {
		ratio := float64(compressedSize) / float64(originalSize)
		compressionRatio.With().Observe(ratio)
		saved := int64(originalSize - compressedSize)
		compressionSaved.With().Add(saved)
	}
}

// shouldNotCompress проверяет, нужно ли пропускать сжатие для данного Content-Type.
func shouldNotCompress(ct string) bool {
	ct = strings.ToLower(ct)
	skipTypes := []string{
		"image/",
		"audio/",
		"video/",
		"application/zip",
		"application/gzip",
		"application/x-gzip",
		"application/compress",
		"application/pdf",
		"application/octet-stream",
	}
	for _, skip := range skipTypes {
		if strings.HasPrefix(ct, skip) {
			return true
		}
	}
	return false
}
