package assistant

import (
	"context"

	"stairplatform/internal/application/stair"
)

// stairCalculator — порт расчётных возможностей конвейера (обычно
// *stair.Service). Ассистенты не знают ни о БД, ни о доменах: «истина»
// остаётся в stair.Service (AI-0000, ADR-0006). Для тестов порт легко
// заменяется фейком без перезаписи всей логики экспертов.
type stairCalculator interface {
	// Calculate выполняет полный сквозной расчёт конфигурации.
	Calculate(ctx context.Context, cfg stair.Config, opts stair.Options) (*stair.Result, error)
	// Optimize находит детерминированный оптимум конфигурации по цели.
	Optimize(ctx context.Context, cfg stair.Config, opts stair.Options, req stair.OptimizeRequest) (*stair.OptimizeResult, error)
	// ValidateConfig осуществляет дешёвую валидацию конфигурации до расчёта.
	ValidateConfig(cfg stair.Config) error
}

// Tools — набор инструментов (Tool Calling), доступных экспертам.
// Каждая операция обязательно проходит через существующий конвейер;
// новых вычислительных путей AI не создаёт (AI-0000).
type Tools struct {
	calc stairCalculator
}

// Calculate — тул полного расчёта конфигурации.
func (t *Tools) Calculate(ctx context.Context, cfg stair.Config, opts stair.Options) (*stair.Result, error) {
	return t.calc.Calculate(ctx, cfg, opts)
}

// Optimize — тул поиска оптимальной конфигурации по целевой метрике.
func (t *Tools) Optimize(ctx context.Context, cfg stair.Config, opts stair.Options, req stair.OptimizeRequest) (*stair.OptimizeResult, error) {
	return t.calc.Optimize(ctx, cfg, opts, req)
}

// ValidateConfig — тул предварительной валидации входа.
func (t *Tools) ValidateConfig(cfg stair.Config) error {
	return t.calc.ValidateConfig(cfg)
}
