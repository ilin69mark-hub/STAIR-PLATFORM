// Package stair реализует application layer лестницы (BC-002): единый
// оркестратор сквозного расчёта проекта. Мета-уровень (прикладные
// сервисы) зависят от доменов и движков, но не от транспорта, HTTP,
// БД или UI (ADR-0006, DOM-0008). Все результаты детерминированы.
package stair

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
	// CeilingHeight — мм, абсолютная отметка низа перекрытия (Z). По ней
	// геометрия считает настоящий просвет под перекрытием; 0 = не задано.
	CeilingHeight engineering.Length
	// ApproachSpace — свободное пространство перед первой ступенью марша
	// (норма 1000–1200 мм): зона, в которой человек должен встать перед
	// началом подъёма. Учитывается в fit-check (модель сдвигается от стены
	// на ApproachSpace) и в подборе вариантов. Применяется ко ВСЕМ типам
	// марша (S-167); раньше здесь было ограничение «только прямой», которое
	// расходилось с Geometry Engine.
	ApproachSpace  engineering.Length
	LowerStepCount int // n1 — число ступеней нижнего марша
	// TurnKind — тип поворота для маршей с площадкой (L/U): площадка
	// (platform, по умолчанию) либо поворотные ступени (winder, только U).
	TurnKind engineering.TurnKind
	// WinderCount — число поворотных ступеней (только TurnWinder, U-образный).
	WinderCount int
	// Material — материал КАРКАСА: косоуры, подступенки
	// (код каталога MFG-0005); пустое значение — автоназначение по толщине.
	Material dommfg.MaterialCode
	// TreadMaterial — материал СТУПЕНЕЙ: проступи, площадки, поворотные
	// ступени. Пустое значение → наследуется от Material. Задаёт и толщину:
	// у проступи из дерева минимум 20 мм, у стальной — от 2 мм.
	TreadMaterial dommfg.MaterialCode
	// RiserThickness — толщина подступенка. Ноль → равно толщине ступени
	// (поведение до разделения материалов).
	RiserThickness engineering.Length
	// Перила и направления (CONF-RAILING/DIRECTION).
	Railing        engineering.RailingSide   // прямой марш
	RailingLower   engineering.RailingSide   // L/U: первый марш
	RailingLanding engineering.RailingSide   // L/U: площадка
	RailingUpper   engineering.RailingSide   // L/U: второй марш
	Direction      engineering.TurnDirection // L/U: поворот площадки
	// FloorFixing / CeilingFixing — основания крепления (CONF-FIXING,
	// решение владельца 2026-10-08): бетон или дерево. Пусто → бетон.
	// Влияют только на состав крепежа в смете, геометрию не трогают.
	FloorFixing   engineering.FixingBase
	CeilingFixing engineering.FixingBase
	// ExtraFasteners — дополнительный крепёж по объекту, коды позиций
	// каталога в формате «КОД» или «КОД@количество» (см.
	// manufacturing.parseExtraFasteners).
	ExtraFasteners []string
	// Герметик (CONF-SEALANT): толщина слоя, мм, и доп. картриджи по объекту.
	SealantLayerMM         float64
	ExtraSealantCartridges int
}

// Options — опциональные настройки расчёта; нулевое значение даёт дефолты.
type Options struct {
	// ComfortStep — шаг комфорта S (600–640); 0 → solver.DefaultComfortStep.
	ComfortStep float64
	// Rates — ставки цены. ЯВНО заданные ставки имеют приоритет; иначе
	// источник выбирает сервис (см. RatesResolver), и nil → engprc.DefaultRates().
	Rates *engprc.Rates
	// TenantID — контекст, для которого сервис разрешает ставки (CRITICAL-03).
	//
	// Раньше выбор источника ставок делал транспорт: публичная витрина
	// подкладывала ставки магазина (publicStoreRates), а девять
	// авторизованных маршрутов — нет, и те падали в engprc.DefaultRates().
	// Одна и та же лестница стоила по-разному в зависимости от маршрута.
	// Теперь транспорт только ОБОЗНАЧАЕТ контекст (tenant), а политику
	// выбора источника применяет единственный код в application-слое.
	//
	// Пусто → ставки магазина не запрашиваются, действуют встроенные ставки
	// движка. Так остаются внутренние вызовы без tenant-контекста
	// (graphql, ассистент, перебор оптимизации).
	TenantID string
	// MachineRates — ставки машинных операций; nil → engmfg.DefaultMachineRates().
	MachineRates *engmfg.MachineRates

	// skipVariations — не генерировать интерактивные вариации A/B/C
	// (variation.ForRoomFit / attachVariations).
	//
	// Поле НЕЭКСПОРТИРУЕМОЕ намеренно: его может установить только внутренний
	// перебор оптимизации, и никакой внешний вызывающий.
	//
	// SEC-003/PERF (2026-09-26): генерация вариаций — самая дорогая часть
	// конвейера, когда лестница не вписывается в помещение. Замер: Calculate
	// L-образной лестницы, не вписывающейся в комнату, — 138 мс против
	// 0.6 мс у вписывающейся (в 230 раз). Причина: variation.ForRoomFit
	// перебирает двойной вложенный цикл (высота ступени × ширина марша)
	// и для каждой комбинации прогоняет полный конвейер.
	//
	// Для РАНЖИРОВАНИЯ кандидатов вариации не нужны: оценщику достаточно
	// res.Validation.Blocking и res.Price. Поэтому перебор Optimize
	// пропускает их, а лучший кандидат пересчитывается полностью
	// (optimize.go, финальный Calculate) — и вариации в ответе остаются.
	skipVariations bool

	// PreviewMesh — отдать в Mesh облегчённую сетку для показа (S-165).
	//
	// Объём, площадь, раскрой и цена в этом случае БЕРУТСЯ из
	// производственной модели, как всегда: облегчённая сборка идёт вторым
	// проходом и используется только для картинки. Поэтому цена на витрине
	// и в админке совпадает по построению, а не «мы стараемся не
	// допустить».
	//
	// Плотности в Б-Rep неразделимы (дуга — уже многоугольник), поэтому
	// вариант «просто огрубить тот же меш» дал бы разные объёмы и разные
	// цены — измерено: одна проступь R=10 теряет 0,62 % объёма при
	// уменьшении точности.
	PreviewMesh bool
}

// Result — сквозной результат расчёта проекта (все этапы конвейера).
type Result struct {
	Validation     validation.Result
	Flight         solver.FlightResult  // прямой марш
	LShape         *solver.LShapeResult // L-образный марш (Flight == LShape)
	UShape         *solver.UShapeResult // П-образный марш (Flight == UShape)
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
	StepThickness engineering.Length
	RailingHeight engineering.Length
	// Width — ширина марша для 2D-схем. DOM-005 (2026-09-26): её не было ни в
	// Result, ни в Snapshot, поэтому админские чертежи рисовались при
	// зашитой ширине 900 мм, тогда как объём/цена/BOM считались для
	// фактической. Витрина при этом рисовала план по реальной width_mm.
	Width             engineering.Length
	Riser             bool
	StringerThickness engineering.Length
	// Hints — подсказки по полям живой формы (S-173). Ключ — имя поля формы
	// («stringerThicknessMM»), значение — готовый текст для человека.
	//
	// Раньше норматив на косоур считался ДУБЛИРУЮЩЕЙ таблицей в TypeScript
	// (frontend/shared/src/config.ts, где в комментарии прямо стояло
	// «Зеркало timber_stringer.go»). Это нарушало 02_PROJECT_PRINCIPLES
	// P-012 «запрещается дублирование инженерных правил» — ровно тот класс
	// ошибок, что уже случился в проекте со ставками. Теперь единственный
	// источник — engineering/timber_stringer.go, а форма получает текст
	// готовым из ответа :validate.
	Hints map[string]string
	// StringerSpine — эхо ширины спинки косоура-гребёнки металлокаркаса,
	// отмеренной ПО НОРМАЛИ к линии посадок (CombFrame.StringerSpine,
	// чертеж 2026-10-05: 300 мм). Нужно 2D-схеме: боковой профиль гребёнки
	// рисуется именно этой глубиной, а не толщиной листа. Для дерева —
	// толщина косоура, гребёнки там нет.
	StringerSpine engineering.Length
	// Эхо перил/направлений (CONF-RAILING/DIRECTION).
	Railing        engineering.RailingSide
	RailingLower   engineering.RailingSide
	RailingLanding engineering.RailingSide
	RailingUpper   engineering.RailingSide
	Direction      engineering.TurnDirection
	// Эхо поворота (CONF-TURN-KIND): тип поворота и число поворотных ступеней.
	TurnKind    engineering.TurnKind
	WinderCount int
}

// RatesResolver — источник ставок расчёта магазина для конкретного tenant'а
// (CRITICAL-03, 2026-09-27).
//
// Интерфейс объявлен ЗДЕСЬ, на стороне потребителя, а не в application/store:
// application-слой расчёта не должен зависеть от сервиса магазина целиком.
// Наличие интерфейса означает, что store.Service удовлетворяет ему
// структурно — без импорта и без цикла.
//
// Контракт: nil-ставки и ошибка НЕ являются фатальными для расчёта —
// вызывающий (Service.resolveRates) логирует и откатывается на встроенные
// ставки движка, чтобы сбой прайса не ронял геометрию.
type RatesResolver interface {
	ResolveRates(ctx context.Context, tenantID string) (*engprc.Rates, error)
}

// Service — прикладной сервис расчёта лестницы. Является единственной
// точкой входа сквозного конвейера для любых транспортов.
type Service struct {
	constraints *constraint.ConstraintSet
	// rates — источник ставок магазина; nil → встроенные ставки движка для
	// всех tenant'ов (поведение до CRITICAL-03).
	rates RatesResolver
}

// NewService создаёт сервис со стандартным профилем правил (EDR-0002).
func NewService() *Service {
	return &Service{
		constraints: constraint.StandardProfile("STANDARD"),
	}
}

// NewServiceWithRates создаёт сервис, разрешающий ставки магазина через rs.
// rs может быть nil — тогда поведение совпадает с NewService.
func NewServiceWithRates(rs RatesResolver) *Service {
	return &Service{
		constraints: constraint.StandardProfile("STANDARD"),
		rates:       rs,
	}
}

// resolveRates применяет ПРИЕДИНУЮЮ политику выбора источника ставок.
//
// Порядок приоритета:
//  1. opts.Rates != nil — явно заданные ставки (внутренние переборы,
//     тесты, специальные сценарии). Транспорт сюда не попадает: клиентские
//     ставки отвергаются на входе (SEC-001, rates_guard.go).
//  2. s.rates == nil или opts.TenantID == "" — встроенные ставки движка.
//  3. иначе — ставки магазина tenant'а из opts.TenantID.
//
// Сбой шага 3 НЕ превращается в ошибку расчёта: сбой прайса не должен
// отменять геометрию. Факт отката логируется — иначе расхождение цен
// между маршрутами станет невидимым.
func (s *Service) resolveRates(ctx context.Context, opts Options) *engprc.Rates {
	if opts.Rates != nil {
		return opts.Rates
	}
	if s.rates == nil || opts.TenantID == "" {
		return nil
	}
	rates, err := s.rates.ResolveRates(ctx, opts.TenantID)
	if err != nil {
		slog.Error("stair: store rates unavailable, using engine defaults",
			"tenant_id", opts.TenantID, "error", err)
		return nil
	}
	return rates
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
	// API-001 (forensic 2026-09-24): геометрия экструдирует косоур/ступень на
	// толщину; при толщине 0 движок возвращает техническую ошибку
	// («extrusion distance must be positive»), которая доезжала до HTTP как
	// 500. Толщина 0 — не «предупреждение нормы», а невозможная геометрия:
	// отдаём блокирующий результат валидации с подсказкой.
	if c.StringerThickness.Millimeters() <= 0 || c.StepThickness.Millimeters() <= 0 {
		field, value := "толщина косоура", c.StringerThickness.Millimeters()
		if c.StepThickness.Millimeters() <= 0 {
			field, value = "толщина ступени", c.StepThickness.Millimeters()
		}
		vr := buildInputResult(&solver.InputError{
			Code:    constraint.GEO_STRINGER_THICKNESS,
			Field:   capitalizeFirst(field),
			Message: capitalizeFirst(field) + " должна быть положительной",
			Guide: fmt.Sprintf("Укажите %s больше 0 мм (сейчас %.0f мм): без неё лестница не может быть построена.",
				field, value),
			Fix: "Задайте толщину косоура и ступени в мм",
		})
		return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
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
	reg, err := engmfg.DefaultMaterialRegistry()
	if err != nil {
		return nil, fmt.Errorf("stair: cost: %w", err)
	}
	ds, err := engmfg.PrepareCost(pkg, reg, *rates)
	if err != nil {
		return nil, fmt.Errorf("stair: cost: %w", err)
	}

	priceRates := engprc.DefaultRates()
	if r := s.resolveRates(ctx, opts); r != nil {
		priceRates = *r
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
		if gi.Code == "room_fit" && !opts.skipVariations {
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
	//
	// SEC-003/PERF: внутри перебора оптимизации вариации не строятся
	// (см. Options.skipVariations) — они нужны только в ответе.
	if !opts.skipVariations {
		attachVariations(ctx, &res.Validation, c, s.constraints)
	}
	res.Mesh = gen.Mesh
	res.RailingMesh = gen.RailingMesh
	if opts.PreviewMesh {
		// Второй проход: та же модель, огрублённая тесселяция, только меш.
		// Числа отсюда НЕ берутся — они уже посчитаны по производственной
		// модели выше.
		if prev, perr := geometry.GeneratePreviewMesh(ctx, c); perr == nil {
			res.Mesh = prev.Mesh
			res.RailingMesh = prev.RailingMesh
			res.RoomMesh = prev.RoomMesh
		}
	}
	res.RoomMesh = gen.RoomMesh
	res.Package = pkg
	res.Cost = ds
	res.Price = price
	res.StepThickness = c.StepThickness
	res.RailingHeight = c.RailingHeight
	res.Width = c.Width
	res.Riser = c.Riser
	res.StringerThickness = c.StringerThickness
	res.StringerSpine = engineering.Length(c.CombOrDefault().StringerSpine)
	res.Railing = c.Railing
	res.RailingLower = c.RailingLower
	res.RailingLanding = c.RailingLanding
	res.RailingUpper = c.RailingUpper
	res.Direction = c.Direction
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
	default:
		// DOM-001 (forensic 2026-09-24): неизвестный тип марша больше НЕ
		// трактуется как прямой. Раньше эта ветка молча считала «diagonal»
		// прямым маршем — пользователь получал чужую геометрию, а если
		// конвейер спотыкался ниже — 500. Теперь это блокирующая входная
		// ошибка с перечнем допустимых значений.
		vr := buildInputResult(&solver.InputError{
			Code:    constraint.GEO_HEIGHT,
			Field:   "Тип марша",
			Message: "Неизвестный тип марша",
			Guide:   "Допустимые типы марша: straight (прямой), l_shape (Г-образный), u_shape (П-образный).",
			Fix:     "Выберите тип марша из списка",
		})
		return &Result{Validation: s.advise(ctx, cfg, c, comfort, vr)}, nil
	}
	// Нормативы на косоур (решение владельца 2026-10-05): деревянная балка и
	// листовая сварная. Минимальная толщина зависит от длины марша по проекции
	// и ширины, а узнать длину можно ТОЛЬКО после решалки — до неё её нет.
	// Поэтому проверки живут здесь, а не в normalize.
	res.Validation.Issues = append(res.Validation.Issues, timberStringerIssues(cfg, c, res)...)
	res.Validation.Issues = append(res.Validation.Issues, plateStringerIssues(c, res)...)
	res.Hints = stringerNormHints(cfg, c, res)
	return res, nil
}

// plateStringerIssues сверяет листовой (сварный) косоур с нормативом: таблица
// ключом имеет ТОЛЩИНУ ЛИСТА, а длина марша, ширина, глубина тела и высота
// заготовки — её пределы. Проверяются два независимых требования:
//
//   - толщина листа достаточна для длины и ширины марша;
//   - ширина заготовки гребёнки (CombFrame.StringerSpine) достаточна для
//     этой толщины: 8 мм → от 250 мм, 10 мм → от 300, 12 мм → от 350.
//
// Уровень — ПРЕДУПРЕЖДЕНИЕ, как и норматив на дерево: он подсказывает, но не
// блокирует расчёт. Перевод в блокирующий — одна строка Severity.
func plateStringerIssues(c *engineering.StairConfiguration, res *Result) []validation.Issue {
	// Правило про ЛИСТОВОЙ каркас: у дерева своя таблица (timberStringerIssues).
	if !strings.HasPrefix(string(c.Material), "STEEL") && !strings.HasPrefix(string(c.Material), "METAL") {
		return nil
	}
	run := flightRuns(res)
	if len(run) == 0 {
		return nil
	}
	width := c.Width.Millimeters()
	thickness := c.StringerThickness.Millimeters()
	spine := c.CombOrDefault().StringerSpine

	var issues []validation.Issue
	for i, sg := range run {
		sp := engineering.PlateStringerSpecFor(thickness, sg.run, width)
		var reasons []string
		if !sp.BandFound {
			reasons = append(reasons, fmt.Sprintf(
				"Толщина листа %.0f мм в нормативе не значится: таблица описывает 8, 10 и 12 мм.",
				thickness))
		}
		if sp.ThicknessTooThin {
			// Полоса берётся ПО ДЛИНЕ МАРША, а не по выбранной толщине:
			// иначе текст назвал бы марш 4050 мм строкой «до 2.5 – 3.0 м».
			runBand := engineering.PlateStringerBandForRun(sg.run)
			reasons = append(reasons, fmt.Sprintf(
				"Марш %.0f мм по проекции попадает в строку «%s», для неё нужен лист от %.0f мм (сейчас %.0f мм).",
				sg.run, runBand.RunLabel, sp.RequiredMM, thickness))
		}
		if sp.WidthBeyondNarrow {
			reasons = append(reasons, fmt.Sprintf(
				"Ширина марша %.0f мм больше %.0f мм — за пределами норматива (самая широкая строка до 1.4 м).",
				width, engineering.PlateStringerBands[len(engineering.PlateStringerBands)-1].WidthMaxMM))
		}
		if sp.BandFound && spine+0.1 < sp.Band.BlankMinMM {
			reasons = append(reasons, fmt.Sprintf(
				"Ширина заготовки гребёнки %.0f мм меньше %.0f мм, положенных норме для листа %.0f мм.",
				spine, sp.Band.BlankMinMM, thickness))
		}
		if len(reasons) == 0 {
			continue
		}
		el := "stringer"
		if len(run) > 1 {
			el = fmt.Sprintf("stringer:%d", i)
		}
		guide := ""
		if sp.BandFound {
			guide = fmt.Sprintf(
				"Листовой косоур %.0f мм: предел марша %s, ширина %s, глубина тела %s, заготовка %s. Особенность конструкции: %s",
				thickness, sp.Band.RunLabel, sp.Band.WidthLabel,
				sp.Band.BodyLabel, sp.Band.BlankLabel(), sp.Band.Notes)
		}
		issues = append(issues, validation.Issue{
			ID:       "plate_stringer_norm",
			Code:     constraint.GEO_STRINGER_THICKNESS_SHEET,
			Severity: constraint.SeverityWarning,
			Element:  el,
			Param:    "Толщина косоура",
			Message: fmt.Sprintf("Листовой косоур: норматив не выполнен (%s)",
				joinReasons(reasons)),
			Guide: guide,
			Fix: fmt.Sprintf("Возьмите лист по нормативу: от %.0f мм, заготовка от %.0f мм",
				sp.RequiredMM, minPlateBlankFor(sp.RequiredMM)),
		})
	}
	return issues
}

// flightRun — длина марша по проекции из результата решалки. У L/П-лестницы
// маршей два, и норматив применяется к каждому отдельно.
type flightRun struct {
	run float64
}

// flightRuns — длины всех маршей изделия, детерминированно по типам марша.
func flightRuns(res *Result) []flightRun {
	switch {
	case res.Flight.Run > 0:
		return []flightRun{{res.Flight.Run.Millimeters()}}
	case res.LShape != nil:
		return []flightRun{
			{res.LShape.LowerRun.Millimeters()},
			{res.LShape.UpperRun.Millimeters()},
		}
	case res.UShape != nil:
		return []flightRun{
			{res.UShape.LowerRun.Millimeters()},
			{res.UShape.UpperRun.Millimeters()},
		}
	default:
		return nil
	}
}

// joinReasons — reasons через «; », чтобы причина читалась одним предложением.
func joinReasons(reasons []string) string {
	out := ""
	for i, r := range reasons {
		if i > 0 {
			out += "; "
		}
		out += r
	}
	return out
}

// minPlateBlankFor — минимальная высота заготовки для требуемой толщины.
func minPlateBlankFor(thicknessMM float64) float64 {
	if b, ok := engineering.PlateStringerBandForThickness(thicknessMM); ok {
		return b.BlankMinMM
	}
	return engineering.PlateStringerBands[0].BlankMinMM
}

// timberStringerIssues сверяет толщину деревянного косоура с нормативом по
// длине и ширине КАЖДОГО марша. У L/П-лестницы два марша, и норматив
// применяется к каждому отдельно: короткий нижний при длинном верхнем не
// вытянет толщину обоих.
//
// Возвращает ПРЕДУПРЕЖДЕНИЕ, а не блокировку — тем же уровнем, что и
// действующее правило GEO-STRINGER-THICKNESS (SeverityWarning). Норматив
// владельца носит рекомендательный характер: он подсказывает толщину и
// размеры заготовки, но не отменяет уже посчитанные изделия. Перевести в
// блокирующее — одна строка Severity ниже.
//
// Косоур проверяется по МАТЕРИАЛУ КАРКАСА: правило про балку, а ступень из
// дуба при стальном каркасе косоур не делает.
func timberStringerIssues(cfg Config, c *engineering.StairConfiguration, res *Result) []validation.Issue {
	if !strings.HasPrefix(string(c.Material), "WOOD") {
		return nil // листовой металлокаркас: балки нет
	}
	width := c.Width.Millimeters()
	thickness := c.StringerThickness.Millimeters()
	type seg struct {
		name string
		run  float64
	}
	var segs []seg
	switch {
	case res.Flight.Run > 0:
		segs = append(segs, seg{"марша", res.Flight.Run.Millimeters()})
	case res.LShape != nil:
		segs = append(segs,
			seg{"нижнего марша", res.LShape.LowerRun.Millimeters()},
			seg{"верхнего марша", res.LShape.UpperRun.Millimeters()})
	case res.UShape != nil:
		segs = append(segs,
			seg{"нижнего марша", res.UShape.LowerRun.Millimeters()},
			seg{"верхнего марша", res.UShape.UpperRun.Millimeters()})
	default:
		return nil
	}
	var issues []validation.Issue
	for i, sg := range segs {
		if sg.run <= 0 {
			continue
		}
		sp := engineering.TimberStringerSpecFor(sg.run, width)
		if thickness+0.1 >= sp.MinThicknessMM {
			continue
		}
		// Метка элемента: у двух маршей она должна различаться, иначе
		// фронтенд покажет две одинаковые строки предупреждений.
		el := "stringer"
		if len(segs) > 1 {
			el = fmt.Sprintf("stringer:%d", i)
		}
		why := ""
		if sp.ThicknessByWidth {
			why = fmt.Sprintf(" Ширина марша %.0f мм больше %.0f мм, поэтому толщина %.0f мм берётся по ширине, а не по длине.",
				width, float64(engineering.TimberWideWidthMM), float64(engineering.TimberWideThickMM))
		}
		support := ""
		if sp.Band.NeedsSupport {
			support = fmt.Sprintf(" Длина марша по проекции %.0f мм — сверх %.0f мм: по нормативу нужна промежуточная опора (столб); правило опор вносится позже, поэтому расчёт не блокируется.",
				sg.run, engineering.TimberStringerBands[len(engineering.TimberStringerBands)-2].RunMaxMM)
		}
		issues = append(issues, validation.Issue{
			ID:       "timber_stringer_thickness",
			Code:     constraint.GEO_STRINGER_THICKNESS,
			Severity: constraint.SeverityWarning,
			Element:  el,
			Param:    "Толщина косоура",
			Message: fmt.Sprintf("Толщина косоура %.0f мм меньше нормативных %.0f мм для %s длиной по проекции %.0f мм",
				thickness, sp.MinThicknessMM, sp.Band.RunLabel, sg.run),
			Guide: fmt.Sprintf(
				"Деревянный косоур: полоса норматива «%s» (ширина марша «%s») требует толщину не менее %.0f мм, заготовку %s, глубину тела после выпила гребёнки — %s. Сейчас %.0f мм.%s%s",
				sp.Band.RunLabel, sp.Band.WidthLabel, sp.MinThicknessMM,
				sp.Band.BlankLabel(), sp.Band.BodyLabel(), thickness, why, support),
			Fix: fmt.Sprintf("Увеличьте толщину косоура до %.0f мм или возьмите заготовку по нормативу", sp.MinThicknessMM),
		})
	}
	return issues
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
	// Предел — РАБОЧЕЕ ПОЛЕ лазерного стола 3000×1500, а не купленный лист.
	// Лист 6000×1500 режется на две заготовки 3000×1500, и деталь длиннее
	// заготовки делится на куски со сварным стыком «шип-паз»; не влезает в
	// поле только такая деталь, которую нечем разрезать — по ширине.
	fieldLen, fieldWid := feas.FieldLength, feas.FieldWidth
	if fieldLen <= 0 || fieldWid <= 0 {
		fieldLen, fieldWid = dommfg.BlankLengthMM, dommfg.BlankWidthMM
	}
	thick := feas.Thickness
	if thick <= 0 {
		if material == "" {
			var err error
			material, err = engmfg.DefaultMaterialForThickness(stringerThick.Millimeters())
			if err != nil {
				material = dommfg.MaterialCode("STEEL-S235")
			}
		}
		if reg, err := engmfg.DefaultMaterialRegistry(); err == nil {
			if m, ok := reg.Find(material); ok {
				thick = m.FixedTreadThicknessMm
				if thick <= 0 {
					thick = stringerThick.Millimeters()
				}
			}
		}
	}
	return validation.Result{
		Valid:    false,
		Blocking: true,
		Issues: []validation.Issue{{
			ID:       "ISSUE-MFG",
			Code:     constraint.MFG_SHEET,
			Severity: constraint.SeverityError,
			Element:  "stringer",
			Message:  "косоур не помещается в поле лазерного стола",
			Param:    "Изготовление",
			Guide: fmt.Sprintf(
				"Косоур %.0f×%.0f мм не влезает в рабочее поле лазерного стола %.0f×%.0f мм (лист %.0f мм, отступ %.0f, зазор %.0f). По длине длинный косоур режется кусками со сварным стыком «шип-паз», но ширина %.0f мм превышает поле вместе с отступом — уменьшите ширину марша или измените число ступеней.",
				feas.PartLen, feas.PartWid, fieldLen, fieldWid, dommfg.StockSheetLengthMM,
				dommfg.CutEdgeOffsetMM(thick), dommfg.CutGapMM(thick), feas.PartWid),
			Fix: "Уменьшите ширину марша или высоту подъёма",
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
		StepHeight: cfg.Height}
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
	c.CeilingHeight = cfg.CeilingHeight
	c.ApproachSpace = cfg.ApproachSpace
	// Свободное пространство перед первой ступенью обязательно (норма 1000–1200 мм)
	// для всех типов марша; пустое значение трактуется как 1000 мм.
	if c.ApproachSpace.Millimeters() == 0 {
		c.ApproachSpace = engineering.Length(1000)
	}
	c.LowerStepCount = cfg.LowerStepCount
	c.TurnKind = cfg.TurnKind
	c.WinderCount = cfg.WinderCount
	c.Material = string(cfg.Material)
	// Материал ступеней и толщина подступенка. Разрешение наследования
	// (TreadMaterial → Material, RiserThickness → StepThickness) делается
	// ОДИН раз здесь, а не в потребителях: геометрия, производство и расчёт
	// получают уже явные значения, и «кто что inherits» не надо помнить в
	// каждом месте.
	c.TreadMaterial = string(cfg.TreadMaterial)
	if c.TreadMaterial == "" {
		c.TreadMaterial = c.Material
	}
	c.RiserThickness = cfg.RiserThickness
	if c.RiserThickness <= 0 {
		c.RiserThickness = c.StepThickness
	}
	c.Railing = cfg.Railing
	c.RailingLower = cfg.RailingLower
	c.RailingLanding = cfg.RailingLanding
	c.RailingUpper = cfg.RailingUpper
	c.Direction = cfg.Direction
	// Основания крепления (CONF-FIXING). Значения не из перечня отбрасываются
	// ЗДЕСЬ, а не в Validate: пустое значение допустимо (бетон), а вот
	// «concretee» должно превратиться в бетон, а не уронить расчёт на
	// 422 — клиентский конфиг приходит из витрины и может быть устаревшим.
	c.FloorFixing = cfg.FloorFixing
	if !c.FloorFixing.Valid() {
		c.FloorFixing = engineering.BaseConcrete
	}
	c.CeilingFixing = cfg.CeilingFixing
	if !c.CeilingFixing.Valid() {
		c.CeilingFixing = engineering.BaseConcrete
	}
	c.ExtraFasteners = append([]string(nil), cfg.ExtraFasteners...)
	c.SealantLayerMM = cfg.SealantLayerMM
	c.ExtraSealantCartridges = cfg.ExtraSealantCartridges
	// Разрешение материалов (MFG-0005). Каркас и ступени проверяются
	// РАЗДЕЛЬНО: раньше один материал должен был поддерживать и толщину
	// косоура, и толщину ступени, из-за чего стальной косоур 8 мм с деревянной
	// проступью отвергался (у дерева MinThickness = 20 мм).
	//
	// Проверяем: материал каркаса ↔ толщина косоура;
	//           материал ступеней ↔ толщины ступени И ПОДСТУПЕНКА.
	treadCode := cfg.TreadMaterial
	if treadCode == "" {
		treadCode = cfg.Material
	}
	riserTh := cfg.RiserThickness.Millimeters()
	if riserTh <= 0 {
		riserTh = cfg.StepThickness.Millimeters()
	}
	if cfg.Material != "" || treadCode != "" {
		reg, err := engmfg.DefaultMaterialRegistry()
		if err != nil {
			return nil, configInputError(fmt.Errorf(
				"stair: catalog unavailable: %w", err))
		}
		// Толщина ступени из металла НЕ выбирается (решение владельца
		// 2026-10-05): в каталоге у стали одна ступень — 5 мм. Значит клиент
		// может прислать что угодно (старая форма предлагала 3–8 мм, старые
		// проекты хранят 6 мм), и это «что угодно» обязано схлопнуться в 5 мм
		// ДО проверки толщин, до геометрии и до раскроя.
		//
		// Раньше здесь стоял обратный порядок: сначала проверка, потом всё
		// остальное, — и клиентские 6 мм проходили как законное значение.
		if mat, ok := reg.Find(treadCode); ok && mat.HasFixedTreadThickness() {
			c.StepThickness = engineering.Length(mat.FixedTreadThicknessMm)
			if cfg.RiserThickness <= 0 || riserTh == cfg.StepThickness.Millimeters() {
				// Подступенок из того же листа: без собственного выбора он
				// идёт толщиной ступени, иначе у лестницы были бы две разные
				// толщины листа из одного проката.
				riserTh = mat.FixedTreadThicknessMm
			}
		}
		// Радиус скругления носа — производная величина от материала ступеней
		// (Material.TreadNoseRadiusMm): у металла он нулевой, поэтому
		// металлическая лестница не получает ни геометрии фаски, ни операции
		// фрезеровки, и её цена не меняется. Именно здесь, а не в калькуляторе:
		// только в прикладном слое известен материал ДЕТАЛИ, а движок
		// геометрии о материалах не знает вовсе.
		if mat, ok := reg.Find(treadCode); ok {
			c.TreadNoseRadiusMM = engineering.Length(mat.TreadNoseRadiusMm)
		} else {
			c.TreadNoseRadiusMM = 0
		}
		// Радиус не может превышать толщину ступени: иначе нос не поместится в
		// деталь. Каталог проверяет это против минимальной толщины, но при
		// толщине 20 мм и радиусе 8 мм проверка по каталогу проходит, а
		// конкретная конфигурация может оказаться тоньше.
		if r := c.TreadNoseRadiusMM.Millimeters(); r > 0 {
			if st := cfg.StepThickness.Millimeters(); r > st {
				c.TreadNoseRadiusMM = engineering.Length(st)
			}
		}

		// Толщина детали проверяется по материтету ЭТОЙ детали: каркасная
		// толщина — против каркасного материала, ступенная и подступенковая —
		// против материала ступеней. Подступенок режется из материала ступеней
		// (engine/manufacturing: PartRiser идёт в ветку cfg.TreadMaterial), и
		// клиентское riser_thickness_mm, если оно недопустимо для материала
		// ступеней, отбрасывается в пользу толщины ступени. Иначе сохранённые
		// конфигурации «стальной подступенок 6 мм + дубовые ступени» (их
		// полно накопилось до смены правила) падали бы с 422.
		type thicknessCheck struct {
			code dommfg.MaterialCode
			t    float64
			name string
			// tread — деталь ступени: у неё своя нижняя граница
			// (Material.MinTreadThicknessMm, у дуба 40 мм под скругление R10),
			// а косоур ограничен общим MinThickness. Поэтому проверка
			// выбирается по виду детали, а не по названию.
			tread bool
		}
		checks := []thicknessCheck{
			{cfg.Material, cfg.StringerThickness.Millimeters(), "косоура", false},
			// Толщина ступени берётся ИЗ c, а не из cfg: выше она уже могла
			// быть приведена к единственной толщине материала. Иначе клиентские
			// 40 мм проверялись бы как 40 и отвергались, хотя в расчёт ушло бы
			// 5 мм, — то есть ошибка была бы про число, которого в изделии нет.
			{treadCode, c.StepThickness.Millimeters(), "ступени", true},
		}
		if mat, ok := reg.Find(treadCode); ok && !mat.SupportsThickness(riserTh) {
			riserTh = cfg.StepThickness.Millimeters()
		}
		// Толщина подступенка уходит дальше и в геометрию (builder.go читает
		// cfg.RiserThickness), поэтому нормализованное значение обязано
		// попасть в c, а не остаться в локальной переменной.
		c.RiserThickness = engineering.Length(riserTh)
		checks = append(checks, thicknessCheck{treadCode, riserTh, "подступенка", false})
		for _, ck := range checks {
			if ck.code == "" {
				continue
			}
			mat, ok := reg.Find(ck.code)
			if !ok {
				return nil, configInputError(fmt.Errorf(
					"stair: material %q not found in catalog", ck.code))
			}
			okThickness := mat.SupportsThickness(ck.t)
			if ck.tread {
				okThickness = mat.SupportsTreadThickness(ck.t)
			}
			// Толщина ЛИСТОВОГО косоура у металла не входной параметр: её
			// выводит норматив по длине и ширине марша (NormalizePlateStringer,
			// вызывается в решалке после того, как длина марша станет известна).
			// Проверять присланное число против выпуска нельзя: клиент по
			// старой форме присылает конструктивные 50 мм от деревянной модели
			// (ширина сечения бруса), и такой запрос отбраковывался с
			// MFG-MATERIAL ДО того, как норматив успевал его заменить.
			if !ck.tread && engineering.IsPlateFrameMaterial(string(ck.code)) {
				okThickness = true
			}
			if !okThickness {
				return nil, configInputError(fmt.Errorf(
					"stair: material %q does not support thickness %v mm of %s",
					ck.code, ck.t, ck.name))
			}
			// Габариты, гарантируемые изготовлением в этом материале
			// (MFG-0012): превышение обращается в понятную ошибку, чтобы
			// пользователь видел предел каждого материала, а не прогон
			// конвейера. Проверяем по обоим материалам: по ширине марша и по
			// высоте подъёма ограничен каждый из выпускаемых листов.
			if float64(c.Width.Millimeters()) > mat.MaxWidthMm {
				return nil, configInputError(fmt.Errorf(
					"stair: width %v mm exceeds maximum %v mm for material %q",
					c.Width.Millimeters(), int(mat.MaxWidthMm), ck.code))
			}
			if float64(c.Height.Millimeters()) > mat.MaxHeightMm {
				return nil, configInputError(fmt.Errorf(
					"stair: rise height %v mm exceeds maximum %v mm for material %q",
					c.Height.Millimeters(), int(mat.MaxHeightMm), ck.code))
			}
		}
	}

	// Границы ИЗДЕЛИЯ (S-169, решение владельца): за ними ничего не считаем
	// и ничего не строим — понятная ошибка вместо меша и цены. Проверка стоит
	// ДО Validate и до Generate, поэтому конвейер не запускается вовсе.
	//
	// Отдельно от конверта производства ниже: тот ловит предел конкретного
	// материала («для дуба максимум 4550»), здесь — границы самого изделия.
	// Ноль и отрицательные значения НЕ ловятся здесь: их отсекает
	// StairConfiguration.Validate («должно быть положительным») — иначе мы
	// перехватили бы чужую, более точную формулировку (DOM-005).
	if h := c.Height.Millimeters(); h > 0 && h < engineering.MinFloorHeightMM {
		return nil, configInputError(fmt.Errorf(
			"stair: height %v mm is below minimum %v mm", h, engineering.MinFloorHeightMM))
	}
	if w := c.Width.Millimeters(); w > 0 && w < engineering.MinFlightWidthMM {
		return nil, configInputError(fmt.Errorf(
			"stair: width %v mm is below minimum %v mm", w, engineering.MinFlightWidthMM))
	}
	if w := c.Width.Millimeters(); w > engineering.MaxFlightWidthMM {
		return nil, configInputError(fmt.Errorf(
			"stair: width %v mm exceeds maximum %v mm", w, engineering.MaxFlightWidthMM))
	}
	if h := c.Height.Millimeters(); h > engineering.MaxFloorHeightMM {
		return nil, configInputError(fmt.Errorf(
			"stair: height %v mm exceeds maximum %v mm", h, engineering.MaxFloorHeightMM))
	}

	// Энвелоп платформы (MFG-0012): гарантия изготовления только до этих
	// пределов (совпадают с лимитами конструкторов). Вход сверх них
	// отклоняется понятной ошибкой вместо прогона конвейера.
	if c.Height.Millimeters() > maxSupportedHeightMM {
		return nil, configInputError(fmt.Errorf(
			"stair: rise height %v mm exceeds supported maximum %v mm",
			c.Height.Millimeters(), maxSupportedHeightMM))
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
	// maxSupportedHeightMM — максимальная высота подъёма. Источник истины —
	// engineering.MaxFloorHeightMM (S-169): раньше здесь стояло собственное
	// 6000 мм, из-за чего верхняя граница жила в двух местах. Дубликат убран.
	// Лист 6000×1500 режется на две заготовки по 3000 мм, поэтому деталь
	// длиннее 6000 мм не изготовить — это производственный предел.
	maxSupportedHeightMM = engineering.MaxFloorHeightMM
)

// stringerNormHints собирает текстовую подсказку по нормативу косоура для
// поля формы «Толщина косоура» (S-173).
//
// Отдаётся ВСЕГДА, а не только при нарушении: подсказка нужна человеку
// при вводе, а не только когда норма нарушена. Текст собирается из того же
// timber_stringer.go / plate_stringer.go, что и предупреждения выше, —
// поэтому подсказка и проверка не могут разойтись.
//
// Ключ — имя поля формы фронтенда. Формат ключа зафиксирован здесь и в
// adaptValidation (frontend/shared/src/liveValidate.ts).
func stringerNormHints(cfg Config, c *engineering.StairConfiguration, res *Result) map[string]string {
	runs := flightRuns(res)
	if len(runs) == 0 {
		return nil
	}
	width := c.Width.Millimeters()
	thickness := c.StringerThickness.Millimeters()
	isTimber := strings.HasPrefix(string(c.Material), "WOOD")

	// Деревянная балка: полоса берётся по ДЛИНЕ марша, листовой каркас — по
	// толщине листа. Для двух маршей берём самую требовательную строку: одна
	// подсказка на поле, а не две одинаковых.
	band, runMM := worstTimberBand(runs, width)
	var parts []string
	switch {
	case isTimber && band != nil:
		parts = []string{
			fmt.Sprintf("норматив %s: от %.0f мм", band.RunLabel, timberMinThickness(*band, width)),
			fmt.Sprintf("заготовка %s", band.BlankLabel()),
			fmt.Sprintf("тело %s", band.BodyLabel()),
		}
		if width > engineering.TimberWideWidthMM {
			parts = append(parts, fmt.Sprintf("толщина по ширине марша > %.0f мм", float64(engineering.TimberWideWidthMM)))
		}
	case !isTimber:
		ps := engineering.PlateStringerSpecFor(thickness, runMM, width)
		runBand := engineering.PlateStringerBandForRun(runMM)
		parts = []string{fmt.Sprintf("лист %.0f мм: марш %s", ps.RequiredMM, runBand.RunLabel)}
		if ps.BandFound && !ps.WidthBeyondNarrow {
			parts = append(parts,
				fmt.Sprintf("ширина %s", runBand.WidthLabel),
				fmt.Sprintf("тело %s", runBand.BodyLabel),
				fmt.Sprintf("заготовка %s", runBand.BlankLabel()))
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return map[string]string{"stringerThicknessMM": strings.Join(parts, " · ")}
}

// timberMinThickness — толщина по нормативу с учётом правила широкого
// марша: при ширине больше TimberWideWidthMM толщина берётся по ширине,
// а не по длине.
func timberMinThickness(b engineering.TimberStringerBand, width float64) float64 {
	if width > engineering.TimberWideWidthMM {
		return engineering.TimberWideThickMM
	}
	return b.MinThicknessMM
}

// worstTimberBand — самая требовательная полоса из полос по длинам маршей:
// при L- и П-образном марше длины разные, и подсказка должна показывать
// худший случай, иначе человек увидит «всё хорошо» там, где второй марш
// требует толще.
func worstTimberBand(runs []flightRun, width float64) (*engineering.TimberStringerBand, float64) {
	var worst *engineering.TimberStringerBand
	var worstRun float64
	worstMin := 0.0
	for _, r := range runs {
		if r.run <= 0 {
			continue
		}
		b := engineering.TimberStringerBandFor(r.run)
		min := timberMinThickness(b, width)
		if worst == nil || min > worstMin {
			worstMin = min
			bb := b
			worst = &bb
			worstRun = r.run
		}
	}
	return worst, worstRun
}
