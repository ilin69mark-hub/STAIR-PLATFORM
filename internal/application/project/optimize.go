package project

import (
	"context"
	"fmt"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/stair"
)

// OptimizeOutcome — итог оптимизации конфигурации проекта (EDR-0032).
type OptimizeOutcome struct {
	Result      *stair.OptimizeResult // результат поиска оптимума
	Calculation *Calculation          // сохранённый расчёт лучшей конфигурации (nil, если не найдена)
}

// Optimize находит оптимальную конфигурацию проекта (EDR-0032) и, если она
// существует, сохраняет её вместе с расчётом (аналогично Calculate). Для
// сохранённой конфигурации фиксируется шаг комфорта лучшего кандидата.
// Требуется роль owner/editor (право на изменение, EDR-0008); проект в
// статусе in_review не принимает оптимизацию (EDR-0010).
func (s *Service) Optimize(ctx context.Context, tenantID, userID, projectID string, cfg stair.Config, opts stair.Options, oreq stair.OptimizeRequest) (*OptimizeOutcome, error) {
	me, ok, err := s.member(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if !me.Role.HasPermission(PermissionProjectEdit) {
		s.record(ctx, tenantID, userID, projectID, audit.ActionAuthzDenied, audit.ResultDenied, "optimize: editor required")
		return nil, ErrForbidden
	}

	p, err := s.repo.GetProject(ctx, tenantID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if p.Status == StatusInReview {
		return nil, fmt.Errorf("project: optimize while in review: %w", ErrConflict)
	}

	out, err := s.calc.Optimize(ctx, cfg, opts, oreq)
	if err != nil {
		return nil, fmt.Errorf("project: optimize: %w", err)
	}
	if !out.Valid {
		return &OptimizeOutcome{Result: out}, nil
	}

	snap := NewSnapshot(projectID, out.BestResult)
	savedOpts := opts
	if out.ComfortStep > 0 {
		savedOpts.ComfortStep = out.ComfortStep
	}
	calc, err := s.repo.SaveCalculationWithConfig(ctx, tenantID, toConfigEntity(projectID, out.BestConfig, savedOpts), snap)
	if err == nil {
		s.record(ctx, tenantID, userID, projectID, audit.ActionProjectModified, audit.ResultOK, "configuration optimized")
	}
	return &OptimizeOutcome{Result: out, Calculation: calc}, err
}
