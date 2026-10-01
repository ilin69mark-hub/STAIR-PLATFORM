package stair

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/optimization"
)

// OptimizeTarget — целевая метрика оптимизации (меньше = лучше).
type OptimizeTarget string

const (
	// TargetPrice — итоговая цена (FinalPrice).
	TargetPrice OptimizeTarget = "price"
	// TargetCost — себестоимость (ProductionCost).
	TargetCost OptimizeTarget = "cost"
	// TargetMaterial — стоимость материала (Material).
	TargetMaterial OptimizeTarget = "material"
)

// Valid проверяет известность целевой метрики.
func (t OptimizeTarget) Valid() bool {
	switch t {
	case TargetPrice, TargetCost, TargetMaterial:
		return true
	}
	return false
}

// OptimizeRequest — параметры поиска оптимума (EDR-0032). Нулевые границы
// выводятся автоматически из высоты подъёма и нормативных ограничений.
//
// SEC-003 (2026-09-26): границы, приходящие от клиента, больше не
// подставляются «как есть». Раньше `step_count_max: 2000000000`,
// `comfort_step_min_mm: 600`, `comfort_step_max_mm: 640`,
// `comfort_step_grid_mm: 1e-7` давали ~10^25 кандидатов, каждый из которых
// прогонял полный конвейер (солвер → геометрия → тесселяция → производство
// → раскрой → цена). Один HTTP-запрос занимал ядро CPU все 60 секунд
// таймаута маршрута (воспроизведено: 60.00 s → 499). Теперь действуют
// потолки MaxStepCountSpan / MaxComfortPoints / MaxCandidates (см. ниже),
// а пересечение с ними — ошибка ввода 422, а не тихое усечение.
type OptimizeRequest struct {
	Target          OptimizeTarget // по умолчанию TargetPrice
	Maximize        bool
	StepCountMin    int
	StepCountMax    int
	ComfortStepMin  float64
	ComfortStepMax  float64
	ComfortStepGrid float64
}

// Потолки пространства поиска (SEC-003).
//
// Обоснование значений. Нормативное окно само по себе узкое: при
// GEO-STEP-HEIGHT 150–200 мм и H ≤ 6000 мм (maxSupportedHeightMM) число
// ступеней n удовлетворяет 150·n ≤ H ≤ 200·n, то есть n ∈ [H/200, H/150] —
// максимум 40 значений для предельной высоты. Шаг комфорта S ∈ [600,640] с
// сеткой 1 мм даёт 41 точку. Прямой марш: 40 × 41 = 1640 кандидатов.
//
// Для L/U маршей добавляется перебор n1 ∈ [1, n−1], то есть ��множитель
// до n. При n = 40 это 1640 × 40 = 65 600 — уже многовато для синхронного
// HTTP-ответа при полном конвейере на кандидата. Поэтому потолки заданы с
// запасом на L/U и одновременно как абсолютный предел:
//
//	MaxStepCountSpan = 48   — ступеней в окне (нормативное окно ≤ 40);
//	MaxComfortPoints = 48   — точек сетки по шагу комфорта;
//	MaxLowerStepSpan = 48   — значений n1 (нормативное ≤ 39);
//	MaxCandidates     = 16384 — итоговый потолок кандидатов.
//
// MaxCandidates — последний рубеж: даже если все три диапазона в допустимых
// пределах, их произведение не должно превышать потолок. Обоснование: после
// устранения вариаций в переборе (Options.skipVariations) один кандидат
// стоит ~0.7 мс, поэтому 16 384 кандидата — это ~11 с, что укладывается в
// таймаут маршрута (60 с) с запасом и остаётся осмысленным полным поиском.
//
// Проверка, что НОРМАТИВНЫЙ (авто) запрос не отвергается: худший случай —
// H = 6000 мм, L/U-марш. Окно ступеней ceil(6000/200)=30 .. floor(6000/150)=40
// (11 значений), n1 ∈ [1,39] (39), сетка комфорта 600..640 с шагом 1 мм
// (41 точка) — всего 17 589 кандидатов. Это БОЛЬШЕ MaxCandidates, поэтому
// предельный авто-запрос для высокой L/U-лестницы честно отвергается с
// указанием, как сузить диапазон, вместо того чтобы висеть 60 секунд.
// Типичные высоты (≤ 3200 мм) дают ≤ 6 174 кандидатов и проходят.
const (
	MaxStepCountSpan = 48
	MaxComfortPoints = 48
	MaxLowerStepSpan = 48
	MaxCandidates    = 16384

	// MinComfortStepGrid — минимальный шаг сетки шага комфорта, мм.
	//
	// Равен 1 мм не произвольно: при диапазоне 600–640 это 41 точка, что
	// укладывается в MaxComfortPoints. Мельче сетка бессмысленна (float64
	// не различает 640.0000001 и 640.0000002 после 12 знаков) и стоит
	// десятки миллионов лишних итераций.
	MinComfortStepGrid = 1.0
)

// ErrSearchSpaceTooLarge — запрошенное пространство поиска превышает
// потолки SEC-003. Прикладной слой возвращает его как ошибку ввода, а
// транспорт отдаёт 422.
var ErrSearchSpaceTooLarge = errors.New("stair: optimization search space exceeds limit")

// searchSpaceError — описание превышения с указанием затронутой границы,
// чтобы клиент видел, что именно исправить.
func searchSpaceError(param string, got, limit int) error {
	return fmt.Errorf("%w: %s=%d exceeds limit %d", ErrSearchSpaceTooLarge, param, got, limit)
}

// comfortPoints — число точек сетки по шагу комфорта. Вырожденный диапазон
// (sMax ≤ sMin, в т.ч. спираль, где S не является свободным параметром)
// даёт одну точку — ровно как в optimization.Search.
func comfortPoints(sMin, sMax, sGrid float64) int {
	if sMax <= sMin {
		return 1
	}
	pts := int(math.Floor((sMax-sMin)/sGrid)) + 1
	if pts < 1 {
		return 1
	}
	return pts
}

// OptimizeResult — итог поиска оптимума (EDR-0032 §3.4).
type OptimizeResult struct {
	Valid       bool
	Evaluated   int
	Target      OptimizeTarget
	Objective   float64 // значение цели лучшего кандидата (мажорные единицы валюты)
	BestConfig  Config
	BestResult  *Result // полный расчёт лучшей конфигурации
	ComfortStep float64 // шаг комфорта лучшего кандидата, мм
}

// Optimize выполняет детерминированный поиск оптимальной конфигурации марша
// (EDR-0032): перебор числа ступеней (и разбивки для L/U-маршей) по сетке
// шага комфорта; лучшим считается валидный кандидат с минимальным значением
// цели (цена/себестоимость/материал). Оценщик — существующий конвейер
// Calculate. Результат детерминирован (ADR-0003). Контекст отменяется
// между оценками (B2, EDR-0033 §3.1).
func (s *Service) Optimize(ctx context.Context, cfg Config, opts Options, req OptimizeRequest) (*OptimizeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()
	out, err := s.optimize(ctx, cfg, opts, req)
	optimizeDuration.With(flightLabel(cfg.Flight), string(outTarget(req.Target))).Observe(time.Since(start).Seconds())
	return out, err
}

func (s *Service) optimize(ctx context.Context, cfg Config, opts Options, req OptimizeRequest) (*OptimizeResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: optimize: %w", err)
	}
	if req.Target == "" {
		req.Target = TargetPrice
	}
	if !req.Target.Valid() {
		return nil, fmt.Errorf("stair: unknown optimization target %q", req.Target)
	}
	if _, err := buildConfiguration(cfg); err != nil {
		return nil, err
	}

	hm := cfg.Height.Millimeters()
	// Нормативный диапазон высоты ступени 150–200 мм (EDR-0002).
	nMin := int(math.Ceil(hm / 200.0))
	nMax := int(math.Floor(hm / 150.0))
	if nMin < 1 {
		nMin = 1
	}
	// SEC-003: нормативное окно — истина, клиент его только УТОЧНЯЕТ внутри.
	// Отсекаем запрос, выходящий за потолок, ДО подстановки, чтобы
	// чудовищное значение не успело стать границей перебора.
	if req.StepCountMin > 0 {
		if req.StepCountMin < nMin {
			// Клиент сузил окно ниже нормы — берём его значение, если оно
			// не выходит за потолок абсолютного окна снизу.
			if req.StepCountMin < 1 {
				return nil, searchSpaceError("step_count_min", req.StepCountMin, 1)
			}
		}
		nMin = req.StepCountMin
	}
	if req.StepCountMax > 0 {
		nMax = req.StepCountMax
	}
	// Абсолютный потолок по ширине окна: клиент не может запросить
	// перебор шире, чем допускает maxSupportedHeightMM.
	if nMax-nMin+1 > MaxStepCountSpan {
		return nil, searchSpaceError("step_count window", nMax-nMin+1, MaxStepCountSpan)
	}
	// Спиральный марш: шаг проступи определяется радиусом, а не шагом
	// комфорта (EDR-0007 §4). Расширяем диапазон радиусным ограничением
	// bWalk ∈ [260, 320] (2π·(R−W/3)/n), чтобы не пропустить допустимые n.
	if cfg.Flight == engineering.FlightSpiral {
		rw := cfg.OuterRadius.Millimeters() - cfg.Width.Millimeters()/3.0
		if rw > 0 {
			nLo := int(math.Floor(2 * math.Pi * rw / 320.0))
			nHi := int(math.Ceil(2 * math.Pi * rw / 260.0))
			if nLo > 0 && nLo < nMin {
				nMin = nLo
			}
			if nHi > nMax {
				nMax = nHi
			}
		}
	}
	if nMin > nMax {
		return &OptimizeResult{Valid: false, Target: req.Target}, nil
	}

	// Шаг комфорта [600, 640] (EDR-0001); спиральный марш не имеет его как
	// свободного параметра — диапазон вырождается в одну точку.
	sMin, sMax, sGrid := 600.0, 640.0, 2.0
	if cfg.Flight == engineering.FlightSpiral {
		sMin, sMax = 0, 0
	}
	if req.ComfortStepMin > 0 {
		sMin = req.ComfortStepMin
	}
	if req.ComfortStepMax > 0 {
		sMax = req.ComfortStepMax
	}
	if req.ComfortStepGrid > 0 {
		sGrid = req.ComfortStepGrid
	}
	// SEC-003: сетка шага комфорта не может быть мельче MinComfortStepGrid.
	if sGrid < MinComfortStepGrid {
		return nil, searchSpaceError("comfort_step_grid_mm", int(sGrid*1000), int(MinComfortStepGrid*1000))
	}
	// SEC-003: и число точек сетки ограничено. Проверка обязательна: иначе
	// широкий диапазон с мелким шагом (600..640 шагом 0.5 мм = 81 точка)
	// прошёл бы, хотя MaxComfortPoints объявлен.
	if pts := comfortPoints(sMin, sMax, sGrid); pts > MaxComfortPoints {
		return nil, searchSpaceError("comfort grid points", pts, MaxComfortPoints)
	}

	// Диапазон ступеней нижнего марша: [1, n-1] для L/U; вырожден иначе.
	n1Min, n1Max := 1, 1
	if cfg.Flight == engineering.FlightLShape || cfg.Flight == engineering.FlightUShape {
		n1Max = nMax - 1
		if n1Max < 1 {
			n1Max = 1
		}
		// SEC-003: перебор n1 умножает пространство на n — ограничиваем.
		if n1Max-n1Min+1 > MaxLowerStepSpan {
			n1Max = n1Min + MaxLowerStepSpan - 1
		}
	}
	// SEC-003: абсолютный потолок кандидатов — последний рубеж.
	if cand := (nMax - nMin + 1) * (n1Max - n1Min + 1) * comfortPoints(sMin, sMax, sGrid); cand > MaxCandidates {
		return nil, searchSpaceError("candidate count", cand, MaxCandidates)
	}

	// Оценщик: кандидат → конфигурация → конвейер → цель.
	eval := func(k optimization.Candidate) (bool, optimization.Objective) {
		cand := cfg
		cand.StepHeight = engineering.Length(hm / float64(k.StepCount))
		cand.LowerStepCount = k.LowerStepCount
		o := opts
		if k.ComfortStep > 0 {
			o.ComfortStep = k.ComfortStep
		}
		// SEC-003/PERF: ранжированию вариации не нужны — оценщик смотрит
		// только на Blocking и Price. Без этого каждый невписывающийся
		// кандидат стоил 138 мс вместо 0.6 мс.
		o.skipVariations = true
		res, err := s.Calculate(ctx, cand, o)
		if err != nil || res.Validation.Blocking || res.Price == nil {
			return false, 0
		}
		return true, optimization.Objective(objectiveValue(res, req.Target))
	}

	r := optimization.Search(ctx, eval, optimization.Options{
		StepCountMin: nMin,
		StepCountMax: nMax,
		LowerStepMin: n1Min,
		LowerStepMax: n1Max,
		ComfortMin:   sMin,
		ComfortMax:   sMax,
		ComfortStep:  sGrid,
		Maximize:     req.Maximize,
	})
	if r.Cancelled {
		return nil, fmt.Errorf("stair: optimize: %w", ctx.Err())
	}

	out := &OptimizeResult{Valid: r.Valid, Evaluated: r.Evaluated, Target: req.Target}
	if !r.Valid {
		return out, nil
	}

	// Полный расчёт лучшего кандидата (повторный детерминированный прогон).
	best := cfg
	best.StepHeight = engineering.Length(hm / float64(r.Best.StepCount))
	best.LowerStepCount = r.Best.LowerStepCount
	bo := opts
	if r.Best.ComfortStep > 0 {
		bo.ComfortStep = r.Best.ComfortStep
	}
	br, err := s.Calculate(ctx, best, bo)
	if err != nil {
		return nil, fmt.Errorf("stair: optimize best: %w", err)
	}
	out.BestConfig = best
	out.BestResult = br
	out.Objective = objectiveValue(br, req.Target)
	out.ComfortStep = r.Best.ComfortStep
	return out, nil
}

// objectiveValue возвращает целевую метрику результата в мажорных единицах
// валюты (для ранжирования и вывода).
func objectiveValue(res *Result, t OptimizeTarget) float64 {
	if res.Price == nil {
		return 0
	}
	cur := res.Price.Currency
	switch t {
	case TargetCost:
		return res.Price.ProductionCost.Major(cur)
	case TargetMaterial:
		return res.Price.Material.Major(cur)
	case TargetPrice:
		return res.Price.FinalPrice.Major(cur)
	default:
		return res.Price.FinalPrice.Major(cur)
	}
}

// outTarget нормализует целевую метрику для label метрики (пустое значение
// — цена по умолчанию).
func outTarget(t OptimizeTarget) OptimizeTarget {
	if t == "" {
		return TargetPrice
	}
	return t
}
