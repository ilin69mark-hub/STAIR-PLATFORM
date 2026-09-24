// Package redaction — маскирование чувствительных данных в строках.
//
// Изначально redactSensitive жил в internal/transport/http (S-111,
// DEBUG-LOGGING-PROD-BODIES) и маскировал тела запросов/ответов в
// debug-логах. С S-140 тот же список переиспользуется Sentry-скраббером
// (internal/infrastructure/sentry), поэтому логика вынесена в общий пакет.
package redaction

import "regexp"

// чувствительные ключи: password/passwd/secret/token/authorization/cookie/
// api_key/client_secret + session/csrf (S-143, S-141 №2: cookie-токены
// session/session_admin/csrf/csrf_admin, stair_session) — в JSON-полях и
// form/query-парах. Компилируются один раз на процесс.
var (
	jsonFieldRe = regexp.MustCompile(`(?i)("(?:password|passwd|secret|token|authorization|cookie|api[-_]?key|client[-_]?secret|session(?:[-_]?admin)?|csrf(?:[-_]?admin)?|stair[-_]?session)"\s*:\s*")([^"]*)(")`)
	formFieldRe = regexp.MustCompile(`(?i)((?:password|passwd|secret|token|authorization|cookie|api[-_]?key|client[-_]?secret|session(?:[-_]?admin)?|csrf(?:[-_]?admin)?|stair[-_]?session)=)([^&\s"]*)`)
)

// Sensitive маскирует значения чувствительных полей в строке (JSON-тела и
// form/query-encoded). Список ключей — из S-111 (redactSensitive) +
// S-143 (S-141 №2): password, passwd, secret, token, authorization, cookie,
// api_key, client_secret, session(+_admin), csrf(+_admin), stair_session.
// Нечувствительные данные (email, username и т.п.) не трогает; ключи вроде
// session_id/session_name не попадают под session (точное совпадение).
func Sensitive(s string) string {
	s = jsonFieldRe.ReplaceAllString(s, `${1}***${3}`)
	s = formFieldRe.ReplaceAllString(s, `${1}***`)
	return s
}
