package http

import (
	"context"
	"errors"
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

// stubRedisCounter — управляемый redisCounter (S-147, S-141 №5):
// скриптует результаты INCR/EXPIRE и считает вызовы DEL.
type stubRedisCounter struct {
	incrN     int64
	incrErr   error
	expireErr error
	expired   int
	deleted   int
}

func (s *stubRedisCounter) Incr(_ context.Context, _ string) *redis.IntCmd {
	return redis.NewIntResult(s.incrN, s.incrErr)
}

func (s *stubRedisCounter) Expire(_ context.Context, _ string, _ time.Duration) *redis.BoolCmd {
	s.expired++
	return redis.NewBoolResult(true, s.expireErr)
}

func (s *stubRedisCounter) Del(_ context.Context, _ ...string) *redis.IntCmd {
	s.deleted++
	return redis.NewIntResult(1, nil)
}

// TestRedisLimiterExpireFailureDeletesKey — ловушка S-141 №5 (CWE-399):
// INCR ok + EXPIRE err → ключ удалён (нет вечного 429), запрос разрешён.
func TestRedisLimiterExpireFailureDeletesKey(t *testing.T) {
	stub := &stubRedisCounter{incrN: 1, expireErr: errors.New("boom")}
	l := newRedisRateLimiter(stub, 5, time.Minute)
	if !l.Allow("10.0.0.1") {
		t.Fatal("first request must be allowed")
	}
	if stub.expired != 1 {
		t.Fatalf("EXPIRE calls = %d, want 1", stub.expired)
	}
	if stub.deleted != 1 {
		t.Fatalf("DEL calls = %d, want 1 (key without TTL must not stick)", stub.deleted)
	}
}

// TestRedisLimiterRedisDownFailOpen — INCR err → поведение зафиксировано:
// allow + метрика (fail-open осознанный, EDR-0014 §4.2).
func TestRedisLimiterRedisDownFailOpen(t *testing.T) {
	stub := &stubRedisCounter{incrErr: errors.New("down")}
	l := newRedisRateLimiter(stub, 5, time.Minute)
	if !l.Allow("10.0.0.2") {
		t.Fatal("want allow on redis error (fail-open)")
	}
	if stub.expired != 0 || stub.deleted != 0 {
		t.Fatalf("no EXPIRE/DEL on INCR failure, got expired=%d deleted=%d", stub.expired, stub.deleted)
	}
}

// TestRedisLimiterNormalCounting — штатный путь не сломан: первое окно
// ставит TTL, превышение лимита отклоняется.
func TestRedisLimiterNormalCounting(t *testing.T) {
	stub := &stubRedisCounter{incrN: 1}
	l := newRedisRateLimiter(stub, 5, time.Minute)
	if !l.Allow("10.0.0.3") {
		t.Fatal("first request must be allowed")
	}
	if stub.expired != 1 || stub.deleted != 0 {
		t.Fatalf("want EXPIRE=1 DEL=0, got %d/%d", stub.expired, stub.deleted)
	}
	stub.incrN = 6
	if l.Allow("10.0.0.3") {
		t.Fatal("request over limit must be denied")
	}
}
