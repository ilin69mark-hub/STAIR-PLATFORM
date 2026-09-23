// Package sentry — интеграция с Sentry (error tracking, S-140).
//
// Принципы (решение человека, S-127):
//   - выключено по умолчанию: без STAIR_SENTRY_DSN SDK не инициализируется,
//     ноль поведения — ни одного запроса наружу, ни одной горутины SDK;
//   - DSN задаётся только через env (STAIR_SENTRY_DSN), в коде/чарте его нет;
//   - трассировка (TracesSampleRate) выключена по умолчанию: шлём только
//     ошибки/паники, честный tracing-контур — OpenTelemetry (tracing пакет);
//   - PII-скраб по умолчанию (BeforeSend): см. scrub.go.
package sentry

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	sentrysdk "github.com/getsentry/sentry-go"
)

// FlushTimeout — сколько ждём отправки буфера событий при shutdown.
const FlushTimeout = 2 * time.Second

// Config — конфигурация Sentry-интеграции.
type Config struct {
	// DSN — Sentry DSN (STAIR_SENTRY_DSN). Пусто → SDK не инициализируется
	// (no-op, наружу ничего не шлётся).
	DSN string
	// Environment — окружение (STAIR_ENVIRONMENT: development/staging/production).
	Environment string
	// ServiceName — имя сервиса для Sentry (тег/дифференциация api vs worker).
	ServiceName string
	// Release — версия релиза (version.Version из ldflags; напр. "v1.2.3").
	Release string
	// TracesSampleRate — доля транзакций (0.0-1.0). 0 = только ошибки,
	// без трассировки (дефолт; tracing-бокс чеклиста остаётся за OTel).
	TracesSampleRate float64
}

// Init инициализирует Sentry SDK. При пустом DSN — безопасный no-op:
// возвращает пустой shutdown и nil-ошибку. Возвращаемый shutdown флашит
// буфер событий (ждём до FlushTimeout) — вызывать при остановке процесса.
func Init(cfg Config) (shutdown func(), err error) {
	if cfg.DSN == "" {
		return func() {}, nil
	}

	if err := sentrysdk.Init(sentrysdk.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      cfg.Environment,
		Release:          cfg.Release,
		ServerName:       cfg.ServiceName,
		TracesSampleRate: cfg.TracesSampleRate,
		BeforeSend:       scrubEvent,
	}); err != nil {
		return nil, fmt.Errorf("sentry: init: %w", err)
	}

	slog.Info("sentry: enabled",
		"service", cfg.ServiceName,
		"environment", cfg.Environment,
		"release", cfg.Release,
		"traces_sample_rate", cfg.TracesSampleRate,
	)

	return func() { _ = sentrysdk.Flush(FlushTimeout) }, nil
}

// CapturePanic отправляет панику в Sentry вместе с контекстом запроса
// (метод, путь, request-данные — через PII-скраб BeforeSend). Если SDK
// выключен (DSN не задан) — безопасный no-op. r может быть nil (паника вне
// HTTP-запроса); stack — уже собранный debug.Stack() (кладу в extra).
func CapturePanic(recovered any, r *http.Request, stack []byte) {
	hub := sentrysdk.CurrentHub()
	if hub == nil || recovered == nil || hub.Client() == nil {
		return
	}

	hub.WithScope(func(scope *sentrysdk.Scope) {
		if r != nil {
			scope.SetRequest(r)
			scope.SetTag("method", r.Method)
			scope.SetTag("path", r.URL.Path)
		}
		if len(stack) > 0 {
			scope.SetContext("panic", sentrysdk.Context{"stack": string(stack)})
		}
		// Recover использует стек паникующей горутины (клиент.Recover →
		// runtime stack); при недоступном клиенте возвращает nil — никто
		// ничего не шлёт.
		_ = hub.Recover(recovered)
	})
}
