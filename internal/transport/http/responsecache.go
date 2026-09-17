package http

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	"stairplatform/internal/infrastructure/metrics"
)

var (
	cacheRegistry = metrics.NewRegistry()

	cacheHits = cacheRegistry.Counter(
		"response_cache_hits_total",
		"Total cache hits",
	)
	cacheMisses = cacheRegistry.Counter(
		"response_cache_misses_total",
		"Total cache misses",
	)
)

// ResponseCacheEntry элемент кеша ответов.
type ResponseCacheEntry struct {
	Body       []byte
	StatusCode int
	Headers    http.Header
	ExpiresAt  time.Time
}

// ResponseCacheConfig конфигурация кеша ответов.
type ResponseCacheConfig struct {
	// MaxEntries — максимальное количество записей в кеше.
	MaxEntries int
	// DefaultTTL — время жизни кеша по умолчанию.
	DefaultTTL time.Duration
	// Methods — HTTP методы для кеширования (обычно GET).
	Methods []string
}

// ResponseCacheMiddleware кеширует GET-ответы в памяти.
// Подходит для статических данных (материалы, профили, ассортимент).
func ResponseCacheMiddleware(cfg ResponseCacheConfig) func(http.Handler) http.Handler {
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 256
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 5 * time.Minute
	}
	if len(cfg.Methods) == 0 {
		cfg.Methods = []string{http.MethodGet}
	}

	cache := &responseCache{
		entries: make(map[string]*ResponseCacheEntry),
		config:  cfg,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Проверяем метод
			methodAllowed := false
			for _, m := range cfg.Methods {
				if r.Method == m {
					methodAllowed = true
					break
				}
			}
			if !methodAllowed {
				next.ServeHTTP(w, r)
				return
			}

			// Identity-запросы (Cookie/API-key) НЕ кэшируем и не отдаём из кэша:
			// ответы зависят от субъекта (session user / api key) и могли бы
			// «протечь» другому пользователю (например, списки заказов).
			if isIdentityBearerRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Проверяем кеш
			key := r.URL.Path + "?" + r.URL.RawQuery
			if entry, ok := cache.Get(key); ok {
				for k, v := range entry.Headers {
					w.Header()[k] = v
				}
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(entry.StatusCode)
				_, _ = w.Write(entry.Body)
				cacheHits.With().Inc()
				return
			}

			// Оборачиваем ResponseWriter для захвата ответа
			cw := &cacheWriter{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				headers:        make(http.Header),
			}
			next.ServeHTTP(cw, r)

			// Сохраняем в кеш только успешные ответы, не выставляющие cookies
			// (Set-Cookie — признак сессионного/персонального ответа).
			if cw.statusCode >= 200 && cw.statusCode < 300 && cw.Header().Get("Set-Cookie") == "" {
				cache.Set(key, &ResponseCacheEntry{
					Body:       cw.body.Bytes(),
					StatusCode: cw.statusCode,
					Headers:    cw.headers,
					ExpiresAt:  time.Now().Add(cfg.DefaultTTL),
				})
			}

			w.Header().Set("X-Cache", "MISS")
			cacheMisses.With().Inc()
		})
	}
}

// isIdentityBearerRequest определяет, несёт ли запрос признаки аутентификации:
// session-cookie (браузер) или Authorization: Bearer <api-key> (интеграции).
func isIdentityBearerRequest(r *http.Request) bool {
	authorization := r.Header.Get("Authorization")
	if len(authorization) > 0 {
		return true
	}
	cookies := r.Cookies()
	for _, c := range cookies {
		// Кэшу важны только авторизационные слова; персональные данные
		// (например, cookies согласий) на кэшируемость не влияют.
		// Учитываются оба app-origin (store: session/csrf; admin: _admin).
		switch c.Name {
		case sessionCookieName, csrfCookieName,
			sessionCookieName + adminOriginCookieSuffix,
			csrfCookieName + adminOriginCookieSuffix:
			if c.Value != "" {
				return true
			}
		}
	}
	return false
}

// responseCache thread-safe in-memory cache.
type responseCache struct {
	mu      sync.RWMutex
	entries map[string]*ResponseCacheEntry
	config  ResponseCacheConfig
}

func (c *responseCache) Get(key string) (*ResponseCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry, true
}

func (c *responseCache) Set(key string, entry *ResponseCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity
	if len(c.entries) >= c.config.MaxEntries {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.entries {
			if oldestKey == "" || v.ExpiresAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.ExpiresAt
			}
		}
		if oldestKey != "" {
			delete(c.entries, oldestKey)
		}
	}

	c.entries[key] = entry
}

// Size возвращает текущий размер кеша.
func (c *responseCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Invalidate удаляет запись из кеша по ключу.
func (c *responseCache) Invalidate(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[key]; ok {
		delete(c.entries, key)
		return true
	}
	return false
}

// InvalidatePrefix удаляет все записи с заданным префиксом пути.
func (c *responseCache) InvalidatePrefix(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for key := range c.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.entries, key)
			count++
		}
	}
	return count
}

// Clear очищает весь кеш.
func (c *responseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*ResponseCacheEntry)
}

// cacheWriter захватывает ответ для кеширования.
type cacheWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	headers    http.Header
	statusCode int
}

func (w *cacheWriter) WriteHeader(code int) {
	w.statusCode = code
	// Копируем заголовки
	for k, v := range w.Header() {
		w.headers[k] = v
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *cacheWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *cacheWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
