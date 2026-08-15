// Package metrics реализует собственный реестр метрик в Prometheus
// text-формате на чистой стандартной библиотеке (EDR-0021 §3.2), без
// external-зависимостей (офлайн-сборка DEV-0009). Метрики безопасны при
// конкуренции (sync/atomic). Поддерживаются Counter, Gauge и Histogram
// с label-векторами.
package metrics

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Registry — набор метрик с label-векторами.
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*CounterVec
	gauges     map[string]*Gauge
	histograms map[string]*HistogramVec
	// helpText хранит HELP-строку для метрик, чьи векторы уже объявлены.
	counterHelp   map[string]string
	gaugeHelp     map[string]string
	histogramHelp map[string]string
}

// NewRegistry создаёт пустой реестр.
func NewRegistry() *Registry {
	return &Registry{
		counters:      make(map[string]*CounterVec),
		gauges:        make(map[string]*Gauge),
		histograms:    make(map[string]*HistogramVec),
		counterHelp:   make(map[string]string),
		gaugeHelp:     make(map[string]string),
		histogramHelp: make(map[string]string),
	}
}

// Counter объявляет (или возвращает существующий) Counter-вектор.
func (r *Registry) Counter(name, help string, labels ...string) *CounterVec {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cv, ok := r.counters[name]; ok {
		return cv
	}
	cv := &CounterVec{
		name:   name,
		help:   help,
		labels: normLabels(labels),
		vecs:   make(map[string]*Counter),
	}
	r.counters[name] = cv
	r.counterHelp[name] = help
	return cv
}

// Gauge объявляет (или возвращает существующий) Gauge.
func (r *Registry) Gauge(name, help string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g := &Gauge{name: name, help: help}
	r.gauges[name] = g
	r.gaugeHelp[name] = help
	return g
}

// Histogram объявляет (или возвращает существующий) Histogram-вектор.
// buckets должны быть строго возрастающими и положительными; пустой список —
// дефолт.
func (r *Registry) Histogram(name, help string, buckets []float64, labels ...string) *HistogramVec {
	r.mu.Lock()
	defer r.mu.Unlock()
	if hv, ok := r.histograms[name]; ok {
		return hv
	}
	hv := &HistogramVec{
		name:    name,
		help:    help,
		labels:  normLabels(labels),
		buckets: normBuckets(buckets),
		vecs:    make(map[string]*Histogram),
	}
	r.histograms[name] = hv
	r.histogramHelp[name] = help
	return hv
}

// Write выводит все метрики в Prometheus text-формате (EDR-0021 §3.3).
func (r *Registry) Write(w io.Writer) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	bw := bufio.NewWriter(w)

	names := sortedKeys(r.counters)
	for _, n := range names {
		cv := r.counters[n]
		fmt.Fprintf(bw, "# HELP %s %s\n", cv.name, r.counterHelp[n])
		fmt.Fprintf(bw, "# TYPE %s counter\n", cv.name)
		cv.mu.RLock()
		for _, key := range sortedKeys(cv.vecs) {
			c := cv.vecs[key]
			fmt.Fprintf(bw, "%s%s %d\n", cv.name, labelSelector(cv.labels, key), c.v.Load())
		}
		cv.mu.RUnlock()
	}

	for _, n := range sortedKeys(r.gauges) {
		g := r.gauges[n]
		fmt.Fprintf(bw, "# HELP %s %s\n", n, r.gaugeHelp[n])
		fmt.Fprintf(bw, "# TYPE %s gauge\n", n)
		fmt.Fprintf(bw, "%s %s\n", n, strconv.FormatFloat(g.Value(), 'g', -1, 64))
	}

	for _, n := range sortedKeys(r.histograms) {
		hv := r.histograms[n]
		fmt.Fprintf(bw, "# HELP %s %s\n", hv.name, r.histogramHelp[n])
		fmt.Fprintf(bw, "# TYPE %s histogram\n", hv.name)
		hv.mu.RLock()
		for _, key := range sortedKeys(hv.vecs) {
			h := hv.vecs[key]
			sel := labelSelector(hv.labels, key)
			cumul := uint64(0)
			for _, ub := range hv.buckets {
				h.mu.RLock()
				n := h.bucketCounts[ub]
				h.mu.RUnlock()
				cumul += n
				fmt.Fprintf(bw, "%s_bucket%s le=\"%s\" %d\n", hv.name, sel, formatBound(ub), cumul)
			}
			h.mu.RLock()
			total := h.count
			sum := h.sum
			h.mu.RUnlock()
			fmt.Fprintf(bw, "%s_bucket%s le=%q %d\n", hv.name, sel, "+Inf", total)
			fmt.Fprintf(bw, "%s_sum%s %s\n", hv.name, sel, strconv.FormatFloat(sum, 'g', -1, 64))
			fmt.Fprintf(bw, "%s_count%s %d\n", hv.name, sel, total)
		}
		hv.mu.RUnlock()
	}
	return bw.Flush()
}

// --- Counter-вектор ---

// CounterVec — вектор счётчиков по label-значениям.
type CounterVec struct {
	mu     sync.RWMutex
	name   string
	help   string
	labels []string
	vecs   map[string]*Counter
}

// Counter — атомарный счётчик.
type Counter struct {
	v atomic.Int64
}

// With возвращает счётчик для заданных label-значений.
func (cv *CounterVec) With(labels ...string) *Counter {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	key := labelsKey(labels)
	c, ok := cv.vecs[key]
	if !ok {
		c = &Counter{}
		cv.vecs[key] = c
	}
	return c
}

// Inc увеличивает счётчик на 1.
func (c *Counter) Inc() { c.v.Add(1) }

// Add увеличивает счётчик на delta (>=0).
func (c *Counter) Add(delta int64) { c.v.Add(delta) }

// Value возвращает текущее значение.
func (c *Counter) Value() int64 { return c.v.Load() }

// --- Gauge ---

// Gauge — абсолютное значение (хранится как биты float64 в atomic.Uint64,
// т.к. atomic.Float64 недоступен в этом окружении).
type Gauge struct {
	name string
	help string
	v    atomic.Uint64
}

// Set устанавливает значение.
func (g *Gauge) Set(v float64) { g.v.Store(math.Float64bits(v)) }

// Add прибавляет delta.
func (g *Gauge) Add(delta float64) {
	for {
		old := g.v.Load()
		nv := math.Float64frombits(old) + delta
		if g.v.CompareAndSwap(old, math.Float64bits(nv)) {
			return
		}
	}
}

// Value возвращает текущее значение.
func (g *Gauge) Value() float64 { return math.Float64frombits(g.v.Load()) }

// --- Histogram-вектор ---

// HistogramVec — вектор гистограмм по label-значениям.
type HistogramVec struct {
	mu      sync.RWMutex
	name    string
	help    string
	labels  []string
	buckets []float64
	vecs    map[string]*Histogram
}

// Histogram — распределение наблюдений по bucket'ам.
type Histogram struct {
	mu           sync.RWMutex
	bounds       []float64 // верхние границы (копия из вектора)
	bucketCounts map[float64]uint64
	count        uint64
	sum          float64
}

// With возвращает гистограмму для заданных label-значений.
func (hv *HistogramVec) With(labels ...string) *Histogram {
	hv.mu.Lock()
	defer hv.mu.Unlock()
	key := labelsKey(labels)
	h, ok := hv.vecs[key]
	if !ok {
		h = &Histogram{
			bounds:       append([]float64(nil), hv.buckets...),
			bucketCounts: make(map[float64]uint64, len(hv.buckets)),
		}
		hv.vecs[key] = h
	}
	return h
}

// Observe регистрирует наблюдение v: общий счётчик, накопление суммы и
// инкремент того bucket'а, в который попадает v (нижняя граница строго
// меньше, верхняя >= v). Write аккумулирует кумулятивные значения.
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.sum += v
	for _, ub := range h.bounds {
		if v <= ub {
			h.bucketCounts[ub]++
			return
		}
	}
}

// --- Вспомогательное ---

func normLabels(l []string) []string {
	if l == nil {
		return []string{}
	}
	return append([]string(nil), l...)
}

func normBuckets(b []float64) []float64 {
	if len(b) == 0 {
		return []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	}
	seen := make(map[float64]bool)
	var out []float64
	for _, v := range b {
		if v > 0 && !seen[v] && !math.IsNaN(v) && !math.IsInf(v, 0) {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Float64s(out)
	return out
}

// labelsKey — уникальный ключ вектора из label-значений (разделитель \x00).
func labelsKey(values []string) string {
	return strings.Join(values, "\x00")
}

// labelSelector собирает {label="value",...} из имён и ключа.
func labelSelector(names []string, key string) string {
	if len(names) == 0 {
		return ""
	}
	vals := strings.Split(key, "\x00")
	var b strings.Builder
	b.WriteString("{")
	for i, name := range names {
		if i > 0 {
			b.WriteString(",")
		}
		v := ""
		if i < len(vals) {
			v = vals[i]
		}
		b.WriteString(name)
		b.WriteString("=")
		b.WriteString(strconv.Quote(v))
	}
	b.WriteString("}")
	return b.String()
}

// formatBound форматирует верхнюю границу bucket'а (float64 → Prometheus le).
func formatBound(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
