package engineering

import "fmt"

// FlightType — тип лестничного марша.
type FlightType string

// Границы ИЗДЕЛИЯ (S-169): за ними калькулятор ничего не считает. Это НЕ
// нормативные диапазоны — норма (высота ступени 150–200, проступь 260–320,
// угол 30–45°) живёт в Constraint Engine и даёт предупреждения плюс варианты
// подбора. Здесь же предел изделия: такой лестницы мы не делаем.
//
// Проверено опытом: у геометрического движка СОБСТВЕННЫХ пределов нет, он
// строит и 50 мм, и 12000 мм. Нижняя граница высоты выведена из нормы угла:
// при H = 820 мм наклон 28,5° (вне 30–45°), при H = 1020 мм — 30,4°, уже в
// норме. Верхняя граница 5000 мм и предел ширины 6000 мм — производственные:
// лист 6000×1500 режется на две заготовки, и деталь длиннее не изготовить.
//
// Проверяются НЕ в StairConfiguration.Validate: домен описывает, что такое
// корректная геометрия, а не что мы продаём. Границы изделия живут на
// границе приложения (application/stair, рядом с конвертом производства),
// чтобы юнит-тесты ядра могли считать условные значения.
const (
	MinFloorHeightMM = 1000.0 // высота этажа, мм
	MaxFloorHeightMM = 5000.0
	MinFlightWidthMM = 600.0 // ширина марша, мм
	MaxFlightWidthMM = 6000.0
)

const (
	FlightStraight FlightType = "straight"
	FlightLShape   FlightType = "l_shape"
	FlightUShape   FlightType = "u_shape"
)

// Valid — принадлежит ли значение известному типу марша.
// DOM-001 (forensic 2026-09-24): раньше enum не проверялся нигде —
// StairConfiguration.Validate() проверял только пустое значение, а
// application/stair.checkedSolve в default-ветке считал ЛЮБОЙ неизвестный
// Flight прямой маршем. Неизвестный «diagonal» доезжал до геометрии и
// отдавался клиенту как 500 internal_error.
func (f FlightType) Valid() bool {
	switch f {
	case FlightStraight, FlightLShape, FlightUShape:
		return true
	}
	return false
}

// BuildingPurpose — назначение здания (S-236, п. 67 библии нормативов).
//
// Это не украшение: нормы на лестницы разные для жилых, общественных и
// производственных зданий, и проверять их одним набором нельзя. Профиль
// выбирает набор правил (constraint.ProfileFor), а не двигает габариты
// сам: пока подтверждённых чисел по назначению мало, профили различаются
// строгостью правил (S-239), а не выдуманными значениями.
type BuildingPurpose string

const (
	// PurposeHouse — одноквартирный дом, СП 55.13330. Профиль по умолчанию
	// (решение владельца 2026-10-13): пока назначение не задано, действует
	// именно он.
	PurposeHouse BuildingPurpose = "house"
	// PurposeFlat — многоквартирный жилой дом, СП 54.13330.
	PurposeFlat BuildingPurpose = "flat"
	// PurposePublic — общественное здание, СП 118.13330.
	PurposePublic BuildingPurpose = "public"
	// PurposeIndustrial — производственное здание, СП 56.13330.
	PurposeIndustrial BuildingPurpose = "industrial"
)

// PurposeEffective — назначение с правилом по умолчанию: пусто = дом.
func (p BuildingPurpose) PurposeEffective() BuildingPurpose {
	switch p {
	case PurposeHouse, PurposeFlat, PurposePublic, PurposeIndustrial:
		return p
	}
	return PurposeHouse
}

// Valid — допустимое значение назначения. Пусто допустим (дом).
func (p BuildingPurpose) Valid() bool {
	switch p {
	case "", PurposeHouse, PurposeFlat, PurposePublic, PurposeIndustrial:
		return true
	}
	return false
}

// RailingSide — сторона установки перил (CONF-RAILING). Отсчёт сторон
// определяется со стороны первой ступени по ходу подъёма: слева от
// смотрящего вперёд человека — левые перила, справа — правые.
type RailingSide string

const (
	RailingNone  RailingSide = "none" // без перил
	RailingLeft  RailingSide = "left"
	RailingRight RailingSide = "right"
	RailingBoth  RailingSide = "both" // с двух сторон
)

// RailingType — ТИП ограждения. Это разные конструкции, а не варианты
// одного: стальное собрано из круглых труб (поручень, стойки, ригели),
// стеклянное — из стеклянных панелей в опорах. Смешивать их в одной
// сборке нельзя, поэтому тип задаётся явно, а не выводится из наличия
// заполнения.
//
// По умолчанию (пустая строка в конфиге) трактуется как стальное —
// это текущее поведение движка.
type RailingType string

const (
	RailingTypeSteel RailingType = "steel" // нержавеющая сталь, круглое сечение
	RailingTypeGlass RailingType = "glass" // стеклянные панели
)

// Effective возвращает тип ограждения; пустое значение = стальное.
func (t RailingType) Effective() RailingType {
	if t == RailingTypeGlass {
		return RailingTypeGlass
	}
	return RailingTypeSteel
}

// RailingSideValid — допускает ли значение роль стороны перил.
func (s RailingSide) Valid() bool {
	switch s {
	case RailingNone, RailingLeft, RailingRight, RailingBoth:
		return true
	}
	return false
}

// TurnDirection — направление поворота площадки L/П-образного марша:
// влево или вправо относительно хода подъёма (CONF-DIRECTION).
type TurnDirection string

const (
	TurnLeft  TurnDirection = "left"
	TurnRight TurnDirection = "right"
)

// Valid — допускает ли значение направление поворота.
func (d TurnDirection) Valid() bool {
	return d == TurnLeft || d == TurnRight
}

// TurnKind — тип поворота маршей с площадкой (L/U): площадка (платформа)
// либо поворотные ступени (EDR-0006 §4, поворот на 180° для U-образного).
// Пустое значение трактуется как TurnPlatform (обратная совместимость).
type TurnKind string

const (
	TurnPlatform TurnKind = "platform" // площадка (по умолчанию)
	TurnWinder   TurnKind = "winder"   // поворотные ступени (только U-образный)
)

// Valid — допускает ли значение тип поворота.
func (k TurnKind) Valid() bool {
	switch k {
	case TurnPlatform, TurnWinder, "":
		return true
	}
	return false
}

// FixingBase — основание крепления лестницы: пол и перекрытие (CONF-FIXING,
// решение владельца 2026-10-08).
//
// Разделение нужно потому, что крепёж в бетон и в дерево разный: в бетон
// глухарь вворачивается через дюбель, в дерево — прямо. Пустое значение
// трактуется как бетон (BaseConcrete) — лестничные пятки крепятся к
// перекрытию, а не к полу.
type FixingBase string

const (
	BaseConcrete FixingBase = "concrete" // бетон: глухарь через дюбель
	BaseWood     FixingBase = "wood"     // дерево: только глухарь
)

// OrDefault возвращает основание, подставляя бетон на пустое значение.
func (b FixingBase) OrDefault() FixingBase {
	if b == "" {
		return BaseConcrete
	}
	return b
}

// Valid — допускает ли значение основание крепления.
func (b FixingBase) Valid() bool {
	switch b {
	case BaseConcrete, BaseWood, "":
		return true
	}
	return false
}

// NeedsPlug — требуется ли дюбель для этого основания.
func (b FixingBase) NeedsPlug() bool { return b.OrDefault() == BaseConcrete }

// StairConfiguration — параметрическая конфигурация лестницы.
// Параметры являются единственным источником истины геометрии (BC-002).
type StairConfiguration struct {
	Width             Length
	Height            Length
	Length            Length
	Angle             Angle
	Flight            FlightType
	StepCount         int
	StepHeight        Length
	StepWidth         Length
	TreadDepth        Length
	Clearance         Length
	RailingHeight     Length
	StringerLength    Length
	StringerThickness Length
	StepThickness     Length
	// Riser — строить ли подступенки (вертикальные грани под проступями).
	// При false модель имеет открытые ступени; по умолчанию — с подступенками.
	Riser bool
	// Марш с площадкой (L-образный EDR-0005, П-образный EDR-0006): число
	// ступеней нижнего марша и ширина площадки. Используются только при
	// Flight == FlightLShape || Flight == FlightUShape.
	LowerStepCount int
	LandingWidth   Length // Wp — ширина площадки (вдоль верхнего марша, Y в плане)
	// LandingDepth — глубина площадки L/U-марша (вдоль нижнего марша, X в
	// плане). При 0 трактуется как Width (площадка квадратная). Для площадочных
	// маршей должно быть >= Width (проверяется в Validate).
	LandingDepth Length
	// RoomWidth / RoomLength — габариты помещения (периметр, мм) для
	// визуальной проверки вписываемости лестницы (fit-check). Не влияют на
	// геометрию марша; нулевые значения — проверка не выполняется.
	RoomWidth  Length
	RoomLength Length
	// CeilingHeight — абсолютная отметка низа перекрытия над маршем (Z в мм,
	// та же система координат, что и Height: пол первого этажа — Z = 0).
	// По ней геометрия считает НАСТОЯЩИЙ просвет: для каждой ступени берётся
	// вертикаль от верха проступи до перекрытия, и проверяется худшая ступень —
	// самая высокая из тех, что ещё находятся ПОД перекрытием.
	//
	// Ноль (по умолчанию) — перекрытие не задано, проверка просвета не
	// выполняется. Это НЕ то же самое, что просвет 0: поле Clearance —
	// заявленное пользователем число (оно проверяется Constraint Engine как
	// GEO-CLEARANCE), CeilingHeight — геометрический факт из проёма.
	//
	// Решение владельца 2026-10-12 (S-168): просвет должен считаться по
	// геометрии, а не быть полем, которое пользователь вводит вручную.
	CeilingHeight Length
	// ApproachSpace — свободное пространство перед первой ступенью марша
	// (норма 1000–1200 мм). Применяется ко ВСЕМ типам марша (прямой, L, П):
	// модель сдвигается от стены на ApproachSpace, и эта зона входит в
	// габаритный бокс, поэтому требуемая ширина помещения в fit-check
	// включает её. Нулевое значение трактуется как 1000 мм.
	//
	// Решение владельца 2026-10-12 (S-167): раньше здесь было «только при
	// Flight == FlightStraight», а Geometry Engine сдвигал модель при любом
	// типе (engine.go, approachShift). Поведение приведено к коду.
	ApproachSpace Length
	// TurnKind — тип поворота для маршей с площадкой (L/U): площадка
	// (TurnPlatform) либо поворотные ступени (TurnWinder, только U-образный,
	// EDR-0006 §4). Пустое значение — площадка (обратная совместимость).
	// Для TurnWinder поле LandingWidth трактуется как ширина просвета
	// (well width) между двумя маршами, а число поворотных ступеней задаёт
	// WinderCount.
	TurnKind TurnKind
	// WinderCount — число поворотных ступеней (только TurnWinder, U-образный).
	// Каждая поворотная ступень поднимает на ту же высоту h, что и прямые
	// марши; общее число ступеней n = LowerStepCount + WinderCount + верхних.
	WinderCount int
	// Material — материал КАРКАСА: косоуры и подступенки
	// (код каталога MFG-0005, например "STEEL-S235"); пустое значение —
	// автоназначение по толщине.
	//
	// Каркас и ступени — разные детали с разными требованиями к толщине:
	// косоур из стали — 6–10 мм, проступь из дуба — от 20 мм. Один материал
	// на всю лестницу физически невозможен, поэтому материал ступеней вынесен
	// в TreadMaterial.
	Material string
	// TreadMaterial — материал СТУПЕНЕЙ: проступи, площадки и поворотные
	// ступени (код каталога MFG-0005). Пустое значение → наследуется от
	// Material (лестница целиком из одного материала, как было до разделения).
	TreadMaterial string
	// RiserThickness — толщина ПОДСТУПЕНКА. Раньше подступенок строился той
	// же толщиной, что и проступь, потому что материал был один и деталь
	// была одна. С разделением материалов это перестаёт работать: проступь
	// из дуба 40 мм и подступенок из той же стали 40 мм — разные детали.
	//
	// Ноль → наследуется от StepThickness (поведение до разделения не
	// меняется, старые конфиги и тесты дают тот же результат).
	RiserThickness Length
	// Перила (CONF-RAILING): сторона установки. Стороны отсчитываются со
	// стороны первой ступени по ходу подъёма (слева — левые, справа —
	// правые). Для прямого марша — единственный выбор Railing; для маршей
	// с площадкой перила задаются по сегментам.
	Railing RailingSide
	// RailingType — тип ограждения: стальное (круглые трубы) или
	// стеклянное (панели). Пусто = стальное.
	RailingType RailingType
	// Purpose — назначение здания (S-236): выбирает нормативный профиль
	// правил. Пусто = дом (СП 55), решение владельца 2026-10-13.
	Purpose BuildingPurpose
	// Перила по сегментам маршей с площадкой (только L/U):
	// первый марш → площадка → второй марш.
	RailingLower   RailingSide
	RailingLanding RailingSide
	RailingUpper   RailingSide
	// Направление поворота площадки L/U-марша (CONF-DIRECTION).
	Direction TurnDirection
	// TreadNoseRadiusMM — радиус скругления носа ступеней (филёнка).
	//
	// ПРОИЗВОДНАЯ ВЕЛИЧИНА: покупатель её не выбирает, она вычисляется из
	// материала ступеней (Material.TreadNoseRadiusMm) в прикладном слое и
	// потому НЕ хранится — при восстановлении проекта пересчитывается заново
	// из материала, который хранится.
	//
	// Это не украшение, а операция: скругление снимает материал и требует
	// фрезеровки, поэтому оно входит в цену (см. milling:chamfer). У металла
	// радиус нулевой, и тогда ни геометрия, ни производство о нём не знают —
	// цена металлической лестницы от этого поля не зависит.
	TreadNoseRadiusMM Length
	// Comb — спецификация металлокаркасного узла: косоур-гребёнка со спиной,
	// пять пяток выреза и отверстия в них (см. CombFrame).
	//
	// Как и TreadNoseRadiusMM, ПРОИЗВОДНАЯ от покупателя величина: приходит
	// из чертежа узла, а не от пользователя, и потому не хранится — при
	// восстановлении проекта берётся по умолчанию (CombOrDefault).
	Comb *CombFrame
	// FloorFixing / CeilingFixing — основания крепления: пятка пола и пятка
	// выхода на перекрытие (CONF-FIXING, решение владельца 2026-10-08).
	// Влияют только на состав крепежа в смете: бетон → глухарь через дюбель,
	// дерево → глухарь без дюбеля. Геометрию не меняют.
	FloorFixing   FixingBase
	CeilingFixing FixingBase
	// ExtraFasteners — ДОПОЛНИТЕЛЬНЫЙ крепёж по объекту: коды позиций
	// каталога, которые нужны именно этой лестнице (например, если пол
	// неровный и ставят дополнительные комплекты). Формат элемента — код
	// («SCREW-8x80») либо код и количество через «@» («SCREW-8x80@12»):
	// разделитель именно «@», потому что код позиции сам содержит «x»
	// в размере. Считается в смете
	// сверх автоматического крепежа по отверстиям.
	//
	// Неизвестные коды отбрасываются с предупреждением, а не с ошибкой:
	// справочник крепежа редактируется в админке, и старый проект не должен
	// переставать считаться из-за того, что позицию убрали из каталога.
	ExtraFasteners []string
	// SealantLayerMM — толщина слоя герметика, мм (CONF-SEALANT). Расход
	// считается по формуле V = S × t, поэтому эта величина прямо умножается
	// на площадь швов. 0 → DefaultSealantLayerMM (1 мм).
	SealantLayerMM float64
	// ExtraSealantCartridges — ДОПОЛНИТЕЛЬНЫЕ картриджи герметика по объекту
	// («если нужно больше»). Прибавляются к автоматическому расходу, а не
	// заменяют его.
	ExtraSealantCartridges int
}

// Validate проверяет конфигурацию лестницы.
// Отрицательные размеры запрещены (BC-002 geometry invariants).
func (c *StairConfiguration) Validate() error {
	if c.Width.Millimeters() <= 0 {
		return fmt.Errorf("stair: width must be positive")
	}
	if c.Height.Millimeters() <= 0 {
		return fmt.Errorf("stair: height must be positive")
	}
	if c.Flight == "" {
		return fmt.Errorf("stair: flight type is required")
	}
	// DOM-001 (forensic 2026-09-24): принадлежность множеству известных
	// типов марша. Раньше проверялось только «не пусто», и неизвестное
	// значение (например «diagonal») проходило валидацию и ломало конвейер
	// ниже по стеку (500 клиенту).
	if !c.Flight.Valid() {
		return fmt.Errorf("stair: unknown flight type %q (expected straight|l_shape|u_shape)", c.Flight)
	}
	if c.StepCount < 1 {
		return fmt.Errorf("stair: step count must be positive")
	}
	if c.StringerThickness.Millimeters() < 0 {
		return fmt.Errorf("stair: stringer thickness must not be negative")
	}
	if c.StepThickness.Millimeters() < 0 {
		return fmt.Errorf("stair: step thickness must not be negative")
	}
	if c.TreadDepth.Millimeters() < 0 {
		return fmt.Errorf("stair: tread depth must not be negative")
	}
	if c.Clearance.Millimeters() < 0 {
		return fmt.Errorf("stair: clearance must not be negative")
	}
	if c.RailingHeight.Millimeters() < 0 {
		return fmt.Errorf("stair: railing height must not be negative")
	}
	if c.StringerLength.Millimeters() < 0 {
		return fmt.Errorf("stair: stringer length must not be negative")
	}
	if c.LowerStepCount < 0 {
		return fmt.Errorf("stair: lower step count must not be negative")
	}
	if c.LandingWidth.Millimeters() < 0 {
		return fmt.Errorf("stair: landing width must not be negative")
	}
	if c.LandingDepth.Millimeters() < 0 {
		return fmt.Errorf("stair: landing depth must not be negative")
	}
	if c.RoomWidth.Millimeters() < 0 {
		return fmt.Errorf("stair: room width must not be negative")
	}
	if c.RoomLength.Millimeters() < 0 {
		return fmt.Errorf("stair: room length must not be negative")
	}
	if c.ApproachSpace.Millimeters() < 0 {
		return fmt.Errorf("stair: approach space must not be negative")
	}
	// Свободное пространство перед маршем обязательно в диапазоне 1000–1200 мм
	// для ВСЕХ типов марша (прямой, L, П) — зона нужна перед входом на
	// подъём независимо от формы. Пустое/нулевое значение трактуется как
	// 1000 (см. buildConfiguration и geometry.approachShift).
	a := c.ApproachSpace.Millimeters()
	if a == 0 {
		a = 1000
	}
	if a < 1000 || a > 1200 {
		return fmt.Errorf("stair: approach space must be within 1000–1200 mm for %s", c.Flight)
	}
	if c.TurnKind != "" && !c.TurnKind.Valid() {
		return fmt.Errorf("stair: invalid turn kind (platform|winder)")
	}
	// Основания крепления (CONF-FIXING). Пустое значение допустимо и
	// трактуется как бетон; недопустимо только значение не из перечня —
	// иначе опечатка в конфиге молча убрала бы из сметы дюбели.
	if !c.FloorFixing.Valid() {
		return fmt.Errorf("stair: invalid floor fixing base %q (expected concrete|wood)", c.FloorFixing)
	}
	if !c.CeilingFixing.Valid() {
		return fmt.Errorf("stair: invalid ceiling fixing base %q (expected concrete|wood)", c.CeilingFixing)
	}
	// Герметик (CONF-SEALANT). Слой 0 — дефолт, отрицательный — ошибка:
	// отрицательная толщина дала бы отрицательный объём и в смете появился
	// бы герметик «с минусом».
	if c.SealantLayerMM < 0 {
		return fmt.Errorf("stair: sealant layer must not be negative")
	}
	if c.ExtraSealantCartridges < 0 {
		return fmt.Errorf("stair: extra sealant cartridges must not be negative")
	}
	if (c.Flight == FlightLShape || c.Flight == FlightUShape) && c.StepCount > 1 {
		if c.TurnKind == TurnWinder {
			// Поворотные ступени: LandingWidth — ширина просвета между
			// маршами (Wp>0), число поворотных ступеней WinderCount≥3.
			if c.WinderCount < 3 {
				return fmt.Errorf("stair: winder count must be at least 3 for %s", c.Flight)
			}
			if c.LandingWidth.Millimeters() <= 0 {
				return fmt.Errorf("stair: well width must be positive for winder %s", c.Flight)
			}
			if c.LowerStepCount < 1 || c.LowerStepCount >= c.StepCount {
				return fmt.Errorf("stair: lower step count must be in [1, %d] for %s", c.StepCount-1, c.Flight)
			}
		} else {
			if c.LandingWidth.Millimeters() < c.Width.Millimeters() {
				return fmt.Errorf("stair: landing width must be at least the flight width for %s", c.Flight)
			}
			if c.LandingDepth.Millimeters() > 0 && c.LandingDepth.Millimeters() < c.Width.Millimeters() {
				return fmt.Errorf("stair: landing depth must be at least the flight width for %s", c.Flight)
			}
			if c.LowerStepCount < 1 || c.LowerStepCount >= c.StepCount {
				return fmt.Errorf("stair: lower step count must be in [1, %d] for %s", c.StepCount-1, c.Flight)
			}
		}
	}
	// Значения перил/направлений (CONF-RAILING/DIRECTION): пустое
	// значение допустимо (не задано), любое другое должно быть валидным.
	if (c.Railing != "" && !c.Railing.Valid()) ||
		(c.RailingType != "" && c.RailingType.Effective() != c.RailingType) ||
		(c.RailingLower != "" && !c.RailingLower.Valid()) ||
		(c.RailingLanding != "" && !c.RailingLanding.Valid()) ||
		(c.RailingUpper != "" && !c.RailingUpper.Valid()) {
		return fmt.Errorf("stair: invalid railing sides (none|left|right|both)")
	}
	if c.Direction != "" && !c.Direction.Valid() {
		return fmt.Errorf("stair: invalid turn direction (left|right)")
	}
	// Назначение здания (S-236): неизвестное значение молча уехало бы в
	// профиль «дом» — пользователь бы выбрал «общественное», получил бы
	// проверку частного дома и не узнал бы об этом.
	if !c.Purpose.Valid() {
		return fmt.Errorf("stair: invalid building purpose (house|flat|public|industrial)")
	}
	return nil
}

// NewStairConfiguration создаёт и валидирует конфигурацию главной лестницы.
func NewStairConfiguration(width, height Length, flight FlightType) (*StairConfiguration, error) {
	cfg := &StairConfiguration{
		Width:      width,
		Height:     height,
		Flight:     flight,
		StepCount:  1,
		StepHeight: height,
		Riser:      true, // подступенки по умолчанию (продуктовый дефолт конструктора)
	}
	return cfg, cfg.Validate()
}
