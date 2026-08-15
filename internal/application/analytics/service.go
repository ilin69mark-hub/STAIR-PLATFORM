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
