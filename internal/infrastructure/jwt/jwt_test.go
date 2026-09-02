package jwt

import (
	"testing"
	"time"
)

func TestGenerateAndValidateTokenPair(t *testing.T) {
	svc := NewService(Config{
		Secret:          "test-secret-key",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "test",
	})

	pair, err := svc.GenerateTokenPair("user-123", "tenant-456", "test@example.com", "admin")
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("expected access token")
	}
	if pair.RefreshToken == "" {
		t.Error("expected refresh token")
	}
	if pair.ExpiresAt == 0 {
		t.Error("expected expires_at")
	}

	// Validate access token
	claims, err := svc.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	if claims.UserID != "user-123" {
		t.Errorf("expected user_id 'user-123', got %q", claims.UserID)
	}
	if claims.TenantID != "tenant-456" {
		t.Errorf("expected tenant_id 'tenant-456', got %q", claims.TenantID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", claims.Email)
	}
	if claims.Role != "admin" {
		t.Errorf("expected role 'admin', got %q", claims.Role)
	}
}

func TestValidateInvalidToken(t *testing.T) {
	svc := NewService(Config{
		Secret:          "test-secret-key",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "test",
	})

	_, err := svc.ValidateAccessToken("invalid-token")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidateExpiredToken(t *testing.T) {
	svc := NewService(Config{
		Secret:          "test-secret-key",
		AccessTokenTTL:  -1 * time.Minute, // Already expired
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "test",
	})

	pair, err := svc.GenerateTokenPair("user-123", "tenant-456", "test@example.com", "user")
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	_, err = svc.ValidateAccessToken(pair.AccessToken)
	if err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestValidateWrongSecret(t *testing.T) {
	svc1 := NewService(Config{
		Secret:          "secret-1",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "test",
	})
	svc2 := NewService(Config{
		Secret:          "secret-2",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "test",
	})

	pair, err := svc1.GenerateTokenPair("user-123", "tenant-456", "test@example.com", "user")
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	_, err = svc2.ValidateAccessToken(pair.AccessToken)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken with wrong secret, got %v", err)
	}
}
