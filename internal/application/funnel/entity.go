// Package funnel реализует прикладной слой воронки витрины: приём
// клиентских событий и отчёты «где идёт трафик, где затык, где бросают».
//
// Отличие от application/analytics: тот bounded context — бизнес-отчёты по
// ОПЕРАЦИОННЫМ таблицам (проекты, производство, деньги) и он read-only
// (в его доктрине прямо сказано «новых миграций нет»). Здесь наоборот:
// единственный источник данных — события, которые присылает витрина, и
// таблица web_events (миграция 000035). Смешивать их в одном пакете нельзя:
// разные источники, разные сроки жизни и разные правила приватности.
//
// Правила приватности зашиты в валидацию, а не в договорённости:
//   - IP и User-Agent не сохраняются, visitor — HMAC с солью сервера;
//   - имя события берётся только из каталога AllEvents, произвольные строки
//     отбрасываются (публичный эндпоинт не должен позволять писать в отчёт
//     что угодно);
//   - значение пропса — скаляр длиной не более MaxPropValueLen, чтобы в
//     событие не могло утечь то, что человек ввёл в поле формы;
//   - согласие — часть записи: событие принимается только с актуальной
//     версией политики.
package funnel

import (
	"context"
	"errors"
	"regexp"
	"time"
)

// Имена событий. Каталог закрыт: иначе публичный эндпоинт превращается в
// мусорную корзину, из которой уже нельзя построить воронку.
//
// Схема читается так: <область>.<что случилось>. Области:
//
//	page       — переходы между экранами;
//	funnel     — шаги конструктора;
//	blocker    — что мешает: ошибки полей и ответы сервера;
//	cta        — намерения (нажать «Рассчитать», отправить заявку);
//	session    — границы визита: начало и уход со страницы.
const (
	EventPageView     = "page.view"
	EventFunnelOpen   = "funnel.constructor_open"
	EventFunnelStep   = "funnel.step_done"
	EventBlockerField = "blocker.field_invalid"
	EventBlockerAPI   = "blocker.api_error"
	EventCTAQuote     = "cta.quote_clicked"
	EventCTAOrder     = "cta.order_clicked"
	EventSessionStart = "session.start"
	EventSessionLeave = "session.leave"
)

// AllEvents — полный каталог имён. Синхронизируется с CHECK в миграции
// 000035 тестом TestMigrationEventNamesMatchCatalog: расхождение роняет
// сборку, пока миграция не обновлена.
func AllEvents() []string {
	return []string{
		EventPageView,
		EventFunnelOpen,
		EventFunnelStep,
		EventBlockerField,
		EventBlockerAPI,
		EventCTAQuote,
		EventCTAOrder,
		EventSessionStart,
		EventSessionLeave,
	}
}

// EventNamePattern — форма имени, продублированная в CHECK миграции.
var EventNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){0,3}$`)

// KnownEvent сообщает, входит ли имя в каталог. Сверка с шаблоном НЕ
// substitute: шаблон проверяет форму, а каталог — разрешённость. Иначе
// публичный эндпоинт принимал бы любое «foo.bar» и в отчёте появились бы
// события, которых никто не задумывал.
func KnownEvent(name string) bool {
	for _, n := range AllEvents() {
		if n == name {
			return true
		}
	}
	return false
}

// Границы приёма. Числа совпадают с CHECK в миграции 000035.
const (
	// MaxBatchEvents — событий в одном запросе. Больше — отбрасываем целиком:
	// такой «пакет» не бывает от честного клиента, а разбирать мегабайты чужого
	// мусора мы не будем.
	MaxBatchEvents = 20
	// MaxPropCount — ключей в пропсе.
	MaxPropCount = 12
	// MaxPropKeyLen — длина ключа пропа.
	MaxPropKeyLen = 32
	// MaxPropValueLen — длина строкового значения пропа.
	MaxPropValueLen = 64
	// MaxPathLen — длина маршрута.
	MaxPathLen = 128
	// MaxBrowserLen — длина семейства браузера.
	MaxBrowserLen = 32
	// MaxPropBytes — суммарный размер сериализованного props.
	MaxPropBytes = 1024
)

var (
	// ErrConsentRequired — событие пришло без актуального согласия. Это не
	// ошибка клиента, а отказ: ответ 202 без записи, чтобы витрина не
	// получала ошибку и не пыталась повторять.
	ErrConsentRequired = errors.New("funnel: consent required")
	// ErrBatchTooLarge — больше MaxBatchEvents событий в запросе.
	ErrBatchTooLarge = errors.New("funnel: too many events in batch")
	// ErrInvalidSession — пустой или нечитаемый session_id.
	ErrInvalidSession = errors.New("funnel: invalid session id")
)

// Event — одно принятое событие. Props хранится картой: JSONB нужен только
// репозиторию, транспорт не должен знать про jsonb.
type Event struct {
	SessionID      string
	Name           string
	Props          map[string]any
	Path           string
	Visitor        string
	Browser        string
	ConsentVersion int
	OccurredAt     time.Time
}

// IngestResult — что сделал сервис с пачкой. Dropped посчитан, а не спрятан:
// иначе ошибиться в каталоге невозможно заметить, и воронка молча рассыплется.
type IngestResult struct {
	Accepted int `json:"accepted"`
	Dropped  int `json:"dropped"`
}

// FunnelStep — шаг воронки: сколько сессий и уникальных посетителей его
// дошло. Share — доля от всех посетителей окна.
type FunnelStep struct {
	Name      string  `json:"name"`
	Sessions  int     `json:"sessions"`
	Visitors  int     `json:"visitors"`
	Share     float64 `json:"share"`
	StepShare float64 `json:"step_share"`
}

// Blocker — точка затруднения: событие блокировки с самым частым пропом.
type Blocker struct {
	Event  string `json:"event"`
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// AbandonPoint — где именно бросают: последнее событие перед уходом.
type AbandonPoint struct {
	LastEvent string  `json:"last_event"`
	Sessions  int     `json:"sessions"`
	Share     float64 `json:"share"`
	AvgSec    float64 `json:"avg_seconds"`
}

// FunnelReport — ответ «где идёт трафик, где затык, где бросает».
type FunnelReport struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	// Visitors — уникальные посетители окна; 0 при выключенной
	// идентификации, тогда смотрим VisitorIdentityEnabled.
	Visitors               int            `json:"visitors"`
	VisitorIdentityEnabled bool           `json:"visitor_identity_enabled"`
	Sessions               int            `json:"sessions"`
	Events                 int            `json:"events"`
	Steps                  []FunnelStep   `json:"steps"`
	Blockers               []Blocker      `json:"blockers"`
	Abandons               []AbandonPoint `json:"abandons"`
	AvgSecAll              float64        `json:"avg_seconds"`
}

// StepOrder — порядок шагов воронки: только ДВИЖЕНИЕ вперёд.
//
// Блокировок (blocker.*) здесь нет намеренно. Это не шаг пути, а препятствие
// на нём, и в таблице шагов он ломал переходы: на живых данных «Дошёл до
// конструктора» показывал 150% от предыдущего шага, потому что шагом шёл
// затык, у которого другая природа счёта. У блокировок своя таблица
// («Где спотыкаются»).
func StepOrder() []string {
	return []string{
		EventSessionStart,
		EventPageView,
		EventFunnelOpen,
		EventFunnelStep,
		EventCTAQuote,
		EventCTAOrder,
	}
}

// Repository — порт хранения (BE-0005). Запись и чтение разнесены намеренно:
// публичный эндпоинт только пишет, админский только читает.
type Repository interface {
	// InsertEvents сохраняет пачку событий. Частичная запись недопустима:
	// возвращать ошибку и откатывать всё, иначе отчёт считает половину визита.
	InsertEvents(ctx context.Context, events []Event) error
	// SessionCount — число сессий с любым событием в окне.
	SessionCount(ctx context.Context, from, to time.Time) (int, error)
	// VisitorCount — число УНИКАЛЬНЫХ посетителей в окне. Считается по
	// visitor (хеш IP+UA). Без соли на сервере все хеши пустые и число равно
	// нулю, поэтому вместе с ним отдаётся VisitorIdentityEnabled: «0
	// посетителей» и «считать нечем» — разные вещи, и путать их нельзя.
	VisitorCount(ctx context.Context, from, to time.Time) (int, error)
	// EventCount — число событий в окне.
	EventCount(ctx context.Context, from, to time.Time) (int, error)
	// StepCounts — число СЕССИЙ и УНИКАЛЬНЫХ ПОСЕТИТЕЛЕЙ, дошедших до каждого
	// имени события, и средняя длительность сессии в секундах по шагу.
	StepCounts(ctx context.Context, from, to time.Time) (map[string]StepStat, error)
	// TopBlockers — самые частые блокировки, сгруппированные по событию и
	// пропу reason (для blocker.field_invalid это имя поля).
	TopBlockers(ctx context.Context, from, to time.Time, limit int) ([]Blocker, error)
	// AbandonPoints — распределение последнего события перед session.leave.
	AbandonPoints(ctx context.Context, from, to time.Time, limit int) ([]AbandonPoint, error)
	// AvgSessionSeconds — средняя длительность визита по session.leave.
	AvgSessionSeconds(ctx context.Context, from, to time.Time) (float64, error)
	// Cleanup удаляет события старше порога. Возвращает число удалённых строк.
	Cleanup(ctx context.Context, before time.Time) (int64, error)
}

// StepStat — агрегат по шагу воронки. Сессии и посетители расходятся: один
// человек может открыть конструктор пять раз за месяц, и для оценки объёма
// трафика нужны оба числа.
type StepStat struct {
	Sessions  int
	Visitors  int
	AvgSecond float64
}
