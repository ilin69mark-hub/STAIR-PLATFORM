// Package oidc реализует изолированный OIDC Authorization Code flow клиент
// (EDR-0017) на стандартной библиотеке: Discovery, token exchange, JWKS
// RS256-верификация id_token. Не требует внешних зависимостей (офлайн-сборка).
package oidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"stairplatform/internal/application/auth"
)

// Client — реализация auth.OIDCProvider: OIDC Authorization Code flow.
type Client struct {
	httpClient   *http.Client
	discoveryURL string
	clientID     string
	clientSecret string
	redirectURL  string
	providerName string

	mu     sync.RWMutex
	disco  *discovery
	keys   []jwkKey
	keysAt time.Time
}

type discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

// Config — параметры OIDC-провайдера (EDR-0017 §3.2).
type Config struct {
	ProviderName string
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// New создаёт клиент. Issuer пустой → Enabled() == false (SSO выключен).
func New(cfg Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		discoveryURL: strings.TrimSuffix(cfg.Issuer, "/") +
			"/.well-known/openid-configuration",
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURL:  cfg.RedirectURL,
		providerName: cfg.ProviderName,
	}
}

// Enabled возвращает true, если issuer сконфигурирован.
func (c *Client) Enabled() bool {
	return c.discoveryURL != "/.well-known/openid-configuration"
}

// Name возвращает имя провайдера для кнопки/аудита.
func (c *Client) Name() string {
	if c.providerName == "" {
		return "sso"
	}
	return c.providerName
}

var _ auth.OIDCProvider = (*Client)(nil)

// discoveryOnce загружает и кэширует OIDC discovery (EDR-0017 §3.2).
func (c *Client) discoveryOnce(ctx context.Context) (*discovery, error) {
	c.mu.RLock()
	if c.disco != nil {
		c.mu.RUnlock()
		return c.disco, nil
	}
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("oidc: discovery request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc: discovery fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc: discovery status %d", resp.StatusCode)
	}
	var d discovery
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oidc: discovery read: %w", err)
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("oidc: discovery decode: %w", err)
	}
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" || d.JWKSURI == "" {
		return nil, errors.New("oidc: discovery missing required endpoints")
	}
	c.mu.Lock()
	if c.disco == nil {
		c.disco = &d
	}
	c.mu.Unlock()
	return c.disco, nil
}

// AuthCodeURL строит authorization URL (state, nonce, PKCE S256).
func (c *Client) AuthCodeURL(state, nonce, codeChallenge, method string) (string, error) {
	d, err := c.discoveryOnce(context.Background())
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("client_id", c.clientID)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("redirect_uri", c.redirectURL)
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", method)
	return d.AuthorizationEndpoint + "?" + q.Encode(), nil
}

// tokenResponse — ответ token endpoint (OIDC).
type tokenResponse struct {
	IDToken string `json:"id_token"`
}

// Exchange обменивает authorization code на id_token (PKCE verifier).
func (c *Client) Exchange(ctx context.Context, code, codeVerifier string) (string, error) {
	d, err := c.discoveryOnce(ctx)
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", c.redirectURL)
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("code_verifier", codeVerifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.TokenEndpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("oidc: token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("oidc: token exchange: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("oidc: token read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oidc: token status %d", resp.StatusCode)
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("oidc: token decode: %w", err)
	}
	if tr.IDToken == "" {
		return "", errors.New("oidc: no id_token in response")
	}
	return tr.IDToken, nil
}

// VerifyIDToken проверяет подпись, iss/aud/exp/nonce и возвращает claims.
func (c *Client) VerifyIDToken(ctx context.Context, rawToken, nonce string) (*auth.IDTokenClaims, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("oidc: malformed id_token")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token header: %w", err)
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token payload: %w", err)
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token signature: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("oidc: id_token header decode: %w", err)
	}
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("oidc: unsupported alg %q", header.Alg)
	}
	var payload struct {
		Iss   string `json:"iss"`
		Aud   string `json:"aud"`
		Sub   string `json:"sub"`
		Exp   int64  `json:"exp"`
		Nonce string `json:"nonce"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("oidc: id_token payload decode: %w", err)
	}
	d, err := c.discoveryOnce(ctx)
	if err != nil {
		return nil, err
	}
	if payload.Iss != d.Issuer {
		return nil, errors.New("oidc: iss mismatch")
	}
	if payload.Aud != c.clientID {
		return nil, errors.New("oidc: aud mismatch")
	}
	if time.Now().Unix() >= payload.Exp {
		return nil, errors.New("oidc: id_token expired")
	}
	if payload.Nonce != nonce {
		return nil, errors.New("oidc: nonce mismatch")
	}
	if err := c.verifySignature(ctx, header.Kid,
		parts[0]+"."+parts[1], sigBytes); err != nil {
		return nil, err
	}
	return &auth.IDTokenClaims{
		Issuer:   payload.Iss,
		Audience: payload.Aud,
		Subject:  payload.Sub,
		Email:    payload.Email,
		Name:     payload.Name,
	}, nil
}

// jwkKey — RSA public key из JWKS (n/e base64url).
type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// fetchKeys загружает и кэширует JWKS (на 1 час).
func (c *Client) fetchKeys(ctx context.Context) ([]jwkKey, error) {
	c.mu.RLock()
	if len(c.keys) > 0 && time.Since(c.keysAt) < time.Hour {
		c.mu.RUnlock()
		return c.keys, nil
	}
	c.mu.RUnlock()

	d, err := c.discoveryOnce(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.JWKSURI, nil)
	if err != nil {
		return nil, fmt.Errorf("oidc: jwks request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc: jwks fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc: jwks status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oidc: jwks read: %w", err)
	}
	var jwks struct {
		Keys []jwkKey `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("oidc: jwks decode: %w", err)
	}
	c.mu.Lock()
	c.keys = jwks.Keys
	c.keysAt = time.Now()
	c.mu.Unlock()
	return jwks.Keys, nil
}

// verifySignature проверяет RS256-подпись по JWKS (kid или единственный ключ).
func (c *Client) verifySignature(ctx context.Context, kid, data string, sig []byte) error {
	keys, err := c.fetchKeys(ctx)
	if err != nil {
		return err
	}
	var target *jwkKey
	for i := range keys {
		if kid != "" && keys[i].Kid == kid {
			target = &keys[i]
			break
		}
	}
	if target == nil && len(keys) == 1 {
		target = &keys[0]
	}
	if target == nil || target.Kty != "RSA" {
		return errors.New("oidc: no matching RSA key in jwks")
	}
	pub, err := jwkToRSA(target)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(data))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		return errors.New("oidc: signature verification failed")
	}
	return nil
}

// jwkToRSA конвертирует JWK RSA (n/e) в *rsa.PublicKey.
func jwkToRSA(k *jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("oidc: jwk n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("oidc: jwk e: %w", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	if e <= 1 {
		return nil, errors.New("oidc: invalid jwk e")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

// RandHex — генератор случайных байт в hex (для тестов).
func RandHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
