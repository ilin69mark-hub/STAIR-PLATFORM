package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"stairplatform/internal/application/analytics"
	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/integrations"
	"stairplatform/internal/application/payments"
	"stairplatform/internal/application/project"

	"stairplatform/internal/application/stair"
	appstorage "stairplatform/internal/application/storage"
	"stairplatform/internal/infrastructure/database"
	"stairplatform/internal/infrastructure/health"
	"stairplatform/internal/infrastructure/oidc"
	paymentsinfra "stairplatform/internal/infrastructure/payments"
	"stairplatform/internal/infrastructure/queue"
	infstorage "stairplatform/internal/infrastructure/storage"
	transporthttp "stairplatform/internal/transport/http"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	instanceID := os.Getenv("STAIR_INSTANCE_ID")
	shutdownTimeout := envDuration("STAIR_SHUTDOWN_TIMEOUT", 10*time.Second)
	region := os.Getenv("STAIR_REGION")

	addr := os.Getenv("STAIR_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// Подключение к БД (MV-08, persistence). STAIR_DATABASE_URL обязателен.
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

	// Применяем миграции при старте (идемпотентно).
	if err := database.Migrate(ctx, pool, "migrations", "up"); err != nil {
		slog.Error("database migrate failed", "error", err)
		os.Exit(1)
	}

	auditSvc := audit.NewService(database.NewAuditRepository(pool))

	// Analytics (EDR-0028, Phase F): read-only агрегации Usage Analytics.
	analyticsSvc := analytics.NewService(database.NewAnalyticsRepository(pool))

	// Readyness/честная очередь заданий (EDR-0020): Redis-бэкенд при наличии
	// STAIR_REDIS_ADDR, иначе in-memory (single-instance). Queue нужна
	// интеграциям (ERP quote, EDR-0023) и воркеру.
	queueBackend := newAPIQueueBackend(os.Getenv("STAIR_REDIS_ADDR"))
	defer queueBackend.Close()

	stairSvc := stair.NewService()
	projectSvc := project.NewService(
		database.NewProjectRepository(pool),
		stairSvc,
		project.DefaultRules(),
		auditSvc,
	)
	authSvc := auth.NewService(database.NewAuthRepository(pool), sessionTTL(), auditSvc)
	intSvc := integrations.NewService(database.NewIntegrationRepository(pool), queueBackend.Queue())

	// Storage (EDR-0026 §3.5): объектное хранилище из окружения. Бэкенд
	// задаётся STAIR_STORAGE_BACKEND (filesystem|s3); сбой конфигурации —
	// фатален (сервис доступен только при валидном хранилище).
	storageSvc, err := newStorageService()
	if err != nil {
		slog.Error("storage init failed", "error", err)
		os.Exit(1)
	}

	// Payments (EDR-0027 §3.3): платёжные интенты + входящий webhook PSP.
	// maxAge=0 → верификатор использует MaxTimestampAge (5 мин).
	paymentSvc := payments.NewService(
		database.NewPaymentRepository(pool),
		paymentsinfra.NewMockProvider(envString("STAIR_PAYMENT_BASE_URL", "http://localhost:8080")),
		paymentsinfra.NewVerifier(),
		0,
	)
	paymentWebhookSecret := os.Getenv("STAIR_PAYMENT_WEBHOOK_SECRET")

	// SSO (EDR-0017 §3.2): OIDC-провайдер из окружения. Пока STAIR_SSO_ISSUER
	// не задан — SSO выключен (публичный ключ, кнопка на фронте не видна).
	if issuer := os.Getenv("STAIR_SSO_ISSUER"); issuer != "" {
		authSvc = authSvc.WithOIDCProvider(oidc.New(oidc.Config{
			ProviderName: envString("STAIR_SSO_PROVIDER", "sso"),
			Issuer:       issuer,
			ClientID:     os.Getenv("STAIR_SSO_CLIENT_ID"),
			ClientSecret: os.Getenv("STAIR_SSO_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("STAIR_SSO_REDIRECT_URL"),
		}))
	}

	cfg := transporthttp.Config{
		CookieSecure:          envBool("STAIR_COOKIE_SECURE", false),
		LoginRateLimit:        envInt("STAIR_LOGIN_RATE_LIMIT", 10),
		LoginRateWindow:       time.Minute,
		RegisterRateLimit:     envInt("STAIR_REGISTER_RATE_LIMIT", 5),
		RegisterRateWindow:    time.Minute,
		RedisAddr:             os.Getenv("STAIR_REDIS_ADDR"),
		MaxBodyBytes:          1 << 20,
		InstanceID:            instanceID,
		ShutdownTimeout:       shutdownTimeout,
		Region:                region,
		Integrations:          intSvc,
		Storage:               storageSvc,
		Payments:              paymentSvc,
		PaymentsWebhookSecret: paymentWebhookSecret,
		Analytics:             analyticsSvc,
	}

	// Readiness (EDR-0018 §3.2): SELECT 1 + Redis PING.
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
		defer redisClient.Close()
	}
	cfg.Readiness = &health.Checker{
		DB:    pool,
		Redis: redisClient,
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           transporthttp.NewRouter(stairSvc, projectSvc, authSvc, cfg, auditSvc),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("api server starting", "addr", addr, "instance_id", instanceID, "region", region)
		errCh <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("api server failed", "error", err)
			os.Exit(1)
		}
	case sig := <-stop:
		slog.Info("shutdown signal received", "signal", sig.String())
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("api server stopped")
}

// sessionTTL возвращает время жизни сессии из STAIR_SESSION_TTL
// (секунды; дефолт 24ч = 86400с).
func sessionTTL() time.Duration {
	sec := envInt("STAIR_SESSION_TTL", 86400)
	return time.Duration(sec) * time.Second
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
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

// envDuration возвращает длительность из env (time.Duration синтаксис).
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

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// newStorageService создаёт сервис объектного хранилища (EDR-0026 §3.5)
// по STAIR_STORAGE_BACKEND и параметрам окружения.
func newStorageService() (*appstorage.Service, error) {
	store, err := infstorage.NewObjectStore(
		os.Getenv("STAIR_STORAGE_BACKEND"),
		infstorage.Options{
			FSRoot:    envString("STAIR_STORAGE_DIR", "./storage"),
			Endpoint:  os.Getenv("STAIR_S3_ENDPOINT"),
			Bucket:    os.Getenv("STAIR_S3_BUCKET"),
			Region:    envString("STAIR_S3_REGION", "us-east-1"),
			AccessKey: os.Getenv("STAIR_S3_ACCESS_KEY"),
			SecretKey: os.Getenv("STAIR_S3_SECRET_KEY"),
			PathStyle: envBool("STAIR_S3_PATH_STYLE", true),
		},
	)
	if err != nil {
		return nil, err
	}
	return appstorage.NewService(store), nil
}

// apiQueueBackend оборачивает выбранный бэкенд очереди заданий (EDR-0020):
// Redis List при доступном STAIR_REDIS_ADDR, иначе in-memory fallback.
// Используется интеграциями (ERP quote-send, EDR-0023) для постановки
// заданий, которые выполняет отдельный процесс worker.
type apiQueueBackend struct {
	jobq queue.JobQueue
	rl   *redis.Client
}

func newAPIQueueBackend(addr string) *apiQueueBackend {
	if addr == "" {
		return &apiQueueBackend{jobq: queue.NewMemoryQueue()}
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("api: redis unavailable, using memory queue", "addr", addr, "error", err)
		_ = client.Close()
		return &apiQueueBackend{jobq: queue.NewMemoryQueue()}
	}
	return &apiQueueBackend{jobq: queue.NewRedisQueue(client, "stair-jobs", 2*time.Second), rl: client}
}

func (b *apiQueueBackend) Queue() queue.JobQueue { return b.jobq }

func (b *apiQueueBackend) Close() {
	if b.rl != nil {
		_ = b.rl.Close()
	}
}
