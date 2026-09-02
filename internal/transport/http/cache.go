package http

import (
	"net/http"
	"strings"
	"time"
)

// CachePolicy определяет стратегию кеширования.
type CachePolicy struct {
	// MaxAge — максимальное время кеширования в секундах.
	MaxAge int
	// Private — если true, браузер может кешировать, прокси — нет.
	Private bool
	// NoStore — если true, запрещает любое кеширование.
	NoStore bool
	// MustRevalidate — если true, после истечения срока нужно перепроверять.
	MustRevalidate bool
}

var (
	// CacheNoCache — не кешировать (API ответы).
	CacheNoCache = CachePolicy{NoStore: true}
	// CacheShort — короткое кеширование (5 минут, для изменчивых данных).
	CacheShort = CachePolicy{MaxAge: 300, Private: true, MustRevalidate: true}
	// CacheMedium — среднее кеширование (1 час).
	CacheMedium = CachePolicy{MaxAge: 3600, Private: true}
	// CacheLong — долгое кеширование (1 день, для статических ресурсов).
	CacheLong = CachePolicy{MaxAge: 86400}
	// CacheImmutable — бесконечное кеширование (content-hashed assets).
	CacheImmutable = CachePolicy{MaxAge: 31536000}
)

// String возвращает значение Cache-Control заголовка.
func (cp CachePolicy) String() string {
	if cp.NoStore {
		return "no-store"
	}
	var parts []string
	parts = append(parts, "max-age="+itoa(cp.MaxAge))
	if cp.Private {
		parts = append(parts, "private")
	}
	if cp.MustRevalidate {
		parts = append(parts, "must-revalidate")
	}
	return strings.Join(parts, ", ")
}

// CacheMiddleware добавляет Cache-Control заголовки к ответам.
// pathPolicy — маппинг путей к политикам. Если путь не найден — CacheNoCache.
func CacheMiddleware(policies map[string]CachePolicy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			policy := findCachePolicy(r.URL.Path, policies)
			w.Header().Set("Cache-Control", policy.String())
			next.ServeHTTP(w, r)
		})
	}
}

// findCachePolicy ищет наилучшее совпадение policy для пути.
func findCachePolicy(path string, policies map[string]CachePolicy) CachePolicy {
	// Точное совпадение
	if p, ok := policies[path]; ok {
		return p
	}
	// Prefix match (длиннейший)
	bestLen := 0
	best := CacheNoCache
	for prefix, p := range policies {
		if strings.HasPrefix(path, prefix) && len(prefix) > bestLen {
			best = p
			bestLen = len(prefix)
		}
	}
	return best
}

// CacheHeaders устанавливает дополнительные заголовки кеширования.
func CacheHeaders(w http.ResponseWriter, policy CachePolicy, lastModified time.Time) {
	if !lastModified.IsZero() {
		w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	}
	if policy.NoStore {
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}
}
