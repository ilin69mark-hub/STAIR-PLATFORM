package http

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"stairplatform/internal/infrastructure/redisconf"
	"stairplatform/internal/infrastructure/security"
)

// redisCounter — минимальный набор операций Redis для распределённого
// лимитера (S-147): интерфейс вместо *redis.Client ради fault-инъекции в
// тестах (сбой EXPIRE без живого Redis). *redis.Client его реализует.
type redisCounter interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

// redisRateLimiter — распределённый лимитер через Redis (EDR-0014 §3.2).
// Счётчик на ключ `ratelimit:{ip}` с TTL = окно (INCR + EXPIRE).
type redisRateLimiter struct {
	client redisCounter
	limit  int
	window time.Duration
}

// newRedisRateLimiter создаёт лимитер поверх переданного клиента.
func newRedisRateLimiter(client redisCounter, limit int, window time.Duration) *redisRateLimiter {
	return &redisRateLimiter{client: client, limit: limit, window: window}
}

// Allow инкрементирует счётчик для ip. При недоступности Redis возвращает
// true (fallback: не блокировать вход) и пишет в лог + метрику
// rate_limit_fail_open_total — по инварианту EDR-0014 §4.2 лимитер не
// должен ронять аутентификацию, но fail-open теперь осознанный и виден
// (S-141 №5, CWE-307).
func (l *redisRateLimiter) Allow(ip string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	key := "ratelimit:" + ip
	n, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		security.RateLimitFailOpen.With("redis_unavailable").Inc()
		slog.Warn("rate limiter: redis unavailable, allowing request (fail-open)",
			"ip", ip, "error", err)
		return true
	}
	if n == 1 {
		// Первый запрос из окна: ставим TTL. Сбой EXPIRE раньше
		// проглатывался (`_ =`) — ключ оставался без TTL и давал вечный
		// 429 для IP (самопоражение, за NAT бьёт по всем). Теперь ключ
		// удаляем: окно начнётся заново следующим запросом (S-141 №5,
		// CWE-399); удаление видно в rate_limit_expire_cleanup_total.
		if err := l.client.Expire(ctx, key, l.window).Err(); err != nil {
			security.RateLimitExpireCleanup.With().Inc()
			slog.Warn("rate limiter: EXPIRE failed, key deleted to avoid sticky limit",
				"ip", ip, "error", err)
			if derr := l.client.Del(ctx, key).Err(); derr != nil {
				slog.Warn("rate limiter: DEL after EXPIRE failure failed",
					"ip", ip, "error", derr)
			}
		}
	}
	return n <= int64(l.limit)
}

// newRateLimiterStrategy строит лимитер по конфигурации: redis (если адрес
// задан и клиент доступен) либо memory. Вызывается в NewRouter (P2-11).
func newRateLimiterStrategy(ctx context.Context, addr string, limit int, window time.Duration) RateLimiter {
	if addr == "" {
		return newRateLimiter(ctx, limit, window)
	}
	client := redis.NewClient(redisconf.FromEnv(addr))
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pingCancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		slog.Warn("rate limiter: redis unavailable, falling back to memory",
			"addr", addr, "error", err)
		_ = client.Close()
		return newRateLimiter(ctx, limit, window)
	}
	return newRedisRateLimiter(client, limit, window)
}
