package http

import (
	"log/slog"
	"net/http"
	"time"
)

// ErrorLoggingMiddleware логирует ошибки в структурированном формате.
func ErrorLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &errorStatusWriter{ResponseWriter: w, statusCode: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(sw, r)
		dur := time.Since(start)

		if sw.statusCode >= 400 {
			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.statusCode),
				slog.Duration("duration", dur),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("request_id", RequestID(r.Context())),
				slog.String("user_agent", r.UserAgent()),
			}

			if sw.statusCode >= 500 {
				slog.Error("server error", attrs...)
			} else {
				slog.Warn("client error", attrs...)
			}
		}
	})
}

// errorStatusWriter захватывает status code для логирования.
type errorStatusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *errorStatusWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *errorStatusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
