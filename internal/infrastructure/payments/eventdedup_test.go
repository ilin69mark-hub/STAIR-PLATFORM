package payments

// S-133: Redis-дедупликация на реальном тестовом Redis (прецедент queue:
// skip при недоступности; в CI redis нет — тест скипается там).

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testDedupRedis(t *testing.T) *redis.Client {
	t.Helper()
	// Тестовый Redis: test-compose маппит 6379→6380 хоста (S-133), dev-стек —
	// на 6379. Перебираем, иначе skip (прецедент queue/redis_test.go).
	for _, addr := range []string{"127.0.0.1:6380", "127.0.0.1:6379"} {
		client := redis.NewClient(&redis.Options{Addr: addr})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := client.Ping(ctx).Err()
		cancel()
		if err == nil {
			return client
		}
		_ = client.Close()
	}
	t.Skip("redis unavailable (tried 6380, 6379), skipping")
	return nil
}

func TestEventDedupKey(t *testing.T) {
	if got := eventDedupKey("stripe", "evt_1"); got != "stair:webhook:stripe:evt_1" {
		t.Fatalf("key = %q", got)
	}
}

func TestRedisEventDeduperRoundTrip(t *testing.T) {
	client := testDedupRedis(t)
	defer func() { _ = client.Close() }()
	d := NewRedisEventDeduper(client)
	if d == nil {
		t.Fatal("nil deduper")
	}
	ctx := context.Background()
	evt := fmt.Sprintf("evt-%d", time.Now().UnixNano())

	fresh, err := d.CheckAndMark(ctx, "stripe", evt)
	if err != nil || !fresh {
		t.Fatalf("first: fresh=%v err=%v", fresh, err)
	}
	again, err := d.CheckAndMark(ctx, "stripe", evt)
	if err != nil || again {
		t.Fatalf("replay: fresh=%v err=%v, want seen", again, err)
	}
	if err := d.Clear(ctx, "stripe", evt); err != nil {
		t.Fatalf("clear: %v", err)
	}
	afterClear, err := d.CheckAndMark(ctx, "stripe", evt)
	if err != nil || !afterClear {
		t.Fatalf("after clear: fresh=%v err=%v", afterClear, err)
	}
	_ = d.Clear(ctx, "stripe", evt)
}
