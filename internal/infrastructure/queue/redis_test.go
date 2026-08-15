package queue

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// testRedisClient возвращает клиент к локальному Redis; nil — недоступен.
func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := "127.0.0.1:6379"
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("redis at %s unavailable, skipping integration test", addr)
	}
	return client
}

// TestRedisQueueRoundTrip: LPUSH/BRPOP FIFO через Redis List.
func TestRedisQueueRoundTrip(t *testing.T) {
	client := testRedisClient(t)
	defer client.Close()

	key := "test-jobq-" + time.Now().UTC().Format("150405.000000000")
	q := NewRedisQueue(client, key, time.Second)
	ctx := context.Background()
	defer client.Del(ctx, key)

	j1 := mustJob(t, "typ.one", map[string]string{"a": "1"})
	j2 := mustJob(t, "typ.two", nil)
	if err := q.Enqueue(ctx, j1); err != nil {
		t.Fatalf("enqueue j1: %v", err)
	}
	if err := q.Enqueue(ctx, j2); err != nil {
		t.Fatalf("enqueue j2: %v", err)
	}

	got1, ok, err := q.Dequeue(ctx)
	if err != nil || !ok {
		t.Fatalf("dequeue first: ok=%v err=%v", ok, err)
	}
	if got1.ID != j1.ID {
		t.Fatalf("first = %s, want %s (FIFO)", got1.ID, j1.ID)
	}
	got2, ok, err := q.Dequeue(ctx)
	if err != nil || !ok {
		t.Fatalf("dequeue second: ok=%v err=%v", ok, err)
	}
	if got2.ID != j2.ID {
		t.Fatalf("second = %s, want %s (FIFO)", got2.ID, j2.ID)
	}
}
