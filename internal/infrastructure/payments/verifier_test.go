package payments

import (
	"fmt"
	"testing"
	"time"

	"stairplatform/internal/infrastructure/integrations"
)

func formatTs(ts int64) string { return fmt.Sprintf("%d", ts) }

func TestVerifier(t *testing.T) {
	v := NewVerifier()
	if v == nil {
		t.Fatal("nil")
	}
	secret := "s3cr3t"
	body := []byte(`{"id":"evt1"}`)
	now := time.Now().Unix()
	sigBytes, _ := integrations.Sign(secret, now, body)
	sig := integrations.FormatSignature(sigBytes)
	if err := v.Verify(secret, formatTs(now), sig, body, time.Minute); err != nil {
		t.Fatalf("verify valid: %v", err)
	}
	// expired: sign with old time
	oldTs := time.Now().Add(-2 * time.Minute).Unix()
	oldSigBytes, _ := integrations.Sign(secret, oldTs, body)
	oldSig := integrations.FormatSignature(oldSigBytes)
	if err := v.Verify(secret, formatTs(oldTs), oldSig, body, time.Minute); err == nil {
		t.Fatal("want error for expired")
	}
}
