package metrics

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestCounterIncr(t *testing.T) {
	r := NewRegistry()
	cv := r.Counter("http_requests_total", "Total HTTP requests", "method", "path", "status")
	cv.With("GET", "/x", "200").Inc()
	cv.With("GET", "/x", "200").Inc()
	cv.With("POST", "/x", "400").Add(3)

	if got := cv.With("GET", "/x", "200").Value(); got != 2 {
		t.Fatalf("GET count = %d, want 2", got)
	}
	if got := cv.With("POST", "/x", "400").Value(); got != 3 {
		t.Fatalf("POST count = %d, want 3", got)
	}

	var buf bytes.Buffer
	if err := r.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"# HELP http_requests_total Total HTTP requests",
		"# TYPE http_requests_total counter",
		`http_requests_total{method="GET",path="/x",status="200"} 2`,
		`http_requests_total{method="POST",path="/x",status="400"} 3`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestGauge(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("go_goroutines", "Goroutines")
	g.Set(12)
	g.Add(8)
	if got := g.Value(); got != 20 {
		t.Fatalf("gauge = %v, want 20", got)
	}

	var buf bytes.Buffer
	if err := r.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "go_goroutines 20") {
		t.Fatalf("gauge output wrong:\n%s", buf.String())
	}
}

func TestHistogram(t *testing.T) {
	r := NewRegistry()
	buckets := []float64{0.1, 1.0, 10.0}
	hv := r.Histogram("http_request_duration_seconds", "Duration", buckets, "method", "path")

	h := hv.With("GET", "/y")
	h.Observe(0.05)  // <= 0.1
	h.Observe(0.5)   // <= 1.0
	h.Observe(5.0)   // <= 10.0
	h.Observe(100.0) // только +Inf

	var buf bytes.Buffer
	if err := r.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`http_request_duration_seconds_bucket{method="GET",path="/y"} le="0.1" 1`,
		`http_request_duration_seconds_bucket{method="GET",path="/y"} le="1" 2`,
		`http_request_duration_seconds_bucket{method="GET",path="/y"} le="10" 3`,
		`http_request_duration_seconds_bucket{method="GET",path="/y"} le="+Inf" 4`,
		`http_request_duration_seconds_count{method="GET",path="/y"} 4`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
}

func TestLabelEscaping(t *testing.T) {
	r := NewRegistry()
	cv := r.Counter("m", "h", "path")
	cv.With(`/api/v1/projects and "quote"`).Inc()

	var buf bytes.Buffer
	if err := r.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), `path="/api/v1/projects and \"quote\""`) {
		t.Fatalf("escaping wrong:\n%s", buf.String())
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	cv := r.Counter("c", "h", "method")
	hv := r.Histogram("d", "h", []float64{0.5, 1.0}, "method")
	g := r.Gauge("g", "h")

	const goroutines = 32
	const per = 1000
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			method := "GET"
			if id%2 == 0 {
				method = "POST"
			}
			c := cv.With(method)
			h := hv.With(method)
			for j := 0; j < per; j++ {
				c.Inc()
				h.Observe(0.2)
			}
			g.Set(float64(id))
		}(i)
	}
	wg.Wait()

	var buf bytes.Buffer
	if err := r.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	// Все 32k инкрементов распределены по GET/POST.
	if !strings.Contains(out, `c{method="GET"} 16000`) {
		t.Fatalf("GET count missing:\n%s", out)
	}
	if !strings.Contains(out, `c{method="POST"} 16000`) {
		t.Fatalf("POST count missing:\n%s", out)
	}
}

func TestDefaultBuckets(t *testing.T) {
	r := NewRegistry()
	hv := r.Histogram("d", "h", nil)
	if len(hv.buckets) == 0 {
		t.Fatalf("expected default buckets")
	}
}
