package manufacturing

import (
	"fmt"
	"sort"
)

// PurchasedDemand — потребность в одной покупной позиции (S-162).
//
// Физическая величина, как FastenerDemand: сколько ШТУК покупать. Денег
// здесь нет — цену подставляет Pricing по ставкам справочника.
//
// Отдельный тип, потому что у покупных деталей есть признак, которого нет у
// крепежа: материал. У ограждения это STAINLESS-304, и закупочная цена
// считается от него, а не от материала каркаса.
type PurchasedDemand struct {
	// Code — код позиции в пакете (например RAL-POST-01). Идёт в смету,
	// чтобы за позицией можно было вернуться к спецификации.
	Code PartNumber
	// Name — название для сметы.
	Name string
	// Kind — вид детали (PartRailTop, PartPost, …).
	Kind PartKind
	// Material — материал позиции (STAINLESS-304).
	Material MaterialCode
	// Quantity — штук.
	Quantity Quantity
}

// PurchasedDemandFor собирает потребность по покупным деталям.
//
// Позиции с одинаковым кодом и материалом схлопываются: восемь стоек — это
// одна строка «стойка, 8 шт», а не восемь строк по одной. Иначе смета
// разрасталась бы на треть.
func PurchasedDemandFor(parts []Part) []PurchasedDemand {
	byKey := map[string]*PurchasedDemand{}
	var order []string

	for i := range parts {
		p := &parts[i]
		if !p.Purchased {
			continue
		}
		key := string(p.Kind) + "|" + string(p.Material) + "|" + string(p.Number)
		if d, ok := byKey[key]; ok {
			d.Quantity += 1
			continue
		}
		byKey[key] = &PurchasedDemand{
			Code:     p.Number,
			Name:     string(p.Number),
			Kind:     p.Kind,
			Material: p.Material,
			Quantity: 1,
		}
		order = append(order, key)
	}

	// Стабильный порядок: иначе строки сметы прыгали бы между расчётами
	// одного и того же проекта.
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	out := make([]PurchasedDemand, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out
}

// TotalPurchasedQuantity — всего покупных деталей. Используется в проверках
// и в сводке расчёта.
func TotalPurchasedQuantity(ds []PurchasedDemand) Quantity {
	var n Quantity
	for _, d := range ds {
		n += d.Quantity
	}
	return n
}

// String — для диагностики и логов.
func (d PurchasedDemand) String() string {
	return fmt.Sprintf("%s (%s) × %d", d.Name, d.Material, d.Quantity)
}
