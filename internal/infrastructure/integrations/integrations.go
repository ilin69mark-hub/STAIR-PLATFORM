// Package integrations implements the webhook platform (Phase E, EDR-0023):
// outbound delivery (WebhookClient with HMAC-SHA256 signature) and inbound
// verification (WebhookReceiver). Standard library only (DEV-0009). The
// package is infrastructure-layer (ADR-0006): it knows nothing about HTTP
// routes, application services or the queue.
package integrations

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SignatureVersion — версия формата подписи (заголовок X-Stair-Signature).
// Формат: v1:<hex hmac-sha256>.
const SignatureVersion = 1

// HeaderSignature — заголовок подписи (API-0015).
const HeaderSignature = "X-Stair-Signature"

// HeaderTimestamp — заголовок unix-времени подписи (replay-защита).
const HeaderTimestamp = "X-Stair-Timestamp"

// DefaultTimeout — таймаут HTTP-запроса webhook.
const DefaultTimeout = 10 * time.Second

// MaxTimestampAge — допустимое окно для входящих webhook (replay-защита).
const MaxTimestampAge = 5 * time.Minute

// Client — HTTP-клиент для отправки webhook наружу (EDR-0023 §3.1).
type Client struct {
	httpc   *http.Client
	timeout time.Duration
}

// NewClient создаёт webhook-клиент с таймаутом; timeout <= 0 → DefaultTimeout.
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		httpc:   &http.Client{Timeout: timeout},
		timeout: timeout,
	}
}

// Send выполняет POST payload по url с HMAC-подписью и заголовками
// X-Stair-Signature/X-Stair-Timestamp (EDR-0023 §3.1, API-0015). Любой
// статус вне [200,300) — ошибка. URL вне localhost должен быть https.
func (c *Client) Send(ctx context.Context, target, secret string, payload []byte) error {
	if err := validateOutboundURL(target); err != nil {
		return err
	}
	ts := time.Now().Unix()
	sig, err := Sign(secret, ts, payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("integrations: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderSignature, FormatSignature(sig))
	req.Header.Set(HeaderTimestamp, fmt.Sprintf("%d", ts))

	resp, err := c.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("integrations: send webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("integrations: webhook %s returned %d: %s", target, resp.StatusCode, truncate(body))
	}
	return nil
}

// validateOutboundURL — https вне localhost; dev/tests разрешают http://127.0.0.1.
func validateOutboundURL(target string) error {
	u, err := url.Parse(target)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("integrations: invalid webhook url %q", target)
	}
	host := u.Hostname()
	isLocal := host == "127.0.0.1" || host == "localhost" || host == "::1"
	if u.Scheme != "https" && !isLocal {
		return fmt.Errorf("integrations: https required for %q", target)
	}
	return nil
}

// Verify проверяет подпись входящего webhook (EDR-0023 §3.2): HMAC-SHA256
// over (timestamp + "." + body), timestamp-окно ≤ MaxTimestampAge и
// constant-time сравнение.
func Verify(secret string, tsUnix, sigValue string, body []byte, maxAge time.Duration) error {
	if maxAge <= 0 {
		maxAge = MaxTimestampAge
	}
	if tsUnix == "" || sigValue == "" {
		return errors.New("integrations: missing signature headers")
	}
	ts, err := parseTimestamp(tsUnix)
	if err != nil {
		return err
	}
	if now := time.Now(); now.Sub(ts) > maxAge || ts.Sub(now) > maxAge {
		return fmt.Errorf("integrations: timestamp %d out of window", ts.Unix())
	}
	sig, err := Sign(secret, ts.Unix(), body)
	if err != nil {
		return err
	}
	if !hmac.Equal(sig, []byte(normalizeSigValue(sigValue))) {
		return errors.New("integrations: signature mismatch")
	}
	return nil
}

// Sign вычисляет HMAC-SHA256 over (timestamp + "." + body) и возвращает hex.
func Sign(secret string, ts int64, body []byte) ([]byte, error) {
	if secret == "" {
		return nil, errors.New("integrations: empty secret")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := fmt.Fprintf(mac, "%d.", ts); err != nil {
		return nil, err
	}
	if _, err := mac.Write(body); err != nil {
		return nil, err
	}
	return []byte(hex.EncodeToString(mac.Sum(nil))), nil
}

// FormatSignature формирует значение заголовка X-Stair-Signature.
func FormatSignature(hexSig []byte) string {
	return fmt.Sprintf("v%d:%s", SignatureVersion, string(hexSig))
}

func normalizeSigValue(v string) string {
	if i := strings.IndexByte(v, ':'); i >= 0 {
		return v[i+1:]
	}
	return v
}

func parseTimestamp(s string) (time.Time, error) {
	ts, err := strconv.ParseInt(s, 10, 64)
	if err != nil || ts <= 0 {
		return time.Time{}, errors.New("integrations: invalid timestamp")
	}
	return time.Unix(ts, 0), nil
}

func truncate(b []byte) string {
	if len(b) > 200 {
		b = b[:200]
	}
	return strings.TrimSpace(string(b))
}
