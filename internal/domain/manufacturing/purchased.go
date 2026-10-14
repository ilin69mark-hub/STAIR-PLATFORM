package manufacturing

import (
	"fmt"
	"sort"

	"stairplatform/internal/domain/engineering"
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

// PartKind — вид детали из PartKind.
//
// Сопоставление каталога и видов детали нужно потому, что у КАЖДОГО
// экземпляра свой номер (PST-01, PST-02, …), и цена искалась бы по номеру
// экземпляра, которого в справочнике нет. Цена у ПОЗИЦИИ: стойка стоит
// столько-то независимо от того, сколько её на лестнице.
type PurchasedPart struct {
	// CatalogID — устойчивый идентификатор из каталога (RAIL-P, POST-001).
	CatalogID string
	// Name — человекочитаемое имя для сметы.
	Name string
	// Kind — вид детали в производстве.
	Kind PartKind
}

// purchasedKindByCatalogID — вид детали для каждой позиции каталога.
//
// Ключ — ID из engineering.PurchasedParts(). Таблица намеренно полная: тест
// сверяет её с каталогом, и позиция каталога без вида детали роняет сборку.
// Иначе новая покупная деталь появилась бы в каталоге и молча ушла бы в
// смету без вида и без цены.
var purchasedKindByCatalogID = map[string]PartKind{
	"RAIL-P":   PartRailTop,
	"POST-001": PartPost,
	"RIGL-001": PartRigel,
	"TIP-001":  PartTip,
	"FLNG-001": PartFlange,
	"SLIP-001": PartSlip,
	"DECO-001": PartDeco,
	"ENDP-001": PartEndCap,
}

// PurchasedParts — позиции каталога покупных деталей с видом детали.
//
// Собирается ИЗ engineering.PurchasedParts(): каталог объявлен источником
// правды о деталях, и до S-180 у него не было ни одного вызывающего. Здесь он
// наконец задействован — иначе список покупных деталей жил бы в двух местах.
func PurchasedParts() []PurchasedPart {
	byID := map[PartKind]PurchasedPart{}
	for _, spec := range engineering.PurchasedParts() {
		kind, ok := purchasedKindByCatalogID[spec.ID]
		if !ok {
			// Позиция каталога без вида детали: пропускаем и сообщаем через
			// тест сверки. Молча пропустить нельзя — деталь выпала бы из сметы.
			continue
		}
		byID[kind] = PurchasedPart{CatalogID: spec.ID, Name: spec.Name, Kind: kind}
	}
	out := make([]PurchasedPart, 0, len(byID))
	for _, p := range byID {
		out = append(out, p)
	}
	// Стабильный порядок: строки сметы не должны прыгать между расчётами.
	sort.Slice(out, func(i, j int) bool { return out[i].CatalogID < out[j].CatalogID })
	return out
}

// PurchasedPartCatalogID — ID каталога для вида детали, "" если вида нет.
func PurchasedPartCatalogID(kind PartKind) string {
	for _, p := range PurchasedParts() {
		if p.Kind == kind {
			return p.CatalogID
		}
	}
	return ""
}
