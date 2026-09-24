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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/stair"
)

// ErrInvalid — некорректный вход (неизвестный kind, невалидная конфигурация).
var ErrInvalid = errors.New("assistant: invalid input")

// ErrNoFeasible — для входных данных нет допустимой конфигурации.
var ErrNoFeasible = errors.New("assistant: no feasible configuration for the input")

// ErrForbidden — доступ запрещён (S-142, IDOR-фикс S-141 №1): запрос
// ссылается на проект (project_id из тела), членом которого вызывающий
// не является; conversation-memory такому запросу не отдаётся и не
// дописывается. Маппится в 403 на транспортном слое.
var ErrForbidden = errors.New("assistant: forbidden")

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

// AnalysisRequest — запрос инженерного/производственного/ценового анализа:
// конкретная конфигурация + опциональные расчётные параметры (D2–D4).
type AnalysisRequest struct {
	Config  stair.Config
	Options stair.Options
	// ProjectID — скоуп conversation-memory (S-135, AI-0006); пустая —
	// память для запроса не используется. Перед чтением/записью памяти
	// сервис проверяет членство вызывающего в проекте (S-142, EDR-0008):
	// не-член получает ErrForbidden.
	ProjectID string
	// HistoryLimit — число последних сообщений диалога в контекст
	// (0 → DefaultHistoryLimit=10, cap MaxHistoryLimit=50; <0 → без истории).
	HistoryLimit int
}

// Service — прикладной сервис AI-ассистентов. Инверсия зависимостей:
// получает stairCalculator (обычно *stair.Service) и опционально
// *audit.Service; аудит — best-effort (EDR-0013 §4.1), сбой журнала не
// проваливает ответ. RAG (AI-0007) и память (AI-0006) — опциональные
// порты: без них сервис работает в локальном режиме (S-135: RAG off по
// умолчанию без env).
type Service struct {
	tools  *Tools
	router ModelRouter
	audit  *audit.Service
	now    func() time.Time

	// RAG (S-135, AI-0007): nil — ретривер выключен, деградация в локальный
	// детерминированный режим.
	retriever Retriever
	// topK — число чанков на запрос (default 5, диапазон 1..50).
	topK int
	// Память диалога (S-135, AI-0006): nil — память выключена.
	memory MemoryStore
	// projectAuthz — проверка членства в проекте (S-142, IDOR-фикс S-141
	// №1): обязателен при включённой памяти, иначе доступ к памяти
	// fail-closed (без авторизатора conversation-memory не работает).
	projectAuthz ProjectAuthorizer
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
// конфигурации STAIR_AI_*. Ранее заданный бюджет (WithBudget) сохраняется.
func (s *Service) WithPrimaryBackend(b Backend) *Service {
	budget := s.router.budget
	s.router = NewModelRouter(b, s.router.local)
	s.router.budget = budget
	return s
}

// WithBudget подключает глобальный дневной лимит LLM-попыток (S-148,
// S-141 №14): исчерпан — только локальный бэкенд. Порядок относительно
// WithPrimaryBackend не важен (бюджет сохраняется).
func (s *Service) WithBudget(b *Budget) *Service {
	s.router = s.router.WithBudget(b)
	return s
}

// WithRAG подключает ретривер корпуса (S-135, AI-0007): найденные чанки
// попадают в контекст модели, цитаты источников — в Response.Notes,
// chunk ids — в detail аудита. Конфигурации пользователей в индекс не
// складываются; tenant-фильтр обязателен в каждом запросе (SEC-0005).
// topK — число чанков (default 5, clamp 1..50).
func (s *Service) WithRAG(r Retriever, topK int) *Service {
	s.retriever = r
	if topK < 1 {
		topK = 5
	}
	if topK > 50 {
		topK = 50
	}
	s.topK = topK
	return s
}

// WithMemory подключает conversation-memory (S-135, AI-0006): последние
// historyLimit сообщений проекта добавляются в контекст, после ответа
// диалог дописывается в хранилище (скоуп tenant+project). TTL-очистку
// выполняет хранилище/фоновый prune (STAIR_AI_MEMORY_TTL).
func (s *Service) WithMemory(m MemoryStore) *Service {
	s.memory = m
	return s
}

// WithProjectAuthz подключает проверку членства в проекте (S-142,
// IDOR-фикс S-141 №1): project_id приходит из тела запроса, поэтому перед
// чтением/записью conversation-memory сервис обязан убедиться, что
// вызывающий — член проекта (реализует project.Service). Без авторизатора
// память не работает (fail-closed).
func (s *Service) WithProjectAuthz(a ProjectAuthorizer) *Service {
	s.projectAuthz = a
	return s
}

// historyLimitOf извлекает history_limit из типизированного запроса.
func historyLimitOf(req Request) int {
	switch r := req.(type) {
	case DesignRequest:
		return clampHistoryLimit(r.HistoryLimit)
	case AnalysisRequest:
		return clampHistoryLimit(r.HistoryLimit)
	default:
		return 0 // не типизированный запрос — памяти нет
	}
}

// projectIDOf извлекает скоуп проекта из типизированного запроса.
func projectIDOf(req Request) string {
	switch r := req.(type) {
	case DesignRequest:
		return r.ProjectID
	case AnalysisRequest:
		return r.ProjectID
	default:
		return ""
	}
}

// summaryOf — детерминированное текстовое резюме запроса: используется как
// (а) контекстный запрос RAG, (б) user-сообщение conversation-memory.
// Персональных данных в конфигурации нет (S-127).
func summaryOf(kind Kind, req Request) string {
	s := "kind=" + string(kind)
	cfg := stair.Config{}
	switch r := req.(type) {
	case DesignRequest:
		cfg = r.Config
		if r.Preferences.Priority != "" {
			s += " priority=" + string(r.Preferences.Priority)
		}
	case AnalysisRequest:
		cfg = r.Config
	default:
		return s
	}
	if cfg.Width.Millimeters() > 0 {
		s += " width_mm=" + fmtFloat(cfg.Width.Millimeters())
	}
	if cfg.Height.Millimeters() > 0 {
		s += " height_mm=" + fmtFloat(cfg.Height.Millimeters())
	}
	if cfg.Flight != "" {
		s += " flight=" + string(cfg.Flight)
	}
	if cfg.OuterRadius.Millimeters() > 0 {
		s += " outer_radius_mm=" + fmtFloat(cfg.OuterRadius.Millimeters())
	}
	return s
}

func fmtFloat(v float64) string {
	b := make([]byte, 0, 24)
	b = strconv.AppendFloat(b, v, 'f', -1, 64)
	return string(b)
}

// jsonMarshal — компактный JSON без HTML-экранирования (для памяти/контекста).
func jsonMarshal(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Ask обрабатывает запрос ассистента kind: планирование → tool calls →
// сборка ответа → RAG-контекст/память → комментарий модели; аудит и
// метрика. Ошибка — при невозможности выполнить анализ (невалидный вход,
// сбой тула, отмена) либо ErrForbidden, когда запрос ссылается на проект,
// членом которого вызывающий не является (S-142). Сбой RAG/памяти —
// best-effort: ответ не проваливается (AI-0003 деградация).
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

	projectID := projectIDOf(req)
	historyLimit := historyLimitOf(req)

	// S-142 (IDOR-фикс S-141 №1): project_id приходит из тела запроса.
	// Проверка членства выполняется ДО чтения (RecentMessages) и записи
	// (remember): не-член получает ErrForbidden, память не читается и не
	// дописывается (единая проверка на весь запрос).
	if s.memory != nil && projectID != "" {
		if err := s.authorizeProject(ctx, tenantID, userID, projectID, kind); err != nil {
			return nil, err
		}
	}

	var history []MemoryMessage
	if s.memory != nil && projectID != "" {
		if h, herr := s.memory.RecentMessages(ctx, tenantID, projectID, historyLimit); herr == nil {
			history = h
		} else {
			// Память — best-effort: падение хранилища не валит ответ.
			slog.WarnContext(ctx, "assistant: memory load failed", "err", herr)
		}
	}

	start := s.now()
	resp, intent, aerr := exp.analyze(ctx, s.tools, req)
	if aerr != nil {
		s.auditResult(ctx, tenantID, userID, kind, audit.ResultFailed, aerr.Error())
		assistantDuration.With(string(kind), "false").Observe(s.now().Sub(start).Seconds())
		return nil, aerr
	}

	// RAG-контекст (S-135): чанки → контекст модели + цитаты в Notes.
	var ragChunks []Chunk
	if s.retriever != nil {
		if chunks, rerr := s.retriever.Retrieve(ctx, tenantID, summaryOf(kind, req), s.topK); rerr == nil {
			ragChunks = chunks
			if ctxBlock, cites := ChunkContext(chunks); ctxBlock != "" {
				intent.Context += "\n\nРелевантные фрагменты документации (RAG):\n" + ctxBlock
				resp.Notes = append(append([]string(nil), resp.Notes...), cites...)
			}
		} else {
			// RAG — best-effort (AI-0003): сбой ретривера не проваливает ответ.
			slog.WarnContext(ctx, "assistant: RAG retrieve failed", "tenant", tenantID, "err", rerr)
		}
	}

	// Память в контекст модели (только последние N; PII-правило S-127).
	if len(history) > 0 {
		intent.Context = HistoryBlock(history) + "\n" + intent.Context
	}

	ans, rerr := s.router.Infer(ctx, intent, resp)
	if rerr != nil {
		s.auditResult(ctx, tenantID, userID, kind, audit.ResultFailed, rerr.Error())
		assistantDuration.With(string(kind), "false").Observe(s.now().Sub(start).Seconds())
		return nil, rerr
	}

	// Сохранение диалога (скоуп project; старые сообщения вычищает TTL).
	if s.memory != nil && projectID != "" {
		if merr := s.remember(ctx, tenantID, projectID, kind, req, ans, resp); merr != nil {
			slog.WarnContext(ctx, "assistant: memory save failed", "err", merr)
		}
	}

	// Аудит: detail — контекст выполнения + использованные чанки RAG.
	detail := intent.Context
	if len(ragChunks) > 0 {
		ids := make([]string, 0, len(ragChunks))
		for _, c := range ragChunks {
			ids = append(ids, c.Citation())
		}
		detail += "\nrag_sources=" + strings.Join(ids, ",")
	}
	s.auditResult(ctx, tenantID, userID, kind, audit.ResultOK, detail)
	assistantDuration.With(string(kind), "true").Observe(s.now().Sub(start).Seconds())
	return &Result{Kind: kind, Response: *resp, Commentary: ans.Text}, nil
}

// authorizeProject проверяет членство вызывающего в проекте перед
// чтением/записью conversation-memory (S-142, IDOR-фикс S-141 №1).
// Не-член — ErrForbidden (отказ аудируется); сбой проверки/отсутствие
// авторизатора — ошибка (fail-closed: память без авторизации недоступна).
func (s *Service) authorizeProject(ctx context.Context, tenantID, userID, projectID string, kind Kind) error {
	if s.projectAuthz == nil {
		return fmt.Errorf("assistant: project authorization not configured (memory requires WithProjectAuthz, S-142)")
	}
	ok, err := s.projectAuthz.IsMember(ctx, tenantID, userID, projectID)
	if err != nil {
		return fmt.Errorf("assistant: project membership check: %w", err)
	}
	if !ok {
		s.auditResult(ctx, tenantID, userID, kind, audit.ResultDenied, "conversation-memory: project membership denied")
		return fmt.Errorf("%w: not a member of project", ErrForbidden)
	}
	return nil
}

// Forget стирает conversation-memory проекта (право на забвение, S-141
// №13, GDPR Art.17/152-ФЗ): self-service purge через
// DELETE /api/v1/assistant/memory. Требуется членство в проекте (та же
// семантика, что в Ask, S-142): не-член — ErrForbidden (отказ аудируется).
// Возвращает число удалённых сообщений. Purge kind-agnostic: строки памяти
// не атрибутированы ни kind, ни user_id — стирание выполняется через purge
// проектов пользователя; каскад при удалении tenant/project — на уровне БД
// (ON DELETE CASCADE, миграция 000026).
func (s *Service) Forget(ctx context.Context, tenantID, userID, projectID string) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if tenantID == "" || projectID == "" {
		return 0, fmt.Errorf("%w: tenant and project required", ErrInvalid)
	}
	if s.memory == nil {
		return 0, fmt.Errorf("assistant: memory not configured")
	}
	// Членство — fail-closed, как в authorizeProject, но с собственным
	// аудитом (purge не привязан к kind — dedicate action ниже).
	if s.projectAuthz == nil {
		return 0, fmt.Errorf("assistant: project authorization not configured (memory requires WithProjectAuthz, S-142)")
	}
	ok, err := s.projectAuthz.IsMember(ctx, tenantID, userID, projectID)
	if err != nil {
		return 0, fmt.Errorf("assistant: project membership check: %w", err)
	}
	if !ok {
		s.auditMemory(ctx, tenantID, userID, audit.ResultDenied, "conversation-memory purge denied: not a member")
		return 0, fmt.Errorf("%w: not a member of project", ErrForbidden)
	}
	n, err := s.memory.DeleteMessages(ctx, tenantID, projectID)
	if err != nil {
		return 0, err
	}
	s.auditMemory(ctx, tenantID, userID, audit.ResultOK, "conversation-memory purged")
	return n, nil
}

// auditMemory пишет событие purge памяти с dedicated action (purge
// kind-agnostic — kind-coupled auditResult здесь неприменим).
func (s *Service) auditMemory(ctx context.Context, tenantID, userID string, result audit.Result, detail string) {
	if s.audit == nil {
		return
	}
	e := &audit.Event{
		ActorID:      userID,
		TenantID:     tenantID,
		Action:       "ai.assist.memory.purged",
		ResourceType: "assistant.memory",
		Result:       result,
		Detail:       detail,
		CreatedAt:    s.now().UTC(),
	}
	_ = s.audit.Record(ctx, e)
}

// remember дописывает user+assistant сообщения диалога в хранилище
// (детерминированное резюме запроса и итоговый ответ).
func (s *Service) remember(ctx context.Context, tenantID, projectID string, kind Kind, req Request, ans *Answer, resp *Response) error {
	if resp == nil || ans == nil {
		return nil
	}
	now := s.now().UTC()
	msgs := []MemoryMessage{
		{TenantID: tenantID, ProjectID: projectID, Role: "user", Content: summaryOf(kind, req), CreatedAt: now},
	}
	if b, err := jsonMarshal(Result{Kind: kind, Response: *resp, Commentary: ans.Text}); err == nil {
		msgs = append(msgs, MemoryMessage{
			TenantID: tenantID, ProjectID: projectID, Role: "assistant",
			Content: truncateRunes(b, 4000), CreatedAt: now.Add(time.Millisecond),
		})
	}
	return s.memory.AppendMessages(ctx, msgs)
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
	case KindEngineering:
		return engineeringExpert{}, nil
	case KindManufacturing:
		return manufacturingExpert{}, nil
	case KindPricing:
		return pricingExpert{}, nil
	default:
		return nil, fmt.Errorf("%w: unknown assistant kind %q", ErrInvalid, kind)
	}
}
