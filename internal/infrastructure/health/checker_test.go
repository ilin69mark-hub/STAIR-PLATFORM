package health

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestReadySkipsNilProviders: если провайдеры nil, инстанс «готов» с пустым
// списком проверок (ни одна зависимость не проверяется).
func TestReadySkipsNilProviders(t *testing.T) {
	c := &Checker{}
	ready, checks := c.Ready(context.Background())
	if !ready {
		t.Fatalf("expected ready=true with nil providers, got false")
	}
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %v", checks)
	}
}

// TestReadyRedisError: клиент Redis на несуществующий порт → не готов.
func TestReadyRedisError(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer client.Close()

	c := &Checker{
		Redis:   client,
		Timeout: 500 * time.Millisecond,
	}
	ready, checks := c.Ready(context.Background())
	if ready {
		t.Fatalf("expected not ready when redis down, got ready")
	}
	if v := checks["redis"]; v == "ok" {
		t.Fatalf("expected redis check to be error, got ok: %v", checks)
	}
}
