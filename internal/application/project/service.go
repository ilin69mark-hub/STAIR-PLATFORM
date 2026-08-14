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
	if !me.Role.CanEdit() {
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

// RequestReview запрашивает ревью проекта (EDR-0010): переводит проект
// draft|changes_requested → in_review. Требуется роль owner/editor
// (CanEdit); повторный запрос из in_review — ErrConflict (реализация
// Repository). Не-член — ErrNotFound.
func (s *Service) RequestReview(ctx context.Context, tenantID, userID, projectID, comment string) (*ProjectReview, error) {
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
	return s.repo.RequestReview(ctx, tenantID, projectID, userID, comment)
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
	if !me.Role.CanManage() {
		return nil, ErrForbidden
	}
	return s.repo.DecideReview(ctx, tenantID, projectID, reviewID, userID, decision, comment)
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
