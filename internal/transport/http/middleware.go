package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"stairplatform/internal/application/audit"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// requestIDHeader — заголовок передачи idempotency request id (v1).
const requestIDHeader = "X-Request-Id"

// withRequestID возвращает контекст с request id: используется входящий
// заголовок (если передан), иначе генерируется новый.
func withRequestID(ctx context.Context, headerID string) (context.Context, string) {
	id := headerID
	if id == "" {
		id = newRequestID()
	}
	return context.WithValue(ctx, requestIDKey, id), id
}

// RequestID возвращает request id из контекста (пустая строка — нет).
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Крайний случай: детерминированный суррогат по времени.
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000")))
	}
	return hex.EncodeToString(b[:])
}

// withLogging логирует каждый запрос: method, path, status, длительность,
// request id; восстанавливает панику в 500 JSON (без падения процесса).
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx, rid := withRequestID(r.Context(), r.Header.Get(requestIDHeader))
		// Метаданные запроса для аудита (EDR-0013: Request ID, IP).
		ctx = audit.WithMeta(ctx, audit.Meta{RequestID: rid, IP: clientIP(r)})
		r = r.WithContext(ctx)

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			if rec := recover(); rec != nil {
				// Единственный recover — в PanicRecoveryMiddleware (см. recovery.go).
				// Здесь паника только фиксируется в статусе/логах/метриках и
				// пробрасывается дальше, чтобы ответ 500 написал recovery-обёртка.
				sw.status = http.StatusInternalServerError
				recordHTTPMetrics(r.Method, r.URL.Path, sw.status, time.Since(start))
				logAttrs := slogAttrs(rid, r, sw, start)
				slog.LogAttrs(r.Context(), slog.LevelError, "http request panicked", logAttrs...)
				panic(rec)
			}
			logAttrs := slogAttrs(rid, r, sw, start)
			slog.LogAttrs(r.Context(), slog.LevelInfo, "http request", logAttrs...)
			recordHTTPMetrics(r.Method, r.URL.Path, sw.status, time.Since(start))
		}()

		w.Header().Set(requestIDHeader, rid)
		next.ServeHTTP(sw, r)
	})
}

// slogAttrs формирует атрибуты лога HTTP-запроса.
func slogAttrs(rid string, r *http.Request, sw *statusWriter, start time.Time) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("request_id", rid),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Int("status", sw.status),
		slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000.0),
		slog.String("remote_ip", clientIP(r)),
		slog.String("user_agent", r.UserAgent()),
	}
	if uid := userID(r.Context()); uid != "" {
		attrs = append(attrs, slog.String("user_id", uid))
	}
	return attrs
}

// statusWriter запоминает код ответа для логгирования.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
