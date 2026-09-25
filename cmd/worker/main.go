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
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"stairplatform/internal/application/jobs"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/database"
	"stairplatform/internal/infrastructure/envguard"
	infintegrations "stairplatform/internal/infrastructure/integrations"
	"stairplatform/internal/infrastructure/queue"
	"stairplatform/internal/infrastructure/redisconf"
	"stairplatform/internal/infrastructure/secrets"
	"stairplatform/internal/infrastructure/sentry"
	"stairplatform/internal/infrastructure/tracing"
	"stairplatform/internal/version"
)

// tracerName — имя OpenTelemetry-трейсера воркера (Jaeger service: stair-platform-worker).
const tracerName = "stair-platform-worker"

// isProductionEnv — нормализованный продакшен-детект (production|prod,
// EqualFold). Неканоничное значение STAIR_ENVIRONMENT не должно отключать
// SSRF-политику и требование STAIR_SECRETS_KEY (S-104, S-113).
// Реализация вынесена в internal/infrastructure/envguard (S-151: единый
// fail-closed детект для api и worker).
func isProductionEnv(environment string) bool {
	return envguard.IsProduction(environment)
}

// webhookPolicy строит SSRF-политику webhook-доставки (S1-1, S-104).
// В проде (production|prod, EqualFold) loopback запрещён; внутренние хосты —
// только через allowHostsCSV (STAIR_WEBHOOK_ALLOW_HOSTS). В dev loopback
// разрешён (обратная совместимость, локальные интеграции).
func webhookPolicy(environment, allowHostsCSV string) infintegrations.Policy {
	policy := infintegrations.Policy{}
	if !isProductionEnv(environment) {
		policy.AllowLoopback = true
	}
	for _, h := range strings.Split(allowHostsCSV, ",") {
		if h = strings.TrimSpace(h); h != "" {
			policy.AllowHosts = append(policy.AllowHosts, h)
		}
	}
	return policy
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info(version.String())

	// S-151: fail-closed на STAIR_ENVIRONMENT — пустое/неизвестное значение
	// молча отключало SSRF-политику и требование STAIR_SECRETS_KEY.
	if _, err := envguard.Validate(os.Getenv("STAIR_ENVIRONMENT")); err != nil {
		logger.Error("invalid STAIR_ENVIRONMENT", "error", err)
		os.Exit(1)
	}

	// Трассировка (OTLP → Jaeger/Tempo; STAIR_TRACING_ENABLED="true").
	tracingShutdown, err := tracing.InitTracer(context.Background(), tracing.Config{
		Enabled:     os.Getenv("STAIR_TRACING_ENABLED") == "true",
		ServiceName: tracerName,
		Endpoint:    os.Getenv("STAIR_TRACING_ENDPOINT"),
		SampleRate:  envFloat64("STAIR_TRACING_SAMPLE_RATE", 0.1),
		Environment: os.Getenv("STAIR_ENVIRONMENT"),
	})
	if err != nil {
		slog.Error("worker: failed to init tracing", "error", err)
		os.Exit(1)
	}
	defer func() { _ = tracingShutdown(context.Background()) }()

	// S-140: Sentry error tracking (см. cmd/api/main.go — та же политика:
	// DSN только через env, пусто = выключено, трассировка 0 по умолчанию).
	sentryShutdown, err := sentry.Init(sentry.Config{
		DSN:              os.Getenv("STAIR_SENTRY_DSN"),
		Environment:      os.Getenv("STAIR_ENVIRONMENT"),
		ServiceName:      "stair-platform-worker",
		Release:          version.Version,
		TracesSampleRate: envFloat64("STAIR_SENTRY_TRACES_SAMPLE_RATE", 0),
	})
	if err != nil {
		slog.Error("worker: failed to init sentry", "error", err)
		os.Exit(1)
	}
	defer sentryShutdown()

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
		database.NewIntegrationRepository(pool), newJobsService(pool), retentionDays)
	// S-104: нормализованный продакшен-детект (production|prod, EqualFold).
	// Неканоничное значение STAIR_ENVIRONMENT не должно отключать SSRF-политику.
	// S1-1: SSRF-политика webhook-доставки. В проде loopback запрещён,
	// внутренние хосты — только через STAIR_WEBHOOK_ALLOW_HOSTS.
	reg.withWebhookClient(infintegrations.NewPolicyClient(0,
		webhookPolicy(os.Getenv("STAIR_ENVIRONMENT"), os.Getenv("STAIR_WEBHOOK_ALLOW_HOSTS"))))
	// S1-2: шифрование webhook-секретов at rest. Без STAIR_SECRETS_KEY —
	// legacy-plaintext (dev); в проде ключ обязателен (см. .env.production).
	// AUDIT-EXCEPTION(E01): мастер-ключ задаёт человек, см. docs/SECURITY_EXCEPTIONS.yml
	if keyHex := os.Getenv("STAIR_SECRETS_KEY"); keyHex != "" {
		key, err := secrets.KeyFromHex(keyHex)
		if err != nil {
			slog.Error("STAIR_SECRETS_KEY invalid (need 64 hex chars = 32 bytes)", "error", err)
			os.Exit(1)
		}
		box, err := secrets.NewBox(key)
		if err != nil {
			slog.Error("secrets box init failed", "error", err)
			os.Exit(1)
		}
		reg.withSecretCrypter(box)
		slog.Info("worker: webhook secrets encryption enabled")
	} else if isProductionEnv(os.Getenv("STAIR_ENVIRONMENT")) {
		slog.Error("STAIR_SECRETS_KEY is not set (webhook secrets encryption mandatory in production)")
		os.Exit(1)
	}

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

// newJobsService создаёт сервис фоновых заданий (EDR-0035) для воркера:
// только выполнение (calc не nil), очередь не нужна.
func newJobsService(pool *pgxpool.Pool) *jobs.Service {
	return jobs.NewService(database.NewCalcJobRepository(pool), nil, stair.NewService().Calculate)
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
	client := redis.NewClient(redisconf.FromEnv(addr))
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
	ctx, span := otel.Tracer(tracerName).Start(ctx, "job.process")
	span.SetAttributes(
		attribute.String("job.id", job.ID),
		attribute.String("job.type", job.Type),
		attribute.Int("job.attempts", job.Attempts),
	)
	defer span.End()

	err := reg.Handle(ctx, job)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.Warn("worker: job failed", "job_id", job.ID, "type", job.Type, "error", err)
	} else {
		slog.Info("worker: job done", "job_id", job.ID, "type", job.Type)
	}

	if err == nil {
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

func envFloat64(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
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
