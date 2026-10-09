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
	normalizePlateStringer(cfg, cfg.Length.Millimeters())
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

// normalizePlateStringer приводит стальной косоур к нормативу листового
// косоура: толщина берётся из таблицы по длине и ширине марша, ширина
// заготовки гребёнки поднимается до нормы.
//
// ПОЧЕМУ ЗДЕСЬ, А НЕ В ПРИКЛАДНОМ СЛОЕ. Норматив keyed на длину марша, а она
// появляется только после Solve+Apply. При этом Validate должен увидеть уже
// приведённую толщину: иначе клиент, приславший конструктивные 50 мм от
// деревянной модели, получил бы ложное «лист должен быть 3-8 мм» на
// собственное, только что вычисленное, значение.
func normalizePlateStringer(cfg *engineering.StairConfiguration, runs ...float64) {
	engineering.NormalizePlateStringer(cfg, runs)
}
