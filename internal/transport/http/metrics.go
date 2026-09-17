package http

import (
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/circuitbreaker"
	"stairplatform/internal/infrastructure/metrics"
	"stairplatform/internal/infrastructure/security"
)

// Глобальный реестр метрик HTTP-слоя (EDR-0021 §3.4). Пакетная переменная:
// один реестр на инстанс, инициализируется при сборке пакета; labels region
// и node (instance id) добавляются в applyConfig.
var (
	httpMetricsReg = metrics.NewRegistry()

	httpRequests = httpMetricsReg.Counter(
		"http_requests_total", "Total HTTP requests", "method", "path", "status", "region", "node",
	)
	httpDuration = httpMetricsReg.Histogram(
		"http_request_duration_seconds", "HTTP request duration", nil, "method", "path", "region", "node",
	)
	goGoroutines   = httpMetricsReg.Gauge("go_goroutines", "Current goroutines")
	goMemAlloc     = httpMetricsReg.Gauge("go_memstats_alloc_bytes", "Memory allocated")
	processUptime  = httpMetricsReg.Gauge("process_uptime_seconds", "Process uptime")
	processStarted = time.Now()

	// DB pool metrics
	dbPoolActive    = httpMetricsReg.Gauge("db_pool_active_connections", "Active DB connections")
	dbPoolIdle      = httpMetricsReg.Gauge("db_pool_idle_connections", "Idle DB connections")
	dbPoolMax       = httpMetricsReg.Gauge("db_pool_max_connections", "Max DB connections")
	dbPoolOpen      = httpMetricsReg.Gauge("db_pool_open_connections", "Open DB connections")
	dbPoolWaitCount = httpMetricsReg.Counter("db_pool_wait_count_total", "DB pool wait count")
)

// metricsConfig хранит конфигурацию метрик (region, instance).
type metricsConfig struct {
	region   string
	instance string
}

var metricsCfg = &metricsConfig{}

// SetMetricsConfig устанавливает конфигурацию метрик (вызывается из applyConfig).
func SetMetricsConfig(region, instance string) {
	metricsCfg.region = region
	metricsCfg.instance = instance
}

// uuidSegment — сегмент пути, являющийся UUID (канонизация label path,
// EDR-0021 инвариант 4).
var uuidSegment = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// canonicalPath заменяет UUID-сегменты пути на {id} (низкая кардинальность
// label path в /metrics).
func canonicalPath(p string) string {
	segs := splitPath(p)
	for i, s := range segs {
		if uuidSegment.MatchString(s) {
			segs[i] = "{id}"
		}
	}
	return joinPath(segs)
}

func splitPath(p string) []string {
	out := []string{}
	start := 0
	for i := 0; i <= len(p); i++ {
		if i == len(p) || p[i] == '/' {
			if i > start {
				out = append(out, p[start:i])
			}
			start = i + 1
		}
	}
	return out
}

func joinPath(segs []string) string {
	out := ""
	for _, s := range segs {
		out += "/" + s
	}
	return out
}

// metricLabels собирает label-значения для векторов (method,path,status +
// region,node).
func metricLabels(method, path, status string) []string {
	return []string{method, path, status, metricsCfg.region, metricsCfg.instance}
}

// recordHTTPMetrics регистрирует метрику завершённого HTTP-запроса
// (EDR-0021 §3.4). Вызывается из withLogging (доступен status, duration).
func recordHTTPMetrics(method, path string, status int, dur time.Duration) {
	p := canonicalPath(path)
	st := strconv.Itoa(status)
	httpRequests.With(metricLabels(method, p, st)...).Inc()
	httpDuration.With(metricLabels(method, p, st)...).Observe(dur.Seconds())
}

// runtimeMetricsCache кэширует runtime-метрики для избежания
// ReadMemStats на каждый /metrics запрос.
var runtimeMetricsCache = struct {
	lastUpdate time.Time
	goroutines int
	memAlloc   uint64
	mu         sync.Mutex
}{}

// refreshRuntimeMetrics обновляет runtime-gauges (goroutines, mem, uptime).
// Кэширует результат на 10 секунд — ReadMemStats это stop-the-world.
func refreshRuntimeMetrics() {
	runtimeMetricsCache.mu.Lock()
	defer runtimeMetricsCache.mu.Unlock()

	if time.Since(runtimeMetricsCache.lastUpdate) < 10*time.Second {
		// Используем кэшированные значения.
		goGoroutines.Set(float64(runtimeMetricsCache.goroutines))
		goMemAlloc.Set(float64(runtimeMetricsCache.memAlloc))
		processUptime.Set(time.Since(processStarted).Seconds())
		return
	}

	goGoroutines.Set(float64(runtime.NumGoroutine()))
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	goMemAlloc.Set(float64(m.Alloc))
	processUptime.Set(time.Since(processStarted).Seconds())

	runtimeMetricsCache.lastUpdate = time.Now()
	runtimeMetricsCache.goroutines = runtime.NumGoroutine()
	runtimeMetricsCache.memAlloc = m.Alloc
}

// dbPoolMetricsCache хранит последний счётчик EmptyAcquireCount для расчёта
// дельты: pgxpool.Stat().EmptyAcquireCount() — кумулятивный (с момента
// создания пула), а Prometheus-счётчик db_pool_wait_count_total также должен
// расти только на инкремент (иначе каждое добавление задваивает значение).
var dbPoolMetricsCache = struct {
	mu            sync.Mutex
	lastWaitCount int64
}{}

// CollectDBPoolMetrics собирает метрики пула БД (вызывается периодически).
func CollectDBPoolMetrics(pool *pgxpool.Pool) {
	if pool == nil {
		return
	}
	stat := pool.Stat()
	dbPoolActive.Set(float64(stat.AcquiredConns()))
	dbPoolIdle.Set(float64(stat.IdleConns()))
	dbPoolMax.Set(float64(stat.MaxConns()))
	dbPoolOpen.Set(float64(stat.TotalConns()))

	// Кумулятивный счётчик → дельта с прошлого сбора.
	total := stat.EmptyAcquireCount()
	dbPoolMetricsCache.mu.Lock()
	delta := total - dbPoolMetricsCache.lastWaitCount
	dbPoolMetricsCache.lastWaitCount = total
	dbPoolMetricsCache.mu.Unlock()
	if delta > 0 {
		dbPoolWaitCount.With().Add(delta)
	}
}

// handleMetrics — GET /metrics (Prometheus text-format, EDR-0021 §6).
// Публичный: не требует auth/CSRF/rate-limit (мониторинг не должен
// блокироваться). Пишет HTTP-реестр, реестр прикладного слоя расчёта,
// circuit breaker и rate limiter метрики.
func handleMetrics(w http.ResponseWriter, _ *http.Request) {
	refreshRuntimeMetrics()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if err := httpMetricsReg.Write(w); err != nil {
		writeError(w, http.StatusInternalServerError, "metrics", "Не удалось сохранить метрики.")
		return
	}
	if err := stair.ServiceMetricsReg.Write(w); err != nil {
		writeError(w, http.StatusInternalServerError, "metrics", "Не удалось сохранить метрики.")
		return
	}
	// Circuit breaker metrics
	if err := circuitbreaker.CBRegistry.Write(w); err != nil {
		writeError(w, http.StatusInternalServerError, "metrics", "Не удалось сохранить CB метрики.")
		return
	}
	// Rate limiter metrics
	if err := security.RateLimitRegistry.Write(w); err != nil {
		writeError(w, http.StatusInternalServerError, "metrics", "Не удалось сохранить rate limit метрики.")
		return
	}
	// Compression metrics
	if err := compressionRegistry.Write(w); err != nil {
		writeError(w, http.StatusInternalServerError, "metrics", "Не удалось сохранить compression метрики.")
		return
	}
	// Response cache metrics
	if err := cacheRegistry.Write(w); err != nil {
		writeError(w, http.StatusInternalServerError, "metrics", "Не удалось сохранить cache метрики.")
	}
}
