package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInternalOnlyMiddleware(t *testing.T) {
	h := InternalOnlyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	// internal: 127.0.0.1
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("127.0.0.1 want 200 got %d", rec.Code)
	}
	// external
	req2 := httptest.NewRequest("GET", "/metrics", nil)
	req2.RemoteAddr = "8.8.8.8:1234"
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != 403 {
		t.Fatalf("8.8.8.8 want 403 got %d", rec2.Code)
	}
	// 10.x
	req3 := httptest.NewRequest("GET", "/metrics", nil)
	req3.RemoteAddr = "10.1.2.3:1111"
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)
	if rec3.Code != 200 {
		t.Fatalf("10.x want 200 got %d", rec3.Code)
	}
}

func TestExtractIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	if extractIP(req) != "1.2.3.4" {
		t.Fatalf("got %q", extractIP(req))
	}
	req.RemoteAddr = "badaddr"
	if extractIP(req) != "badaddr" {
		t.Fatalf("badaddr fallback")
	}
}

func TestIsInternalIP(t *testing.T) {
	// test via allowedNets from middleware: just check InternalOnly via request
	if isInternalIP("not-an-ip", nil) {
		t.Fatal("invalid ip should be false")
	}
}
