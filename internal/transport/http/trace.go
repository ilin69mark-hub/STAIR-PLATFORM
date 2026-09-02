package http

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("stair-platform/api")

// TraceMiddleware создаёт OpenTelemetry span для каждого HTTP-запроса.
// Извлекает trace context из входящих заголовков (W3C Trace Context).
// Устанавливает span attributes: method, path, status_code, user_id.
// Добавляет span events для ошибок и медленных запросов (>1s).
func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем контекст трассировки из входящих заголовков
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Создаём span
		spanName := r.Method + " " + r.URL.Path
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.user_agent", r.UserAgent()),
				attribute.String("net.peer.ip", clientIP(r)),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Оборачиваем ResponseWriter для захвата status code
		sw := &traceStatusWriter{ResponseWriter: w, span: span}
		r = r.WithContext(ctx)

		start := time.Now()
		next.ServeHTTP(sw, r)
		dur := time.Since(start)

		// Устанавливаем финальные attributes
		span.SetAttributes(
			attribute.Int("http.status_code", sw.status),
			attribute.Float64("http.duration_ms", float64(dur.Milliseconds())),
		)

		if sw.status >= 500 {
			span.SetStatus(codes.Error, http.StatusText(sw.status))
			span.AddEvent("server_error", trace.WithAttributes(
				attribute.Int("status_code", sw.status),
			))
		} else if sw.status >= 400 {
			span.SetStatus(codes.Error, http.StatusText(sw.status))
		}

		// Событие для медленных запросов (>1s)
		if dur > time.Second {
			span.AddEvent("slow_request", trace.WithAttributes(
				attribute.Float64("duration_ms", float64(dur.Milliseconds())),
			))
		}
	})
}

// TraceContextMiddleware извлекает trace_id и span_id из контекста
// и добавляет их в заголовки ответа для client-side tracing.
func TraceContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		if span != nil {
			sc := span.SpanContext()
			if sc.HasTraceID() {
				w.Header().Set("X-Trace-Id", sc.TraceID().String())
			}
			if sc.HasSpanID() {
				w.Header().Set("X-Span-Id", sc.SpanID().String())
			}
		}
		next.ServeHTTP(w, r)
	})
}

// traceStatusWriter захватывает HTTP status code для span attributes.
type traceStatusWriter struct {
	http.ResponseWriter
	status int
	span   trace.Span
}

func (w *traceStatusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *traceStatusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap поддерживает http.ResponseController.
func (w *traceStatusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
