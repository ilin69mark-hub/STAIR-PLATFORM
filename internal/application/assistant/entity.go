// Package assistant реализует AI Layer прикладного уровня (Phase D,
// EDR-0036): интеллектуальные ассистенты поверх существующих конвейеров.
// AI не содержит предметной бизнес-логики и не является источником
// истины (AI-0000): ассистенты работают только через Tool Calling на
// stair.Service (расчёт/оптимизация/валидация), собирают структурированный
// детерминированный ответ (ADR-0003) из результатов движков и дополняют
// его комментарием от модели (ModelRouter: локальный детерминированный
// бэкенд по умолчанию, опциональный OpenAI-совместимый первичный с
// фолбэком — AI-0003). Каждый запрос аудируется (AI-0001: «AI полностью
// аудируем»).
//
// Мета-уровень зависит от доменов/движков/инфраструктурных портов, но не
// от HTTP/БД/UI (ADR-0006, DOM-0008).
package assistant

import (
	"context"
	"errors"
	"fmt"
	"time"

	"stairplatform/internal/application/audit"
)

// ErrInvalid — некорректный вход (неизвестный kind, невалидная конфигурация).
var ErrInvalid = errors.New("assistant: invalid input")

// ErrNoFeasible — для входных данных нет допустимой конфигурации.
var ErrNoFeasible = errors.New("assistant: no feasible configuration for the input")

// Kind — тип ассистента (одно из направлений Phase D).
type Kind string

// Типы ассистентов (D1–D4).
const (
	KindDesign        Kind = "design"
	KindEngineering   Kind = "engineering"
	KindManufacturing Kind = "manufacturing"
	KindPricing       Kind = "pricing"
)

// Valid проверяет известность типа ассистента.
func (k Kind) Valid() bool {
	switch k {
	case KindDesign, KindEngineering, KindManufacturing, KindPricing:
		return true
	}
	return false
}

// Action возвращает audit.Action для запроса ассистента.
func (k Kind) Action() audit.Action {
	return audit.Action("ai.assist." + string(k))
}

// Finding — замечание ассистента (severity: info|warning|error).
type Finding struct {
	Severity string `json:"severity"`
	Element  string `json:"element,omitempty"`
	Message  string `json:"message"`
}

// Suggestion — конкретная рекомендация (правка/действие).
type Suggestion struct {
	Message   string `json:"message"`
	Rationale string `json:"rationale,omitempty"`
}

// Alternative — альтернативный вариант (используется Design наряду с
// главной рекомендацией).
type Alternative struct {
	Title  string  `json:"title"`
	Rating float64 `json:"rating"` // 0..1 — близость к оптимуму
	Reason string  `json:"reason,omitempty"`
}

// Response — структурный (детерминированный) ответ ассистента. Строится
// только из результатов Tool Calling существующих движков; «истина»
// остаётся в конвейере.
type Response struct {
	Recommendation string        `json:"recommendation"`
	Rating         float64       `json:"rating"` // 0..1 — уверенность/применимость
	Alternatives   []Alternative `json:"alternatives,omitempty"`
	Findings       []Finding     `json:"findings,omitempty"`
	Suggestions    []Suggestion  `json:"suggestions,omitempty"`
	Tradeoffs      []string      `json:"tradeoffs,omitempty"`
	Notes          []string      `json:"notes,omitempty"`
}

// Result — итоговый ответ ассистента: детерминированная структура плюс
// комментарий модели (локальный эксперт или LLM первичного бэкенда).
type Result struct {
	Kind       Kind     `json:"kind"`
	Response   Response `json:"response"`
	Commentary string   `json:"commentary"`
}

// Request — общий интерфейс входных данных ассистента (реализуется
// DesignRequest/AnalysisRequest и др.).
type Request interface{}

// Service — прикладной сервис AI-ассистентов. Инверсия зависимостей:
// получает stairCalculator (обычно *stair.Service) и опционально
// *audit.Service; аудит — best-effort (EDR-0013 §4.1), сбой журнала не
// проваливает ответ.
type Service struct {
	tools  *Tools
	router ModelRouter
	audit  *audit.Service
	now    func() time.Time
}

// NewService создаёт сервис ассистентов с локальным (детерминированным)
// бэкендом комментариев.
func NewService(calc stairCalculator, auditSvc ...*audit.Service) *Service {
	s := &Service{
		tools:  &Tools{calc: calc},
		router: NewModelRouter(nil, localComment),
		now:    time.Now,
	}
	if len(auditSvc) > 0 {
		s.audit = auditSvc[0]
	}
	return s
}

// WithPrimaryBackend подключает первичный (LLM) бэкенд с фолбэком на
// локальный (AI-0003). Вызывается из композиции только при наличии
// конфигурации STAIR_AI_*.
func (s *Service) WithPrimaryBackend(b Backend) *Service {
	s.router = NewModelRouter(b, s.router.local)
	return s
}

// Ask обрабатывает запрос ассистента kind: планирование → tool calls →
// сборка ответа → комментарий модели; аудит и метрика. Ошибка — только
// при невозможности выполнить анализ (невалидный вход, сбой тула, отмена).
func (s *Service) Ask(ctx context.Context, tenantID, userID string, kind Kind, req Request) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenant required", ErrInvalid)
	}
	exp, err := s.expert(kind)
	if err != nil {
		return nil, err
	}

	start := s.now()
	resp, intent, aerr := exp.analyze(ctx, s.tools, req)
	if aerr != nil {
		s.auditResult(ctx, tenantID, userID, kind, audit.ResultFailed, aerr.Error())
		assistantDuration.With(string(kind), "false").Observe(s.now().Sub(start).Seconds())
		return nil, aerr
	}

	ans, rerr := s.router.Infer(ctx, intent, resp)
	if rerr != nil {
		s.auditResult(ctx, tenantID, userID, kind, audit.ResultFailed, rerr.Error())
		assistantDuration.With(string(kind), "false").Observe(s.now().Sub(start).Seconds())
		return nil, rerr
	}

	s.auditResult(ctx, tenantID, userID, kind, audit.ResultOK, intent.Context)
	assistantDuration.With(string(kind), "true").Observe(s.now().Sub(start).Seconds())
	return &Result{Kind: kind, Response: *resp, Commentary: ans.Text}, nil
}

// auditResult записывает событие аудита best-effort. ActorID/TenantID —
// из контекста транспорта (EDR-0013 §4.2).
func (s *Service) auditResult(ctx context.Context, tenantID, userID string, kind Kind, result audit.Result, detail string) {
	if s.audit == nil {
		return
	}
	e := &audit.Event{
		ActorID:      userID,
		TenantID:     tenantID,
		Action:       kind.Action(),
		ResourceType: "assistant." + string(kind),
		Result:       result,
		Detail:       detail,
		CreatedAt:    s.now().UTC(),
	}
	if err := s.audit.Record(ctx, e); err != nil {
		// best-effort: аудит не должен ломать ответ ассистента.
		_ = err
	}
}

// intent — промежуточный материал для генерации комментария модели
// (build от эксперта, потребляется ModelRouter).
type intent struct {
	Kind         string
	Instructions string
	UserTask     string
	Context      string // нарративный контекст результатов тулов (для LLM и логов)
	Data         []byte // строгий JSON-контекст (для первичного бэкенда)
}

// expert — обработчик ассистента заданного вида.
type expert interface {
	analyze(ctx context.Context, tools *Tools, req Request) (*Response, *intent, error)
}

// expert возвращает обработчик заданного типа ассистента.
func (s *Service) expert(kind Kind) (expert, error) {
	switch kind {
	case KindDesign:
		return designExpert{}, nil
	// D2–D4 (engineering/manufacturing/pricing) подключаются в следующих
	// фичах Phase D (EDR-0037..0039); до этого kind неизвестен.
	default:
		return nil, fmt.Errorf("%w: unknown assistant kind %q", ErrInvalid, kind)
	}
}
