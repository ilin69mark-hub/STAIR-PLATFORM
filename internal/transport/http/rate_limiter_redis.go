package http

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisRateLimiter — распределённый лимитер через Redis (EDR-0014 §3.2).
// Счётчик на ключ `ratelimit:{ip}` с TTL = окно (INCR + EXPIRE).
type redisRateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

// newRedisRateLimiter создаёт лимитер поверх переданного клиента.
func newRedisRateLimiter(client *redis.Client, limit int, window time.Duration) *redisRateLimiter {
	return &redisRateLimiter{client: client, limit: limit, window: window}
}

// Allow инкрементирует счётчик для ip. При недоступности Redis возвращает
// true (fallback: не блокировать вход) и пишет в лог — по инварианту
// EDR-0014 §4.2 лимитер не должен ронять аутентификацию.
func (l *redisRateLimiter) Allow(ip string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	key := "ratelimit:" + ip
	n, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		slog.Warn("rate limiter: redis unavailable, allowing request",
			"ip", ip, "error", err)
		return true
	}
	if n == 1 {
		// Первый запрос из окна: ставим TTL.
		_ = l.client.Expire(ctx, key, l.window).Err()
	}
	return n <= int64(l.limit)
}

// newRateLimiterStrategy строит лимитер по конфигурации: redis (если адрес
// задан и клиент доступен) либо memory. Используется applyConfig.
func newRateLimiterStrategy(addr string, limit int, window time.Duration) RateLimiter {
	if addr == "" {
		return newRateLimiter(limit, window)
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("rate limiter: redis unavailable, falling back to memory",
			"addr", addr, "error", err)
		_ = client.Close()
		return newRateLimiter(limit, window)
	}
	return newRedisRateLimiter(client, limit, window)
}
