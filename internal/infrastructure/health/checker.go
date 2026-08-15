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

// Ready выполняет пробы и возвращает готовность инстанса. checks — имя
// проверки → "ok" либо описание ошибки. Redis пропускается, если клиент не
// сконфигурирован (необязательная зависимость, fallback EDR-0014).
func (c *Checker) Ready(ctx context.Context) (bool, map[string]string) {
	checks := make(map[string]string)
	ready := true

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	if c.DB != nil {
		if err := c.dbProbe(ctx, timeout); err != nil {
			checks["database"] = "error: " + err.Error()
			ready = false
		} else {
			checks["database"] = "ok"
		}
	}

	if c.Redis != nil {
		if err := c.redisProbe(ctx, timeout); err != nil {
			checks["redis"] = "error: " + err.Error()
			ready = false
		} else {
			checks["redis"] = "ok"
		}
	}

	return ready, checks
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
