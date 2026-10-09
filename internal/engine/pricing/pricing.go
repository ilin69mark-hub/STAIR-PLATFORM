// Package pricing реализует Pricing Engine (PRC-0001): детерминированный
// расчёт стоимости изделия на основе производственного набора данных
// Manufacturing Platform (ManufacturingCostDataset). Движок не выполняет
// инженерных расчётов (PRC-0001 Non-Goals); стоимость считается по
// цепочке Material → Machine → Labor → Overhead → Margin → Discount → Tax
// в базовой валюте проекта (PRC-0013). Результат воспроизводим.
package pricing

import (
	"fmt"
	"math"

	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
)

// Rates — ставки расчёта стоимости (PRC-0004). Детерминированные
// константы каталога, переопределяемые до вызова Price.
type Rates struct {
	Currency domprc.Currency

	// Material — цена материала за килограмм по коду (PRC-0006).
	Material map[dommfg.MaterialCode]domprc.Money
	// Fastener — цена крепежа за ШТУКУ по коду позиции каталога (MNT-0001).
	// Отдельная карта: у крепежа своя единица (шт), и смешать его с ценой за
	// кг нельзя — иначе 210 глухарей уехали бы в килограммы.
	Fastener map[string]domprc.Money
	// Sealant — цена герметика за КАРТРИДЖ по коду позиции (CONF-SEALANT).
	// Отдельная карта: единица картриджа, и смешивать её с ценой за кг или за
	// штуку нельзя.
	Sealant map[string]domprc.Money
	// MachinePerHour — ставка оборудования за час (PRC-0008).
	MachinePerHour domprc.Money
	// LaborPerHour — ставка труда за час (PRC-0007).
	LaborPerHour domprc.Money

	OverheadPercent domprc.Rate // PRC-0009 — % от (Material+Machine+Labor)
	MarginPercent   domprc.Rate // PRC-0010 — % от Production Cost
	DiscountPercent domprc.Rate // PRC-0011 — % от Gross
	TaxPercent      domprc.Rate // PRC-0012 — % от PreTax
}

// Validate проверяет корректность ставок.
func (r Rates) Validate() error {
	if err := r.Currency.Validate(); err != nil {
		return err
	}
	if len(r.Material) == 0 {
		return fmt.Errorf("pricing: rates have no material prices")
	}
	for code, price := range r.Material {
		if price < 0 {
			return fmt.Errorf("pricing: material %q has negative price", code)
		}
	}
	if r.MachinePerHour < 0 || r.LaborPerHour < 0 {
		return fmt.Errorf("pricing: hourly rates must not be negative")
	}
	for code, price := range r.Fastener {
		if price < 0 {
			return fmt.Errorf("pricing: fastener %q has negative price", code)
		}
	}
	for code, price := range r.Sealant {
		if price < 0 {
			return fmt.Errorf("pricing: sealant %q has negative price", code)
		}
	}
	for _, rate := range []domprc.Rate{
		r.OverheadPercent, r.MarginPercent, r.DiscountPercent, r.TaxPercent,
	} {
		if err := rate.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// DefaultRates возвращает демонстрационные ставки в RUB для каталога
// DefaultMaterialRegistry (переопределяются в проде справочниками).
func DefaultRates() Rates {
	return Rates{
		Currency: domprc.CurrencyRUB,
		Material: map[dommfg.MaterialCode]domprc.Money{
			"STEEL-S235":  domprc.NewMoney(10000), // 100 ₽/кг
			"WOOD-OAK":    domprc.NewMoney(8000),  // 80 ₽/кг
			"WOOD-WALNUT": domprc.NewMoney(22000), // 220 ₽/кг
			"WOOD-ASH":    domprc.NewMoney(6000),  // 60 ₽/кг
			"WOOD-SOFT":   domprc.NewMoney(2500),  // 25 ₽/кг
		},
		// Крепёж: 15 ₽ за штуку (решение владельца 2026-10-08). Это
		// ЗАПАСНАЯ цена на случай пустого прайса магазина — сама цифра
		// меняется в админке, здесь она только чтобы расчёт не обнулялся.
		Fastener:        DefaultFastenerPrices(),
		Sealant:         DefaultSealantPrices(),
		MachinePerHour:  domprc.NewMoney(800000), // 8 000 ₽/час
		LaborPerHour:    domprc.NewMoney(300000), // 3 000 ₽/час
		OverheadPercent: domprc.MustRate(20),
		MarginPercent:   domprc.MustRate(30),
		DiscountPercent: domprc.MustRate(5),
		TaxPercent:      domprc.MustRate(20),
	}
}

// Price рассчитывает стоимость изделия по производственному набору
// (PRC-0001). Предусловия: валидный dataset и ставки. Расчёт:
//   - Material: расход листа с отходами (sheetMass = Mass × SheetArea/PartArea)
//     × цена за кг;
//   - Machine/Labor: время (мин) × ставка за час ÷ 60;
//   - Overhead % от (Material+Machine+Labor) → Production Cost;
//   - Margin % от Production → Gross; Discount % от Gross → PreTax;
//   - Tax % от PreTax → Final Price.
//
// Каждый компонент детализируется позицией с источником. Результат
// детерминирован и валидируется.
func Price(ds *dommfg.ManufacturingCostDataset, rates Rates) (*domprc.PriceBreakdown, error) {
	if ds == nil {
		return nil, fmt.Errorf("pricing: cost dataset is required")
	}
	if err := ds.Validate(); err != nil {
		return nil, fmt.Errorf("pricing: %w", err)
	}
	if err := rates.Validate(); err != nil {
		return nil, err
	}

	b := &domprc.PriceBreakdown{Currency: rates.Currency}

	for _, c := range ds.MaterialConsumption {
		price, ok := rates.Material[c.MaterialCode]
		if !ok {
			return nil, fmt.Errorf("pricing: no price for material %q", c.MaterialCode)
		}
		if c.PartArea <= 0 {
			return nil, fmt.Errorf("pricing: material %q has no part area", c.MaterialCode)
		}
		// Расход листа с учётом отходов: масса деталей масштабируется
		// отношением площадей листов к площади деталей.
		sheetMass := c.Mass * (c.SheetArea / c.PartArea)
		amount := moneyFromQuantity(sheetMass, price)
		b.Material = b.Material.Add(amount)
		b.Lines = append(b.Lines, domprc.CostComponent{
			Name:     "Material " + string(c.MaterialCode),
			Category: domprc.CategoryMaterial,
			Amount:   amount,
			Source:   "material:" + string(c.MaterialCode),
		})
	}

	// Крепёж (MNT-0001). Считается ПОШТУЧНО по потребности из отверстий
	// пяток. Сколько отверстий вырезано лазером — столько и штук в смете.
	// Позиция без цены в прайсе не считается нулём молча: сумма не
	// увеличивается, но строка в смете остаётся, чтобы в расхождении
	// «просверлено 210 отверстий, а в смете 0 ₽» было видно.
	for _, f := range ds.FastenerConsumption {
		price, ok := rates.Fastener[f.Code]
		if !ok {
			continue
		}
		amount := moneyFromQuantity(float64(f.Quantity), price)
		b.Fastener = b.Fastener.Add(amount)
		b.Lines = append(b.Lines, domprc.CostComponent{
			Name:     fmt.Sprintf("Крепёж: %s — %s", f.Name, f.Purpose),
			Category: domprc.CategoryFastener,
			Amount:   amount,
			Source:   fmt.Sprintf("fastener:%s,qty:%d,purpose:%s", f.Code, f.Quantity, f.Purpose),
		})
	}

	// Герметик (CONF-SEALANT). Количество — картриджи с округлением вверх:
	// покупают картридж, а не «0,64 литра».
	// Герметик (CONF-SEALANT) — ДВЕ строки, а не одна:
	//
	//   1. автоматический расход по геометрии + запас на брак;
	//   2. добавленный по объекту.
	//
	// Разделено уже сейчас, потому что впереди владелец заведёт строки
	// перерасхода и недорасхода для контроля склада и финансов. В одной
	// строке «3 картриджа» нельзя было бы отличить лежащий на швы запас от
	// докупленного руками, а расхождение с фактом на объекте как раз и есть
	// вопрос «откуда взялась разница».
	if sl := ds.SealantConsumption; sl != nil {
		price, ok := rates.Sealant[sl.Code]
		if ok {
			addLine := func(name string, count int, src string) {
				if count <= 0 {
					return
				}
				amount := moneyFromQuantity(float64(count), price)
				b.Sealant = b.Sealant.Add(amount)
				b.Lines = append(b.Lines, domprc.CostComponent{
					Name:     name,
					Category: domprc.CategorySealant,
					Amount:   amount,
					Source:   src,
				})
			}
			addLine(
				fmt.Sprintf("Герметик: %s — расход по швам + запас %.0f%%", sl.Name, sl.ReservePercent),
				sl.AutoCartridges,
				fmt.Sprintf("sealant:%s,cartridges:%d,area_m2:%.4f,layer_mm:%.1f,volume_l:%.4f,reserve_pct:%.0f",
					sl.Code, sl.AutoCartridges, sl.AreaM2, sl.LayerMM, sl.VolumeL, sl.ReservePercent),
			)
			addLine(
				fmt.Sprintf("Герметик: %s — добавлено по объекту", sl.Name),
				sl.ExtraCartridges,
				fmt.Sprintf("sealant:%s,cartridges:%d,purpose:object", sl.Code, sl.ExtraCartridges),
			)
		}
	}

	machine := moneyFromMinutes(ds.EstimatedMachineTime, rates.MachinePerHour)
	b.Machine = machine
	b.Lines = append(b.Lines, domprc.CostComponent{
		Name: "Machine time", Category: domprc.CategoryMachine,
		Amount: machine, Source: "machine:laser-cutting",
	})

	labor := moneyFromMinutes(ds.EstimatedLaborTime, rates.LaborPerHour)
	b.Labor = labor
	b.Lines = append(b.Lines, domprc.CostComponent{
		Name: "Labor time", Category: domprc.CategoryLabor,
		Amount: labor, Source: "labor:manual-finishing",
	})

	base := b.Material.Add(b.Machine).Add(b.Labor).Add(b.Fastener).Add(b.Sealant)
	b.Overhead = base.MulRate(rates.OverheadPercent)
	b.Lines = append(b.Lines, domprc.CostComponent{
		Name: "Overhead", Category: domprc.CategoryOverhead,
		Amount: b.Overhead, Source: "overhead:rate",
	})
	b.ProductionCost = base.Add(b.Overhead)

	b.Margin = b.ProductionCost.MulRate(rates.MarginPercent)
	b.Lines = append(b.Lines, domprc.CostComponent{
		Name: "Margin", Category: domprc.CategoryMargin,
		Amount: b.Margin, Source: "margin:rate",
	})
	gross := b.ProductionCost.Add(b.Margin)

	b.Discount = gross.MulRate(rates.DiscountPercent)
	b.Lines = append(b.Lines, domprc.CostComponent{
		Name: "Discount", Category: domprc.CategoryDiscount,
		Amount: b.Discount, Source: "discount:rate",
	})
	b.PreTax = gross.Sub(b.Discount)

	b.Tax = b.PreTax.MulRate(rates.TaxPercent)
	b.Lines = append(b.Lines, domprc.CostComponent{
		Name: "Tax", Category: domprc.CategoryTax,
		Amount: b.Tax, Source: "tax:rate",
	})
	b.FinalPrice = b.PreTax.Add(b.Tax)

	if err := b.Validate(); err != nil {
		return nil, err
	}
	return b, nil
}

// moneyFromQuantity считает сумму: quantity × unitPrice (минорные единицы
// за единицу количества) с округлением half-up к целой минорной единице.
func moneyFromQuantity(quantity float64, unitPrice domprc.Money) domprc.Money {
	if quantity <= 0 || math.IsNaN(quantity) || math.IsInf(quantity, 0) {
		return 0
	}
	return domprc.NewMoney(int64(math.Floor(quantity*float64(unitPrice) + 0.5)))
}

// moneyFromMinutes считает стоимость времени: minutes × ставка/час ÷ 60
// с округлением half-up к целой минорной единице (PRC-0008/PRC-0007).
func moneyFromMinutes(minutes float64, perHour domprc.Money) domprc.Money {
	if minutes <= 0 || math.IsNaN(minutes) || math.IsInf(minutes, 0) {
		return 0
	}
	return domprc.NewMoney(int64(math.Floor(minutes*float64(perHour)/60 + 0.5)))
}
