// Package pricing реализует EDM для Pricing Platform (BC-…, PRC-0001):
// детерминированный расчёт стоимости изделия на основе производственных
// данных Manufacturing Platform. Все финансовые значения (цены, ставки,
// наценки, налоги) появляются здесь — Manufacturing их не содержит
// (правила MFG-0015). Внутренние вычисления — в базовой валюте проекта
// (PRC-0013); конвертация валют — вне MVP.
// Домен не зависит от HTTP, БД, ORM, UI и AI (ADR-0006).
package pricing

import (
	"fmt"
	"math"
)

// Currency — валюта (ISO 4217): код и количество десятичных знаков
// (минорные единицы). Все расчёты проекта выполняются в одной базовой
// валюте (PRC-0013), поэтому операции Money не конвертируют валюты.
type Currency struct {
	Code     string
	Decimals int
}

// Базовые валюты платформы.
var (
	CurrencyRUB = Currency{Code: "RUB", Decimals: 2}
	CurrencyEUR = Currency{Code: "EUR", Decimals: 2}
	CurrencyUSD = Currency{Code: "USD", Decimals: 2}
)

// Validate проверяет корректность валюты.
func (c Currency) Validate() error {
	if len(c.Code) != 3 {
		return fmt.Errorf("pricing: currency code must be ISO 4217 (3 letters), got %q", c.Code)
	}
	if c.Decimals < 0 || c.Decimals > 4 {
		return fmt.Errorf("pricing: currency %q has invalid precision %d", c.Code, c.Decimals)
	}
	return nil
}

func (c Currency) scale() float64 {
	return math.Pow10(c.Decimals)
}

// Money — неизменяемая денежная сумма в минорных единицах валюты
// (центы). Целочисленное представление гарантирует детерминированный и
// точный расчёт стоимости без ошибок округления float.
type Money int64

// NewMoney создаёт сумму из минорных единиц (переносим из фиксированной
// точки без потери точности).
func NewMoney(minor int64) Money { return Money(minor) }

// FromMajor создаёт сумму из старших единиц (десятичная запись) валюты c
// округлением half-up к точности валюты. Отрицательные значения запрещены.
func (c Currency) FromMajor(v float64) (Money, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("pricing: invalid amount %v", v)
	}
	if v < 0 {
		return 0, fmt.Errorf("pricing: amount must not be negative, got %v", v)
	}
	scaled := v * c.scale()
	if scaled > float64(math.MaxInt64) {
		return 0, fmt.Errorf("pricing: amount %v overflows integer range", v)
	}
	return Money(int64(math.Floor(scaled + 0.5))), nil
}

// Minor возвращает значение в минорных единицах.
func (m Money) Minor() int64 { return int64(m) }

// Major возвращает значение в старших единицах валюты c (только для отображения).
func (m Money) Major(c Currency) float64 { return float64(m) / c.scale() }

// Add складывает суммы (одна базовая валюта).
func (m Money) Add(o Money) Money { return m + o }

// Sub вычитает сумму. Результат может быть отрицательным при некорректной
// цепочке ставок; валидация PriceBreakdown это предотвращает.
func (m Money) Sub(o Money) Money { return m - o }

// MulRate умножает сумму на ставку Rate с округлением half-up к целой
// минорной единице. Детерминированная целочисленная арифметика.
func (m Money) MulRate(r Rate) Money {
	num := int64(m) * int64(r)
	if num < 0 {
		return Money(-((-num + rateScale/2) / rateScale))
	}
	return Money((num + rateScale/2) / rateScale)
}
