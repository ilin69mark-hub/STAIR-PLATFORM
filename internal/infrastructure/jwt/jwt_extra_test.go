package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("s")
	if cfg.Secret != "s" || cfg.Issuer != "stair-platform" {
		t.Fatalf("default cfg mismatch: %+v", cfg)
	}
	if cfg.AccessTokenTTL != 15*time.Minute || cfg.RefreshTokenTTL != 7*24*time.Hour {
		t.Fatalf("ttl mismatch: %+v", cfg)
	}
}

func TestNewServiceDefaults(t *testing.T) {
	svc := NewService(Config{Secret: "x"})
	if svc.cfg.AccessTokenTTL != 15*time.Minute || svc.cfg.RefreshTokenTTL != 7*24*time.Hour || svc.cfg.Issuer != "stair-platform" {
		t.Fatalf("defaults not applied: %+v", svc.cfg)
	}
}

func TestContextWithClaims(t *testing.T) {
	claims := &Claims{UserID: "u1", TenantID: "t1"}
	ctx := ContextWithClaims(context.Background(), claims)
	got, ok := ClaimsFromContext(ctx)
	if !ok || got.UserID != "u1" {
		t.Fatalf("got %v ok %v", got, ok)
	}
	if _, ok := ClaimsFromContext(context.Background()); ok {
		t.Fatal("expected not ok for empty ctx")
	}
}

func TestValidateWrongAlg(t *testing.T) {
	svc := NewService(Config{Secret: "secret", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour, Issuer: "test"})
	// craft none alg token
	token := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{UserID: "u"})
	str, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := svc.ValidateAccessToken(str); err != ErrInvalidToken {
		t.Fatalf("want ErrInvalidToken for none alg, got %v", err)
	}
}

func TestRefreshTokensFlow(t *testing.T) {
	svc := NewService(Config{Secret: "secret", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour, Issuer: "test"})
	pair, _ := svc.GenerateTokenPair("u", "t", "e@ex.ru", "user")
	// valid refresh -> returns refresh-requires-lookup error (stub)
	if _, err := svc.RefreshTokens(pair.RefreshToken); err == nil || err.Error() == "" {
		t.Fatal("expected refresh error")
	}
	if _, err := svc.RefreshTokens("bad"); err != ErrInvalidToken {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
	// expired refresh
	svc2 := NewService(Config{Secret: "secret", AccessTokenTTL: time.Minute, RefreshTokenTTL: -time.Minute, Issuer: "test"})
	pair2, _ := svc2.GenerateTokenPair("u", "t", "e@ex.ru", "user")
	if _, err := svc2.RefreshTokens(pair2.RefreshToken); err != ErrExpiredToken {
		t.Fatalf("want ErrExpiredToken, got %v", err)
	}
}

func TestRefreshWrongAlg(t *testing.T) {
	svc := NewService(Config{Secret: "secret", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour, Issuer: "test"})
	token := jwt.NewWithClaims(jwt.SigningMethodNone, &jwt.RegisteredClaims{Subject: "u"})
	str, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := svc.RefreshTokens(str); err != ErrInvalidToken {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}
