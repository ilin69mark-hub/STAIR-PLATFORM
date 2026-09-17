package database

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestEnvConfig(t *testing.T) {
	cfg := EnvConfig("postgres://x", 5, 2, time.Hour, 10*time.Minute)
	if cfg.MaxConns != 5 || cfg.MinConns != 2 {
		t.Fatalf("got %+v", cfg)
	}
	cfg2 := EnvConfig("postgres://x", 0, 0, 0, 0)
	if cfg2.MaxConns != 10 || cfg2.MinConns != 1 {
		t.Fatalf("default %+v", cfg2)
	}
	if got := clampInt32(100); got != 100 {
		t.Fatalf("want 100 got %d", got)
	}
	if got := clampInt32(math.MaxInt32 + 100); got != math.MaxInt32 {
		t.Fatalf("want MaxInt32 got %d", got)
	}
}

func TestLogSlowQuery(t *testing.T) {
	// Just ensure no panic for slow and fast paths
	LogSlowQuery("SELECT 1", 10*time.Millisecond)
	LogSlowQuery("SELECT 1", time.Second)
}

func TestConnectBadURL(t *testing.T) {
	if _, err := Connect(context.Background(), Config{URL: ""}); err == nil {
		t.Fatal("want error for empty url")
	}
}
