// Package optimization реализует детерминированный поиск оптимума
// конфигурации лестницы (Phase B, B1, EDR-0032). Движок не выполняет
// инженерных расчётов: он перебирает дискретную сетку кандидатов и
// вызывает внешний оценщик (Evaluator), который возвращает валидность
// кандидата и значение целевой функции. Порядок обхода фиксирован
// (n → n1 → S), при равных значениях цели выигрывает первый встреченный
// кандидат — результат детерминирован (ADR-0003).
package optimization

import "context"

// Candidate — точка поиска: параметры разбивки марша.
type Candidate struct {
	StepCount      int     // n — число ступеней
	LowerStepCount int     // n1 — число ступеней нижнего марша (L/U)
	ComfortStep    float64 // S — шаг комфорта, мм (straight/L/U)
}

// Objective — значение целевой функции (меньше = лучше, если не Maximize).
type Objective float64

// Evaluator — детерминированная функция-оценщик кандидата: возвращает
// валидность конфигурации и значение цели. Должна быть идемпотентной и
// не иметь побочных эффектов.
type Evaluator func(c Candidate) (valid bool, value Objective)

// Options — границы поиска.
type Options struct {
	StepCountMin int
	StepCountMax int
	// LowerStepMin/LowerStepMax — диапазон числа ступеней нижнего марша.
	// Для маршей без площадки (straight/spiral) диапазон вырождается
	// (LowerStepMin == LowerStepMax) — движок всё равно пробегает его один раз.
	LowerStepMin int
	LowerStepMax int
	// ComfortMin/ComfortMax/ComfortStep — сетка шага комфорта (мм).
	// ComfortStep <= 0 → шаг 1 мм; ComfortMax <= ComfortMin → одна точка.
	ComfortMin  float64
	ComfortMax  float64
	ComfortStep float64
	// Maximize — true → ищем максимум цели, иначе минимум.
	Maximize bool
}

// Result — итог поиска.
type Result struct {
	Best      Candidate
	Value     Objective
	Valid     bool
	Evaluated int  // общее число оценок (включая невалидные)
	Cancelled bool // true — поиск прерван отменой контекста (B2, EDR-0033)
}

// Search выполняет детерминированный исчерпывающий поиск по сетке
// n × n1 × S. Возвращает первый (в порядке обхода) лучший валидный
// кандидат; при отсутствии допустимых кандидатов — Result{Valid: false}.
// Контекст проверяется между оценками: при отмене возвращается текущее
// накопленное состояние с Cancelled: true. Отмена не меняет результат при
// её отсутствии (детерминизм ADR-0003).
func Search(ctx context.Context, eval Evaluator, opt Options) Result {
	var out Result
	if ctx == nil {
		ctx = context.Background()
	}
	if eval == nil {
		return out
	}
	if opt.StepCountMax < opt.StepCountMin {
		return out
	}

	grid := opt.ComfortStep
	if grid <= 0 {
		grid = 1
	}
	comfortPts := comfortPoints(opt.ComfortMin, opt.ComfortMax, grid)

	n1Min := opt.LowerStepMin
	n1Max := opt.LowerStepMax
	if n1Max < n1Min {
		n1Max = n1Min
	}

	for n := opt.StepCountMin; n <= opt.StepCountMax; n++ {
		for n1 := n1Min; n1 <= n1Max; n1++ {
			for _, s := range comfortPts {
				if err := ctx.Err(); err != nil {
					out.Cancelled = true
					return out
				}
				cand := Candidate{StepCount: n, LowerStepCount: n1, ComfortStep: s}
				valid, v := eval(cand)
				out.Evaluated++
				if !valid {
					continue
				}
				if !out.Valid ||
					(opt.Maximize && v > out.Value) ||
					(!opt.Maximize && v < out.Value) {
					out.Best = cand
					out.Value = v
					out.Valid = true
				}
			}
		}
	}
	return out
}

// comfortPoints строит точки сетки [min, max] с шагом grid (max включён).
// При max <= min возвращает одну точку [min].
func comfortPoints(min, max, grid float64) []float64 {
	if max <= min {
		return []float64{min}
	}
	npts := int((max-min)/grid) + 1
	pts := make([]float64, 0, npts)
	for i := 0; i < npts; i++ {
		pts = append(pts, min+float64(i)*grid)
	}
	return pts
}
