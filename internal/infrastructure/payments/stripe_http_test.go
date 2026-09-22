package payments

// S-133: HTTP-пути StripeProvider через httptest (без сети).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testProvider(url string) *StripeProvider {
	p := NewStripeProvider("sk_test", "whsec_test")
	p.baseURL = url
	return p
}

func TestCreateCheckoutSessionOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/checkout/sessions" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.Form.Get("mode") != "payment" {
			t.Errorf("mode = %q", r.Form.Get("mode"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cs_123","url":"https://checkout.stripe.com/pay/cs_123","status":"open","created":1700000000}`))
	}))
	defer srv.Close()

	sess, err := testProvider(srv.URL).CreateCheckoutSession(context.Background(), CheckoutParams{
		OrderID: "order-1", Amount: 5000, Currency: "usd",
		SuccessURL: "https://x/success", CancelURL: "https://x/cancel",
	})
	if err != nil {
		t.Fatalf("CreateCheckoutSession: %v", err)
	}
	if sess.ID != "cs_123" || sess.CheckoutURL == "" || sess.Amount != 5000 || sess.Currency != "usd" {
		t.Fatalf("session wrong: %+v", sess)
	}
	if sess.Status != CheckoutStatusPending {
		t.Fatalf("new session status = %q, want pending", sess.Status)
	}
}

func TestCreateCheckoutSessionErrors(t *testing.T) {
	ctx := context.Background()
	newParams := func() CheckoutParams {
		return CheckoutParams{OrderID: "o", Amount: 100, Currency: "usd"}
	}

	badStatus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	defer badStatus.Close()
	if _, err := testProvider(badStatus.URL).CreateCheckoutSession(ctx, newParams()); err == nil {
		t.Fatal("non-200: want error")
	}

	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{broken`))
	}))
	defer badJSON.Close()
	if _, err := testProvider(badJSON.URL).CreateCheckoutSession(ctx, newParams()); err == nil {
		t.Fatal("bad json: want error")
	}
}

func TestGetSessionStatuses(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name       string
		payment    string
		wantStatus string
	}{
		{"paid", "paid", CheckoutStatusCompleted},
		{"unpaid", "unpaid", CheckoutStatusPending},
		{"no_payment_required", "no_payment_required", CheckoutStatusCompleted},
		{"unknown maps pending", "weird", CheckoutStatusPending},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/checkout/sessions/cs_1" {
					t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"cs_1","status":"complete","amount_total":5000,"currency":"usd","payment_status":"` + c.payment + `","created":1700000000,"metadata":{"order_id":"o-1"}}`))
			}))
			defer srv.Close()

			sess, err := testProvider(srv.URL).GetSession(ctx, "cs_1")
			if err != nil {
				t.Fatalf("GetSession: %v", err)
			}
			if sess.Status != c.wantStatus {
				t.Fatalf("status = %q, want %q", sess.Status, c.wantStatus)
			}
			if sess.Amount != 5000 || sess.Currency != "usd" || sess.Metadata["order_id"] != "o-1" {
				t.Fatalf("fields wrong: %+v", sess)
			}
		})
	}
}

func TestGetSessionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"no such session"}`))
	}))
	defer srv.Close()
	if _, err := testProvider(srv.URL).GetSession(context.Background(), "cs-404"); err == nil {
		t.Fatal("404: want error")
	}
}
