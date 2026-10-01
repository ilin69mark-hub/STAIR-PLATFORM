// Package audit реализует прикладной слой Audit System (Phase G,
// EDR-0013): персистентный журнал событий безопасности (SEC-0013).
// Application зависит от порта Repository (BE-0005); транспорт и БД —
// вне слоя (ADR-0006). Запись события — синхронная, best-effort:
// сбой журнала не ломает бизнес-операцию.
package audit

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Action — вид события аудита (SEC-0013 Audit Events, каталог EDR-0013 §3.3).
type Action string

const (
	ActionAuthRegister      Action = "auth.register"
	ActionAuthLogin         Action = "auth.login"
	ActionAuthLogout        Action = "auth.logout"
	ActionAuthLoginDenied   Action = "auth.login_denied"
	ActionAuthzDenied       Action = "authz.denied"
	ActionUserRoleChanged   Action = "user.role_changed"
	ActionUserStatusChanged Action = "user.status_changed"
	ActionSettingsUpdated   Action = "settings.updated"
	ActionDataExported      Action = "data.exported"
	ActionApiKeyCreated     Action = "api_key.created"
	ActionApiKeyRevoked     Action = "api_key.revoked"
	ActionSsoLogin          Action = "sso.login"
	ActionSsoLoginDenied    Action = "sso.login_denied"
	ActionSsoLinked         Action = "sso.linked"
	ActionProjectCreated    Action = "project.created"
	ActionProjectModified   Action = "project.modified"
	ActionMemberAdded       Action = "member.added"
	ActionMemberRoleChanged Action = "member.role_changed"
	ActionMemberRemoved     Action = "member.removed"
	ActionConfigApproved    Action = "config.approved"
	ActionConfigRestored    Action = "config.restored"
	ActionReviewRequested   Action = "review.requested"
	ActionReviewSigned      Action = "review.signed"
	ActionReviewChanges     Action = "review.changes"
	// AI-ассистенты (Phase D, EDR-0036..0039; AI-0001 «AI полностью аудируем»).
	ActionAiAssistDesign        Action = "ai.assist.design"
	ActionAiAssistEngineering   Action = "ai.assist.engineering"
	ActionAiAssistManufacturing Action = "ai.assist.manufacturing"
	ActionAiAssistPricing       Action = "ai.assist.pricing"
	// ActionAiAssistMemoryPurged — стирание памяти диалога проекта
	// (assistant.Forget). Раньше код был строковым литералом прямо в
	// assistant/entity.go и потому НЕ был в реестре — с введением
	// allowlist (SEC-002) такое действие стало бы невалидным, и событие
	// об отказе в стирании памяти перестало бы писаться вовсе.
	ActionAiAssistMemoryPurged Action = "ai.assist.memory.purged"

	// Действия с расчётом лестницы (EDR-0023/0033 + клиентские клики):
	// серверная эмиссия при расчёте и клиентские события (применение
	// варианта/совета, изменение поля).
	ActionStairCalculated        Action = "stair.calculated"
	ActionStairConfigChanged     Action = "stair.config_changed"
	ActionStairSuggestionApplied Action = "stair.suggestion_applied"
	ActionStairVariationApplied  Action = "stair.variation_applied"

	// Заказы (EDR-0027): изменение статуса администратором.
	ActionOrderStatusChanged Action = "order.status_changed"
	ActionPaymentRefunded    Action = "payment.refunded"

	// Отзывы: CRUD операции администратора.
	ActionTestimonialCreated Action = "testimonial.created"
	ActionTestimonialUpdated Action = "testimonial.updated"
	ActionTestimonialDeleted Action = "testimonial.deleted"

	// Клиентские события живой валидации (EDR-0013 §4.2, S-P5). Коды
	// приходят из админ-конструктора (frontend/src/components/ProjectDetail.tsx)
	// и были в нём задолго до появления констант здесь — из-за чего
	// allowlist (см. ClientActions) обязан их объявлять, иначе легитимные
	// клики отбрасывались бы как подделка.
	ActionStairLiveSuggestionApplied Action = "stair.live_suggestion_applied"
	ActionStairLiveVariationApplied  Action = "stair.live_variation_applied"
)

// allActions — реестр ВСЕХ известных действий аудита. Единственный источник
// истины для Valid() и ClientActions() (ADR-0003: одна формула/список в
// одном месте; дублирование списков — источник расхождений).
//
// SEC-002 (2026-09-26): до фикса `audit.Action` был обычной строкой без
// проверки, а POST /api/v1/audit принимал её из тела без allowlist. Любой
// аутентифицированный пользователь мог записать в журнал безопасности
// произвольное действие (например "user.role_changed") с жёстко заданным
// result="ok" — то есть подделать доказательство. Реестр + Valid() +
// ClientActions() закрывают путь на уровне домена, а не только в обработчике.
var allActions = []Action{
	ActionAuthRegister, ActionAuthLogin, ActionAuthLogout, ActionAuthLoginDenied,
	ActionAuthzDenied, ActionUserRoleChanged, ActionUserStatusChanged,
	ActionSettingsUpdated, ActionDataExported, ActionApiKeyCreated,
	ActionApiKeyRevoked, ActionSsoLogin, ActionSsoLoginDenied, ActionSsoLinked,
	ActionProjectCreated, ActionProjectModified, ActionMemberAdded,
	ActionMemberRoleChanged, ActionMemberRemoved, ActionConfigApproved,
	ActionConfigRestored, ActionReviewRequested, ActionReviewSigned,
	ActionReviewChanges,
	ActionAiAssistDesign, ActionAiAssistEngineering, ActionAiAssistManufacturing,
	ActionAiAssistPricing, ActionAiAssistMemoryPurged,
	ActionStairCalculated, ActionStairConfigChanged, ActionStairSuggestionApplied,
	ActionStairVariationApplied, ActionStairLiveSuggestionApplied,
	ActionStairLiveVariationApplied,
	ActionOrderStatusChanged, ActionPaymentRefunded,
	ActionTestimonialCreated, ActionTestimonialUpdated, ActionTestimonialDeleted,
}

// clientActions — действия, которые клиент имеет право записать сам через
// POST /api/v1/audit (EDR-0013 §4.2: «клиентские события (клики, применение
// вариантов/советов, изменение полей)»).
//
// Всё остальное (auth.*, user.*, payment.*, settings.*, data.*, member.*,
// config.*, review.*, sso.*, ai.*) — исключительно серверная эмиссия.
// Если клиент сможет записать "user.role_changed" или "payment.refunded",
// журнал безопасности перестаёт быть доказательством.
var clientActions = []Action{
	ActionStairConfigChanged,
	ActionStairSuggestionApplied,
	ActionStairVariationApplied,
	ActionStairLiveSuggestionApplied,
	ActionStairLiveVariationApplied,
}

// AllActions возвращает копию реестра всех известных действий.
func AllActions() []Action {
	out := make([]Action, len(allActions))
	copy(out, allActions)
	return out
}

// ClientActions возвращает копию allowlist действий, записываемых клиентом.
func ClientActions() []Action {
	out := make([]Action, len(clientActions))
	copy(out, clientActions)
	return out
}

// Valid — принадлежит ли действие реестру известных.
//
// Проверяется на границе доверия (транспорт) и в репозитории (второй рубеж
// перед БД), чтобы неизвестное значение не могло попасть в журнал ни через
// новый маршрут, ни через прямую запись.
func (a Action) Valid() bool {
	for _, k := range allActions {
		if k == a {
			return true
		}
	}
	return false
}

// ClientRecordable — разрешено ли клиенту записать это действие самому.
func (a Action) ClientRecordable() bool {
	if !a.Valid() {
		return false
	}
	for _, k := range clientActions {
		if k == a {
			return true
		}
	}
	return false
}

// Result — исход события.
type Result string

const (
	// ResultUnset — нулевое значение типа. НЕ хранится в БД: перед записью
	// приводится к ResultOK (Result.OrOK).
	ResultUnset  Result = ""
	ResultOK     Result = "ok"
	ResultDenied Result = "denied"
	ResultFailed Result = "failed"
)

// OrOK — безопасное значение исхода для записи: «не задано» трактуется
// как «ок» (AUDIT-002, 2026-09-27).
//
// Почему ноль обязан быть безопасным, а не невалидным. Result — обычное
// поле структуры: забыть его при заполнении литерала не ловит ни
// компилятор, ни vet. Раньше пустой Result отвергался дважды — в
// Event.Validate и CHECK'ом БД audit_events_result_check, — а девять вызовов
// Record из двенадцати глушили ошибку через `_ =`. Итог: действие
// происходило, ответ клиенту был 200, а в журнале аудита не появлялось НИЧЕГО.
// Дыра не давала ни ошибки, ни ворнинга — обнаружить её было нечем.
//
// «Ок» как умолчание безопасен именно потому, что исход выбирает сервер: пустое
// поле не может стать «отказом» и замаскировать реальные отказы. Придумывать
// «failed» по умолчанию нельзя — это исказило бы расследование в другую
// сторону. Невалидным остаётся только НЕпустой мусор (см. Result.Valid).
func (r Result) OrOK() Result {
	if r == ResultUnset {
		return ResultOK
	}
	return r
}

// Valid — известен ли исход события. Клиент не может выбрать Result: в
// handleRecordAudit он жёстко задан как ResultOK, иначе поддельная запись
// могла бы выглядеть как отказ (result="denied") и маскировать реальные
// отказы, выдавая их за успех.
//
// ResultUnset здесь допустим осознанно: он означает «исход не задан» и
// разрешает записать событие, которое до этого молча терялось. Перед
// записью он приводится к ResultOK, поэтому в БД пустая строка не попадает
// (и не может попасть — там стоит CHECK result IN ('ok','denied','failed')).
// Любое другое значение по-прежнему отвергается: «неизвестный исход» — это
// ошибка программиста, и она должна быть громкой.
func (r Result) Valid() bool {
	switch r {
	case ResultUnset, ResultOK, ResultDenied, ResultFailed:
		return true
	}
	return false
}

// maxResourceTypeLen / maxResourceIDLen / maxDetailLen — границы полей,
// приходящих из тела запроса POST /api/v1/audit. Серверные значения
// существенно короче; лимиты не дают записать в TEXT-колонку мегабайт
// и не мешают обычным клиентским событиям.
const (
	maxResourceTypeLen = 64
	maxResourceIDLen   = 128
	maxDetailLen       = 4096
)

// Event — запись аудита (SEC-0013 Audit Metadata). Актор и tenant берутся
// из контекста запроса (EDR-0013 §4), никогда из тела.
type Event struct {
	ID           string
	ActorID      string
	TenantID     string
	ProjectID    string // пустая — глобальное событие
	Action       Action
	ResourceType string
	ResourceID   string
	Result       Result
	Detail       string
	RequestID    string
	IP           string
	CreatedAt    time.Time
}

// ErrInvalidEvent — событие не проходит доменную валидацию.
var ErrInvalidEvent = errors.New("audit: invalid event")

// Validate — доменная проверка события перед записью (второй рубеж после
// allowlist в транспорте; SEC-002).
//
// Проверяется:
//   - действие есть в реестре allActions;
//   - исход известен: один из Result* либо ResultUnset («не задан»);
//   - tenant и actor заданы (системные события передают actor пустым —
//     для них tenant обязателен, actor допускается пустым);
//   - длины resource_type / resource_id / detail в пределах лимитов.
func (e *Event) Validate() error {
	if e == nil {
		return fmt.Errorf("%w: nil event", ErrInvalidEvent)
	}
	if !e.Action.Valid() {
		return fmt.Errorf("%w: unknown action %q", ErrInvalidEvent, e.Action)
	}
	if !e.Result.Valid() {
		return fmt.Errorf("%w: unknown result %q", ErrInvalidEvent, e.Result)
	}
	if e.TenantID == "" {
		return fmt.Errorf("%w: tenant is required", ErrInvalidEvent)
	}
	if len(e.ResourceType) > maxResourceTypeLen {
		return fmt.Errorf("%w: resource_type too long (%d > %d)", ErrInvalidEvent, len(e.ResourceType), maxResourceTypeLen)
	}
	if len(e.ResourceID) > maxResourceIDLen {
		return fmt.Errorf("%w: resource_id too long (%d > %d)", ErrInvalidEvent, len(e.ResourceID), maxResourceIDLen)
	}
	if len(e.Detail) > maxDetailLen {
		return fmt.Errorf("%w: detail too long (%d > %d)", ErrInvalidEvent, len(e.Detail), maxDetailLen)
	}
	return nil
}

// Repository — порт доступа к данным аудита (BE-0005).
type Repository interface {
	// Insert сохраняет событие (append-only). Реализация обязана отклонять
	// события, не прошедшие Event.Validate (SEC-002).
	Insert(ctx context.Context, e *Event) error
	// ListByProject возвращает события проекта внутри tenant
	// (SEC-0005), новые сверху; ErrNotFound — проекта нет в tenant.
	ListByProject(ctx context.Context, tenantID, projectID string, limit int) ([]*Event, error)
	// ListByTenant возвращает события tenant (глобальный аудит, admin),
	// новые сверху.
	ListByTenant(ctx context.Context, tenantID string, limit int) ([]*Event, error)
}

// DefaultListLimit — сколько событий возвращают ListByProject/ListByTenant
// по умолчанию. DOM-006 (2026-09-26): выборка шла без LIMIT, поэтому
// GET /audit отдавал весь журнал tenant'а одним ответом.
const DefaultListLimit = 500

// MaxListLimit — верхняя граница, даже если вызывающий запросит больше.
// Журнал аудита — приложение с постоянным ростом, поэтому «отдай всё»
// здесь означает «положи память процесса».
const MaxListLimit = 2000
