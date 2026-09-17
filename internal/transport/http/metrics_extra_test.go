package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/circuitbreaker"
	"stairplatform/internal/infrastructure/security"
)

var errWriteLimit = errors.New("test write limit reached")

type failAtBytes struct {
	http.ResponseWriter
	written int
	limit   int
}

func (f *failAtBytes) Write(p []byte) (int, error) {
	remaining := f.limit - f.written
	if remaining <= 0 {
		return 0, errWriteLimit
	}
	if len(p) > remaining {
		f.written += remaining
		return remaining, errWriteLimit
	}
	f.written += len(p)
	return len(p), nil
}

func TestCollectDBPoolMetricsNilPool(t *testing.T) {
	CollectDBPoolMetrics(nil)
}

func TestCollectDBPoolMetricsEmptyPool(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@localhost:5432/db")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	CollectDBPoolMetrics(pool)
	CollectDBPoolMetrics(pool)
}

func registrySizes(t *testing.T) []int {
	t.Helper()
	httpRequests.With(metricLabels("POST", "/api/v1/test", "200")...).Inc()
	refreshRuntimeMetrics()
	regs := []struct {
		name string
		reg  interface{ Write(io.Writer) error }
	}{
		{"http", httpMetricsReg},
		{"stair", stair.ServiceMetricsReg},
		{"cb", circuitbreaker.CBRegistry},
		{"ratelimit", security.RateLimitRegistry},
		{"compression", compressionRegistry},
		{"cache", cacheRegistry},
	}
	sizes := make([]int, len(regs))
	for i, r := range regs {
		var buf bytes.Buffer
		if err := r.reg.Write(&buf); err != nil {
			t.Fatalf("%s registry Write: %v", r.name, err)
		}
		sizes[i] = buf.Len()
	}
	return sizes
}

func TestHandleMetricsWriteErrors(t *testing.T) {
	prefix := 0
	for i, sz := range registrySizes(t) {
		if sz == 0 {
			continue
		}
		limit := prefix
		if i == 0 {
			limit = 1
		}
		rec := httptest.NewRecorder()
		fw := &failAtBytes{ResponseWriter: rec, limit: limit}
		handleMetrics(fw, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("registry %d limit %d: expected 500, got %d", i, limit, rec.Code)
		}
		prefix += sz
	}
}

func TestHandleMetricsOK(t *testing.T) {
	httpRequests.With(metricLabels("POST", "/api/v1/stairs:calculate", "200")...).Inc()
	rec := httptest.NewRecorder()
	handleMetrics(rec, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("go_goroutines")) {
		t.Fatalf("expected runtime metrics in body")
	}
}
