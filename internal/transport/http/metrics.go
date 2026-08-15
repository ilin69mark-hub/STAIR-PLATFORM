package http

import (
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/metrics"
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
)

var (
	// regionLabel — текущий регион (STAIR_REGION); "" — по умолчанию.
	regionLabel   = ""
	instanceLabel = ""
)

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
	return []string{method, path, status, regionLabel, instanceLabel}
}

// recordHTTPMetrics регистрирует метрику завершённого HTTP-запроса
// (EDR-0021 §3.4). Вызывается из withLogging (доступен status, duration).
func recordHTTPMetrics(method, path string, status int, dur time.Duration) {
	p := canonicalPath(path)
	st := strconv.Itoa(status)
	httpRequests.With(metricLabels(method, p, st)...).Inc()
	httpDuration.With(metricLabels(method, p, st)...).Observe(dur.Seconds())
}

// refreshRuntimeMetrics обновляет runtime-gauges (goroutines, mem, uptime).
func refreshRuntimeMetrics() {
	goGoroutines.Set(float64(runtime.NumGoroutine()))
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	goMemAlloc.Set(float64(m.Alloc))
	processUptime.Set(time.Since(processStarted).Seconds())
}

// handleMetrics — GET /metrics (Prometheus text-format, EDR-0021 §6).
// Публичный: не требует auth/CSRF/rate-limit (мониторинг не должен
// блокироваться). Пишет HTTP-реестр и реестр прикладного слоя расчёта
// (stair_calculate_duration_seconds и др., B2, EDR-0033 §3.2).
func handleMetrics(w http.ResponseWriter, _ *http.Request) {
	refreshRuntimeMetrics()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if err := httpMetricsReg.Write(w); err != nil {
		writeError(w, http.StatusInternalServerError, "metrics", "metrics write failed")
		return
	}
	if err := stair.ServiceMetricsReg.Write(w); err != nil {
		writeError(w, http.StatusInternalServerError, "metrics", "metrics write failed")
	}
}
