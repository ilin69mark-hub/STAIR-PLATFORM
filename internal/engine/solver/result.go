// Package solver реализует Solver (EDR-0001) — математическую модель
// расчёта прямого лестничного марша. Единственный источник инженерных
// формул (ADR-0003). Формулы детерминированы; без промежуточных округлений.
package solver

import "stairplatform/internal/domain/engineering"

// FlightResult — результат расчёта марша (EDR-0001 §3, выходные значения).
type FlightResult struct {
	StepCount  int
	StepHeight engineering.Length
	TreadDepth engineering.Length
	Run        engineering.Length
	Stringer   engineering.Length
	Angle      engineering.Angle
}

// Apply записывает результат расчёта в параметрическую конфигурацию.
func (r FlightResult) Apply(cfg *engineering.StairConfiguration) {
	cfg.StepCount = r.StepCount
	cfg.StepHeight = r.StepHeight
	cfg.TreadDepth = r.TreadDepth
	cfg.Length = r.Run
	cfg.StringerLength = r.Stringer
	cfg.Angle = r.Angle
}
