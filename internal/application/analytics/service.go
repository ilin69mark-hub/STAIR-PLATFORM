package analytics

import (
	"context"
	"fmt"
	"time"
)

// Service — прикладной сервис аналитики (BE-0002 Use Cases, EDR-0028).
// Read-only: читает агрегаты tenant из Repository, валидирует параметры.
// Не зависит от транспорта и БД (инверсия зависимостей).
type Service struct {
	repo Repository
}

// NewService создаёт сервис аналитики.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Usage возвращает Usage Analytics за окно [from, to] с гранулярностью g
// (EDR-0028 §3.2). from/to приведены к UTC; если to раньше from —
// ErrInvalidRange. Гранулярность проверяется каталогом.
func (s *Service) Usage(ctx context.Context, tenantID string, from, to time.Time, g Granularity) (*UsageReport, error) {
	if !g.Valid() {
		return nil, ErrInvalidGranularity
	}
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, ErrInvalidRange
	}
	from = from.UTC()
	to = to.UTC()

	totals, err := s.repo.UsageTotals(ctx, tenantID, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics: usage totals: %w", err)
	}
	series, err := s.repo.UsageSeries(ctx, tenantID, from, to, g)
	if err != nil {
		return nil, fmt.Errorf("analytics: usage series: %w", err)
	}
	return &UsageReport{
		From:        from,
		To:          to,
		Granularity: g,
		Totals:      totals,
		Series:      series,
	}, nil
}

// Projects возвращает Project Analytics: агрегаты tenant за окно
// [from, to] и сводку по каждому проекту (EDR-0029 §3.2). from/to
// приведены к UTC; если to раньше from — ErrInvalidRange.
func (s *Service) Projects(ctx context.Context, tenantID string, from, to time.Time) (*ProjectReport, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, ErrInvalidRange
	}
	from = from.UTC()
	to = to.UTC()

	totals, err := s.repo.ProjectTotals(ctx, tenantID, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics: project totals: %w", err)
	}
	rows, err := s.repo.ProjectList(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("analytics: project list: %w", err)
	}
	return &ProjectReport{
		From:     from,
		To:       to,
		Totals:   totals,
		Projects: rows,
	}, nil
}

// Manufacturing возвращает Manufacturing Analytics за окно [from, to] с
// гранулярностью g (EDR-0030 §3.2). from/to приведены к UTC; валидация
// диапазона и гранулярности — как в EDR-0028.
func (s *Service) Manufacturing(ctx context.Context, tenantID string, from, to time.Time, g Granularity) (*ManufacturingReport, error) {
	if !g.Valid() {
		return nil, ErrInvalidGranularity
	}
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, ErrInvalidRange
	}
	from = from.UTC()
	to = to.UTC()

	totals, err := s.repo.ManufacturingTotals(ctx, tenantID, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics: manufacturing totals: %w", err)
	}
	series, err := s.repo.ManufacturingSeries(ctx, tenantID, from, to, g)
	if err != nil {
		return nil, fmt.Errorf("analytics: manufacturing series: %w", err)
	}
	return &ManufacturingReport{
		From:        from,
		To:          to,
		Granularity: g,
		Totals:      totals,
		Series:      series,
	}, nil
}

// Cost возвращает Cost Analytics за окно [from, to] с гранулярностью g
// (EDR-0031 §3.2). from/to приведены к UTC; валидация диапазона и
// гранулярности — как в EDR-0028.
func (s *Service) Cost(ctx context.Context, tenantID string, from, to time.Time, g Granularity) (*CostReport, error) {
	if !g.Valid() {
		return nil, ErrInvalidGranularity
	}
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, ErrInvalidRange
	}
	from = from.UTC()
	to = to.UTC()

	totals, err := s.repo.CostTotals(ctx, tenantID, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics: cost totals: %w", err)
	}
	series, err := s.repo.CostSeries(ctx, tenantID, from, to, g)
	if err != nil {
		return nil, fmt.Errorf("analytics: cost series: %w", err)
	}
	return &CostReport{
		From:        from,
		To:          to,
		Granularity: g,
		Totals:      totals,
		Series:      series,
	}, nil
}
