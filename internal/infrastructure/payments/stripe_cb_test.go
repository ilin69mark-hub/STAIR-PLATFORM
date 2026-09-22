package payments

// S-133: CBStripeAdapter через реальный CB + inner на httptest.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stairplatform/internal/infrastructure/circuitbreaker"
)

func testCBAdapter(t *testing.T, handler http.HandlerFunc) *CBStripeAdapter {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	inner := NewStripeAdapter(testProvider(srv.URL))
	cb := circuitbreaker.New("test-cb", circuitbreaker.Settings{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          time.Second,
		MaxRequests:      1,
	})
	return NewCBStripeAdapter(inner, cb)
}

func okCheckoutHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cs_cb_1","url":"https://pay/x","status":"open","created":1700000000}`))
	}
}

func TestCBAdapterNameAndBreaker(t *testing.T) {
	a := testCBAdapter(t, okCheckoutHandler(t))
	if a.Name() != "stripe" {
		t.Fatalf("Name = %q", a.Name())
	}
	if a.CircuitBreaker() == nil {
		t.Fatal("CircuitBreaker nil")
	}
}

func TestCBAdapterCreateCheckoutOK(t *testing.T) {
	a := testCBAdapter(t, okCheckoutHandler(t))
	id, url, err := a.CreateCheckout(context.Background(), 1500, "USD")
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	if id != "cs_cb_1" || url == "" {
		t.Fatalf("got %q %q", id, url)
	}
}

func TestCBAdapterCreateCheckoutError(t *testing.T) {
	a := testCBAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	})
	if _, _, err := a.CreateCheckout(context.Background(), 1500, "USD"); err == nil {
		t.Fatal("want wrapped cb error")
	}
}

func TestCBAdapterVerifyAndParse(t *testing.T) {
	a := testCBAdapter(t, okCheckoutHandler(t))

	// Невалидная подпись — ошибка через CB.
	if err := a.VerifyWebhookSignature([]byte("x"), "bad"); err == nil {
		t.Fatal("bad signature: want error")
	}

	// Парсинг ok + err через CB.
	payload := []byte(`{"id":"evt_1","type":"checkout.session.completed","data":{"object":{"id":"cs_1","status":"complete","payment_status":"paid","created":1}},"created":1}`)
	ev, err := a.ParseWebhookEvent(payload)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.CheckoutID != "cs_1" {
		t.Fatalf("checkout = %q", ev.CheckoutID)
	}
	if _, err := a.ParseWebhookEvent([]byte(`{broken`)); err == nil {
		t.Fatal("bad json: want error")
	}
}
