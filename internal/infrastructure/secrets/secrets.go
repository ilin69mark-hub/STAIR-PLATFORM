// Package secrets encrypts integration secrets (webhook HMAC keys) at rest
// (S1-2). Secrets for outbound webhooks live in Postgres (column secret_enc);
// they must not be stored in plaintext. A Box uses AES-256-GCM with a master
// key from STAIR_SECRETS_KEY. Sealed values are tagged "enc:v1:..."; values
// without the tag are legacy plaintext rows and are returned as-is, so the
// switch does not require a destructive migration.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// TagV1 — префикс запечатанного значения нового формата.
const TagV1 = "enc:v1:"

// DesealedSize — размер ключа AES-256.
const keySize = 32

// Box запечатывает/распечатывает секреты (AES-256-GCM).
type Box struct {
	aead cipher.AEAD
}

// NewBox создаёт Box с мастер-ключом строго keySize байт (S1-2).
func NewBox(key []byte) (*Box, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("secrets: key must be %d bytes, got %d", keySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secrets: aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: gcm: %w", err)
	}
	return &Box{aead: aead}, nil
}

// KeyFromHex парсит 64-символьный hex-ключ STAIR_SECRETS_KEY.
func KeyFromHex(hexKey string) ([]byte, error) {
	key, err := hex.DecodeString(strings.TrimSpace(hexKey))
	if err != nil {
		return nil, fmt.Errorf("secrets: invalid hex key: %w", err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("secrets: key must be %d bytes (%d hex chars), got %d", keySize, keySize*2, len(key))
	}
	return key, nil
}

// Seal шифрует секрет: случайный nonce + ciphertext, формат enc:v1:<b64>.
func (b *Box) Seal(plain string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secrets: nonce: %w", err)
	}
	ct := b.aead.Seal(nil, nonce, []byte(plain), nil)
	buf := make([]byte, 0, len(nonce)+len(ct))
	buf = append(buf, nonce...)
	buf = append(buf, ct...)
	return TagV1 + base64.StdEncoding.EncodeToString(buf), nil
}

// Open расшифровывает значение. Строки без тега enc:v1: считаются legacy
// plaintext-строками и возвращаются как есть (не-миграционный переход).
func (b *Box) Open(sealed string) (string, error) {
	if !strings.HasPrefix(sealed, TagV1) {
		return sealed, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, TagV1))
	if err != nil {
		return "", fmt.Errorf("secrets: decode: %w", err)
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("secrets: sealed value too short")
	}
	plain, err := b.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("secrets: decrypt: %w", err)
	}
	return string(plain), nil
}
