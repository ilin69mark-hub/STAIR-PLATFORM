package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisQueue — распределённая FIFO-очередь на Redis List (EDR-0020 §3.2):
// LPUSH для Enqueue, BRPOP с таймаутом для Dequeue. Общий Redis даёт
// распределённость между несколькими воркерами/репликами.
type redisQueue struct {
	client       *redis.Client
	key          string
	blockTimeout time.Duration
}

// NewRedisQueue создаёт бэкенд поверх Redis List.
// blockTimeout — максимальное время ожидания в BRPOP (таймаут блокировки);
// <=0 — 2s.
func NewRedisQueue(client *redis.Client, key string, blockTimeout time.Duration) JobQueue {
	if blockTimeout <= 0 {
		blockTimeout = 2 * time.Second
	}
	return &redisQueue{client: client, key: key, blockTimeout: blockTimeout}
}

func (q *redisQueue) Enqueue(ctx context.Context, job Job) error {
	b, err := job.Marshal()
	if err != nil {
		return err
	}
	if err := q.client.LPush(ctx, q.key, b).Err(); err != nil {
		return fmt.Errorf("queue: enqueue redis: %w", err)
	}
	return nil
}

func (q *redisQueue) Dequeue(ctx context.Context) (Job, bool, error) {
	// BRPOP ожидает элемент в течение blockTimeout; пустой ctx timeout
	// должен быть длиннее blockTimeout, иначе BRPOP отменится раньше.
	res, err := q.client.BRPop(ctx, q.blockTimeout, q.key).Result()
	if err == redis.Nil {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("queue: dequeue redis: %w", err)
	}
	// res: [key, value]
	if len(res) < 2 {
		return Job{}, false, nil
	}
	j, err := UnmarshalJob([]byte(res[1]))
	if err != nil {
		return Job{}, false, err
	}
	return j, true, nil
}
