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
	}
	return cfg, cfg.Validate()
}
