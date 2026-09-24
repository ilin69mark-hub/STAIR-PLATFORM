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
	"net"
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
// Защита от SSRF (S1-1): при установке TCP-соединения хост резолвится и
// проверяется по IP-политике (loopback/link-local/private — блок). Пиннинг
// валидированного IP исключает DNS rebinding: запрос идёт только на
// проверенный адрес.
type Client struct {
	httpc   *http.Client
	timeout time.Duration
	policy  Policy
	resolve resolverFunc
}

// Policy — IP-политика исходящих webhook (S1-1).
type Policy struct {
	// AllowLoopback — разрешить loopback (127.0.0.1/localhost/::1): dev/tests.
	AllowLoopback bool
	// AllowHosts — allowlist хостов (case-insensitive), которым разрешено
	// резолвиться в private/loopback/link-local диапазоны (внутренние сервисы).
	AllowHosts []string
}

func (p Policy) allows(host string) bool {
	host = strings.ToLower(host)
	for _, h := range p.AllowHosts {
		if strings.ToLower(h) == host {
			return true
		}
	}
	return false
}

// resolverFunc резолвит имя хоста в IP (инъекция для тестов).
type resolverFunc func(ctx context.Context, host string) ([]net.IP, error)

// NewClient создаёт webhook-клиент с политикой, разрешающей loopback
// (dev/tests, обратная совместимость); timeout <= 0 → DefaultTimeout.
func NewClient(timeout time.Duration) *Client {
	return NewPolicyClient(timeout, Policy{AllowLoopback: true})
}

// NewPolicyClient создаёт webhook-клиент с заданной SSRF-политикой (S1-1).
func NewPolicyClient(timeout time.Duration, policy Policy) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	dialer := &net.Dialer{Timeout: timeout}
	c := &Client{
		timeout: timeout,
		policy:  policy,
		resolve: defaultResolver,
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("integrations: bad addr %q: %w", addr, err)
			}
			ip, err := c.resolveDialIP(ctx, host)
			if err != nil {
				return nil, err
			}
			if ip != nil {
				addr = net.JoinHostPort(ip.String(), port)
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	c.httpc = &http.Client{Transport: transport, Timeout: timeout}
	return c
}

// defaultResolver — системный резолвер (net.DefaultResolver).
func defaultResolver(ctx context.Context, host string) ([]net.IP, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ips = append(ips, a.IP)
	}
	return ips, nil
}

// resolveDialIP возвращает IP, на который физически идёт соединение:
// резолвит хост, пропускает заблокированные адреса и пиннит первый
// разрешённый IP (защита от DNS-rebinding).
//
// S-151 (red-team «SSRF в EKS»): allowlist хостов (STAIR_WEBHOOK_ALLOW_HOSTS)
// открывает loopback/private для внутренних сервисов, но НИКОГДА не
// link-local — 169.254.169.254 (метаданные облака) не имеет легитимного
// бизнес-использования для webhook-доставки.
func (c *Client) resolveDialIP(ctx context.Context, host string) (net.IP, error) {
	allowPrivate := c.policy.AllowLoopback || c.policy.allows(host)
	ips, err := c.resolve(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("integrations: resolve %s: %w", host, err)
	}
	for _, ip := range ips {
		if classifyIP(ip, allowPrivate) {
			continue
		}
		return ip, nil
	}
	return nil, fmt.Errorf("integrations: %s resolves only to blocked addresses (SSRF policy)", host)
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "::ffff:127.0.0.1":
		return true
	}
	return false
}

// classifyIP сообщает, блокируется ли адрес SSRF-политикой (S1-1).
// link-local покрывает метаданные облака 169.254.169.254.
// S-151: link-local блокируется ВСЕГДА — даже для allowlist-хоста
// (STAIR_WEBHOOK_ALLOW_HOSTS открывает loopback/private для внутренних
// сервисов, но не IMDS-эндпоинт облака). allowPrivate включает loopback и
// RFC1918 (allowlist-хостам внутренних сервисов они нужны).
func classifyIP(ip net.IP, allowPrivate bool) bool {
	if ip == nil {
		return false
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsLoopback() {
		return !allowPrivate
	}
	if ip.IsPrivate() {
		return !allowPrivate
	}
	if ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 0 {
			return true
		}
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
		return false
	}
	// IPv6 ULA fc00::/7.
	if b := ip.To16(); b != nil && b[0]&0xfe == 0xfc {
		return true
	}
	return false
}

// Send выполняет POST payload по url с HMAC-подписью и заголовками
// X-Stair-Signature/X-Stair-Timestamp (EDR-0023 §3.1, API-0015). Любой
// статус вне [200,300) — ошибка. URL вне localhost должен быть https.
func (c *Client) Send(ctx context.Context, target, secret string, payload []byte) error {
	if err := validateOutboundURL(target); err != nil {
		return err
	}
	if err := c.validateTarget(ctx, target); err != nil {
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

// validateTarget — SSRF-префляйт (S1-1): проверяет хост до отправки.
// Финальная защита — пиннинг валидированного IP в DialContext. S-151:
// allowlist открывает loopback/private, но не link-local (метаданные облака).
func (c *Client) validateTarget(ctx context.Context, target string) error {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		return fmt.Errorf("integrations: invalid webhook url %q", target)
	}
	host := u.Hostname()
	allowPrivate := c.policy.AllowLoopback || c.policy.allows(host)
	ips, err := c.resolve(ctx, host)
	if err != nil {
		return fmt.Errorf("integrations: resolve %s: %w", host, err)
	}
	for _, ip := range ips {
		if !classifyIP(ip, allowPrivate) {
			return nil
		}
	}
	return fmt.Errorf("integrations: %s blocked by SSRF policy (no allowed address)", host)
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
