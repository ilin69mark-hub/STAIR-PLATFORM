// Package stair реализует application layer лестницы (BC-002): единый
// оркестратор сквозного расчёта проекта. Мета-уровень (прикладные
// сервисы) зависят от доменов и движков, но не от транспорта, HTTP,
// БД или UI (ADR-0006, DOM-0008). Все результаты детерминированы.
package stair

import (
	"context"
	"errors"
	"fmt"
	"time"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	"stairplatform/internal/engine/advisor"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/geometry"
	engmfg "stairplatform/internal/engine/manufacturing"
	engprc "stairplatform/internal/engine/pricing"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
	"stairplatform/internal/engine/variation"
	kerngeo "stairplatform/internal/geometry"
)

// Config — пользовательские параметры лестницы (исходные, до Solver):
// высота подъёма, ширина, тип марша и целевые/производственные параметры.
type Config struct {
	Width             engineering.Length // мм — ширина марша
	Height            engineering.Length // мм — высота подъёма H
	Flight            engineering.FlightType
	StepHeight        engineering.Length // мм — целевая высота ступени h0
	StringerThickness engineering.Length // мм
	StepThickness     engineering.Length // мм
	// Riser — строить подступенки (вертикальные грани под проступями).
	// false — открытые ступени.
	Riser         bool
	Clearance     engineering.Length // мм
	RailingHeight engineering.Length // мм
	// LandingWidth и LowerStepCount — специфичны для маршей с площадкой
	// (EDR-0005 L-образный, EDR-0006 П-образный).
	LandingWidth engineering.Length // мм — ширина площадки Wp (платформа) либо просвета (поворот)
	LandingDepth engineering.Length // мм — глубина площадки (вдоль нижнего марша, X)
	RoomWidth    engineering.Length // мм — габарит помещения по X (для fit-check)
	RoomLength   engineering.Length // мм — габарит помещения по Y (для fit-check)
	// ApproachSpace — свободное пространство перед первой ступенью прямого
	// марша (EDR-0023, норма 1000–1200 мм): зона, в которой человек должен
	// встать перед началом подъёма. Учитывается в fit-check (марш сдвигается
	// от стены на ApproachSpace) и в подборе вариантов. Только прямой марш
	// (FlightStraight); для прочих типов не используется (0).
	ApproachSpace  engineering.Length
	LowerStepCount int // n1 — число ступеней нижнего марша
	// TurnKind — тип поворота для маршей с площадкой (L/U): площадка
	// (platform, по умолчанию) либо поворотные ступени (winder, только U).
	TurnKind engineering.TurnKind
	// WinderCount — число поворотных ступеней (только TurnWinder, U-образный).
	WinderCount int
	// OuterRadius — специфичен для спиральной лестницы (EDR-0007):
	// наружный радиус марша R (радиус колонны r = R − W).
	OuterRadius engineering.Length
	// Material — выбранный материал (код каталога MFG-0005); пустое
	// значение — автоназначение по толщине (текущая политика).
	Material dommfg.MaterialCode
	// Перила и направления (CONF-RAILING/DIRECTION/SPIRAL).
	Railing        engineering.RailingSide     // прямой марш
	RailingLower   engineering.RailingSide     // L/U: первый марш
	RailingLanding engineering.RailingSide     // L/U: площадка
	RailingUpper   engineering.RailingSide     // L/U: второй марш
	Direction      engineering.TurnDirection   // L/U: поворот площадки
	SpiralDir      engineering.SpiralDirection // спираль: закрутка
}

// Options — опциональные настройки расчёта; нулевое значение даёт дефолты.
type Options struct {
	// ComfortStep — шаг комфорта S (600–640); 0 → solver.DefaultComfortStep.
	ComfortStep float64
	// Rates — ставки цены; nil → engprc.DefaultRates().
	Rates *engprc.Rates
	// MachineRates — ставки машинных операций; nil → engmfg.DefaultMachineRates().
	MachineRates *engmfg.MachineRates
}

// Result — сквозной результат расчёта проекта (все этапы конвейера).
type Result struct {
	Validation     validation.Result
	Flight         solver.FlightResult  // прямой марш
	LShape         *solver.LShapeResult // L-образный марш (Flight == LShape)
	UShape         *solver.UShapeResult // П-образный марш (Flight == UShape)
	Spiral         *solver.SpiralResult // спиральный марш (Flight == Spiral)
	Measurement    geometry.Measurement
	GeometryIssues []kerngeo.ValidationIssue
	Mesh           *kerngeo.Mesh                // preview mesh для визуализации (ENG-GEO-0008)
	RailingMesh    *kerngeo.Mesh                // декоративные перила (отдельно, без каркаса в 3D)
	RoomMesh       *kerngeo.Mesh                // декоративный «пол комнаты» (отдельно, для 3D)
	Package        *dommfg.ManufacturingPackage // полные Parts/BOM/CutList/Nesting
	Cost           *dommfg.ManufacturingCostDataset
	Price          *domprc.PriceBreakdown
	// Эхо производственных параметров конфигурации (для 2D-рендера и
	// публичного ответа): толщина проступи, высота перил, наличие
	// подступенков (BC-002 — рендер рисует «как посчитано»).
	StepThickness     engineering.Length
	RailingHeight     engineering.Length
	Riser             bool
	StringerThickness engineering.Length
	// Эхо перил/направлений (CONF-RAILING/DIRECTION/SPIRAL): для спирали
	// Railing уже деривирован из направления закрутки.
	Railing        engineering.RailingSide
	RailingLower   engineering.RailingSide
	RailingLanding engineering.RailingSide
	RailingUpper   engineering.RailingSide
	Direction      engineering.TurnDirection
	SpiralDir      engineering.SpiralDirection
	// Эхо поворота (CONF-TURN-KIND): тип поворота и число поворотных ступеней.
	TurnKind    engineering.TurnKind
	WinderCount int
}

// Service — прикладной сервис расчёта лестницы. Является единственной
// точкой входа сквозного конвейера для любых транспортов.
type Service struct {
	constraints *constraint.ConstraintSet
}

// NewService создаёт сервис со стандартным профилем правил (EDR-0002).
func NewService() *Service {
	return &Service{
		constraints: constraint.StandardProfile("STANDARD"),
	}
}

// Calculate выполняет полный конвейер: Solver → Validation → Geometry →
// Manufacturing → Cost → Price (BC-002, PRC-0002). При blocking-валидации
// возвращает Result с заполненным Validation (не ошибка); ошибка —
// только при невозможности выполнить расчёт (некорректный вход, сбой или
// отмена контекста). Контекст проверяется между стадиями (B2, EDR-0033).
func (s *Service) Calculate(ctx context.Context, cfg Config, opts Options) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	flight := flightLabel(cfg.Flight)
	start := time.Now()
	res, err := s.calculate(ctx, cfg, opts)
	valid := "true"
	if res == nil || res.Validation.Blocking {
		valid = "false"
	}
	if err != nil && res == nil {
		valid = "false"
	}
	calculateDuration.With(flight, valid).Observe(time.Since(start).Seconds())
	return res, err
}

func (s *Service) calculate(ctx context.Context, cfg Config, opts Options) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	c, err := buildConfiguration(cfg)
	if err != nil {
		if vr, ok := inputIssue(err); ok {
			return &Result{Validation: vr}, nil
		}
		return nil, err
	}

	comfort := opts.ComfortStep
	if comfort == 0 {
		comfort = solver.DefaultComfortStep
	}

	res, err := s.checkedSolve(ctx, cfg, c, comfort)
	if err != nil {
		return nil, err
	}
	if res.Validation.Blocking {
		return res, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	gen, err := geometry.Generate(ctx, c)
	if err != nil {
		if vr, ok := inputIssue(err); ok {
			return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
		}
		return nil, fmt.Errorf("stair: geometry: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	pkg, err := engmfg.Manufacture(c, gen)
	if err != nil {
		var feas *engmfg.FeasibilityError
		if errors.As(err, &feas) {
			// Изготовление невозможно (MFG-0012): деталь не помещается на
			// стандартный лист. Возвращаем понятное blocking-сообщение вместо
			// технической ошибки — клиент показывает его в секции валидации.
			return &Result{Validation: manufacturingBlocked(feas, c.StringerThickness, dommfg.MaterialCode(c.Material))}, nil
		}
		if vr, ok := inputIssue(err); ok {
			return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
		}
		return nil, fmt.Errorf("stair: manufacturing: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	rates := opts.MachineRates
	if rates == nil {
		m := engmfg.DefaultMachineRates()
		rates = &m
	}
	ds, err := engmfg.PrepareCost(pkg, engmfg.DefaultMaterialRegistry(), *rates)
	if err != nil {
		return nil, fmt.Errorf("stair: cost: %w", err)
	}

	priceRates := engprc.DefaultRates()
	if opts.Rates != nil {
		priceRates = *opts.Rates
	}
	price, err := engprc.Price(ds, priceRates)
	if err != nil {
		return nil, fmt.Errorf("stair: pricing: %w", err)
	}

	res.Measurement = gen.Measurement
	res.GeometryIssues = gen.Issues
	// Невписываемость лестницы в периметр помещения (room_fit, неблокирующее
	// предупреждение) поднимается в раздел валидации вместе с готовыми
	// вариациями A/B/C (пакет variation), чтобы фронтенд мог показать
	// интерактивные варианты выбора, а не только текст предупреждения.
	for _, gi := range gen.Issues {
		if gi.Code == "room_fit" {
			rw := c.RoomWidth.Millimeters()
			rl := c.RoomLength.Millimeters()
			res.Validation.Issues = append(res.Validation.Issues, validation.Issue{
				ID:       "room_fit",
				Code:     "room_fit",
				Severity: constraint.SeverityWarning,
				Element:  "room",
				Message:  gi.Message,
				Param:    "Помещение",
				Guide: fmt.Sprintf(
					"Лестница не помещается в помещение %.0f×%.0f мм (ширина×длина). Выберите готовый вариант ниже — лестница сохранит остальные параметры.",
					rw, rl),
				Fix:        "Уменьшите габариты лестницы или выберите один из готовых вариантов.",
				Variations: variation.ForRoomFit(ctx, c, s.constraints),
			})
			break
		}
	}
	// Интерактивные вариации для остальных issue (угол наклона и готовые
	// Suggestions советника) — поверх room_fit, чтобы охватить и случай,
	// когда расчёт не заблокирован.
	attachVariations(ctx, &res.Validation, c, s.constraints)
	res.Mesh = gen.Mesh
	res.RailingMesh = gen.RailingMesh
	res.RoomMesh = gen.RoomMesh
	res.Package = pkg
	res.Cost = ds
	res.Price = price
	res.StepThickness = c.StepThickness
	res.RailingHeight = c.RailingHeight
	res.Riser = c.Riser
	res.StringerThickness = c.StringerThickness
	res.Railing = c.Railing
	res.RailingLower = c.RailingLower
	res.RailingLanding = c.RailingLanding
	res.RailingUpper = c.RailingUpper
	res.Direction = c.Direction
	res.SpiralDir = c.SpiralDirection
	res.TurnKind = c.TurnKind
	res.WinderCount = c.WinderCount
	return res, nil
}

// checkedSolve прогоняет конфигурацию через checked-решалку конкретного типа
// марша и советник (advisor.Advise + пакет variation): единый эталон
// валидации для Calculate и Validate. Возвращает Result, заполненный
// Validation (при blocking — только секция validation, как в Calculate) и
// результатом решалки для неблокирующего исхода. Геометрии/производства/
// цены здесь нет — вызов продолжается конвейером либо останавливается.
// advise прогоняет результат через советник (готовые Suggestions) и
// навешивает интерактивные Variations (A/B/C) на блокирующие issue:
// GEO-ANGLE — собственный подбор числа ступеней и шага комфорта,
// остальные — преобразование Suggestions советника. Вызывается и в
// блокирующей, и в неблокирующей ветках, поэтому варианты есть всегда.
func (s *Service) advise(ctx context.Context, cfg Config, c *engineering.StairConfiguration, comfort float64, vr validation.Result) validation.Result {
	// Вход советника собирается из исходной конфигурации: checked-функции
	// зануляют поля при blocking-валидации, а они нужны для подбора вариантов.
	advIn := advisor.Input{
		Flight:          cfg.Flight,
		HeightMm:        cfg.Height.Millimeters(),
		TargetStepMm:    cfg.StepHeight.Millimeters(),
		ComfortMm:       comfort,
		LowerStepCount:  cfg.LowerStepCount,
		TurnKind:        cfg.TurnKind,
		WinderCount:     cfg.WinderCount,
		LandingMm:       cfg.LandingWidth.Millimeters(),
		WidthMm:         cfg.Width.Millimeters(),
		OuterRadiusMm:   cfg.OuterRadius.Millimeters(),
		ClearanceMm:     cfg.Clearance.Millimeters(),
		RailingMm:       cfg.RailingHeight.Millimeters(),
		StringerThickMm: cfg.StringerThickness.Millimeters(),
		StepThicknessMm: cfg.StepThickness.Millimeters(),
		Material:        cfg.Material,
	}
	vrr := advisor.Advise(advIn, s.constraints, vr)
	attachVariations(ctx, &vrr, c, s.constraints)
	return vrr
}

func (s *Service) checkedSolve(ctx context.Context, cfg Config, c *engineering.StairConfiguration, comfort float64) (*Result, error) {
	res := &Result{}
	switch cfg.Flight {
	case engineering.FlightStraight:
		flight, vr, err := solver.SolveChecked(c, s.constraints, comfort)
		if err != nil {
			if vr, ok := inputIssue(err); ok {
				return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
			}
			return nil, err
		}
		if vr.Blocking {
			return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
		}
		res.Validation = vr
		res.Flight = flight
	case engineering.FlightLShape:
		lres, vr, err := solver.SolveCheckedLShape(c, s.constraints, comfort)
		if err != nil {
			if vr, ok := inputIssue(err); ok {
				return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
			}
			return nil, err
		}
		if vr.Blocking {
			return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
		}
		res.Validation = vr
		res.LShape = &lres
	case engineering.FlightUShape:
		ures, vr, err := solver.SolveCheckedUShape(c, s.constraints, comfort)
		if err != nil {
			if vr, ok := inputIssue(err); ok {
				return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
			}
			return nil, err
		}
		if vr.Blocking {
			return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
		}
		res.Validation = vr
		res.UShape = &ures
	case engineering.FlightSpiral:
		sres, vr, err := solver.SolveCheckedSpiral(c, s.constraints)
		if err != nil {
			if vr, ok := inputIssue(err); ok {
				return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
			}
			return nil, err
		}
		if vr.Blocking {
			return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
		}
		res.Validation = vr
		res.Spiral = &sres
	default:
		flight, vr, err := solver.SolveChecked(c, s.constraints, comfort)
		if err != nil {
			if vr, ok := inputIssue(err); ok {
				return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
			}
			return nil, err
		}
		if vr.Blocking {
			return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
		}
		res.Validation = vr
		res.Flight = flight
	}
	return res, nil
}

// Validate выполняет глубокую проверку конфигурации без полного конвейера:
// buildConfiguration → checked-решалка → советник. Не выполняет геометрию
// (твёрдотельные модели), производство, цену и не создаёт версий — только
// та секция, которую фронтенд показывает при расчёте. Предназначен для
// живой валидации при вводе (store и админ-конструктор, S-P5): результат
// повторяет блок validation ответа Calculate; blocking-issue приходят с
// готовыми Suggestions и Variations (A/B/C). Блокирующее состояние — это
// не ошибка: возвращается Result с valid:false (как в Calculate).
func (s *Service) Validate(ctx context.Context, cfg Config, opts Options) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	c, err := buildConfiguration(cfg)
	if err != nil {
		if vr, ok := inputIssue(err); ok {
			return &Result{Validation: vr}, nil
		}
		return nil, err
	}
	comfort := opts.ComfortStep
	if comfort == 0 {
		comfort = solver.DefaultComfortStep
	}
	res, err := s.checkedSolve(ctx, cfg, c, comfort)
	if err != nil {
		return nil, err
	}
	// В Calculate вариации для неблокирующего исхода навешивает стадия
	// geometry; здесь стадии нет — вешаем вариации самостоятельно.
	attachVariations(ctx, &res.Validation, c, s.constraints)
	return res, nil
}

// attachVariations навешивает интерактивные вариации (A/B/C) на issue
// результата валидации. Для угла наклона (GEO-ANGLE) варианты подбираются
// собственным перебором (variation.ForAngle), гарантированно даже когда
// советник не может подобрать их при фиксированной проступи. Для остальных
// issue готовые Suggestions советника превращаются в Variations через
// variation.FromSuggestions. Issue, уже имеющие Variations (напр. room_fit),
// не перезаписываются.
func attachVariations(ctx context.Context, vr *validation.Result, c *engineering.StairConfiguration, set *constraint.ConstraintSet) {
	for i := range vr.Issues {
		it := &vr.Issues[i]
		switch it.Code {
		case constraint.GEO_ANGLE:
			it.Variations = variation.ForAngle(ctx, c, set)
		case constraint.GEO_STEP_HEIGHT,
			constraint.GEO_TREAD_DEPTH,
			constraint.GEO_CLEARANCE,
			constraint.GEO_STRINGER_THICKNESS,
			constraint.SAF_RAILING_HEIGHT,
			constraint.MFG_SHEET,
			constraint.GEO_HEIGHT,
			constraint.GEO_WIDTH,
			constraint.GEO_COMFORT_STEP,
			constraint.GEO_LANDING_WIDTH,
			constraint.GEO_LOWER_STEP,
			constraint.GEO_SPIRAL_RADIUS,
			constraint.GEO_SPIRAL_TREAD,
			constraint.GEO_TREAD_POSITIVE,
			constraint.GEO_STRINGER_NARROW,
			constraint.MFG_MATERIAL:
			if len(it.Variations) == 0 && len(it.Suggestions) > 0 {
				it.Variations = variation.FromSuggestions(it, c)
			}
		default:
			if len(it.Variations) == 0 && len(it.Suggestions) > 0 {
				it.Variations = variation.FromSuggestions(it, c)
			}
		}
	}
}

// flightLabel нормализует тип марша в label метрики (пустое значение —
// прямой марш).
func flightLabel(f engineering.FlightType) string {
	if f == "" {
		return string(engineering.FlightStraight)
	}
	return string(f)
}

// manufacturingBlocked собирает blocking-результат для конфигурации,
// которую нельзя изготовить (MFG-0012): деталь не помещается на
// стандартный лист. Советник не имеет для неё вариантов — выпускаем
// только понятное описание причины.
func manufacturingBlocked(feas *engmfg.FeasibilityError, stringerThick engineering.Length, material dommfg.MaterialCode) validation.Result {
	maxLen, maxWid := 6000.0, 3000.0
	if material == "" {
		var err error
		material, err = engmfg.DefaultMaterialForThickness(stringerThick.Millimeters())
		if err != nil {
			material = dommfg.MaterialCode("STEEL-S235")
		}
	}
	if l, w, ok := engmfg.LargestStockSheet(engmfg.DefaultStockSheetRegistry(), material); ok {
		maxLen, maxWid = l, w
	}
	return validation.Result{
		Valid:    false,
		Blocking: true,
		Issues: []validation.Issue{{
			ID:       "ISSUE-MFG",
			Code:     constraint.MFG_SHEET,
			Severity: constraint.SeverityError,
			Element:  "stringer",
			Message:  "косоур не помещается на стандартный лист",
			Param:    "Изготовление",
			Guide: fmt.Sprintf(
				"Косоур %.0f×%.0f мм (рез %d мм) не помещается на стандартный лист (макс. %.0f×%.0f мм). Изготовление при текущем каталоге листов невозможно — уменьшите высоту подъёма или измените число ступеней, чтобы косоур влез на лист.",
				feas.PartLen, feas.PartWid, int(feas.Kerf), maxLen, maxWid),
			Fix: "Уменьшите высоту подъёма или измените число ступеней",
		}},
	}
}

// buildConfiguration собирает и валидирует параметрическую конфигурацию
// из исходных параметров пользователя.
// ValidateConfig — дешёвая валидация конфигурации до постановки в очередь
// (EDR-0035 §3.4): тот же проверочный шаг, что buildConfiguration в начале
// Calculate, но без конвейера. Используется async-эндпоинтом, чтобы
// заведомо невалидный вход отбраковать сразу (422), а не гнать в воркер.
func ValidateConfig(cfg Config) error {
	if _, err := buildConfiguration(cfg); err != nil {
		return fmt.Errorf("stair: %w", err)
	}
	return nil
}

// ValidateConfig — метод сервиса (тот же шаг, что и функция ValidateConfig);
// нужен транспортному/ассистентному слою через интерфейс (DOM-0008).
func (s *Service) ValidateConfig(cfg Config) error {
	return ValidateConfig(cfg)
}

func buildConfiguration(cfg Config) (*engineering.StairConfiguration, error) {
	c := &engineering.StairConfiguration{
		Width:      cfg.Width,
		Height:     cfg.Height,
		Flight:     cfg.Flight,
		StepCount:  1,
		StepHeight: cfg.Height,
	}
	c.StepHeight = cfg.StepHeight
	c.StringerThickness = cfg.StringerThickness
	c.StepThickness = cfg.StepThickness
	c.Riser = cfg.Riser
	c.Clearance = cfg.Clearance
	c.RailingHeight = cfg.RailingHeight
	c.LandingWidth = cfg.LandingWidth
	c.LandingDepth = cfg.LandingDepth
	c.RoomWidth = cfg.RoomWidth
	c.RoomLength = cfg.RoomLength
	c.ApproachSpace = cfg.ApproachSpace
	// Прямой марш: свободное пространство перед первой ступенью обязательно
	// (норма 1000–1200 мм); пустое значение трактуется как 1000 мм.
	if cfg.Flight == engineering.FlightStraight && c.ApproachSpace.Millimeters() == 0 {
		c.ApproachSpace = engineering.Length(1000)
	}
	c.LowerStepCount = cfg.LowerStepCount
	c.TurnKind = cfg.TurnKind
	c.WinderCount = cfg.WinderCount
	c.OuterRadius = cfg.OuterRadius
	c.Material = string(cfg.Material)
	c.Railing = cfg.Railing
	c.RailingLower = cfg.RailingLower
	c.RailingLanding = cfg.RailingLanding
	c.RailingUpper = cfg.RailingUpper
	c.Direction = cfg.Direction
	c.SpiralDirection = cfg.SpiralDir
	// Спираль: перила автоматически — сторона по направлению закрутки
	// (CONF-SPIRAL-RAILING): по часовой — справа, против часовой — слева.
	// Перила всегда с одной стороны (по наружному краю марша).
	if cfg.Flight == engineering.FlightSpiral {
		c.Railing = cfg.SpiralDir.DefaultRailing()
	}
	// Выбранный материал (MFG-0005): должен быть в каталоге и поддерживать
	// толщины косоура и ступени. Пустой материал — автоназначение по толщине.
	if cfg.Material != "" {
		mat, ok := engmfg.DefaultMaterialRegistry().Find(cfg.Material)
		if !ok {
			return nil, configInputError(fmt.Errorf(
				"stair: material %q not found in catalog", cfg.Material))
		}
		for _, tk := range []struct {
			t    float64
			name string
		}{
			{cfg.StringerThickness.Millimeters(), "косоура"},
			{cfg.StepThickness.Millimeters(), "ступени"},
		} {
			if !mat.SupportsThickness(tk.t) {
				return nil, configInputError(fmt.Errorf(
					"stair: material %q does not support thickness %v mm of %s",
					cfg.Material, tk.t, tk.name))
			}
		}
		// Габариты, гарантируемые изготовлением в выбранном материале
		// (MFG-0012): превышение обращается в понятную ошибку, чтобы
		// пользователь видел предел каждого материала, а не прогон конвейера.
		if float64(c.Width.Millimeters()) > mat.MaxWidthMm {
			return nil, configInputError(fmt.Errorf(
				"stair: width %v mm exceeds maximum %v mm for material %q",
				c.Width.Millimeters(), int(mat.MaxWidthMm), cfg.Material))
		}
		if float64(c.Height.Millimeters()) > mat.MaxHeightMm {
			return nil, configInputError(fmt.Errorf(
				"stair: rise height %v mm exceeds maximum %v mm for material %q",
				c.Height.Millimeters(), int(mat.MaxHeightMm), cfg.Material))
		}
	}
	// Энвелоп платформы (MFG-0012): гарантия изготовления только до этих
	// пределов (совпадают с лимитами конструкторов). Вход сверх них
	// отклоняется понятной ошибкой вместо прогона конвейера.
	if c.Height.Millimeters() > maxSupportedHeightMM {
		return nil, configInputError(fmt.Errorf(
			"stair: rise height %v mm exceeds supported maximum %v mm",
			c.Height.Millimeters(), maxSupportedHeightMM))
	}
	if c.Flight == engineering.FlightSpiral && c.OuterRadius.Millimeters() > maxSupportedRadiusMM {
		return nil, configInputError(fmt.Errorf(
			"stair: spiral outer radius %v mm exceeds supported maximum %v mm",
			c.OuterRadius.Millimeters(), maxSupportedRadiusMM))
	}
	// EDR-0007 §4.4: колонна имеет положительный радиус (R > W). Проверяется
	// здесь с цифрами, чтобы подсказка была конкретной (c.Validate() даёт
	// общий текст без значений).
	if c.Flight == engineering.FlightSpiral && c.OuterRadius.Millimeters() <= c.Width.Millimeters() {
		return nil, &solver.InputError{
			Code:    constraint.GEO_SPIRAL_RADIUS,
			Field:   "Радиус спирали",
			Value:   c.OuterRadius.Millimeters(),
			Min:     c.Width.Millimeters(),
			HasMin:  true,
			Message: "Радиус спирали не превышает ширину марша",
			Guide: fmt.Sprintf(
				"Наружный радиус спирали %.0f мм должен быть больше ширины марша %.0f мм. Колонна в центре спирали имеет радиус = радиус − ширина марша; при %.0f мм колонна исчезает. Увеличьте радиус минимум до %.0f мм или уменьшите ширину марша.",
				c.OuterRadius.Millimeters(), c.Width.Millimeters(),
				c.OuterRadius.Millimeters()-c.Width.Millimeters(),
				c.Width.Millimeters()+1),
			Fix: fmt.Sprintf("Увеличьте радиус спирали минимум до %.0f мм", c.Width.Millimeters()+1),
		}
	}
	if err := c.Validate(); err != nil {
		if inp := configInputError(err); inp != nil {
			return nil, inp
		}
		return nil, fmt.Errorf("stair: %w", err)
	}
	return c, nil
}

const (
	// maxSupportedHeightMM — максимальная высота подъёма, гарантируемая
	// каталогом листов (лимит конструкторов 6000 мм).
	maxSupportedHeightMM = 6000.0
	// maxSupportedRadiusMM — максимальный наружный радиус спирали
	// (лимит конструкторов 5000 мм).
	maxSupportedRadiusMM = 5000.0
)
