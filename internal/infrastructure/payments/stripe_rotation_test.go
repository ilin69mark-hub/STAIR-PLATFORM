package payments

// S-146 (S-141 №9, CWE-754): парсинг подписей Stripe при ротации ключей.
// В окне ротации Stripe присылает НЕСКОЛЬКО v1= в одном заголовке:
// t=<ts>,v1=<подпись-старым>,v1=<подпись-новым>. Раньше len(parts) != 2
// отвергал такой заголовок целиком → ВСЕ вебхуки падали в окно ротации.
// Теперь принимается, если ЛЮБАЯ v1 совпадает с настроенным секретом.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

// signWithSigns считает HMAC-SHA256 от "<ts>.<payload>" каждым секретом —
// подписи реальным кодом, как это делает Stripe.
func signWithSigns(ts int64, payload []byte, secrets ...string) []string {
	sigs := make([]string, 0, len(secrets))
	for _, sec := range secrets {
		mac := hmac.New(sha256.New, []byte(sec))
		_, _ = fmt.Fprintf(mac, "%d.%s", ts, payload) // hmac.Hash: Write никогда не ошибается
		sigs = append(sigs, hex.EncodeToString(mac.Sum(nil)))
	}
	return sigs
}

func TestStripeVerifyWebhookSignatureRotation(t *testing.T) {
	payload := []byte(`{"id":"evt_rot","type":"checkout.session.completed","data":{"object":{"id":"cs_1","status":"complete","payment_status":"paid","amount_total":1000,"currency":"usd"}},"created":1234567890}`)
	now := time.Now().Unix()

	// Настроен НОВЫЙ секрет; заголовок несёт подписи старым и новым —
	// verified (совпала любая v1).
	newSecret, oldSecret := "whsec_test_new", "whsec_test_old" //nolint:gosec // тестовые секреты-фикстуры, не реальные
	provider := NewStripeProvider("sk_test_key", newSecret)
	sigs := signWithSigns(now, payload, oldSecret, newSecret)
	header := fmt.Sprintf("t=%d,v1=%s,v1=%s", now, sigs[0], sigs[1])
	if err := provider.VerifyWebhookSignature(payload, header); err != nil {
		t.Fatalf("rotation multi-v1 (new configured) must verify: %v", err)
	}

	// Порядок v1 не важен: новый первым.
	header = fmt.Sprintf("t=%d,v1=%s,v1=%s", now, sigs[1], sigs[0])
	if err := provider.VerifyWebhookSignature(payload, header); err != nil {
		t.Fatalf("rotation multi-v1 (reversed order) must verify: %v", err)
	}

	// Настроен СТАРЫЙ секрет, заголовок с обоими — verified (старая v1 совпала).
	providerOld := NewStripeProvider("sk_test_key", oldSecret)
	if err := providerOld.VerifyWebhookSignature(payload, header); err != nil {
		t.Fatalf("rotation multi-v1 (old configured) must verify: %v", err)
	}
}

func TestStripeVerifyWebhookSignatureRotationNoMatch(t *testing.T) {
	payload := []byte(`{"id":"evt_rot","type":"checkout.session.completed"}`)
	now := time.Now().Unix()
	provider := NewStripeProvider("sk_test_key", "whsec_test_configured")

	sigs := signWithSigns(now, payload, "whsec_foreign_a", "whsec_foreign_b")
	header := fmt.Sprintf("t=%d,v1=%s,v1=%s", now, sigs[0], sigs[1])
	if err := provider.VerifyWebhookSignature(payload, header); err == nil {
		t.Fatal("multi-v1 with all-foreign signatures must be rejected")
	}
}

func TestStripeVerifyWebhookSignatureRotationMalformed(t *testing.T) {
	payload := []byte(`{"id":"evt_mal"}`)
	provider := NewStripeProvider("sk_test_key", "whsec_test_secret")
	now := time.Now().Unix()
	sig := signWithSigns(now, payload, "whsec_test_secret")[0]

	for _, tc := range []struct {
		name string
		hdr  string
	}{
		{"garbage", "garbage"},
		{"no t", fmt.Sprintf("v1=%s", sig)},
		{"no v1", fmt.Sprintf("t=%d", now)},
		{"empty v1", fmt.Sprintf("t=%d,v1=", now)},
		{"commas only", ",,,"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := provider.VerifyWebhookSignature(payload, tc.hdr); err == nil {
				t.Fatalf("malformed header %q must be rejected", tc.hdr)
			}
		})
	}
}

// TestStripeVerifyWebhookSignatureSingleV1 — регресс: одиночный v1 по-старому
// по-прежнему верифицируется (обычный режим без ротации).
func TestStripeVerifyWebhookSignatureSingleV1(t *testing.T) {
	payload := []byte(`{"id":"evt_single","type":"checkout.session.completed"}`)
	now := time.Now().Unix()
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	provider := NewStripeProvider("sk_test_key", secret)

	header := fmt.Sprintf("t=%d,v1=%s", now, signWithSigns(now, payload, secret)[0])
	if err := provider.VerifyWebhookSignature(payload, header); err != nil {
		t.Fatalf("single v1 must verify as before: %v", err)
	}
}
