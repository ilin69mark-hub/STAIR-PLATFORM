package sentry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"

	sentrysdk "github.com/getsentry/sentry-go"

	"stairplatform/internal/infrastructure/redaction"
)

// Scrubbing-политика (S-127, решение человека: «скраб всё + хеш email»):
//
//   - весь payload события (message, extra, contexts, tags, request data,
//     exception-строки) прогоняется через redaction.Sensitive — маскируются
//     значения password/secret/token/authorization/cookie/api_key/
//     client_secret;
//   - email-адреса (в user, extra, contexts, tags — где бы ни встретились) —
//     только SHA-256 хеш (первые 12 hex): достаточно для группировки по
//     пользователю, но не раскрывает адрес;
//   - IP-адреса (user.ip_address, REMOTE_ADDR, заголовки X-Forwarded-For и
//     аналоги) — дроп;
//   - username/name — дроп (потенциально PII; id-пользователя достаточно);
//   - заголовки Authorization/Cookie/API-ключи — дроп целиком.
//
// Хук вызывается перед отправкой события (client.processEvent): после
// prepareEvent (scope применяется) и до сериализации в transport.go, поэтому
// round-trip через JSON влияет на то, что реально уходит наружу; теряются
// только служебные поля (sdkMetaData), которые в финальный payload и так
// не попадают.

// scrubEvent — BeforeSend-хук: скрабит событие перед отправкой в Sentry.
// Возвращает nil → событие дропнуто (мы не дропаем, только чистим).
func scrubEvent(event *sentrysdk.Event, _ *sentrysdk.EventHint) *sentrysdk.Event {
	if event == nil {
		return nil
	}
	// Маскировка значений чувствительных ключей целиком (ключ+значение в одной
	// строке: JSON-поля, form/query-пары, вложенный JSON внутри строк).
	event = scrubStringsJSON(event)
	// Хеширование email'ов, где бы они ни встретились (extra/contexts/tags/
	// user и т.д.) — «скраб всё + хеш email» (решение S-127).
	event = scrubEmailsJSON(event)

	// User: IP/username/name → дроп (email уже захеширован выше); id остаётся
	// (UUID, не PII).
	u := event.User
	u.IPAddress = ""
	u.Username = ""
	u.Name = ""
	event.User = u

	// Request: page query/тело/заголовки/env — маскируем и дропаем IP.
	if event.Request != nil {
		event.Request = scrubRequest(event.Request)
	}

	return event
}

// scrubStringsJSON — JSON round-trip всего события через redaction.Sensitive:
// маскирует значения чувствительных ключей в любых строковых полях
// (message, extra, contexts, tags, exception, breadcrumbs и т.д.). При
// ошибке unmarshal'а (не должно случаться) — используем исходное событие:
// PII-поля всё равно добраны ниже точечно.
func scrubStringsJSON(event *sentrysdk.Event) *sentrysdk.Event {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Warn("sentry: scrub marshal failed, keep original event", "error", err)
		return event
	}
	scrubbed := redaction.Sensitive(string(data))
	var ev sentrysdk.Event
	if err := json.Unmarshal([]byte(scrubbed), &ev); err != nil {
		slog.Warn("sentry: scrub unmarshal failed, keep original event", "error", err)
		return event
	}
	return &ev
}

// scrubEmailsJSON — обход JSON-дерева события: строковые значения под
// email-ключами (email, user_email, receiver_email, "email" внутри contexts и
// т.п.) заменяются на SHA-256-хеш (первые 12 hex). Покрывает user.email,
// extra, contexts, tags — любые структурированные поля. Хеш детерминирован:
// одинаковый адрес группируется одинаково, сам адрес не раскрывается.
func scrubEmailsJSON(event *sentrysdk.Event) *sentrysdk.Event {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Warn("sentry: emails scrub marshal failed, keep original event", "error", err)
		return event
	}
	var tree any
	if err := json.Unmarshal(data, &tree); err != nil {
		slog.Warn("sentry: emails scrub unmarshal failed, keep original event", "error", err)
		return event
	}
	scrubEmails(tree)
	scrubbed, err := json.Marshal(tree)
	if err != nil {
		slog.Warn("sentry: emails scrub re-marshal failed, keep original event", "error", err)
		return event
	}
	var ev sentrysdk.Event
	if err := json.Unmarshal(scrubbed, &ev); err != nil {
		slog.Warn("sentry: emails scrub unmarshal failed, keep original event", "error", err)
		return event
	}
	return &ev
}

// scrubEmails рекурсивно проходит JSON-дерево и хеширует строки под
// email-ключами. Байтовые/числовые значения (например, bool-флаг
// "send_email") не трогаются.
func scrubEmails(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if s, ok := val.(string); ok {
				if isEmailKey(k) {
					t[k] = hashEmail(s)
				}
				continue
			}
			scrubEmails(val)
		}
	case []any:
		for _, item := range t {
			scrubEmails(item)
		}
	}
}

// isEmailKey — ключ, под которым ожидается email-адрес (регистронезависимо).
func isEmailKey(k string) bool {
	return strings.Contains(strings.ToLower(k), "email")
}

// чувствительные заголовки запроса: значения дропаются целиком.
// (дроп, а не маска — в заголовки может попасть что угодно, включая
// нестандартные схемы токенов, которые регулярка не поймает).
var droppedHeaders = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"cookie":              {},
	"x-api-key":           {},
	"x-forwarded-for":     {},
	"x-real-ip":           {},
	"cf-connecting-ip":    {},
	"true-client-ip":      {},
	"x-client-ip":         {},
	"x-cluster-client-ip": {},
	"forwarded":           {},
}

// scrubRequest чистит Request-часть события: дроп чувствительных и
// IP-заголовков (Authorization/Cookie/X-Forwarded-For и т.п.), маска
// redaction.Sensitive для query-строки, cookies и прочих заголовков,
// дроп REMOTE_ADDR/REMOTE_PORT из env.
func scrubRequest(req *sentrysdk.Request) *sentrysdk.Request {
	for k, v := range req.Headers {
		if _, drop := droppedHeaders[strings.ToLower(k)]; drop {
			delete(req.Headers, k)
			continue
		}
		// на всякий случай — нестандартные ключи с секретами
		req.Headers[k] = redaction.Sensitive(v)
	}

	req.Data = redaction.Sensitive(req.Data)
	req.QueryString = redaction.Sensitive(req.QueryString)
	req.Cookies = redaction.Sensitive(req.Cookies)
	req.URL = redaction.Sensitive(req.URL)

	if req.Env != nil {
		delete(req.Env, "REMOTE_ADDR")
		delete(req.Env, "REMOTE_PORT")
	}
	return req
}

// hashEmail возвращает SHA-256 хеш email'а в hex, первые 12 символов —
// достаточно для группировки инцидентов по пользователю, но не раскрывает
// сам адрес. Нормализуем lowercase+trim: один пользователь с разным
// регистром/пробелами хешируется одинаково.
func hashEmail(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])[:12]
}
