package solver

import (
	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

// SolveChecked решает марш, применяет результат к конфигурации и валидирует
// его (EDR-0003 §3, DOM-0014: расчёт только после Validation).
// При blocking-ошибке (хотя бы один Error) результат в конфигурацию НЕ
// записывается; возвращается validation.Result для диагностики.
func SolveChecked(cfg *engineering.StairConfiguration, set *constraint.ConstraintSet, s ...float64) (FlightResult, validation.Result, error) {
	res, err := Solve(cfg.Height, cfg.StepHeight, s...)
	if err != nil {
		return FlightResult{}, validation.Result{}, err
	}
	res.Apply(cfg)
	vr := validation.Validate(cfg, set)
	if vr.Blocking {
		cfg.StepCount = 0
		cfg.StepHeight = 0
		cfg.TreadDepth = 0
		cfg.Length = 0
		cfg.StringerLength = 0
		cfg.Angle = 0
	}
	return res, vr, nil
}
