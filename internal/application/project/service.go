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

// Service — прикладной сервис проектов (BE-0002 Use Cases): создание
// проекта, сохранение и расчёт конфигурации, получение результатов.
// Оркеструет stair.Service (движки) и Repository (данные).
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

// CreateProject создаёт проект с именем и описанием в tenant (BC-001).
func (s *Service) CreateProject(ctx context.Context, tenantID, name, description string) (*Project, error) {
	if name == "" {
		return nil, fmt.Errorf("project: name is required")
	}
	p := &Project{Name: name, Description: description, Status: s.rules.CreateStatus}
	if err := s.repo.CreateProject(ctx, tenantID, p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetProject возвращает проект по ID внутри tenant.
func (s *Service) GetProject(ctx context.Context, tenantID, id string) (*Project, error) {
	return s.repo.GetProject(ctx, tenantID, id)
}

// ListProjects возвращает проекты tenant'а.
func (s *Service) ListProjects(ctx context.Context, tenantID string) ([]*Project, error) {
	return s.repo.ListProjects(ctx, tenantID)
}

// Calculate сохраняет конфигурацию и результат расчёта проекта (внутри
// tenant). Конфигурация сериализуется из входных параметров (числа в мм);
// результат — снапшот конвейера (экспортный документ). Расчёт атомарно
// связывается с конфигурацией (одна транзакция).
func (s *Service) Calculate(ctx context.Context, tenantID, projectID string, cfg stair.Config, opts stair.Options) (*Calculation, error) {
	if _, err := s.repo.GetProject(ctx, tenantID, projectID); err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}

	// Выполняем конвейер.
	res, err := s.calc.Calculate(cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("project: calculate: %w", err)
	}
	snap := NewSnapshot(projectID, res)

	return s.repo.SaveCalculationWithConfig(ctx, tenantID, toConfigEntity(projectID, cfg, opts), snap)
}

// GetResult возвращает последний расчёт проекта внутри tenant.
func (s *Service) GetResult(ctx context.Context, tenantID, projectID string) (*Calculation, error) {
	return s.repo.GetLatestCalculation(ctx, tenantID, projectID)
}

// GetLatestConfig возвращает последнюю сохранённую конфигурацию проекта
// внутри tenant.
func (s *Service) GetLatestConfig(ctx context.Context, tenantID, projectID string) (*StairConfiguration, error) {
	return s.repo.GetLatestConfiguration(ctx, tenantID, projectID)
}
