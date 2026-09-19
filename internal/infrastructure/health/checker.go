// Package health реализует readiness-пробы зависимостей инстанса
// (EDR-0018 §3.2): PostgreSQL (SELECT 1 через pgxpool) и Redis (PING через
// go-redis). Используется cmd/api и регистрируется как проба /ready.
//
// Структура Checker удовлетворяет интерфейсу transporthttp.ReadinessChecker
// структурно (нет импорта transport-слоя): метод Ready(ctx) (bool,
// map[string]string).
package health

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Checker — набор readiness-проб. Поля-провайдеры могут быть nil (проверка
// пропускается).
type Checker struct {
	DB    *pgxpool.Pool
	Redis *redis.Client

	// Timeout — таймаут каждой отдельной пробы. Дефолт 2s.
	Timeout time.Duration
}

// HealthResult — результат проверки здоровья.
type HealthResult struct {
	Status string           `json:"status"`
	Checks map[string]Check `json:"checks"`
}

// Check — результат одной проверки.
type Check struct {
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration_ms"`
	Message  string        `json:"message,omitempty"`
}

// Ready выполняет пробы и возвращает готовность инстанса. checks — имя
// проверки → "ok" либо описание ошибки. Redis пропускается, если клиент не
// сконфигурирован (необязательная зависимость, fallback EDR-0014).
func (c *Checker) Ready(ctx context.Context) (bool, map[string]string) {
	result := c.CheckDeep(ctx)
	ready := result.Status == "ok"
	simple := make(map[string]string, len(result.Checks))
	for name, ch := range result.Checks {
		if ch.Status == "ok" {
			simple[name] = "ok"
		} else {
			simple[name] = "error: " + ch.Message
		}
	}
	return ready, simple
}

// CheckDeep выполняет глубокую проверку всех зависимостей с замером времени.
func (c *Checker) CheckDeep(ctx context.Context) *HealthResult {
	checks := make(map[string]Check)
	overall := "ok"

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	if c.DB != nil {
		start := time.Now()
		err := c.dbProbe(ctx, timeout)
		dur := time.Since(start)
		if err != nil {
			checks["database"] = Check{Status: "error", Duration: dur, Message: err.Error()}
			overall = "degraded"
		} else {
			stats := c.DB.Stat()
			checks["database"] = Check{
				Status:   "ok",
				Duration: dur,
				Message:  formatPoolStats(stats),
			}
		}
	}

	if c.Redis != nil {
		start := time.Now()
		err := c.redisProbe(ctx, timeout)
		dur := time.Since(start)
		if err != nil {
			checks["redis"] = Check{Status: "error", Duration: dur, Message: err.Error()}
			overall = "degraded"
		} else {
			checks["redis"] = Check{Status: "ok", Duration: dur}
		}
	}

	return &HealthResult{Status: overall, Checks: checks}
}

// dbProbe выполняет SELECT 1 через пул (реальная проверка соединения).
func (c *Checker) dbProbe(ctx context.Context, timeout time.Duration) error {
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var one int
	return c.DB.QueryRow(pctx, `SELECT 1`).Scan(&one)
}

// redisProbe выполняет PING.
func (c *Checker) redisProbe(ctx context.Context, timeout time.Duration) error {
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Redis.Ping(pctx).Err()
}

// formatPoolStats форматирует расширенную статистику пула соединений.
func formatPoolStats(stats *pgxpool.Stat) string {
	return "totalConns=" + itoa(int(stats.TotalConns())) +
		" idleConns=" + itoa(int(stats.IdleConns())) +
		" acquiredConns=" + itoa(int(stats.AcquiredConns())) +
		" maxConns=" + itoa(int(stats.MaxConns())) +
		" emptyAcquireCount=" + itoa(int(stats.EmptyAcquireCount())) +
		" canceledAcquireCount=" + itoa(int(stats.CanceledAcquireCount())) +
		" acquiredCount=" + itoa(int(stats.AcquireCount()))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
