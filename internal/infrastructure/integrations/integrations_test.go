package integrations

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	const secret = "top-secret"
	body := []byte(`{"quote":1}`)
	now := time.Now().Unix()
	sig, err := Sign(secret, now, body)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(sig) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(sig))
	}

	// Корректная подпись проходит с окном, включающим текущий момент.
	if err := Verify(secret, fmt.Sprintf("%d", now), fmt.Sprintf("v1:%s", sig), body, time.Minute); err != nil {
		t.Fatalf("Verify(valid): %v", err)
	}
	// Подделанное тело отклоняется.
	badBody := []byte(`{"quote":2}`)
	if err := Verify(secret, fmt.Sprintf("%d", now), fmt.Sprintf("v1:%s", sig), badBody, time.Minute); err == nil {
		t.Fatal("Verify(expected mismatch) should fail")
	}
	// Неверный секрет отклоняется.
	sig2, _ := Sign("other", now, body)
	if err := Verify(secret, fmt.Sprintf("%d", now), fmt.Sprintf("v1:%s", sig2), body, time.Minute); err == nil {
		t.Fatal("Verify(wrong secret) should fail")
	}
	// Протухший timestamp отклоняется (replay-защита).
	if err := Verify(secret, fmt.Sprintf("%d", now-1000), fmt.Sprintf("v1:%s", sig), body, time.Minute); err == nil {
		t.Fatal("Verify(stale) should fail")
	}
	// Отсутствующие заголовки отклоняются.
	if err := Verify(secret, "", "", body, time.Minute); err == nil {
		t.Fatal("Verify(missing headers) should fail")
	}
}

func TestClientSendHappyPath(t *testing.T) {
	const secret = "s3cr3t"
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем подпись на принимающей стороне.
		sig := r.Header.Get(HeaderSignature)
		ts := r.Header.Get(HeaderTimestamp)
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		if err := Verify(secret, ts, sig, body, time.Minute); err != nil {
			t.Errorf("server verify: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(0)
	payload := []byte(`{"hello":"world"}`)
	if err := client.Send(context.Background(), server.URL, secret, payload); err != nil {
		t.Fatalf("Send: %v", err)
	}
	select {
	case b := <-received:
		if string(b) != string(payload) {
			t.Fatalf("payload mismatch: %q", b)
		}
	case <-time.After(time.Second):
		t.Fatal("no delivery received")
	}
}

func TestClientSendNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(0)
	if err := client.Send(context.Background(), server.URL, "s", []byte("{}")); err == nil {
		t.Fatal("expected error for 500")
	} else if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error should mention status: %v", err)
	}
}

func TestClientSendHTTPSRequired(t *testing.T) {
	client := NewClient(0)
	if err := client.Send(context.Background(), "http://example.com/hook", "s", []byte("{}")); err == nil {
		t.Fatal("expected https-required error for non-local http url")
	}
	// localhost разрешён в dev: https-проверка не должна отклонять.
	if err := validateOutboundURL("http://127.0.0.1:8080/hook"); err != nil {
		t.Fatalf("localhost should be allowed: %v", err)
	}
}

func TestValidateOutboundURL(t *testing.T) {
	for _, bad := range []string{"", "ftp://x", "not-a-url", "http://example.com"} {
		if err := validateOutboundURL(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
	for _, ok := range []string{"https://example.com/hook", "http://127.0.0.1:9000/h"} {
		if err := validateOutboundURL(ok); err != nil {
			t.Errorf("unexpected error for %q: %v", ok, err)
		}
	}
}

func TestFormatSignature(t *testing.T) {
	sig, _ := Sign("s", time.Now().Unix(), []byte("x"))
	v := FormatSignature(sig)
	if !strings.HasPrefix(v, "v1:") {
		t.Fatalf("expected v1: prefix, got %q", v)
	}
	if got := normalizeSigValue(v); got != string(sig) {
		t.Fatalf("normalize returned %q, want %q", got, sig)
	}
}

func TestParseTimestampRejectsBad(t *testing.T) {
	for _, bad := range []string{"", "abc", "-1", "12x", "12.5"} {
		if _, err := parseTimestamp(bad); err == nil {
			t.Errorf("expected error for timestamp %q", bad)
		}
	}
	if ts, err := parseTimestamp("1700000000"); err != nil || ts.Unix() != 1700000000 {
		t.Fatalf("parseTimestamp(1700000000) = %v, %v", ts, err)
	}
}

func BenchmarkSign(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Sign("secret", 1, []byte("payload"))
	}
}
