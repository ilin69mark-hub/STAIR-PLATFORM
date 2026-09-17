// Package database реализует infrastructure слой доступа к PostgreSQL
// (BE-0005): подключение через pgxpool, применение миграций и доступ к
// данным для repository. Домен и application не зависят от этой реализации.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SlowQueryThreshold — порог для логирования медленных запросов.
var SlowQueryThreshold = 500 * time.Millisecond

// Config — параметры подключения к PostgreSQL (BE-0029 Configuration).
type Config struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig(url string) Config {
	return Config{
		URL:             url,
		MaxConns:        10,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	}
}

// EnvConfig возвращает конфигурацию из переменных окружения.
// STAIR_DB_MAX_CONNS, STAIR_DB_MIN_CONNS, STAIR_DB_MAX_CONN_LIFETIME,
// STAIR_DB_MAX_CONN_IDLE_TIME — необязательные параметры.
func EnvConfig(url string, maxConns, minConns int, lifetime, idleTime time.Duration) Config {
	cfg := DefaultConfig(url)
	if maxConns > 0 {
		cfg.MaxConns = clampInt32(maxConns)
	}
	if minConns > 0 {
		cfg.MinConns = clampInt32(minConns)
	}
	if lifetime > 0 {
		cfg.MaxConnLifetime = lifetime
	}
	if idleTime > 0 {
		cfg.MaxConnIdleTime = idleTime
	}
	return cfg
}

func clampInt32(v int) int32 {
	if v <= math.MaxInt32 {
		return int32(v) // #nosec G115 -- value bounded by guard above
	}
	return math.MaxInt32
}

// Connect открывает пул соединений и проверяет доступность (Ping).
func Connect(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("database: url is required")
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("database: parse url: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	return pool, nil
}

// LogSlowQuery логирует медленные запросы.
func LogSlowQuery(query string, duration time.Duration, args ...interface{}) {
	if duration >= SlowQueryThreshold {
		slog.Warn("slow query detected",
			"query", query,
			"duration", duration.String(),
			"args", args,
		)
	}
}
