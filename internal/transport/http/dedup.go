package http

import (
	"bytes"
	"hash/fnv"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// DeduplicateMiddleware предотвращает одновременную обработку
// одинаковых запросов (по ключу). Если запрос уже в обработке —
// возвращает 429 Too Many Requests.
// Подходит для тяжёлых операций (расчёт, экспорт, генерация).
func DeduplicateMiddleware(keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	inflight := make(map[string]*sync.Mutex)
	mu := sync.Mutex{}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			mu.Lock()
			if m, ok := inflight[key]; ok {
				mu.Unlock()
				// Already in-flight — wait or reject
				if !m.TryLock() {
					w.Header().Set("Retry-After", "5")
					http.Error(w, "duplicate request in progress", http.StatusTooManyRequests)
					return
				}
				defer m.Unlock()
				next.ServeHTTP(w, r)
				return
			}

			// New request — register
			m := &sync.Mutex{}
			m.Lock()
			inflight[key] = m
			mu.Unlock()

			defer func() {
				mu.Lock()
				delete(inflight, key)
				mu.Unlock()
				m.Unlock()
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// DeduplicateByKey возвращает ключ для дедупликации мутирующих запросов
// (POST/PUT/PATCH/DELETE). Read-only методы (GET/HEAD/OPTIONS) не
// дедуплицируются: кэш ответов уже закрывает повторяющиеся чтения, а
// ложный ключ не должен отсекать параллельные GET.
//
// Ключ строится по IP + метод + путь + хэш тела. user_id намеренно НЕ
// используется: дедупликация выполняется снаружи mux, тогда как
// аутентификация — внутри, поэтому контекст на этом этапе ещё не содержит
// авторизованного пользователя. Хэш тела (FNV-1a, первые 8 hex-цифр)
// убирает ложные столкновения параллельных запросов с одного IP
// (параллельные e2e-воркеры с разными payload больше не получают 429),
// а повторная отправка того же тела по-прежнему дедуплицируется.
func DeduplicateByKey(r *http.Request) string {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return ""
	}
	// Auth-ручки дедуплицировать нельзя: у них собственные rate-limiter'ы, а
	// параллельный вход одного пользователя из двух вкладок — легитимный
	// сценарий. Прежде dedup отвечал на него 429 «duplicate request».
	if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
		return ""
	}
	base := dedupIP(r) + ":" + r.Method + ":" + r.URL.Path
	sum, ok := dedupBodyHash(r)
	if !ok {
		return base
	}
	return base + ":" + sum
}

// dedupBodyHash читает тело запроса (до 1 МБ), возвращает короткий хэш и
// восстанавливает тело для downstream-обработчика. Пустое тело — ok=false
// (ключ остаётся без суффикса, старое поведение сохраняется).
func dedupBodyHash(r *http.Request) (string, bool) {
	if r.Body == nil {
		return "", false
	}
	const limit = 1 << 20
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return "", false
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	// Тело длиннее лимита — не хэшируем (не блокируем большие аплоады
	// чужим ключом), дедуплицируем только по IP+метод+путь.
	if len(body) > limit || len(body) == 0 {
		return "", false
	}
	h := fnv.New32a()
	_, _ = h.Write(body)
	return strconv.FormatUint(uint64(h.Sum32()), 16), true
}

// dedupIP извлекает IP из RemoteAddr.
func dedupIP(r *http.Request) string {
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}
	return addr
}
