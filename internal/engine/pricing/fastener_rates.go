package pricing

import (
	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
)

// DefaultFastenerPriceRub — запасная цена крепежа, 15 ₽ за штуку (решение
// владельца 2026-10-08).
//
// Это НЕ настоящая цена: настоящая живёт в прайсе магазина и меняется в
// админке без правки кода. Здесь она нужна только на случай пустого прайса,
// чтобы расчёт не обнулял крепёж. Позиции каталога с ценой выше 15 ₽
// владелец проставит в админке.
const DefaultFastenerPriceRub = 15

// DefaultFastenerPrices возвращает запасные цены крепежа по всем позициям
// встроенного каталога: 15 ₽ за штуку.
//
// Берутся коды ИЗ КАТАЛОГА, а не из захардкоженного списка: если в каталог
// добавят новую позицию (например, глухарь 12х80), цена появится у неё
// автоматически, без правки движка. Раньше цены перечислялись вручную, и
// новая позиция каталога молча считалась по нулевой цене.
func DefaultFastenerPrices() map[string]domprc.Money {
	price := domprc.NewMoney(DefaultFastenerPriceRub * 100) // рубли → копейки
	out := map[string]domprc.Money{}
	for _, f := range dommfg.DefaultFastenerRegistry().Fasteners() {
		out[f.Code] = price
	}
	return out
}

// DefaultSealantPriceRub — запасная цена герметика, 500 ₽ за картридж
// (решение владельца 2026-10-08).
//
// Это НЕ настоящая цена: настоящая живёт в ставках магазина и меняется
// владельцем без правки кода. Здесь она нужна, чтобы расчёт не обнулял
// герметик на пустом справочнике — как и цена крепежа 15 ₽.
const DefaultSealantPriceRub = 500

// DefaultSealantPrices возвращает запасные цены герметика по всем позициям
// встроенного каталога: 500 ₽ за картридж.
func DefaultSealantPrices() map[string]domprc.Money {
	price := domprc.NewMoney(DefaultSealantPriceRub * 100) // рубли → копейки
	out := map[string]domprc.Money{}
	for _, sl := range dommfg.DefaultSealantRegistry().Sealants() {
		out[sl.Code] = price
	}
	return out
}
