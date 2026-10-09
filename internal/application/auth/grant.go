// Точечные выдачи прав сотрудникам (EDR-0041).
//
// Прежде права определялись ролью: один человек — один набор. Теперь
// владелец открывает страницу сотрудника и выдаёт метрики построчно. Здесь
// живут: структура выдачи, шаблоны ролей и пять правил, которые делают
// выдачу безопасной. Проверка прав в транспорте — не отсюда, а через
// GrantSet.Has (см. capability.go Satisfies).
//
// Правила (EDR-0041 §4):
//
//  1. Потолок выдачи: выдать можно только то, что есть у выдающего.
//  2. Право выдавать (staff.grants) не выдаётся никому, кроме владельца —
//     иначе цепочку «выдал → выдал» нельзя остановить.
//  3. «Менять» нельзя выдать без «видеть» (Satisfies сам не хранит write
//     без read, а валидация ниже не даёт такой набор прийти).
//  4. Владельца нельзя урезать: нельзя снять его выдачи и нельзя выдать
//     меньше. Владельцев может быть несколько, но хотя бы один остаётся.
//  5. Нельзя выдать самому себе (ни роль, ни права) и нельзя удалить себя.
package auth

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"stairplatform/internal/application/audit"
)

// Grant — одна выданная метрика сотруднику.
type Grant struct {
	Capability Capability
	// Level — выданный уровень: LevelRead, LevelWrite или LevelAction.
	Level     Granted
	GrantedBy string
	GrantedAt time.Time
}

// GrantSet — набор выдач сотрудника. Хранится в user_grants; при
// grants_initialized = false источником прав остаётся роль.
type GrantSet struct {
	grants map[Capability]Granted
}

// NewGrantSet строит набор из пары «метрика — уровень».
func NewGrantSet(pairs ...Grant) *GrantSet {
	s := &GrantSet{grants: make(map[Capability]Granted, len(pairs))}
	for _, g := range pairs {
		s.grants[g.Capability] = g.Level
	}
	return s
}

// GrantSetFromPairs строит набор из плоского среза (репозиторий/транспорт).
func GrantSetFromPairs(caps []Capability, levels []Granted) (*GrantSet, error) {
	if len(caps) != len(levels) {
		return nil, fmt.Errorf("auth: grants mismatch: %d метрик, %d уровней", len(caps), len(levels))
	}
	s := &GrantSet{grants: make(map[Capability]Granted, len(caps))}
	for i, c := range caps {
		s.grants[c] = levels[i]
	}
	return s, nil
}

// Has — проверка «выдано ли сотруднику достаточно для этого уровня».
func (s *GrantSet) Has(c Capability, level Level) bool {
	if s == nil {
		return false
	}
	return Satisfies(s.grants[c], level)
}

// HasAny — есть ли метрика выданная хотя бы как-нибудь. Нужно для «видна ли
// плитка блока»: строка с одним переключателем тоже делает блок видимым.
func (s *GrantSet) HasAny(c Capability) bool {
	if s == nil {
		return false
	}
	return s.grants[c] != GrantedNone
}

// Level — выданный уровень метрики (GrantedNone, если не выдана).
func (s *GrantSet) Level(c Capability) Granted {
	if s == nil {
		return GrantedNone
	}
	return s.grants[c]
}

// Len — количество выданных метрик.
func (s *GrantSet) Len() int {
	if s == nil {
		return 0
	}
	return len(s.grants)
}

// Pairs — набор как срез пар, отсортированный по имени метрики (для
// стабильного JSON и предсказуемых тестов).
func (s *GrantSet) Pairs() []Grant {
	out := make([]Grant, 0, s.Len())
	for c, l := range s.grants {
		out = append(out, Grant{Capability: c, Level: l})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Capability < out[j].Capability })
	return out
}

// Capabilities — метрики, у которых есть хоть какой-то уровень.
func (s *GrantSet) Capabilities() []Capability {
	out := make([]Capability, 0, s.Len())
	for c, l := range s.grants {
		if l != GrantedNone {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// CapabilitiesInBlocks — выданные метрики, сгруппированные по блокам реестра.
// Фронт строит из этого матрицу прав без собственного списка метрик.
func (s *GrantSet) CapabilitiesInBlocks() map[string][]Capability {
	out := make(map[string][]Capability)
	for _, spec := range CapabilityRegistry {
		if s.HasAny(spec.Capability) {
			out[spec.Block] = append(out[spec.Block], spec.Capability)
		}
	}
	return out
}

// With — копия набора с одной переустановленной метрикой.
func (s *GrantSet) With(c Capability, level Granted) *GrantSet {
	next := make(map[Capability]Granted, len(s.grants)+1)
	for k, v := range s.grants {
		next[k] = v
	}
	if level == GrantedNone {
		delete(next, c)
	} else {
		next[c] = level
	}
	return &GrantSet{grants: next}
}

// ValidateGrantSet проверяет набор перед сохранением:
//   - каждая метрика есть в реестре и её уровень ей разрешён;
//   - «менять» всегда «и видеть»: храним write, который Satisfies трактует
//     как «и то, и то», поэтому отдельно требовать read-строку нельзя —
//     достаточно, чтобы write был у метрик, где он разрешён;
//   - метрика-действие выдаётся только уровнем action.
func ValidateGrantSet(s *GrantSet) error {
	if s == nil {
		return nil
	}
	for _, g := range s.Pairs() {
		spec, ok := LookupCapability(g.Capability)
		if !ok {
			return fmt.Errorf("auth: unknown capability %q", g.Capability)
		}
		switch g.Level {
		case GrantedRead:
			if !AllowsLevel(g.Capability, LevelRead) {
				return fmt.Errorf("auth: %q нельзя только читать (%s)", g.Capability, spec.Title)
			}
		case GrantedWrite:
			if !AllowsLevel(g.Capability, LevelWrite) {
				return fmt.Errorf("auth: %q нельзя менять (%s)", g.Capability, spec.Title)
			}
		case GrantedAction:
			if !AllowsLevel(g.Capability, LevelAction) {
				return fmt.Errorf("auth: %q — не действие, а данные (%s)", g.Capability, spec.Title)
			}
		default:
			return fmt.Errorf("auth: неизвестный уровень %q у %q", g.Level, g.Capability)
		}
	}
	return nil
}

// ValidateGrantsFor проверяет потолок выдачи (правило 1) и запрет на выдачу
// права выдавать (правило 2):
//
//	granted — то, что хочет выдать выдающий;
//	ceiling — то, что выдающий имеет на момент выдачи (nil = роль/владелец).
//
// Владелец (ceiling == nil) ограничен только правилом 2.
func ValidateGrantsFor(granted, ceiling *GrantSet) error {
	if err := ValidateGrantSet(granted); err != nil {
		return err
	}
	if ceiling == nil {
		// Владелец: всё, кроме права выдавать.
		if granted.HasAny(CapStaffGrants) {
			return fmt.Errorf("auth: право выдавать права остаётся за владельцем")
		}
		return nil
	}
	for _, g := range granted.Pairs() {
		if g.Capability == CapStaffGrants {
			return fmt.Errorf("auth: право выдавать права остаётся за владельцем")
		}
		// Проверяем именно уровень, а не просто наличие метрики: иначе
		// «видеть заказы» позволяло бы выдать «менять заказы».
		if !ceiling.Has(g.Capability, Level(g.Level)) {
			return fmt.Errorf("auth: нельзя выдать %q на уровне %q: у вас нет такого права", g.Capability, g.Level)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Шаблоны ролей
// ---------------------------------------------------------------------------

// Владелец получает все метрики на максимальном уровне программно, а не
// строкой в таблице: право выдавать (staff.grants) по правилу 2 не
// выдаётся никому, даже владельцу, — оно проверяется отдельно по роли.
// OwnerCapabilities — полный набор метрик.
func OwnerCapabilities() *GrantSet {
	s := &GrantSet{grants: make(map[Capability]Granted, len(CapabilityRegistry))}
	for _, spec := range CapabilityRegistry {
		if spec.Capability == CapStaffGrants {
			// Не хранится: выдача права выдавать — только владельцу, и
			// проверяется по роли, а не по таблице.
			continue
		}
		s.grants[spec.Capability] = MaxGranted(spec)
	}
	return s
}

// RoleTemplate — набор прав по умолчанию для роли. Применяется при создании
// сотрудника; дальше выдачи правятся построчно (EDR-0041 §3.2).
func RoleTemplate(role Role) *GrantSet {
	switch role {
	case RoleOwner:
		return OwnerCapabilities()
	case RoleAdmin:
		// Служебная роль сида: полный набор, как и до перехода на выдачи.
		return OwnerCapabilities()
	case RoleSales:
		// Руководитель отдела продаж: заказы и их статусы, платежи на
		// чтение, воронка, сотрудники, цены на чтение, отзывы. Без
		// себестоимости, возвратов, безопасности, витрины и производства.
		// production.report здесь лишний: плитка «Производство» открылась бы
		// руководителю продаж, а это не его работа.
		return NewGrantSet(
			Grant{Capability: CapStaffView, Level: GrantedRead},
			Grant{Capability: CapStaffCreate, Level: GrantedAction},
			Grant{Capability: CapStaffStatus, Level: GrantedAction},
			Grant{Capability: CapFinanceSales, Level: GrantedWrite},
			Grant{Capability: CapFinancePayments, Level: GrantedRead},
			Grant{Capability: CapFinanceCost, Level: GrantedRead},
			Grant{Capability: CapTrafficFunnel, Level: GrantedRead},
			Grant{Capability: CapTrafficUsage, Level: GrantedRead},
			Grant{Capability: CapTestimonialsLevel, Level: GrantedWrite},
			Grant{Capability: CapPriceMaterials, Level: GrantedRead},
		)
	case RoleUser:
		// Менеджер по продажам: заказы на чтение и свои проекты. Активность
		// платформы (входы, расчёты всех сотрудников) сюда не входит — это
		// метрика владельца, а не продаж.
		return NewGrantSet(
			Grant{Capability: CapFinanceSales, Level: GrantedRead},
		)
	default:
		return &GrantSet{grants: map[Capability]Granted{}}
	}
}

// ValidateOwnerGrants — правило 4: у владельца должен быть полный набор
// метрик. Урезать владельца нельзя, иначе одна ошибка оставит компанию без
// того, кто может выдать права, и восстановить доступ будет нечем.
//
// Владельцев может быть несколько, поэтому проверка — про набор, а не про
// количество владельцев в tenant (это делает сервис через
// CountActiveOwners, чтобы нельзя было убрать последнего).
func ValidateOwnerGrants(s *GrantSet) error {
	full := OwnerCapabilities()
	if s == nil {
		return fmt.Errorf("auth: у владельца не может быть пустой набор прав")
	}
	var missing []string
	for _, c := range full.Capabilities() {
		if !s.HasAny(c) {
			missing = append(missing, string(c))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("auth: у владельца нельзя снимать метрики: %s", strings.Join(missing, ", "))
	}
	return nil
}

// CapTestimonialsLevel — удобный алиас уровня write для отзывов: метрика
// read/write, а руководителю продаж нужна ровно запись отзыва.
const CapTestimonialsLevel = CapSiteReviews

// ---------------------------------------------------------------------------
// Репозиторий
// ---------------------------------------------------------------------------

// GrantsRepository — чтение и замена точечных выдач (EDR-0041 §5).
// Вынесено отдельным интерфейсом, чтобы реализация прав выдач не расползалась
// по базовому Repository.
type GrantsRepository interface {
	// ListGrants возвращает выдачи сотрудника. ErrNotFound не бывает:
	// пустой набор — законное состояние.
	ListGrants(ctx context.Context, userID string) (*GrantSet, error)
	// ReplaceGrants атомарно заменяет набор выдач сотрудника и взводит
	// users.grants_initialized = true. Пустой набор означает «не дал ничего».
	ReplaceGrants(ctx context.Context, tenantID, userID string, s *GrantSet, grantedBy string) error
	// SetGrantsInitialized снимает/ставит флаг «права заданы вручную» —
	// этим возвратом к роли пользуется сервис при неудачной валидации.
	SetGrantsInitialized(ctx context.Context, userID string, initialized bool) error
	// CountActiveOwners возвращает число неудалённых владельцев tenant.
	CountActiveOwners(ctx context.Context, tenantID string) (int, error)
}

// IsOwner — владелец ли роль пользователя.
func (r Role) IsOwner() bool { return r == RoleOwner }

// IsPrivileged — роль, которой нельзя выдать права «по частям» иначе, чем
// владельцу: только владелец может выдать полный набор (правило 4).
func (r Role) IsPrivileged() bool { return r == RoleOwner || r == RoleAdmin }

// CapabilitiesString — набор метрик одной строкой для аудита и логов.
func (s *GrantSet) CapabilitiesString() string {
	caps := s.Capabilities()
	parts := make([]string, 0, len(caps))
	for _, c := range caps {
		parts = append(parts, fmt.Sprintf("%s=%s", c, s.Level(c)))
	}
	return strings.Join(parts, ",")
}

// ---------------------------------------------------------------------------
// Сервис
// ---------------------------------------------------------------------------

// ErrGrantCeiling — попытка выдать право, которого нет у выдающего
// (правило 1). Отдельная ошибка, чтобы транспорт отдал 403 с внятным текстом,
// а не «неизвестно, что не так».
var ErrGrantCeiling = fmt.Errorf("auth: grant exceeds grantor rights")

// ErrGrantOwnerProtected — попытка урезать владельца (правило 4).
var ErrGrantOwnerProtected = fmt.Errorf("auth: owner grants cannot be reduced")

// ErrGrantsNotConfigured — сервис собран без репозитория выдач. Это ошибка
// сборки приложения, а не запроса: отвечать 503 и писать в лог.
var ErrGrantsNotConfigured = fmt.Errorf("auth: grants repository is not configured")

// GrantsService — операции над точечными выдачами: чтение набора и его
// замена с проверкой правил. Отдельный сервис, чтобы не раздувать Service,
// который отвечает за вход, сессии и 2FA.
type GrantsService struct {
	core  *Service
	audit *audit.Service
}

// NewGrantsService собирает сервис выдач поверх Service: он уже умеет отдать
// эффективные права (шаблон роли или таблица выдач) и прочитать сотрудника.
func NewGrantsService(core *Service, auditSvc ...*audit.Service) *GrantsService {
	s := &GrantsService{core: core}
	if len(auditSvc) > 0 {
		s.audit = auditSvc[0]
	} else {
		// Аудит наследуем у core. Раньше здесь был nil, если сервис выдач
		// создавался без явного аудита (так делает Service.SetUserGrants),
		// и выдача прав меняла доступ МОЛЧА: в журнале безопасности её не
		// было, то есть «кто и кому что выдал» не восстановить.
		s.audit = core.audit
	}
	return s
}

// Grants возвращает репозиторий выдач — им же пишет сервис.
func (s *GrantsService) Grants() GrantsRepository { return s.core.grants }

// GetGrants возвращает набор прав сотрудника для показа в матрице.
func (s *GrantsService) GetGrants(ctx context.Context, tenantID, userID string) (*GrantSet, error) {
	u, err := s.core.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return s.core.EffectiveGrants(ctx, u)
}

// SetGrants заменяет набор прав сотрудника.
//
// Проверки по порядку (EDR-0041 §4):
//  1. нельзя выдать себе (правило 5);
//  2. нельзя выдать больше, чем есть у выдающего, и нельзя выдать право
//     выдавать (правила 1 и 2);
//  3. владельцу нельзя снять метрики (правило 4).
//
// Отказ пишется в аудит как authz.denied — по журналу должно быть видно, что
// кто-то пытался расширить доступ.
func (s *GrantsService) SetGrants(ctx context.Context, tenantID string, actor *User, userID string, next *GrantSet) error {
	if actor == nil {
		return ErrForbidden
	}
	actorID := actor.ID
	if actorID == userID {
		s.record(ctx, actorID, tenantID, audit.ActionAuthzDenied, audit.ResultDenied,
			"self grants change: "+userID)
		return ErrForbidden
	}

	target, err := s.core.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if target.TenantID != tenantID {
		return ErrNotFound
	}

	// Потолок выдачи: владелец ограничен только правилом 2 (право выдавать
	// не выдаётся никому), остальные — ещё и своим набором.
	actorGrants, err := s.core.EffectiveGrants(ctx, actor)
	if err != nil {
		return err
	}
	var ceiling *GrantSet
	if !actor.Role.IsOwner() {
		ceiling = actorGrants
	}
	if err := ValidateGrantsFor(next, ceiling); err != nil {
		s.record(ctx, actorID, tenantID, audit.ActionAuthzDenied, audit.ResultDenied,
			"grants denied for "+userID+": "+err.Error())
		if strings.Contains(err.Error(), "у вас нет такого права") {
			return ErrGrantCeiling
		}
		return ErrForbidden
	}

	// Владельца нельзя урезать.
	if target.Role.IsOwner() {
		if err := ValidateOwnerGrants(next); err != nil {
			s.record(ctx, actorID, tenantID, audit.ActionAuthzDenied, audit.ResultDenied,
				"owner grants change denied for "+userID)
			return ErrGrantOwnerProtected
		}
	}

	// Репозиторий выдач — обязательная связь: без неё выдача некуда писать.
	// Раньше здесь был nil-указатель и 500 в проде (выдачу нашла только
	// ручная проверка на стенде), теперь это внятная ошибка.
	if s.core.grants == nil {
		return ErrGrantsNotConfigured
	}
	if err := s.core.grants.ReplaceGrants(ctx, tenantID, userID, next, actorID); err != nil {
		return err
	}

	before, err := s.core.EffectiveGrants(ctx, target)
	if err != nil {
		before = &GrantSet{grants: map[Capability]Granted{}}
	}
	s.record(ctx, actorID, tenantID, audit.ActionUserGrantsChanged, audit.ResultOK,
		target.Email+": "+before.CapabilitiesString()+" → "+next.CapabilitiesString())
	return nil
}

func (s *GrantsService) record(ctx context.Context, actorID, tenantID string, action audit.Action, result audit.Result, detail string) {
	if s.audit == nil {
		return
	}
	m := audit.MetaFrom(ctx)
	if err := s.audit.Record(ctx, &audit.Event{
		ActorID:   actorID,
		TenantID:  tenantID,
		Action:    action,
		Result:    result,
		Detail:    detail,
		RequestID: m.RequestID,
		IP:        m.IP,
	}); err != nil {
		slog.Error("auth: grants audit record failed",
			"action", string(action), "result", string(result), "err", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Эффективные права на Service
// ---------------------------------------------------------------------------

// EffectiveGrants — права сотрудника «как есть»: шаблон роли или точечные
// выдачи.
//
// Пока grants_initialized = false, действует шаблон роли. Это переходный
// путь: он нужен, чтобы переезд на выдачи не отнял доступ у тех, чьи права
// ещё не настроили. Флаг поднимает GrantsService.SetGrants, и с этого
// момента источник прав — только user_grants.
func (s *Service) EffectiveGrants(ctx context.Context, u *User) (*GrantSet, error) {
	if u == nil {
		return NewGrantSet(), nil
	}
	// Репозиторий выдач — обязательная связь. Раньше его отсутствие тихо
	// уводило на шаблон роли: выдачи просто «не работали», и никто об этом
	// не узнавал, пока не нажимал кнопку. Теперь это ошибка сборки.
	if s.grants == nil {
		return nil, ErrGrantsNotConfigured
	}
	if !u.GrantsInitialized {
		return RoleTemplate(u.Role), nil
	}
	return s.grants.ListGrants(ctx, u.ID)
}

// WithGrants подключает репозиторий точечных выдач. Без него EffectiveGrants
// всегда отдаёт шаблон роли — то есть поведение полностью прежнее.
func (s *Service) WithGrants(g GrantsRepository) *Service {
	s.grants = g
	return s
}

// GetUserGrants / SetUserGrants на Service — порт для транспорта. Держать
// сервис выдач отдельным объектом нужно ради аудита и правил, а держать
// два способа дойти до прав — нет: если бы Service считал права сам, а
// GrantsService — по-своему, они рано или поздно разошлись бы.
func (s *Service) GetUserGrants(ctx context.Context, tenantID, userID string) (*GrantSet, error) {
	return NewGrantsService(s).GetGrants(ctx, tenantID, userID)
}

func (s *Service) SetUserGrants(ctx context.Context, tenantID string, actor *User, userID string, next *GrantSet) error {
	return NewGrantsService(s).SetGrants(ctx, tenantID, actor, userID, next)
}

// MaxGranted — высший уровень, который метрика вообще допускает.
//
// Нужен шаблонам ролей: раньше всем «не-действиям» выдавался write, и
// владелец получал «менять» даже для отчётов, которые менять нельзя
// (себестоимость, платежи, воронка). Пользователь бы увидел переключатель
// «менять» на метрике, где его нет, а сервер отбил бы попытку сохранения.
func MaxGranted(spec CapabilitySpec) Granted {
	last := spec.Levels[len(spec.Levels)-1]
	switch last {
	case LevelWrite:
		return GrantedWrite
	case LevelAction:
		return GrantedAction
	default:
		return GrantedRead
	}
}
