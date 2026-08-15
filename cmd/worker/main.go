// Команда worker — процесс фоновых заданий (EDR-0020 §3.4). Потребляет
// JobQueue (Redis List или in-memory fallback), обрабатывает задания по
// реестру и периодически ставит задачи очистки данных (сессии, sso_states,
// аудит-ретенция). Запускается независимо от API:
//
//	STAIR_DATABASE_URL="postgres://..." STAIR_REDIS_ADDR="redis:6379" \
//	  go run ./cmd/worker
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"stairplatform/internal/infrastructure/database"
	"stairplatform/internal/infrastructure/queue"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dbURL := os.Getenv("STAIR_DATABASE_URL")
	if dbURL == "" {
		slog.Error("STAIR_DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := database.Connect(ctx, database.DefaultConfig(dbURL))
	cancel()
	if err != nil {
		slog.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	cleanupInterval := envDuration("STAIR_CLEANUP_INTERVAL", time.Hour)
	retentionDays := envInt("STAIR_AUDIT_RETENTION_DAYS", 90)
	shutdownTimeout := envDuration("STAIR_WORKER_SHUTDOWN_TIMEOUT", 10*time.Second)

	backend := newQueueBackend(os.Getenv("STAIR_REDIS_ADDR"))
	jobq := backend.Queue()
	defer backend.Close()

	reg := newRegistry(database.NewAuthRepository(pool), database.NewAuditRepository(pool),
		database.NewIntegrationRepository(pool), retentionDays)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ctxRun, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	// Периодический enqueue заданий очистки.
	go scheduleCleanup(ctxRun, jobq, cleanupInterval)

	// Цикл потребителя.
	go consume(ctxRun, jobq, reg)

	<-stop
	slog.Info("worker: shutdown signal received")
	cancelRun()
	// Даём потребителю/таймеру завершить текущее задание (drain).
	time.Sleep(shutdownTimeout)
	slog.Info("worker: stopped")
}

// queueBackend оборачивает выбранный бэкенд очереди и его Redis-клиент для
// корректного закрытия.
type queueBackend struct {
	jobq  queue.JobQueue
	redis *redis.Client
}

// newQueueBackend выбирает Redis List (если addr задан и доступен) либо
// in-memory fallback (EDR-0020 §3.2).
func newQueueBackend(addr string) *queueBackend {
	if addr == "" {
		return &queueBackend{jobq: queue.NewMemoryQueue()}
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("worker: redis unavailable, falling back to memory queue",
			"addr", addr, "error", err)
		_ = client.Close()
		return &queueBackend{jobq: queue.NewMemoryQueue()}
	}
	return &queueBackend{jobq: queue.NewRedisQueue(client, "stair-jobs", 2*time.Second), redis: client}
}

func (b *queueBackend) Queue() queue.JobQueue { return b.jobq }

func (b *queueBackend) Close() {
	if b.redis != nil {
		_ = b.redis.Close()
	}
}

// consume — бесконечный цикл потребителя очереди (EDR-0020 §3.4).
func consume(ctx context.Context, jobq queue.JobQueue, reg *registry) {
	for {
		if ctx.Err() != nil {
			return
		}
		job, ok, err := jobq.Dequeue(ctx)
		if err != nil {
			slog.Warn("worker: dequeue failed", "error", err)
			time.Sleep(time.Second)
			continue
		}
		if !ok {
			// Очередь пуста; не крутимся вхолостую.
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		processJob(ctx, jobq, reg, job)
	}
}

// processJob исполняет задание; при ошибке ретраит до MaxAttempts. Повторные
// попытки ставятся обратно в очередь с инкрементом Attempts (распределённо
// корректно даже на Redis-бэкенде).
func processJob(ctx context.Context, jobq queue.JobQueue, reg *registry, job queue.Job) {
	err := reg.Handle(ctx, job)
	if err == nil {
		slog.Info("worker: job done", "job_id", job.ID, "type", job.Type)
		return
	}

	job.Attempts++
	if job.Attempts >= job.MaxAttempts {
		slog.Error("worker: job failed permanently",
			"job_id", job.ID, "type", job.Type, "attempts", job.Attempts, "error", err)
		return
	}

	backoff := time.Duration(job.Attempts) * 5 * time.Second
	slog.Warn("worker: job failed, requeueing for retry",
		"job_id", job.ID, "type", job.Type, "attempts", job.Attempts,
		"max", job.MaxAttempts, "retry_in", backoff, "error", err)
	select {
	case <-ctx.Done():
		return
	case <-time.After(backoff):
	}
	if ctx.Err() != nil {
		return
	}
	job.Attempts-- // Attempts снова счётчик попыток; requeue сохранит его.
	job.Attempts++
	if err := jobq.Enqueue(ctx, job); err != nil {
		slog.Error("worker: requeue failed", "job_id", job.ID, "type", job.Type, "error", err)
	}
}

// scheduleCleanup ставит задания очистки через interval (EDR-0020 §3.4).
func scheduleCleanup(ctx context.Context, jobq queue.JobQueue, interval time.Duration) {
	timer := time.NewTicker(interval)
	defer timer.Stop()

	enqueueCleanup(ctx, jobq)
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			enqueueCleanup(ctx, jobq)
		}
	}
}

// enqueueCleanup создаёт и ставит задания очистки (sessions, sso_states,
// audit) — EDR-0020 §3.3.
func enqueueCleanup(ctx context.Context, jobq queue.JobQueue) {
	for _, typ := range []string{queue.JobCleanupSessions, queue.JobCleanupSsoStates, queue.JobCleanupAudit} {
		job, err := queue.NewJob(typ, nil)
		if err != nil {
			slog.Error("worker: build cleanup job", "type", typ, "error", err)
			continue
		}
		if err := jobq.Enqueue(ctx, job); err != nil {
			slog.Error("worker: enqueue cleanup job", "type", typ, "error", err)
		}
	}
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
