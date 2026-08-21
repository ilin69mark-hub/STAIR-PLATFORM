package engineering

import "fmt"

// FlightType — тип лестничного марша.
type FlightType string

const (
	FlightStraight FlightType = "straight"
	FlightLShape   FlightType = "l_shape"
	FlightUShape   FlightType = "u_shape"
	FlightSpiral   FlightType = "spiral"
)

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

// SpiralDirection — направление закрутки спиральной лестницы
// (CONF-SPIRAL-DIRECTION): по часовой (cw) или против часовой (ccw)
// стрелки при виде сверху.
type SpiralDirection string

const (
	SpiralCW  SpiralDirection = "cw"
	SpiralCCW SpiralDirection = "ccw"
)

// Valid — допускает ли значение направление спирали.
func (d SpiralDirection) Valid() bool {
	return d == SpiralCW || d == SpiralCCW
}

// DefaultRailing — сторона перил спиральной лестницы по её направлению:
// по часовой — справа, против часовой — слева (CONF-SPIRAL-RAILING).
// Перила всегда только с одной стороны — по наружному краю марша.
func (d SpiralDirection) DefaultRailing() RailingSide {
	if d == SpiralCW {
		return RailingRight
	}
	return RailingLeft
}

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
	LandingWidth   Length
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
	// Спиральная лестница (EDR-0007): наружный радиус марша R. Радиус
	// колонны r = R − Width. Используется только при Flight == FlightSpiral.
	OuterRadius Length
	// Material — выбранный конструктором материал (код каталога MFG-0005,
	// например "STEEL-S235"); пустое значение — автоназначение по толщине.
	Material string
	// Перила (CONF-RAILING): сторона установки. Стороны отсчитываются со
	// стороны первой ступени по ходу подъёма (слева — левые, справа —
	// правые). Для прямого марша — единственный выбор Railing; для маршей
	// с площадкой перила задаются по сегментам; для спирали Railing
	// вычисляется из SpiralDirection (CONF-SPIRAL-RAILING).
	Railing RailingSide
	// Перила по сегментам маршей с площадкой (только L/U):
	// первый марш → площадка → второй марш.
	RailingLower   RailingSide
	RailingLanding RailingSide
	RailingUpper   RailingSide
	// Направление поворота площадки L/U-марша (CONF-DIRECTION).
	Direction TurnDirection
	// Направление закрутки спирали (CONF-SPIRAL-DIRECTION).
	SpiralDirection SpiralDirection
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
	if c.TurnKind != "" && !c.TurnKind.Valid() {
		return fmt.Errorf("stair: invalid turn kind (platform|winder)")
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
			if c.LowerStepCount < 1 || c.LowerStepCount >= c.StepCount {
				return fmt.Errorf("stair: lower step count must be in [1, %d] for %s", c.StepCount-1, c.Flight)
			}
		}
	}
	if c.Flight == FlightSpiral {
		// EDR-0007 §4.4: колонна имеет положительный радиус (R > W).
		if c.OuterRadius.Millimeters() <= c.Width.Millimeters() {
			return fmt.Errorf("stair: outer radius must exceed the stair width for %s", c.Flight)
		}
	}
	// Значения перил/направлений (CONF-RAILING/DIRECTION/SPIRAL): пустое
	// значение допустимо (не задано), любое другое должно быть валидным.
	if (c.Railing != "" && !c.Railing.Valid()) ||
		(c.RailingLower != "" && !c.RailingLower.Valid()) ||
		(c.RailingLanding != "" && !c.RailingLanding.Valid()) ||
		(c.RailingUpper != "" && !c.RailingUpper.Valid()) {
		return fmt.Errorf("stair: invalid railing sides (none|left|right|both)")
	}
	if c.Direction != "" && !c.Direction.Valid() {
		return fmt.Errorf("stair: invalid turn direction (left|right)")
	}
	if c.SpiralDirection != "" && !c.SpiralDirection.Valid() {
		return fmt.Errorf("stair: invalid spiral direction (cw|ccw)")
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
