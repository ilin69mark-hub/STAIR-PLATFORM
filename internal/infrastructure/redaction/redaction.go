// Package redaction — маскирование чувствительных данных в строках.
//
// Изначально redactSensitive жил в internal/transport/http (S-111,
// DEBUG-LOGGING-PROD-BODIES) и маскировал тела запросов/ответов в
// debug-логах. С S-140 тот же список переиспользуется Sentry-скраббером
// (internal/infrastructure/sentry), поэтому логика вынесена в общий пакет.
package redaction

import "regexp"

// чувствительные ключи: password/passwd/secret/token/authorization/cookie/
// api_key/client_secret — в JSON-полях и form/query-парах. Компилируются
// один раз на процесс.
var (
	jsonFieldRe = regexp.MustCompile(`(?i)("(?:password|passwd|secret|token|authorization|cookie|api[-_]?key|client[-_]?secret)"\s*:\s*")([^"]*)(")`)
	formFieldRe = regexp.MustCompile(`(?i)((?:password|passwd|secret|token|authorization|cookie|api[-_]?key|client[-_]?secret)=)([^&\s"]*)`)
)

// Sensitive маскирует значения чувствительных полей в строке (JSON-тела и
// form/query-encoded). Список ключей — из S-111 (redactSensitive):
// password, passwd, secret, token, authorization, cookie, api_key,
// client_secret. Нечувствительные данные (email, username и т.п.) не трогает.
func Sensitive(s string) string {
	s = jsonFieldRe.ReplaceAllString(s, `${1}***${3}`)
	s = formFieldRe.ReplaceAllString(s, `${1}***`)
	return s
}
