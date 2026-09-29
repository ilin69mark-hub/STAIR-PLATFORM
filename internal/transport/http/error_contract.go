package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"stairplatform/internal/application/analytics"
	"stairplatform/internal/application/assistant"
	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/integrations"
	"stairplatform/internal/application/jobs"
	"stairplatform/internal/application/order"
	"stairplatform/internal/application/payments"
	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/application/store"
	"stairplatform/internal/application/testimonial"
)

// ЕДИНЫЙ КОНТРАКТ ОШИБОК API (API-002, 2026-09-26).
//
// До этого ф��кса:
//
//  1. Маппинг доменных ошибок на HTTP жил по двум несвязанным механизмам:
//     (a) 62 разбросанных по хендлерам блока `errors.Is(err, X.ErrNotFound)`
//         с РУЧНЫМИ сообщениями на русском (их легко becomes 400/403/500
//         в зависимости от того, вспомнил ли автор про sentinel);
//     (b) MapDomainError() в errors.go, который вообще НИГДЕ не вызывался в
//         production-коде и определял статус ПО ПОДСТРОКЕ текста ошибки
//         ("not found" → 404, "invalid" → 422, ...). Такой маппинг ложный по
//         построению: ошибка драйвера БД «relation ... does not exist» стала бы
//         422, а «duplicate key value» — 409, и внутренние детали ошибки
//         уезжали бы клиенту в message.
//  2. 45 мест в хендлерах делали `if err != nil → 500 internal`, поэтому
//     доменные ErrNotFound/ErrForbidden/ErrInvalid превращались в 500:
//     клиент получал «внутренняя ошибка сервера» вместо 404/403/422.
//
// Теперь единственная воронка — writeServiceError: она разбирает ошибку по
// ТИПУ и SENTINEL (errors.Is), никогда по тексту, а неизвестное логирует и
// отдаёт 500 с request_id.

// errorMapping — код ответа и машинный код для известной доменной ошибки.
type errorMapping struct {
	status int
	code   string
}

// domainErrors — реестр sentinel-ошибок application-слоя. Ключ — сама ошибка,
// сравнение только через errors.Is, поэтому работает и обёртка `fmt.Errorf
// ("...: %w", ErrNotFound)`.
//
// Реестр намеренно полный: тест TestAPI002_DomainErrorRegistryIsExhaustive
// падает, если в application-слое появится новый Err* sentinel, который сюда
// не добавлен (иначе он молча уедет в 500).
var domainErrors = []struct {
	err error
	m   errorMapping
}{
	// --- project ---
	{project.ErrNotFound, errorMapping{http.StatusNotFound, "not_found"}},
	{project.ErrForbidden, errorMapping{http.StatusForbidden, "forbidden"}},
	{project.ErrConflict, errorMapping{http.StatusConflict, "conflict"}},

	// --- auth ---
	{auth.ErrNotFound, errorMapping{http.StatusNotFound, "not_found"}},
	{auth.ErrForbidden, errorMapping{http.StatusForbidden, "forbidden"}},
	{auth.ErrInvalidCreds, errorMapping{http.StatusUnauthorized, "invalid_credentials"}},
	{auth.ErrSessionExpired, errorMapping{http.StatusUnauthorized, "session_expired"}},
	{auth.ErrKeyRevoked, errorMapping{http.StatusUnauthorized, "key_revoked"}},
	{auth.ErrUserDisabled, errorMapping{http.StatusForbidden, "user_disabled"}},
	{auth.ErrWeakPassword, errorMapping{http.StatusUnprocessableEntity, "weak_password"}},
	{auth.ErrInvalidEmail, errorMapping{http.StatusUnprocessableEntity, "invalid_email"}},
	{auth.ErrEmailExists, errorMapping{http.StatusConflict, "email_exists"}},
	{auth.ErrOAuthExists, errorMapping{http.StatusConflict, "oauth_exists"}},
	{auth.ErrInvalidStatus, errorMapping{http.StatusUnprocessableEntity, "invalid_status"}},
	{auth.ErrInvalidPolicy, errorMapping{http.StatusUnprocessableEntity, "invalid_policy"}},
	{auth.ErrUnknownRole, errorMapping{http.StatusUnprocessableEntity, "unknown_role"}},
	{auth.ErrSsoDenied, errorMapping{http.StatusForbidden, "sso_denied"}},
	{auth.ErrSsoNotConfigured, errorMapping{http.StatusUnprocessableEntity, "sso_not_configured"}},
	{auth.ErrSsoState, errorMapping{http.StatusUnprocessableEntity, "sso_state_invalid"}},

	// --- jobs ---
	{jobs.ErrNotFound, errorMapping{http.StatusNotFound, "not_found"}},
	// DB-3 (2026-09-27): задание уже взято другим воркером. На HTTP-пути это
	// недостижимо (RunCalculate глотает ошибку), но оставлять её вне реестра
	// нельзя: реестр перечисляет ВСЕ доменные ошибки, и любая незарегистри-
	// рованная уехала бы в 500 вместо осмысленного ответа.
	{jobs.ErrAlreadyClaimed, errorMapping{http.StatusConflict, "already_claimed"}},
	{jobs.ErrInvalid, errorMapping{http.StatusUnprocessableEntity, "invalid_input"}},

	// --- order ---
	{order.ErrNotFound, errorMapping{http.StatusNotFound, "not_found"}},
	{order.ErrForbidden, errorMapping{http.StatusForbidden, "forbidden"}},
	{order.ErrInvalid, errorMapping{http.StatusUnprocessableEntity, "invalid_input"}},
	// DB-8 (2026-09-27): попытка вывести заказ из терминального статуса.
	// Отдельная ошибка, а не ErrNotFound: заказ существует, запрещён именно
	// переход, и 409 сообщает об этом прямо, тогда как 404 утверждал бы, что
	// заказа нет.
	{order.ErrTerminalStatus, errorMapping{http.StatusConflict, "terminal_status"}},

	// --- payments ---
	// ErrNotFound/ErrInvalid/ErrForbidden/ErrInvalidSignature найдены сканером
	// application-слоя, но в реестр не попали: без них webhook-ошибки и
	// ошибки платёжного намёрения уезжали в 500.
	{payments.ErrNotFound, errorMapping{http.StatusNotFound, "not_found"}},
	{payments.ErrInvalid, errorMapping{http.StatusConflict, "invalid_input"}},
	{payments.ErrForbidden, errorMapping{http.StatusForbidden, "forbidden"}},
	{payments.ErrInvalidSignature, errorMapping{http.StatusUnauthorized, "invalid_signature"}},
	// CRITICAL-04: терминальный статус не переопределён. В webhook-маршруте
	// этот случай перехватывается ДО writeServiceError и отвечает 200
	// (подтверждаем доставку, не поощряя ретраи PSP); здесь — 409 на
	// остальных путях, где конфликт статуса действительно является ошибкой.
	{payments.ErrStatusConflict, errorMapping{http.StatusConflict, "status_conflict"}},
	{payments.ErrInvalidStatus, errorMapping{http.StatusUnprocessableEntity, "invalid_status"}},
	{payments.ErrRefundUnsupported, errorMapping{http.StatusConflict, "refund_unsupported"}},
	{payments.ErrProviderMismatch, errorMapping{http.StatusBadRequest, "provider_mismatch"}},
	{payments.ErrProviderUnavailable, errorMapping{http.StatusServiceUnavailable, "provider_unavailable"}},

	// --- store / testimonials ---
	{store.ErrNotFound, errorMapping{http.StatusNotFound, "not_found"}},
	{store.ErrInvalid, errorMapping{http.StatusUnprocessableEntity, "invalid_input"}},
	{testimonial.ErrNotFound, errorMapping{http.StatusNotFound, "not_found"}},
	{testimonial.ErrInvalid, errorMapping{http.StatusUnprocessableEntity, "invalid_input"}},

	// --- integrations ---
	{integrations.ErrNotFound, errorMapping{http.StatusNotFound, "not_found"}},
	{integrations.ErrForbidden, errorMapping{http.StatusForbidden, "forbidden"}},
	{integrations.ErrInvalid, errorMapping{http.StatusUnprocessableEntity, "invalid_input"}},
	{integrations.ErrNoEndpoint, errorMapping{http.StatusUnprocessableEntity, "no_endpoint"}},
	{integrations.ErrConflict, errorMapping{http.StatusConflict, "conflict"}},

	// --- assistant ---
	{assistant.ErrForbidden, errorMapping{http.StatusForbidden, "forbidden"}},
	{assistant.ErrInvalid, errorMapping{http.StatusUnprocessableEntity, "invalid_input"}},
	{assistant.ErrNoFeasible, errorMapping{http.StatusUnprocessableEntity, "no_feasible"}},

	// --- analytics ---
	{analytics.ErrInvalidRange, errorMapping{http.StatusUnprocessableEntity, "invalid_range"}},
	{analytics.ErrInvalidGranularity, errorMapping{http.StatusUnprocessableEntity, "invalid_granularity"}},

	// --- audit ---
	{audit.ErrInvalidEvent, errorMapping{http.StatusUnprocessableEntity, "invalid_audit_event"}},

	// --- stair (обрабатывается в mapStairError с расширенным текстом; здесь
	// только чтобы не уезжал в 500) ---
	{stair.ErrSearchSpaceTooLarge, errorMapping{http.StatusUnprocessableEntity, "search_space_too_large"}},
}

// domainErrorMapping возвращает маппинг для известной доменной ошибки.
func domainErrorMapping(err error) (errorMapping, bool) {
	for _, e := range domainErrors {
		if errors.Is(err, e.err) {
			return e.m, true
		}
	}
	return errorMapping{}, false
}

// writeServiceError — единая точка ответа на ошибку хендлера.
//
// msg — человекочитаемое сообщение по-русски для клиента. Для известных
// доменных ошибок берётся msg (он уже корректный и не выдаёт внутренних
// деталей), для неизвестных — msg тоже, но ошибка дополнительно логируется:
// клиент не должен получать текст внутренней ошибки в message.
func writeServiceError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	if err == nil {
		return
	}
	// Уже структурированная ошибка транспорта — уважаем её статус/код.
	var appErr *AppError
	if errors.As(err, &appErr) {
		writeAppErrorWithRequestID(w, r, appErr)
		return
	}
	// Отмена клиентом и таймаут: 499/504 вместо 500, иначе мониторинг считает
	// нормальную отмену аварией.
	if errors.Is(err, context.Canceled) {
		writeError(w, 499, "cancelled", "Операция отменена")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		writeError(w, http.StatusGatewayTimeout, "timeout", "Превышено время ожидания")
		return
	}
	if m, ok := domainErrorMapping(err); ok {
		writeError(w, m.status, m.code, msg)
		return
	}
	// Неизвестная ошибка: 500 + лог. Текст ошибки наружу не отдаётся.
	slog.Error("unhandled service error",
		"error", err.Error(),
		"path", r.URL.Path,
		"method", r.Method,
		"request_id", RequestID(r.Context()))
	writeErrorWithRequestID(w, r, http.StatusInternalServerError, "internal", msg)
}

// MapDomainError маппит доменную ошибку на AppError по ТИПУ и sentinel.
//
// API-002: раньше статус определялся подстрокой текста ошибки
// (`strings.Contains(errMsg, "not found")` и т. п.). Такой маппинг неверный по
// построению — он превращал ошибки драйвера БД («relation does not exist» →
// 422) в клиентские ответы и утекал внутренние детали в Message. Теперь
// используется реестр domainErrors (errors.Is), всё остальное → 500.
func MapDomainError(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	if m, ok := domainErrorMapping(err); ok {
		return &AppError{Status: m.status, Code: m.code, Message: err.Error(), Err: err}
	}
	return &AppError{Status: http.StatusInternalServerError, Code: "internal", Message: err.Error(), Err: err}
}
