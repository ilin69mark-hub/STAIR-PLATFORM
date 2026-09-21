package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// configurableIDP — управляемый httptest OIDC IdP для error-веток.
type configurableIDP struct {
	srv       *httptest.Server
	key       *rsa.PrivateKey
	kid       string
	clientID  string
	discoCode int
	discoBody string
	discoCL   int
	jwksCode  int
	jwksBody  string
	jwksCL    int
	tokenCode int
	tokenBody string
	tokenCL   int
}

func newConfigurableIDP(t *testing.T) *configurableIDP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	h := &configurableIDP{
		key: key, kid: "cfg-kid", clientID: "client-1",
		discoCode: 200, jwksCode: 200, tokenCode: 200,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			if h.discoCL > 0 {
				w.Header().Set("Content-Length", strconv.Itoa(h.discoCL))
			}
			w.WriteHeader(h.discoCode)
			body := h.discoBody
			if body == "" {
				body = h.discoJSON("", "")
			}
			_, _ = w.Write([]byte(body))
		case strings.HasSuffix(r.URL.Path, "/jwks"):
			if h.jwksCL > 0 {
				w.Header().Set("Content-Length", strconv.Itoa(h.jwksCL))
			}
			w.WriteHeader(h.jwksCode)
			body := h.jwksBody
			if body == "" {
				pub := h.key.Public().(*rsa.PublicKey)
				body = fmt.Sprintf(`{"keys":[{"kid":%q,"kty":"RSA","n":%q,"e":%q}]}`,
					h.kid, base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
					base64.RawURLEncoding.EncodeToString(bigEndianBytes(pub.E)))
			}
			_, _ = w.Write([]byte(body))
		case strings.HasSuffix(r.URL.Path, "/token"):
			if h.tokenCL > 0 {
				w.Header().Set("Content-Length", strconv.Itoa(h.tokenCL))
			}
			w.WriteHeader(h.tokenCode)
			_, _ = w.Write([]byte(h.tokenBody))
		}
	}))
	h.srv = srv
	return h
}

func (h *configurableIDP) Close() { h.srv.Close() }

func (h *configurableIDP) discoJSON(tokenEndpoint, jwksURI string) string {
	te, ju := h.srv.URL+"/token", h.srv.URL+"/jwks"
	if tokenEndpoint != "" {
		te = tokenEndpoint
	}
	if jwksURI != "" {
		ju = jwksURI
	}
	return fmt.Sprintf(`{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"jwks_uri":%q}`,
		h.srv.URL, h.srv.URL+"/authorize", te, ju)
}

func (h *configurableIDP) client() *Client {
	return New(Config{
		Issuer:       h.srv.URL,
		ClientID:     h.clientID,
		ClientSecret: "secret",
		RedirectURL:  "https://app/cb",
		ProviderName: "cfgidp",
	})
}

func (h *configurableIDP) validPayload() map[string]any {
	return map[string]any{
		"iss": h.srv.URL, "aud": h.clientID, "sub": "sub-1",
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
		"nonce": "n-1", "email": "a@b.c", "name": "N",
	}
}

func (h *configurableIDP) issueIDToken(t *testing.T, payload map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": "RS256", "kid": h.kid, "typ": "JWT"}
	return signJWT(t, h.key, header, payload)
}

func closedServerURL() string {
	ts := httptest.NewServer(http.NotFoundHandler())
	u := ts.URL
	ts.Close()
	return u
}

func TestNameDefaultAndSet(t *testing.T) {
	if got := New(Config{}).Name(); got != "sso" {
		t.Fatalf("default Name = %q", got)
	}
	if got := New(Config{ProviderName: "gitlab"}).Name(); got != "gitlab" {
		t.Fatalf("provider Name = %q", got)
	}
}

func TestRandHexSuccess(t *testing.T) {
	s, err := RandHex(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 32 {
		t.Fatalf("expected 32 hex chars, got %d", len(s))
	}
}

func TestDiscoveryRequestBuildError(t *testing.T) {
	c := New(Config{Issuer: "http://ex%zzam.com", ClientID: "c"})
	if _, err := c.AuthCodeURL("s", "n", "ch", "S256"); err == nil {
		t.Fatal("expected discovery request error")
	}
}

func TestDiscoveryFetchError(t *testing.T) {
	c := New(Config{Issuer: closedServerURL(), ClientID: "c"})
	if _, err := c.AuthCodeURL("s", "n", "ch", "S256"); err == nil {
		t.Fatal("expected discovery fetch error")
	}
}

func TestDiscoveryNonOKStatus(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.discoCode = http.StatusInternalServerError
	if _, err := idp.client().AuthCodeURL("s", "n", "ch", "S256"); err == nil {
		t.Fatal("expected non-200 discovery error")
	}
}

func TestDiscoveryReadError(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.discoCL = 100
	if _, err := idp.client().Exchange(context.Background(), "c", "v"); err == nil {
		t.Fatal("expected discovery read error")
	}
}

func TestDiscoveryMalformedJSON(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.discoBody = "notjson"
	if _, err := idp.client().AuthCodeURL("s", "n", "ch", "S256"); err == nil {
		t.Fatal("expected discovery decode error")
	}
}

func TestDiscoveryMissingEndpoints(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.discoBody = `{"issuer":"x"}`
	if _, err := idp.client().AuthCodeURL("s", "n", "ch", "S256"); err == nil {
		t.Fatal("expected missing endpoints error")
	}
}

func TestExchangeDiscoveryError(t *testing.T) {
	c := New(Config{Issuer: closedServerURL(), ClientID: "c"})
	if _, err := c.Exchange(context.Background(), "code", "v"); err == nil {
		t.Fatal("expected exchange discovery error")
	}
}

func TestExchangeContextCancelled(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Exchange(ctx, "code", "v"); err == nil {
		t.Fatal("expected exchange error for cancelled context")
	}
}

func TestExchangeTokenRequestError(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.discoBody = idp.discoJSON("http://ex%zzam.com", "")
	if _, err := idp.client().Exchange(context.Background(), "code", "v"); err == nil {
		t.Fatal("expected token request error")
	}
}

func TestExchangeTokenFetchError(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	dead := closedServerURL()
	idp.discoBody = idp.discoJSON(dead, "")
	if _, err := idp.client().Exchange(context.Background(), "code", "v"); err == nil {
		t.Fatal("expected token exchange error")
	}
}

func TestExchangeTokenReadError(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.tokenCL = 100
	idp.tokenBody = `{"id_token":"x"}`
	if _, err := idp.client().Exchange(context.Background(), "code", "v"); err == nil {
		t.Fatal("expected token read error")
	}
}

func TestExchangeTokenNonOKStatus(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.tokenCode = http.StatusBadGateway
	if _, err := idp.client().Exchange(context.Background(), "code", "v"); err == nil {
		t.Fatal("expected token status error")
	}
}

func TestExchangeTokenMalformedJSON(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.tokenBody = "notjson"
	if _, err := idp.client().Exchange(context.Background(), "code", "v"); err == nil {
		t.Fatal("expected token decode error")
	}
}

func TestExchangeNoIDToken(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.tokenBody = `{"access_token":"x"}`
	if _, err := idp.client().Exchange(context.Background(), "code", "v"); err == nil {
		t.Fatal("expected no id_token error")
	}
}

func TestVerifyIDTokenMalformed(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	if _, err := c.VerifyIDToken(context.Background(), "a.b", "n"); err == nil {
		t.Fatal("expected malformed token error")
	}
}

func TestVerifyIDTokenBase64DecodeErrors(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	for _, raw := range []string{"!!!.e30.e30", "e30.!!!.e30", "e30.e30.!!!"} {
		if _, err := c.VerifyIDToken(context.Background(), raw, "n"); err == nil {
			t.Fatalf("expected base64 error for %q", raw)
		}
	}
}

func TestVerifyIDTokenHeaderDecodeError(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	raw := base64.RawURLEncoding.EncodeToString([]byte("notjson")) + ".e30.e30"
	if _, err := c.VerifyIDToken(context.Background(), raw, "n"); err == nil {
		t.Fatal("expected header decode error")
	}
}

func TestVerifyIDTokenUnsupportedAlg(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	payload := map[string]any{"iss": h.issuer, "aud": h.clientID, "sub": "s",
		"exp": time.Now().Add(5 * time.Minute).Unix(), "nonce": "n"}
	raw := signJWT(t, h.key, map[string]any{"alg": "HS256"}, payload)
	if _, err := c.VerifyIDToken(context.Background(), raw, "n"); err == nil {
		t.Fatal("expected unsupported alg error")
	}
}

func TestVerifyIDTokenPayloadDecodeError(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte("notjson"))
	raw := header + "." + payload + ".e30"
	if _, err := c.VerifyIDToken(context.Background(), raw, "n"); err == nil {
		t.Fatal("expected payload decode error")
	}
}

func TestVerifyIDTokenDiscoveryFails(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	raw := h.idToken(t, "a@b.c")
	c := New(Config{Issuer: closedServerURL(), ClientID: h.clientID})
	if _, err := c.VerifyIDToken(context.Background(), raw, "nonce-1"); err == nil {
		t.Fatal("expected discovery failure")
	}
}

func TestVerifyIDTokenIssMismatch(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	payload := map[string]any{"iss": "https://other.example", "aud": h.clientID,
		"sub": "s", "exp": time.Now().Add(5 * time.Minute).Unix(), "nonce": "n"}
	raw := signJWT(t, h.key, map[string]any{"alg": "RS256", "kid": h.kid}, payload)
	if _, err := c.VerifyIDToken(context.Background(), raw, "n"); err == nil {
		t.Fatal("expected iss mismatch")
	}
}

func TestVerifyIDTokenExpired(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	payload := map[string]any{"iss": h.issuer, "aud": h.clientID, "sub": "s",
		"exp": time.Now().Add(-time.Minute).Unix(), "nonce": "n"}
	raw := signJWT(t, h.key, map[string]any{"alg": "RS256", "kid": h.kid}, payload)
	if _, err := c.VerifyIDToken(context.Background(), raw, "n"); err == nil {
		t.Fatal("expected expired error")
	}
}

func TestVerifyIDTokenCachesKeys(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	raw := h.idToken(t, "a@b.c")
	for i := 0; i < 2; i++ {
		if _, err := c.VerifyIDToken(context.Background(), raw, "nonce-1"); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}

func TestVerifyIDTokenSingleKeyNoKid(t *testing.T) {
	h := startIdp(t)
	defer h.Close()
	c := newClient(t, h, h.clientID)
	payload := map[string]any{"iss": h.issuer, "aud": h.clientID, "sub": "s",
		"exp": time.Now().Add(5 * time.Minute).Unix(), "nonce": "n"}
	raw := signJWT(t, h.key, map[string]any{"alg": "RS256", "typ": "JWT"}, payload)
	if _, err := c.VerifyIDToken(context.Background(), raw, "n"); err != nil {
		t.Fatalf("VerifyIDToken: %v", err)
	}
}

func TestVerifyIDTokenJWKSRequestError(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.discoBody = idp.discoJSON("", "http://ex%zzam.com")
	c := idp.client()
	raw := idp.issueIDToken(t, idp.validPayload())
	if _, err := c.VerifyIDToken(context.Background(), raw, "n-1"); err == nil {
		t.Fatal("expected jwks request error")
	}
}

func TestVerifyIDTokenJWKSFetchError(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.discoBody = idp.discoJSON("", closedServerURL())
	c := idp.client()
	raw := idp.issueIDToken(t, idp.validPayload())
	if _, err := c.VerifyIDToken(context.Background(), raw, "n-1"); err == nil {
		t.Fatal("expected jwks fetch error")
	}
}

func TestVerifyIDTokenJWKSNonOKStatus(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.jwksCode = http.StatusForbidden
	c := idp.client()
	raw := idp.issueIDToken(t, idp.validPayload())
	if _, err := c.VerifyIDToken(context.Background(), raw, "n-1"); err == nil {
		t.Fatal("expected jwks status error")
	}
}

func TestVerifyIDTokenJWKSReadError(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.jwksCL = 100
	c := idp.client()
	raw := idp.issueIDToken(t, idp.validPayload())
	if _, err := c.VerifyIDToken(context.Background(), raw, "n-1"); err == nil {
		t.Fatal("expected jwks read error")
	}
}

func TestVerifyIDTokenJWKSMalformedJSON(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.jwksBody = "notjson"
	c := idp.client()
	raw := idp.issueIDToken(t, idp.validPayload())
	if _, err := c.VerifyIDToken(context.Background(), raw, "n-1"); err == nil {
		t.Fatal("expected jwks decode error")
	}
}

func TestVerifyIDTokenNoRSAKey(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.jwksBody = `{"keys":[{"kid":"cfg-kid","kty":"EC","n":"x","e":"AQAB"}]}`
	c := idp.client()
	raw := idp.issueIDToken(t, idp.validPayload())
	if _, err := c.VerifyIDToken(context.Background(), raw, "n-1"); err == nil {
		t.Fatal("expected no matching RSA key error")
	}
}

func TestVerifyIDTokenJWKNDecodeError(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.jwksBody = `{"keys":[{"kid":"cfg-kid","kty":"RSA","n":"!","e":"AQAB"}]}`
	c := idp.client()
	raw := idp.issueIDToken(t, idp.validPayload())
	if _, err := c.VerifyIDToken(context.Background(), raw, "n-1"); err == nil {
		t.Fatal("expected jwk n decode error")
	}
}

func TestVerifyIDTokenJWKEDecodeError(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.jwksBody = `{"keys":[{"kid":"cfg-kid","kty":"RSA","n":"AQAB","e":"!"}]}`
	c := idp.client()
	raw := idp.issueIDToken(t, idp.validPayload())
	if _, err := c.VerifyIDToken(context.Background(), raw, "n-1"); err == nil {
		t.Fatal("expected jwk e decode error")
	}
}

func TestVerifyIDTokenJWKInvalidExponent(t *testing.T) {
	idp := newConfigurableIDP(t)
	defer idp.Close()
	idp.jwksBody = `{"keys":[{"kid":"cfg-kid","kty":"RSA","n":"AQAB","e":"AA"}]}`
	c := idp.client()
	raw := idp.issueIDToken(t, idp.validPayload())
	if _, err := c.VerifyIDToken(context.Background(), raw, "n-1"); err == nil {
		t.Fatal("expected invalid jwk e error")
	}
}
