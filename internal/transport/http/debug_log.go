package http

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// DebugLoggingMiddleware логирует тела запросов и ответов для отладки.
// Включается через STAIR_DEBUG_LOGGING=true. НЕ использовать в проде
// из-за чувствительных данных в телах запросов.
//
// PRODUCTION GUARD: Middleware проверяет STAIR_ENVIRONMENT и отказывается
// работать в production, если STAIR_DEBUG_LOGGING не установлен явно.
func DebugLoggingMiddleware(next http.Handler) http.Handler {
	// Проверяем production guard
	env := os.Getenv("STAIR_ENVIRONMENT")
	debugLogging := os.Getenv("STAIR_DEBUG_LOGGING")
	isProduction := strings.EqualFold(env, "production") || strings.EqualFold(env, "prod")

	// В production отказываемся работать, если debug logging не установлен явно
	if isProduction && debugLogging != "true" {
		slog.Warn("debug logging disabled in production environment")
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Читаем тело запроса (макс 64KB)
		var reqBody []byte
		if r.Body != nil {
			reader := io.LimitReader(r.Body, 64*1024)
			reqBody, _ = io.ReadAll(reader)
			_ = r.Body.Close()
			// Восстанавливаем тело для следующего handler
			r.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		start := time.Now()

		// Оборачиваем ResponseWriter для захвата тела ответа
		sw := &bodyCaptureWriter{ResponseWriter: w, body: &bytes.Buffer{}}

		next.ServeHTTP(sw, r)

		dur := time.Since(start)

		// Логируем (обрезаем большие тела)
		logBody := func(b []byte, max int) string {
			if len(b) > max {
				return string(b[:max]) + "...(truncated)"
			}
			return string(b)
		}

		slog.Debug("http debug",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", dur.Milliseconds(),
			"req_body", logBody(reqBody, 1024),
			"resp_body", logBody(sw.body.Bytes(), 1024),
			"content_type", w.Header().Get("Content-Type"),
		)
	})
}

// bodyCaptureWriter захватывает тело ответа для логирования.
type bodyCaptureWriter struct {
	http.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (w *bodyCaptureWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyCaptureWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
