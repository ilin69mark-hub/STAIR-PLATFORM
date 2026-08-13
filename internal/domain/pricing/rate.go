package pricing

import (
	"fmt"
	"math"
)

// rateScale — масштаб Rate: 1.0 = 100000 (проценты с точностью 0.001%).
const rateScale = 100000

// Rate — целочисленное значение процента с точностью 0.001%
// (10.5% = 10500). Целочисленная арифметика даёт детерминированный,
// воспроизводимый расчёт наценок, скидок, налогов и overhead.
type Rate int64

// NewRate создаёт ставку из процентов (десятичная запись) с округлением
// half-up к 0.001%. Отрицательные ставки запрещены.
func NewRate(percent float64) (Rate, error) {
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		return 0, fmt.Errorf("pricing: invalid rate %v", percent)
	}
	if percent < 0 {
		return 0, fmt.Errorf("pricing: rate must not be negative, got %v", percent)
	}
	scaled := percent * 1000
	if scaled > float64(math.MaxInt64) {
		return 0, fmt.Errorf("pricing: rate %v overflows integer range", percent)
	}
	return Rate(int64(math.Floor(scaled + 0.5))), nil
}

// MustRate создаёт ставку из процентов; паникует при невалидном значении
// (для констант-каталогов).
func MustRate(percent float64) Rate {
	r, err := NewRate(percent)
	if err != nil {
		panic(fmt.Sprintf("pricing: %v", err))
	}
	return r
}

// Percent возвращает значение в процентах (для отображения).
func (r Rate) Percent() float64 { return float64(r) / 1000 }

// Validate проверяет корректность ставки.
func (r Rate) Validate() error {
	if r < 0 {
		return fmt.Errorf("pricing: rate must not be negative")
	}
	return nil
}
