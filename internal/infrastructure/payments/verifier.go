package payments

import (
	"time"

	"stairplatform/internal/infrastructure/integrations"
)

// Verifier — адаптер WebhookVerifier поверх integrations.Verify (EDR-0023
// §3.2): проверяет HMAC-SHA256 подпись и timestamp-окно входящего webhook.
type Verifier struct{}

// NewVerifier создаёт верификатор подписей webhook.
func NewVerifier() *Verifier { return &Verifier{} }

// Verify проверяет подпись (см. integrations.Verify).
func (v *Verifier) Verify(secret string, tsUnix, sigValue string, body []byte, maxAge time.Duration) error {
	return integrations.Verify(secret, tsUnix, sigValue, body, maxAge)
}
