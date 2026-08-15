package oidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// signJWT подписывает payload (RS256) закрытым ключом и возвращает raw JWT.
func signJWT(t *testing.T, key *rsa.PrivateKey, header, payload map[string]any) string {
	t.Helper()
	hb, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	hb64 := base64.RawURLEncoding.EncodeToString(hb)
	pb64 := base64.RawURLEncoding.EncodeToString(pb)
	digest := sha256.Sum256([]byte(hb64 + "." + pb64))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return hb64 + "." + pb64 + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// startIdpSpinup поднимает httptest OIDC IdP (discovery + authorize + token + jwks).
type idpHarness struct {
	srv      *httptest.Server
	key      *rsa.PrivateKey
	kid      string
	issuer   string
	clientID string
	rawToken string
}

func startIdp(t *testing.T) *idpHarness {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	kid := "test-key-1"
	h := &idpHarness{key: key, kid: kid, clientID: "client-1"}
	pub := key.Public().(*rsa.PublicKey)
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(bigEndianBytes(pub.E))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 h.issuer,
				"authorization_endpoint": h.issuer + "/authorize",
				"token_endpoint":         h.issuer + "/token",
				"jwks_uri":               h.issuer + "/jwks",
			})
		case strings.HasSuffix(r.URL.Path, "/jwks"):
			json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
				"kid": kid, "kty": "RSA", "n": n, "e": e,
			}}})
		case strings.HasSuffix(r.URL.Path, "/token"):
			w.Header().Set("Content-Type", "application/json")
			id := h.rawToken
			if id == "" {
				id = "missing"
			}
			json.NewEncoder(w).Encode(map[string]string{"id_token": id})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	h.srv = srv
	h.issuer = srv.URL
	h.rawToken = ""
	return h
}

func (h *idpHarness) Close() { h.srv.Close() }

func bigEndianBytes(e int) []byte {
	var b [4]byte
	b[0] = byte(e >> 24)
	b[1] = byte(e >> 16)
	b[2] = byte(e >> 8)
	b[3] = byte(e)
	// Отсекаем ведущие нули.
	start := 0
	for start < 3 && b[start] == 0 {
		start++
	}
	return b[start:]
}

// rawToken — промежуточный захват для token endpoint (поле в harness).
func (h *idpHarness) setRawToken(raw string) { h.rawToken = raw }

// (idpHarness) idToken собирает валидный id_token для claims + nonce.
func (h *idpHarness) idToken(t *testing.T, nonce, sub, email string) string {
	t.Helper()
	payload := map[string]any{
		"iss":   h.issuer,
		"aud":   h.clientID,
		"sub":   sub,
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
		"nonce": nonce,
		"email": email,
		"name":  "OIDC User",
	}
	return signJWT(t, h.key, map[string]any{"alg": "RS256", "kid": h.kid, "typ": "JWT"}, payload)
}

func newClient(t *testing.T, h *idpHarness, clientID string) *Client {
	t.Helper()
	c := New(Config{
		Issuer:       h.issuer,
		ClientID:     clientID,
		ClientSecret: "secret",
		RedirectURL:  "https://app/callback",
		ProviderName: "testidp",
	})
	return c
}

func TestVerifyIDTokenHappyPath(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	raw := h.idToken(t, "nonce-1", "sub-abc", "user@example.com")
	claims, err := c.VerifyIDToken(context.Background(), raw, "nonce-1")
	if err != nil {
		t.Fatalf("VerifyIDToken: %v", err)
	}
	if claims.Subject != "sub-abc" || claims.Email != "user@example.com" {
		t.Fatalf("claims = %+v", claims)
	}
	if claims.Issuer != h.issuer {
		t.Fatalf("issuer = %q", claims.Issuer)
	}
}

func TestVerifyIDTokenRejectsBadSignature(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	// Подпись повреждена.
	raw := h.idToken(t, "nonce-1", "sub-abc", "user@example.com")
	parts := strings.Split(raw, ".")
	raw = parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3})
	if _, err := c.VerifyIDToken(context.Background(), raw, "nonce-1"); err == nil {
		t.Fatal("expected error for bad signature")
	}
}

func TestVerifyIDTokenRejectsNonce(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	raw := h.idToken(t, "nonce-1", "sub-abc", "user@example.com")
	if _, err := c.VerifyIDToken(context.Background(), raw, "wrong-nonce"); err == nil {
		t.Fatal("expected nonce mismatch")
	}
}

func TestVerifyIDTokenRejectsAudience(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	// Клиент с другим client_id.
	c := newClient(t, h, "other-client")
	raw := h.idToken(t, "nonce-1", "sub-abc", "user@example.com")
	if _, err := c.VerifyIDToken(context.Background(), raw, "nonce-1"); err == nil {
		t.Fatal("expected audience mismatch")
	}
}

func TestExchangeGetsIDToken(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	raw := h.idToken(t, "nonce-1", "sub-abc", "sso@example.com")
	h.setRawToken(raw)
	got, err := c.Exchange(context.Background(), "auth-code", "pkce-verifier")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if got != raw {
		t.Fatal("returned raw id_token differs")
	}
}

func TestAuthCodeURLIncludesParams(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	u, err := c.AuthCodeURL("state-1", "nonce-1", "challenge", "S256")
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	if q.Get("state") != "state-1" || q.Get("nonce") != "nonce-1" {
		t.Fatalf("missing state/nonce: %v", q)
	}
	if q.Get("code_challenge") != "challenge" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("missing pkce: %v", q)
	}
	if q.Get("client_id") != h.clientID {
		t.Fatalf("client_id = %q", q.Get("client_id"))
	}
}

func TestDisabledWhenNoIssuer(t *testing.T) {
	c := New(Config{})
	if c.Enabled() {
		t.Fatal("client without issuer must be disabled")
	}
}
