package project

import (
	"context"
	"errors"
	"fmt"

	"stairplatform/internal/application/stair"
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
}

// RuleSet — бизнес-ограничения проекта (MVP-08): допустимые статусы.
type RuleSet struct {
	CreateStatus string
}

// DefaultRules возвращает правила по умолчанию.
func DefaultRules() RuleSet {
	return RuleSet{CreateStatus: "draft"}
}

// NewService создаёт сервис проектов.
func NewService(repo Repository, calc *stair.Service, rules RuleSet) *Service {
	return &Service{repo: repo, calc: calc, rules: rules}
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
	if !me.Role.CanManage() {
		return ErrForbidden
	}
	return s.repo.AddMember(ctx, tenantID, projectID, &ProjectMember{ProjectID: projectID, UserID: userID, Role: role})
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
	if !me.Role.CanManage() {
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
	if !me.Role.CanManage() {
		return ErrForbidden
	}
	return s.repo.UpdateMemberRole(ctx, tenantID, projectID, userID, role)
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
	if !me.Role.CanManage() {
		return ErrForbidden
	}
	return s.repo.RemoveMember(ctx, tenantID, projectID, userID)
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

// Calculate сохраняет конфигурацию и результат расчёта проекта (внутри
// tenant). Требуется роль owner или editor (право на изменение, EDR-0008).
func (s *Service) Calculate(ctx context.Context, tenantID, userID, projectID string, cfg stair.Config, opts stair.Options) (*Calculation, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if !me.Role.CanEdit() {
		return nil, ErrForbidden
	}

	// Выполняем конвейер.
	res, err := s.calc.Calculate(cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("project: calculate: %w", err)
	}
	snap := NewSnapshot(projectID, res)

	return s.repo.SaveCalculationWithConfig(ctx, tenantID, toConfigEntity(projectID, cfg, opts), snap)
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

// GetLatestConfig возвращает последнюю сохранённую конфигурацию проекта
// внутри tenant. Требуется членство.
func (s *Service) GetLatestConfig(ctx context.Context, tenantID, userID, projectID string) (*StairConfiguration, error) {
	if _, ok, err := s.member(ctx, tenantID, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	return s.repo.GetLatestConfiguration(ctx, tenantID, projectID)
}
