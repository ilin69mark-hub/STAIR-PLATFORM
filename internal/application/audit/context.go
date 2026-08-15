package audit

import "context"

// Meta — метаданные запроса для аудита (SEC-0013: Request ID, IP).
// Заполняются транспортным слоем в контексте запроса.
type Meta struct {
	RequestID string
	IP        string
}

type metaCtxKey struct{}

// WithMeta кладёт метаданные запроса в контекст (вызывается транспортом).
func WithMeta(ctx context.Context, m Meta) context.Context {
	return context.WithValue(ctx, metaCtxKey{}, m)
}

// MetaFrom возвращает метаданные запроса из контекста (нулевые — нет).
func MetaFrom(ctx context.Context) Meta {
	m, _ := ctx.Value(metaCtxKey{}).(Meta)
	return m
}
