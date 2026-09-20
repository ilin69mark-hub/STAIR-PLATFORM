package secrets

import (
	"strings"
	"testing"
)

func mustKey(t *testing.T, hexKey string) []byte {
	t.Helper()
	key, err := KeyFromHex(hexKey)
	if err != nil {
		t.Fatalf("KeyFromHex: %v", err)
	}
	return key
}

func TestSealOpenRoundTrip(t *testing.T) {
	key := mustKey(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	b, err := NewBox(key)
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}

	sealed, err := b.Seal("whsec_super_secret")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if !strings.HasPrefix(sealed, TagV1) {
		t.Fatalf("expected %s prefix, got %q", TagV1, sealed)
	}
	if strings.Contains(sealed, "whsec_super_secret") {
		t.Fatalf("sealed value leaks plaintext: %q", sealed)
	}

	plain, err := b.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if plain != "whsec_super_secret" {
		t.Fatalf("round-trip mismatch: %q", plain)
	}
}

func TestSealNonDeterministic(t *testing.T) {
	key := mustKey(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	b, _ := NewBox(key)
	a, _ := b.Seal("x")
	c, _ := b.Seal("x")
	if a == c {
		t.Fatal("two seals of same plaintext must differ (random nonce)")
	}
}

func TestOpenLegacyPlaintextPassthrough(t *testing.T) {
	key := mustKey(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	b, _ := NewBox(key)
	// Старые строки без тега — plaintext; Open возвращает as-is.
	if got, _ := b.Open("whsec_legacy"); got != "whsec_legacy" {
		t.Fatalf("legacy passthrough mismatch: %q", got)
	}
}

func TestOpenWrongKeyFails(t *testing.T) {
	b1, _ := NewBox(mustKey(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
	b2, _ := NewBox(mustKey(t, "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"))
	sealed, _ := b1.Seal("s")
	if _, err := b2.Open(sealed); err == nil {
		t.Fatal("expected error opening with wrong key")
	}
}

func TestOpenTamperedFails(t *testing.T) {
	key := mustKey(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	b, _ := NewBox(key)
	sealed, _ := b.Seal("s")
	tampered := []byte(sealed)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := b.Open(string(tampered)); err == nil {
		t.Fatal("expected error opening tampered value")
	}
}

func TestKeyFromHexErrors(t *testing.T) {
	if _, err := KeyFromHex("short"); err == nil {
		t.Fatal("expected error for non-hex")
	}
	if _, err := KeyFromHex("0123456789abcdef"); err == nil {
		t.Fatal("expected error for wrong-length key")
	}
}

func TestNewBoxKeyLength(t *testing.T) {
	if _, err := NewBox([]byte("short")); err == nil {
		t.Fatal("expected error for short key")
	}
}
