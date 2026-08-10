// Package engineering реализует EDM — Engineering Domain Model (ADR-0015).
// Внутренние вычисления: единицы системы СИ по ADR-0008
// (мм, радианы, float64; без внутренних округлений; точность 1e-6 мм).
// Домен не зависит от HTTP, БД, ORM, UI и AI (ADR-0006).
package engineering

import (
	"fmt"
	"math"
)

// Precision — геометрическая точность платформы (ADR-0008): 0.000001 мм.
const Precision = 1e-6

// Length — неизменяемый размерный тип, значение в миллиметрах.
type Length float64

// NewLength создаёт Length и допускает только валидное конечное значение.
// Отрицательные габариты запрещены (BC-002 Geometry invariants).
func NewLength(mm float64) (Length, error) {
	if math.IsNaN(mm) || math.IsInf(mm, 0) {
		return 0, fmt.Errorf("length: invalid value %v", mm)
	}
	if mm < 0 {
		return 0, fmt.Errorf("length: must not be negative, got %v", mm)
	}
	return Length(mm), nil
}

// Millimeters возвращает значение в мм.
func (l Length) Millimeters() float64 { return float64(l) }

// Equals сравнивает длины с учётом инженерной точности (1e-6 мм).
func (l Length) Equals(o Length) bool {
	return math.Abs(float64(l-o)) <= Precision
}

// Angle — неизменяемый угол; внутренняя единица — радианы (ADR-0008),
// градусы — только для отображения.
type Angle float64

// NewAngleRadians создаёт угол из радиан.
func NewAngleRadians(rad float64) (Angle, error) {
	if math.IsNaN(rad) || math.IsInf(rad, 0) {
		return 0, fmt.Errorf("angle: invalid value %v", rad)
	}
	return Angle(rad), nil
}

// NewAngleDegrees создаёт угол из градусов (конверсия на границе системы).
func NewAngleDegrees(deg float64) (Angle, error) {
	if math.IsNaN(deg) || math.IsInf(deg, 0) {
		return 0, fmt.Errorf("angle: invalid value %v", deg)
	}
	return Angle(deg * math.Pi / 180), nil
}

// Radians возвращает значение в радианах.
func (a Angle) Radians() float64 { return float64(a) }

// Degrees возвращает значение в градусах (только для отображения).
func (a Angle) Degrees() float64 { return float64(a) * 180 / math.Pi }

// Mass — неизменяемый тип массы, значение в килограммах.
type Mass float64

// NewMass создаёт массу; отрицательная масса запрещена.
func NewMass(kg float64) (Mass, error) {
	if math.IsNaN(kg) || math.IsInf(kg, 0) {
		return 0, fmt.Errorf("mass: invalid value %v", kg)
	}
	if kg < 0 {
		return 0, fmt.Errorf("mass: must not be negative, got %v", kg)
	}
	return Mass(kg), nil
}

// Kilograms возвращает значение в кг.
func (m Mass) Kilograms() float64 { return float64(m) }
