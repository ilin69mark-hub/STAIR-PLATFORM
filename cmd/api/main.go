package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"stairplatform/internal/application/analytics"
	appast "stairplatform/internal/application/assistant"
	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/integrations"
	"stairplatform/internal/application/jobs"
	orderapp "stairplatform/internal/application/order"
	"stairplatform/internal/application/payments"
	"stairplatform/internal/application/project"
	testimonialapp "stairplatform/internal/application/testimonial"
	domevents "stairplatform/internal/domain/events"
	"stairplatform/internal/infrastructure/circuitbreaker"

	"stairplatform/internal/application/stair"
	appstorage "stairplatform/internal/application/storage"
	"stairplatform/internal/infrastructure/database"
	"stairplatform/internal/infrastructure/events"
	"stairplatform/internal/infrastructure/health"
	"stairplatform/internal/infrastructure/oidc"
	paymentsinfra "stairplatform/internal/infrastructure/payments"
	"stairplatform/internal/infrastructure/queue"
	infstorage "stairplatform/internal/infrastructure/storage"
	"stairplatform/internal/infrastructure/tracing"
	transporthttp "stairplatform/internal/transport/http"
	ws "stairplatform/internal/transport/websocket"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Initialize tracing if enabled
	tracingShutdown, err := tracing.InitTracer(context.Background(), tracing.Config{
		Enabled:     os.Getenv("STAIR_TRACING_ENABLED") == "true",
		ServiceName: "stair-platform",
		Endpoint:    os.Getenv("STAIR_TRACING_ENDPOINT"),
		SampleRate:  envFloat64("STAIR_TRACING_SAMPLE_RATE", 0.1),
		Environment: envOr("STAIR_ENVIRONMENT", "development"),
	})
	if err != nil {
		slog.Error("failed to init tracing", "error", err)
		os.Exit(1)
	}
	defer func() { _ = tracingShutdown(context.Background()) }()

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
	defer cancel()
	dbCfg := database.EnvConfig(dbURL,
		envInt("STAIR_DB_MAX_CONNS", 10),
		envInt("STAIR_DB_MIN_CONNS", 1),
		envDuration("STAIR_DB_MAX_CONN_LIFETIME", time.Hour),
		envDuration("STAIR_DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
	)
	pool, err := database.Connect(ctx, dbCfg)
	if err != nil {
		slog.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Периодический сбор метрик пула БД (каждые 30 секунд).
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			transporthttp.CollectDBPoolMetrics(pool)
		}
	}()

	// Применяем миграции при старте (идемпотентно).
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer migrateCancel()
	if err := database.Migrate(migrateCtx, pool, "migrations", "up"); err != nil {
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

	stairSvc := stair.NewService()
	projectSvc := project.NewService(
		database.NewProjectRepository(pool),
		stairSvc,
		project.DefaultRules(),
		auditSvc,
	)
	authSvc := auth.NewService(database.NewAuthRepository(pool), sessionTTL(), auditSvc)
	intSvc := integrations.NewService(database.NewIntegrationRepository(pool), queueBackend.Queue())
	// Jobs (EDR-0035): асинхронный расчёт — ставим в очередь, выполняет
	// воркер (calc не нужен API-процессу).
	jobsSvc := jobs.NewService(database.NewCalcJobRepository(pool), queueBackend.Queue(), nil)

	// AI-ассистенты (EDR-0036, Phase D D1): локальный детерминированный
	// бэкенд всегда; первичный OpenAI-совместимый — только при
	// STAIR_AI_BASE_URL (AI-0003: primary → local → error).
	assistantSvc := appast.NewService(stairSvc, auditSvc)
	if aiBase := os.Getenv("STAIR_AI_BASE_URL"); aiBase != "" {
		oai, err := appast.NewOpenAI(appast.OpenAIConfig{
			BaseURL: aiBase,
			APIKey:  os.Getenv("STAIR_AI_API_KEY"),
			Model:   os.Getenv("STAIR_AI_MODEL"),
		})
		if err != nil {
			slog.Error("assistant openai init failed", "error", err)
			os.Exit(1)
		}
		assistantSvc = assistantSvc.WithPrimaryBackend(oai)
	}

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
	var paymentProvider payments.Provider
	var stripeWebhookService transporthttp.StripeWebhookService
	if stripeKey := os.Getenv("STAIR_STRIPE_SECRET_KEY"); stripeKey != "" {
		sp := paymentsinfra.NewStripeProvider(stripeKey, os.Getenv("STAIR_STRIPE_WEBHOOK_SECRET"))
		adapter := paymentsinfra.NewStripeAdapter(sp)
		// Оборачиваем в CircuitBreaker для отказоустойчивости
		cb := circuitbreaker.New("stripe", circuitbreaker.Settings{
			FailureThreshold: envInt("STAIR_CB_FAILURE_THRESHOLD", 5),
			SuccessThreshold: envInt("STAIR_CB_SUCCESS_THRESHOLD", 3),
			Timeout:          envDuration("STAIR_CB_TIMEOUT", 30*time.Second),
			MaxRequests:      envInt("STAIR_CB_MAX_REQUESTS", 3),
		})
		paymentProvider = paymentsinfra.NewCBStripeAdapter(adapter, cb)
		stripeWebhookService = paymentsinfra.NewStripeWebhookService(adapter, logger)
		slog.Info("payments: using Stripe provider with circuit breaker")
	} else {
		paymentProvider = paymentsinfra.NewMockProvider(envString("STAIR_PAYMENT_BASE_URL", "http://localhost:8080"))
		slog.Info("payments: using mock provider")
	}
	paymentSvc := payments.NewService(
		database.NewPaymentRepository(pool),
		paymentProvider,
		paymentsinfra.NewVerifier(),
		0,
	)
	// Stripe webhook: подключаем application payments.Service как обработчик
	// интентов (подпись проверяет ad-hoc Stripe-реализация). Совместимость с
	// тестами сохранена — процессор подключается отдельным сеттером.
	if s, ok := stripeWebhookService.(*paymentsinfra.StripeWebhookServiceImpl); ok {
		s.WithIntentProcessor(paymentSvc)
	}
	paymentWebhookSecret := os.Getenv("STAIR_PAYMENT_WEBHOOK_SECRET")

	// WebSocket + EventBridge для real-time updates
	hub := ws.NewHub()
	go hub.Run()
	eb := ws.NewEventBridge(&eventBusAdapter{bus: events.NewBus()}, hub)
	eb.Start()

	// Retail orders (Store): розничные заказы-лиды клиентского сайта.
	ordersSvc := orderapp.NewService(database.NewOrderRepository(pool))

	// Client testimonials (Store): отзывы клиентов для лендинга и админки.
	testimonialSvc := testimonialapp.NewService(database.NewTestimonialRepository(pool))

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
		QuoteRateLimit:        envInt("STAIR_QUOTE_RATE_LIMIT", 30),
		QuoteRateWindow:       time.Minute,
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
		Jobs:                  jobsSvc,
		Assistant:             assistantSvc,
		Orders:                ordersSvc,
		Testimonials:          testimonialSvc,
		WebSocketHandler:      transporthttp.NewWebSocketHandler(hub, logger, &authTokenValidator{authSvc}),
		SecurityConfig: &transporthttp.SecurityConfig{
			AllowedOrigins: envStringSlice("STAIR_CORS_ORIGINS", []string{"http://localhost:3000"}),
			EnableHSTS:     envBool("STAIR_HSTS_ENABLED", false),
		},
		StripeWebhookService: stripeWebhookService,
	}

	// Readiness (EDR-0018 §3.2): SELECT 1 + Redis PING.
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
		defer func() { _ = redisClient.Close() }()
	}
	cfg.Readiness = &health.Checker{
		DB:    pool,
		Redis: redisClient,
	}

	router := transporthttp.NewRouter(stairSvc, projectSvc, authSvc, cfg, auditSvc)

	srv := &http.Server{
		Addr:              addr,
		Handler:           newRootHandler(router),
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

	// Graceful shutdown — порядок важен: WebSocket → HTTP → Queue → DB
	shutdownStart := time.Now()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()

	// 1. Останавливаем WebSocket hub (прекращаем обработку WS)
	hub.StopGracefully()
	slog.Info("shutdown: websocket hub stopped")

	// 2. Останавливаем HTTP server (прекращаем принимать новые запросы)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown: http server failed", "error", err)
	} else {
		slog.Info("shutdown: http server stopped")
	}

	// 3. Закрываем Redis connection
	if queueBackend != nil {
		queueBackend.Close()
		slog.Info("shutdown: queue backend closed")
	}

	// 4. Закрываем пул БД
	if pool != nil {
		pool.Close()
		slog.Info("shutdown: database pool closed")
	}

	slog.Info("shutdown complete", "duration", time.Since(shutdownStart).String())
}

// newRootHandler собирает корневой mux: API-роутер и, при
// STAIR_PPROF_ENABLED=true, маршруты net/http/pprof (B2, EDR-0033 §3.3).
// pprof по умолчанию выключен: включать только во внутреннем контуре/
// локально (доступ без auth).
func newRootHandler(api http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", api)
	if envBool("STAIR_PPROF_ENABLED", false) {
		// pprof защищен InternalOnlyMiddleware — доступ только из внутренней сети
		internalOnly := transporthttp.InternalOnlyMiddleware
		mux.Handle("/debug/pprof/", internalOnly(http.HandlerFunc(pprof.Index)))
		mux.Handle("/debug/pprof/cmdline", internalOnly(http.HandlerFunc(pprof.Cmdline)))
		mux.Handle("/debug/pprof/profile", internalOnly(http.HandlerFunc(pprof.Profile)))
		mux.Handle("/debug/pprof/symbol", internalOnly(http.HandlerFunc(pprof.Symbol)))
		mux.Handle("/debug/pprof/trace", internalOnly(http.HandlerFunc(pprof.Trace)))
		mux.Handle("/debug/pprof/goroutine", internalOnly(pprof.Handler("goroutine")))
		mux.Handle("/debug/pprof/heap", internalOnly(pprof.Handler("heap")))
		mux.Handle("/debug/pprof/allocs", internalOnly(pprof.Handler("allocs")))
		mux.Handle("/debug/pprof/threadcreate", internalOnly(pprof.Handler("threadcreate")))
		mux.Handle("/debug/pprof/block", internalOnly(pprof.Handler("block")))
		mux.Handle("/debug/pprof/mutex", internalOnly(pprof.Handler("mutex")))
		slog.Info("pprof profiling enabled", "path", "/debug/pprof/")
	}
	return mux
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

func envOr(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
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

// eventBusAdapter адаптирует events.Bus к интерфейсу EventBridge
// (метод Subscribe с func(ctx, event) error вместо events.Handler).
type eventBusAdapter struct {
	bus *events.Bus
}

func (a *eventBusAdapter) Subscribe(typ domevents.EventType, handler func(ctx context.Context, event domevents.Event) error) string {
	return a.bus.Subscribe(typ, handler)
}

func envStringSlice(key string, def []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	parts := []string{}
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}

// authTokenValidator adapts auth.Service to TokenValidator interface.
type authTokenValidator struct {
	svc *auth.Service
}

func (v *authTokenValidator) Authenticate(ctx context.Context, token string) (string, error) {
	user, _, err := v.svc.Authenticate(ctx, token)
	if err != nil {
		return "", err
	}
	return user.ID, nil
}
