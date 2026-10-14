package project

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/stair"
	dommfg "stairplatform/internal/domain/manufacturing"
	kerngeo "stairplatform/internal/geometry"
)

// ErrNotFound — сущность не найдена.
var ErrNotFound = errors.New("project: not found")

// ErrConflict — конфликт состояния (напр., расчёт без сохранённой конфигурации).
var ErrConflict = errors.New("project: conflict")

// ErrForbidden — недостаточно прав (EDR-0008): операция требует роли, которой
// у вызывающего нет.
var ErrForbidden = errors.New("project: forbidden")

// Service — прикладной сервис проектов (BE-0002 Use Cases): создание
// проекта, сохранение и расчёт конфигурации, получение результатов,
// управление участниками (Phase C/EDR-0008). Оркеструет stair.Service
// (движки) и Repository (данные). Авторизация определяет права по роли
// члена: owner/editor — изменение, owner — управление членами, все члены
// (owner/editor/viewer) — чтение.
type Service struct {
	repo  Repository
	calc  *stair.Service
	rules RuleSet
	audit *audit.Service
}

// RuleSet — бизнес-ограничения проекта (MVP-08): допустимые статусы.
type RuleSet struct {
	CreateStatus string
}

// DefaultRules возвращает правила по умолчанию.
func DefaultRules() RuleSet {
	return RuleSet{CreateStatus: "draft"}
}

// NewService создаёт сервис проектов. audit — необязательный журнал
// событий (EDR-0013); nil — запись аудита отключена.
func NewService(repo Repository, calc *stair.Service, rules RuleSet, auditSvc ...*audit.Service) *Service {
	s := &Service{repo: repo, calc: calc, rules: rules}
	if len(auditSvc) > 0 {
		s.audit = auditSvc[0]
	}
	return s
}

// record пишет событие аудита (best-effort, EDR-0013 §4.1).
//
// AUDIT-002 (2026-09-27): отказ записи логируется с action/tenant/actor/
// request_id, а не выбрасывается через `_ =`. Это самая частая точка записи
// аудита в проектах (вызывается на create/modify/review/restore), и раньше
// именно здесь молча терялись события: операция проходила, в журнале не
// появлялось ничего, обнаружить это было нечем. Business-операция при этом
// НЕ отменяется — аудит не должен ронять расчёт, но его пробел обязан быть
// виден в логах.
func (s *Service) record(ctx context.Context, tenantID, userID, projectID string, action audit.Action, result audit.Result, detail string) {
	if s.audit == nil {
		return
	}
	m := audit.MetaFrom(ctx)
	e := &audit.Event{
		ActorID:   userID,
		TenantID:  tenantID,
		ProjectID: projectID,
		Action:    action,
		Result:    result,
		Detail:    detail,
		RequestID: m.RequestID,
		IP:        m.IP,
	}
	if err := s.audit.Record(ctx, e); err != nil {
		slog.Error("audit: event not recorded",
			"action", string(e.Action), "tenant_id", e.TenantID,
			"actor_id", e.ActorID, "project_id", e.ProjectID,
			"request_id", e.RequestID, "error", err)
	}
}

// CreateProject создаёт проект с именем и описанием в tenant (BC-001)
// от имени пользователя ownerID, который становится владельцем (EDR-0008).
func (s *Service) CreateProject(ctx context.Context, tenantID, ownerID, name, description string) (*Project, error) {
	if name == "" {
		return nil, fmt.Errorf("project: name is required")
	}
	if ownerID == "" {
		return nil, fmt.Errorf("project: owner is required")
	}
	p := &Project{Name: name, Description: description, Status: s.rules.CreateStatus}
	if err := s.repo.CreateProject(ctx, tenantID, ownerID, p); err != nil {
		return nil, err
	}
	s.record(ctx, tenantID, ownerID, p.ID, audit.ActionProjectCreated, audit.ResultOK, "project created")
	return p, nil
}

// GetProject возвращает проект по ID внутри tenant (вызывающий — член).
func (s *Service) GetProject(ctx context.Context, tenantID, userID, id string) (*Project, error) {
	return s.repo.GetProject(ctx, tenantID, userID, id)
}

// ListProjects возвращает проекты tenant'а, членом которых является
// вызывающий пользователь (EDR-0008).
func (s *Service) ListProjects(ctx context.Context, tenantID, userID string) ([]*Project, error) {
	return s.repo.ListProjects(ctx, tenantID, userID)
}

// ListTenantProjects возвращает все проекты tenant (EDR-0016 §3):
// используется admin-экспортом и обзором; членство не требуется.
// Право data.export/users.list проверяется в транспорте.
func (s *Service) ListTenantProjects(ctx context.Context, tenantID string) ([]*Project, error) {
	return s.repo.ListTenantProjects(ctx, tenantID)
}

// ListMembers возвращает участников проекта (EDR-0008). Требуется членство.
func (s *Service) ListMembers(ctx context.Context, tenantID, userID, projectID string) ([]*ProjectMember, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	return s.repo.ListMembers(ctx, tenantID, projectID)
}

// AddMember добавляет участника проекта (EDR-0008). Требуется роль owner.
// Добавляемый пользователь должен существовать в том же tenant (защита от
// межтенантных ссылок); проверку выполняет реализация Repository.
func (s *Service) AddMember(ctx context.Context, tenantID, actorID, projectID, userID string, role ProjectRole) error {
	me, ok, err := s.member(ctx, tenantID, actorID, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectManage) {
		s.record(ctx, tenantID, actorID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "add member: owner required")
		return ErrForbidden
	}
	if err := s.repo.AddMember(ctx, tenantID, projectID, &ProjectMember{ProjectID: projectID, UserID: userID, Role: role}); err != nil {
		return err
	}
	s.record(ctx, tenantID, actorID, projectID, audit.ActionMemberAdded, audit.ResultOK, "user="+userID+" role="+string(role))
	return nil
}

// AddMemberByEmail приглашает участника по email (C2, EDR-0008).
// Требуется роль owner. Email резолвится в пользователя того же tenant
// в реализации Repository (SEC-0005). ErrNotFound — нет такого email.
func (s *Service) AddMemberByEmail(ctx context.Context, tenantID, actorID, projectID, email string, role ProjectRole) error {
	me, ok, err := s.member(ctx, tenantID, actorID, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectManage) {
		s.record(ctx, tenantID, actorID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "add member by email: owner required")
		return ErrForbidden
	}
	if email == "" {
		return fmt.Errorf("project: email is required")
	}
	return s.repo.AddMemberByEmail(ctx, tenantID, projectID, email, role)
}

// UpdateMemberRole изменяет роль участника проекта (EDR-0008).
// Требуется роль owner.
func (s *Service) UpdateMemberRole(ctx context.Context, tenantID, actorID, projectID, userID string, role ProjectRole) error {
	me, ok, err := s.member(ctx, tenantID, actorID, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectManage) {
		s.record(ctx, tenantID, actorID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "update member role: owner required")
		return ErrForbidden
	}
	if err := s.repo.UpdateMemberRole(ctx, tenantID, projectID, userID, role); err != nil {
		return err
	}
	s.record(ctx, tenantID, actorID, projectID, audit.ActionMemberRoleChanged, audit.ResultOK, "role="+string(role))
	return nil
}

// RemoveMember удаляет участника проекта (EDR-0008). Требуется роль owner;
// владельца удалить нельзя.
func (s *Service) RemoveMember(ctx context.Context, tenantID, actorID, projectID, userID string) error {
	me, ok, err := s.member(ctx, tenantID, actorID, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectManage) {
		s.record(ctx, tenantID, actorID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "remove member: owner required")
		return ErrForbidden
	}
	if err := s.repo.RemoveMember(ctx, tenantID, projectID, userID); err != nil {
		return err
	}
	s.record(ctx, tenantID, actorID, projectID, audit.ActionMemberRemoved, audit.ResultOK, "member removed")
	return nil
}

// TransferOwnership передаёт владение проектом другому участнику.
//
// Кто может: владелец проекта либо человек с платформенным правом
// staff.project_transfer (его проверяет транспорт и передаёт сюда флагом
// byCapability — сервис доверяет этому флагу, потому что проверить право
// в прикладном уровне нечем).
//
// Кому: только тому, кто уже участник проекта. Передача «на улице» означала
// бы, что человек появится в проекте без приглашения и без своего согласия.
//
// Что с прежним владельцем: он становится редактором, а не исчезает. Он
// работал с проектом, и уйти из него может сам. Так же ведёт себя удаление
// сотрудника: работа не пропадает, меняется человек.
func (s *Service) TransferOwnership(ctx context.Context, tenantID, actorID, projectID, toUserID string, byCapability bool) error {
	me, ok, err := s.member(ctx, tenantID, actorID, projectID)
	if err != nil {
		return err
	}
	if !ok {
		// Не участник проекта: забрать чужой проект может только владелец
		// компании с staff.project_transfer.
		if !byCapability {
			s.record(ctx, tenantID, actorID, projectID, audit.ActionAuthzDenied, audit.ResultDenied,
				"transfer: not a member")
			return ErrForbidden
		}
		// Участника нет, значит проверяем проект через членство будущего
		// владельца: он существует, принадлежит нашему tenant и уже в проекте.
		// Это же отсекает «передачу» несуществующего проекта.
		if _, err := s.repo.GetMember(ctx, tenantID, projectID, toUserID); err != nil {
			return err
		}
	} else if me.Role != RoleOwner {
		s.record(ctx, tenantID, actorID, projectID, audit.ActionAuthzDenied, audit.ResultDenied,
			"transfer: project owner required")
		return ErrForbidden
	}

	if toUserID == "" {
		return fmt.Errorf("%w: не указан новый владелец", ErrConflict)
	}
	if toUserID == actorID {
		// Передать самому себе — не ошибка прав, а пустое действие: владелец
		// уже этот человек.
		s.record(ctx, tenantID, actorID, projectID, audit.ActionAuthzDenied, audit.ResultDenied,
			"transfer: to self")
		return ErrConflict
	}

	if err := s.repo.TransferOwnership(ctx, tenantID, projectID, toUserID); err != nil {
		s.record(ctx, tenantID, actorID, projectID, audit.ActionProjectTransferred, audit.ResultFailed,
			"transfer failed: "+err.Error())
		return err
	}
	s.record(ctx, tenantID, actorID, projectID, audit.ActionProjectTransferred, audit.ResultOK,
		"ownership to "+toUserID)
	return nil
}

// member возвращает членство вызывающего в проекте и признак наличия.
func (s *Service) member(ctx context.Context, tenantID, userID, projectID string) (*ProjectMember, bool, error) {
	m, err := s.repo.GetMember(ctx, tenantID, projectID, userID)
	if err == nil {
		return m, true, nil
	}
	if errors.Is(err, ErrNotFound) {
		return nil, false, nil
	}
	return nil, false, err
}

// IsMember возвращает true, если пользователь является членом проекта
// (EDR-0008). Публичный порт членства для кросс-сервисной авторизации
// (S-142): AI-ассистенты гейтят conversation-memory проверкой членства
// (IDOR-фикс S-141 №1). Чужой/несуществующий проект — (false, nil);
// сбой хранилища — (false, err).
func (s *Service) IsMember(ctx context.Context, tenantID, userID, projectID string) (bool, error) {
	_, ok, err := s.member(ctx, tenantID, userID, projectID)
	return ok, err
}

// AddComment добавляет комментарий к проекту (EDR-0009). Требуется
// членство (owner/editor/viewer). Возвращает созданный комментарий.
func (s *Service) AddComment(ctx context.Context, tenantID, userID, projectID, body string) (*Comment, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	if body == "" {
		return nil, fmt.Errorf("project: comment body is required")
	}
	c := &Comment{ProjectID: projectID, AuthorID: userID, Body: body}
	return s.repo.AddComment(ctx, tenantID, projectID, c)
}

// ListComments возвращает комментарии проекта (EDR-0009). Требуется членство.
func (s *Service) ListComments(ctx context.Context, tenantID, userID, projectID string) ([]*Comment, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	return s.repo.ListComments(ctx, tenantID, projectID)
}

// DeleteComment удаляет комментарий (EDR-0009). Удалять может автор
// комментария или владелец проекта; остальные — ErrForbidden (реализация
// Repository проверяет обе роли). Требуется членство.
func (s *Service) DeleteComment(ctx context.Context, tenantID, userID, projectID, commentID string) error {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return err
	} else if !ok {
		return ErrNotFound
	}
	return s.repo.DeleteComment(ctx, tenantID, projectID, commentID, userID)
}

// Calculate сохраняет конфигурацию и результат расчёта проекта (внутри
// tenant). Требуется роль owner или editor (право на изменение, EDR-0008).
// Проект в статусе in_review не принимает расчёт (EDR-0010): конфигурация
// заморожена до решения ревью → ErrConflict.
func (s *Service) Calculate(ctx context.Context, tenantID, userID, projectID string, cfg stair.Config, opts stair.Options) (*Calculation, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectEdit) {
		s.record(ctx, tenantID, userID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "calculate: editor required")
		return nil, ErrForbidden
	}

	p, err := s.repo.GetProject(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if p.Status == StatusInReview {
		return nil, fmt.Errorf("project: calculate while in review: %w", ErrConflict)
	}

	// Выполняем конвейер.
	res, err := s.calc.Calculate(ctx, cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("project: calculate: %w", err)
	}
	snap := NewSnapshot(projectID, res)

	calc, err := s.repo.SaveCalculationWithConfig(ctx, tenantID, toConfigEntity(projectID, cfg, opts), snap)
	if err == nil {
		s.record(ctx, tenantID, userID, projectID, audit.ActionProjectModified, audit.ResultOK, "configuration calculated")
	}
	return calc, err
}

// Preview рассчитывает конфигурацию без сохранения (EDR-0008, превью
// вариаций). Требуется членство с правом project.read. Результат тот же,
// что у Calculate, но расчёт и конфигурация НЕ попадают в репозиторий —
// пользователь перебирает альтернативы, не порождая сохранённых ревизий.
func (s *Service) Preview(ctx context.Context, tenantID, userID, projectID string, cfg stair.Config, opts stair.Options) (*Snapshot, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectRead) {
		s.record(ctx, tenantID, userID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "preview: reader required")
		return nil, ErrForbidden
	}
	// Выполняем конвейер без сохранения. Сериализация Snapshot в JSON
	// выполняется транспортным слоем (ADR-0006: encoding/json вне application).
	res, err := s.calc.Calculate(ctx, cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("project: preview: %w", err)
	}
	snap := NewSnapshot(projectID, res)
	return &snap, nil
}

// GetResult возвращает последний расчёт проекта внутри tenant. Требуется членство.
func (s *Service) GetResult(ctx context.Context, tenantID, userID, projectID string) (*Calculation, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	return s.repo.GetLatestCalculation(ctx, tenantID, projectID)
}

// ExportCAD возвращает полигональную сетку текущей (последней) сохранённой
// конфигурации проекта (EDR-0022 §3.4, ENG-GEO-0008). Сетка всегда
// пересчитывается детерминированно из параметрической модели. Требуется
// членство с правом project.read.
func (s *Service) ExportCAD(ctx context.Context, tenantID, userID, projectID string) (*kerngeo.Mesh, error) {
	// DOM-003: сетка лестницы без перил (совместимость с прежним поведением).
	// Новым вызывающим следует использовать ExportCADWithRailings.
	mesh, _, err := s.ExportCADWithRailings(ctx, tenantID, userID, projectID)
	return mesh, err
}

// ExportCADWithRailings возвращает сетку лестницы И сетку перил.
//
// DOM-003 (2026-09-26): ExportCAD отдавал только res.Mesh, а перила
// строятся отдельным телом (RailingMesh, CONF-RAILING) и в основной меш не
// входят. В результате экспортируемые DXF/STL/SVG НЕ СОДЕРЖАЛИ ПЕРИЛ ВООБЩЕ,
// хотя пользователь их задавал и видел в 3D. Отдельный метод нужен, чтобы не
// ломать существующую сигнатуру ExportCAD (её использует порт ProjectService).
func (s *Service) ExportCADWithRailings(ctx context.Context, tenantID, userID, projectID string) (*kerngeo.Mesh, *kerngeo.Mesh, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectRead) {
		return nil, nil, ErrForbidden
	}

	sc, err := s.repo.GetLatestConfiguration(ctx, tenantID, projectID)
	if err != nil {
		return nil, nil, err
	}
	if sc == nil {
		return nil, nil, ErrNotFound
	}
	// DOM-003: fromConfigEntity теперь восстанавливает тип поворота, стороны
	// перил, направления и материал, поэтому экспорт совпадает с тем, что
	// пользователь рассчитал и утвердил.
	cfg, opts, err := fromConfigEntity(sc)
	if err != nil {
		return nil, nil, err
	}
	res, err := s.calc.Calculate(ctx, cfg, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("project: export cad: %w", err)
	}
	if res.Mesh == nil {
		return nil, nil, fmt.Errorf("project: export cad: %w", ErrConflict)
	}
	s.record(ctx, tenantID, userID, projectID, audit.ActionProjectModified, audit.ResultOK, "cad export")
	// DOM-003: перила возвращаются отдельным мешем; при railing=none он
	// пустой (не nil), чтобы транспорт не путал «нет перил» с «ошибка».
	return res.Mesh, res.RailingMesh, nil
}

// ExportCutting возвращает производственный пакет последней сохранённой
// конфигурации — всё, что нужно для документов производства: раскрой на
// листах, спецификацию, положение каждой детали в сборке.
//
// Отдельный метод, а не перегрузка ExportCADWithRailings, потому что у
// документа реза другой вход: не меш, а карта раскроя. Меш не знает ни
// номера детали, ни отверстий, ни места детали на листе — то есть ровно того,
// из чего состоит документ реза.
//
// Требуется членство проекта и право чтения, как у остальных выгрузок.
func (s *Service) ExportCutting(ctx context.Context, tenantID, userID, projectID string) (*dommfg.ManufacturingPackage, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectRead) {
		return nil, ErrForbidden
	}
	sc, err := s.repo.GetLatestConfiguration(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if sc == nil {
		return nil, ErrNotFound
	}
	cfg, opts, err := fromConfigEntity(sc)
	if err != nil {
		return nil, err
	}
	res, err := s.calc.Calculate(ctx, cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("project: export cutting: %w", err)
	}
	if res.Package == nil {
		return nil, fmt.Errorf("project: export cutting: %w", ErrConflict)
	}
	s.record(ctx, tenantID, userID, projectID, audit.ActionProjectModified, audit.ResultOK, "cutting export")
	return res.Package, nil
}

// ExportAssembly — геометрия и производственный пакет для сборочного
// чертежа: меш (включая перила) плюс детали с их положением.
//
// Один метод, а не два вызова ExportCADWithRailings + ExportCutting:
// чертёж рисует метки на геометрии, и оба должны прийти из ОДНОГО расчёта.
// Два пересчёта дали бы две копии пакета, и метка могла бы встать на
// геометрию другого пересчёта — расхождение, которое выглядело бы как
// ошибка раскроя.
//
// Требуется членство проекта и право чтения, как у остальных выгрузок.
func (s *Service) ExportAssembly(ctx context.Context, tenantID, userID, projectID string) (*kerngeo.Mesh, *kerngeo.Mesh, *dommfg.ManufacturingPackage, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, nil, nil, err
	}
	if !ok {
		return nil, nil, nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectRead) {
		return nil, nil, nil, ErrForbidden
	}
	sc, err := s.repo.GetLatestConfiguration(ctx, tenantID, projectID)
	if err != nil {
		return nil, nil, nil, err
	}
	if sc == nil {
		return nil, nil, nil, ErrNotFound
	}
	cfg, opts, err := fromConfigEntity(sc)
	if err != nil {
		return nil, nil, nil, err
	}
	res, err := s.calc.Calculate(ctx, cfg, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("project: export assembly: %w", err)
	}
	if res.Mesh == nil || res.Package == nil {
		return nil, nil, nil, fmt.Errorf("project: export assembly: %w", ErrConflict)
	}
	// Слияние мешей — в транспорте через cad.Merge, как у ExportCADWithRailings:
	// сервис отдаёт геометрию как есть, а склейка для отрисовки — дело вывода.
	s.record(ctx, tenantID, userID, projectID, audit.ActionProjectModified, audit.ResultOK, "assembly export")
	return res.Mesh, res.RailingMesh, res.Package, nil
}

// GetLatestConfig возвращает текущую (или последнюю) сохранённую
// конфигурацию проекта внутри tenant. Требуется членство.
func (s *Service) GetLatestConfig(ctx context.Context, tenantID, userID, projectID string) (*StairConfiguration, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	return s.repo.GetLatestConfiguration(ctx, tenantID, projectID)
}

// ListConfigurations возвращает историю ревизий конфигурации проекта
// (EDR-0012, Versioning) по возрастанию номера ревизии. Требуется членство.
func (s *Service) ListConfigurations(ctx context.Context, tenantID, userID, projectID string) ([]*StairConfiguration, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	return s.repo.ListConfigurations(ctx, tenantID, projectID)
}

// GetConfiguration возвращает ревизию конфигурации по ID (EDR-0012).
// Требуется членство; чужая/несуществующая ревизия — ErrNotFound.
func (s *Service) GetConfiguration(ctx context.Context, tenantID, userID, projectID, configurationID string) (*StairConfiguration, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	return s.repo.GetConfigurationByID(ctx, tenantID, projectID, configurationID)
}

// HasConfigAccess возвращает true, если пользователь является членом
// проекта, которому принадлежит конфигурация (S-132c). Используется
// WS-авторизацией: подписка на pipeline:<configID> без членства отклоняется.
func (s *Service) HasConfigAccess(ctx context.Context, userID, configurationID string) (bool, error) {
	if userID == "" || configurationID == "" {
		return false, nil
	}
	return s.repo.HasConfigAccess(ctx, userID, configurationID)
}

// RestoreConfiguration делает ревизию конфигурации текущей (EDR-0012,
// DB-0006 Recovery): прежняя версия снова становится рабочей. Требуется
// роль с правом project.edit; не-член — ErrNotFound.
func (s *Service) RestoreConfiguration(ctx context.Context, tenantID, userID, projectID, configurationID string) (*StairConfiguration, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectEdit) {
		s.record(ctx, tenantID, userID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "restore configuration: editor required")
		return nil, ErrForbidden
	}
	cfg, err := s.repo.GetConfigurationByID(ctx, tenantID, projectID, configurationID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.RestoreConfiguration(ctx, tenantID, projectID, configurationID); err != nil {
		return nil, err
	}
	s.record(ctx, tenantID, userID, projectID, audit.ActionConfigRestored, audit.ResultOK, "revision="+cfg.ID)
	return cfg, nil
}

// MarkConfigurationAsMeasurement помечает ревизию как снятую по факту на
// объекте (решение владельца 2026-10-08). Те же права, что у восстановления
// ревизии: owner/editor — те, кто вносит размеры.
//
// Метка нужна, чтобы в истории проекта было видно, какие размеры пришли от
// клиента по телефону, а какие — с замера. Разница между «примерно 900» и
// «ровно 937» меняет число ступеней и цену, а по дате ревизии не читается.
func (s *Service) MarkConfigurationAsMeasurement(ctx context.Context, tenantID, userID, projectID, configurationID string, mark bool) (*StairConfiguration, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectEdit) {
		s.record(ctx, tenantID, userID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "mark configuration as measurement: editor required")
		return nil, ErrForbidden
	}
	if err := s.repo.MarkConfigurationAsMeasurement(ctx, tenantID, projectID, configurationID, mark); err != nil {
		return nil, err
	}
	cfg, err := s.repo.GetConfigurationByID(ctx, tenantID, projectID, configurationID)
	if err != nil {
		return nil, err
	}
	detail := "revision=" + configurationID + " marked=" + strconv.FormatBool(mark)
	s.record(ctx, tenantID, userID, projectID, audit.ActionConfigRestored, audit.ResultOK, detail)
	return cfg, nil
}

// RequestReview запрашивает ревью проекта (EDR-0010): переводит проект
// draft|changes_requested → in_review. Требуется роль owner/editor
// (право project.edit); повторный запрос из in_review — ErrConflict (реализация
// Repository). Не-член — ErrNotFound.
func (s *Service) RequestReview(ctx context.Context, tenantID, userID, projectID, comment string) (*ProjectReview, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectEdit) {
		s.record(ctx, tenantID, userID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "request review: editor required")
		return nil, ErrForbidden
	}
	rv, err := s.repo.RequestReview(ctx, tenantID, projectID, userID, comment)
	if err == nil {
		s.record(ctx, tenantID, userID, projectID, audit.ActionReviewRequested, audit.ResultOK, "review requested")
	}
	return rv, err
}

// SignOffReview подписывает ревью проекта (EDR-0010): переводит проект
// in_review → approved. Требуется роль owner; автор запроса не может
// подписать собственное ревью (реализация Repository). Не-член —
// ErrNotFound.
func (s *Service) SignOffReview(ctx context.Context, tenantID, userID, projectID, reviewID, comment string) (*ProjectReview, error) {
	return s.decideReview(ctx, tenantID, userID, projectID, reviewID, comment, ReviewApproved)
}

// RequestChanges возвращает проект на доработку (EDR-0010): переводит
// in_review → changes_requested. Требуется роль owner; автор запроса не
// может вернуть собственное ревью. Не-член — ErrNotFound.
func (s *Service) RequestChanges(ctx context.Context, tenantID, userID, projectID, reviewID, comment string) (*ProjectReview, error) {
	return s.decideReview(ctx, tenantID, userID, projectID, reviewID, comment, ReviewChangesRequest)
}

// decideReview — общая логика решения по ревью (sign-off / request changes).
func (s *Service) decideReview(ctx context.Context, tenantID, userID, projectID, reviewID, comment, decision string) (*ProjectReview, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectManage) {
		s.record(ctx, tenantID, userID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "decide review: owner required")
		return nil, ErrForbidden
	}
	rv, err := s.repo.DecideReview(ctx, tenantID, projectID, reviewID, userID, decision, comment)
	if err == nil {
		var action audit.Action
		switch decision {
		case ReviewApproved:
			action = audit.ActionReviewSigned
		case ReviewChangesRequest:
			action = audit.ActionReviewChanges
		default:
			action = audit.ActionReviewRequested
		}
		s.record(ctx, tenantID, userID, projectID, action, audit.ResultOK, "decision="+decision)
	}
	return rv, err
}

// ListReviews возвращает историю ревью проекта (EDR-0010). Требуется членство.
func (s *Service) ListReviews(ctx context.Context, tenantID, userID, projectID string) ([]*ProjectReview, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	return s.repo.ListReviews(ctx, tenantID, projectID)
}

// ApproveConfiguration утверждает ревизию конфигурации (EDR-0011).
// Требуется роль owner; утверждаемая конфигурация должна принадлежать
// проекту внутри tenant (проверка в Repository). Не-член — ErrNotFound;
// editor/viewer — ErrForbidden.
func (s *Service) ApproveConfiguration(ctx context.Context, tenantID, userID, projectID, configurationID, comment string) (*ConfigurationApproval, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectManage) {
		s.record(ctx, tenantID, userID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "approve configuration: owner required")
		return nil, ErrForbidden
	}
	ap, err := s.repo.ApproveConfiguration(ctx, tenantID, projectID, configurationID, userID, comment)
	if err == nil {
		s.record(ctx, tenantID, userID, projectID, audit.ActionConfigApproved, audit.ResultOK, "configuration="+configurationID)
	}
	return ap, err
}

// GetConfigurationApproval возвращает утверждение ревизии (EDR-0011).
// Требуется членство. ErrNotFound — ревизия вне tenant или не утверждена.
func (s *Service) GetConfigurationApproval(ctx context.Context, tenantID, userID, projectID, configurationID string) (*ConfigurationApproval, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	return s.repo.GetConfigurationApproval(ctx, tenantID, projectID, configurationID)
}

// ListApprovals возвращает историю утверждений проекта (EDR-0011).
// Требуется членство.
func (s *Service) ListApprovals(ctx context.Context, tenantID, userID, projectID string) ([]*ConfigurationApproval, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	return s.repo.ListApprovals(ctx, tenantID, projectID)
}
