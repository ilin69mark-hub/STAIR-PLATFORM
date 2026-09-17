package http

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNewRedisRateLimiter(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	l := newRedisRateLimiter(c, 5, time.Minute)
	if l.limit != 5 || l.window != time.Minute {
		t.Fatalf("mismatch %+v", l)
	}
	// Allow should fallback to true when redis unavailable
	if !l.Allow("1.2.3.4") {
		t.Fatal("want true on redis error")
	}
}

func TestNewRateLimiterStrategy(t *testing.T) {
	ctx := context.Background()
	// empty addr -> memory
	rl := newRateLimiterStrategy(ctx, "", 10, time.Minute)
	if rl == nil {
		t.Fatal("nil limiter")
	}
	// bad addr -> fallback to memory
	rl2 := newRateLimiterStrategy(ctx, "127.0.0.1:1", 10, time.Minute)
	if rl2 == nil {
		t.Fatal("nil limiter2")
	}
}
