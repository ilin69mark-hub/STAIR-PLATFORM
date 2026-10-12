package manufacturing

import (
	"fmt"
	"strings"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
)

// groupKey идентифицирует группу геометрически одинаковых деталей.
type groupKey struct {
	kind      dommfg.PartKind
	material  dommfg.MaterialCode
	thickness engineering.Length
	length    engineering.Length
	width     engineering.Length
}

// cutKey — ключ группировки КАРТЫ РАСКРОЯ: габариты плюс контур (S-163).
type cutKey struct {
	groupKey
	profileID string
}

// profileID — короткая подпись контура для ключа группировки. Пустой контур
// даёт пустую строку.
func profileID(p []dommfg.Point2D) string {
	if len(p) == 0 {
		return ""
	}
	var b strings.Builder
	for _, v := range p {
		fmt.Fprintf(&b, "%.1f,%.1f;", v.X, v.Y)
	}
	return b.String()
}

// jointFlags — признаки сварного стыка «шип-паз», накопленные по группе
// одинаковых деталей.
type jointFlags struct {
	tongue  bool
	mortise bool
	onWidth bool
	count   int
}

// kindDescription — человекочитаемое описание детали для BOM.
func kindDescription(kind dommfg.PartKind) string {
	switch kind {
	case dommfg.PartStringer:
		return "Stringer"
	case dommfg.PartTread:
		return "Tread"
	case dommfg.PartRiser:
		return "Riser"
	case dommfg.PartHeel:
		return "Heel (seat bracket)"
	case dommfg.PartWeb:
		return "Web (tooth bracket)"
	case dommfg.PartAnchor:
		return "Anchor (floor/landing bracket)"
	case dommfg.PartFirstWeb:
		return "First-step plate"
	// ---- Ограждение (покупные позиции) ----
	case dommfg.PartRailTop:
		return "Перила (поручень)"
	case dommfg.PartPost:
		return "Стойка опоры"
	case dommfg.PartRigel:
		return "Ригель"
	case dommfg.PartTip:
		return "Наконечник регулируемый"
	case dommfg.PartFlange:
		return "Фланец опоры"
	case dommfg.PartSlip:
		return "Накидная часть"
	case dommfg.PartDeco:
		return "Декоративная накладка"
	case dommfg.PartEndCap:
		return "Торцевая заглушка"
	}
	return string(kind)
}

// buildBOM группирует одинаковые детали в строки спецификации и строит
// карту раскроя (MFG-0002 BOM Generation). Группа идентифицируется типом,
// материалом и габаритами; порядок строк детерминирован — по первому
// появлению группы в Parts.
func buildBOM(parts []dommfg.Part) (dommfg.BOM, dommfg.CutList) {
	var order []groupKey
	groups := make(map[groupKey][]dommfg.PartNumber)

	// Раскрой группируется отдельно: по габаритам И контуру (S-163).
	//
	// Спецификация (BOM) остаётся на габаритах, как и раньше: описание детали
	// не меняется от того, что мы теперь знаем её контур. Иначе две ступени
	// одинакового размера разъехались бы на две строки спецификации — а это
	// уже не уточнение раскроя, а изменение документа для клиента.
	//
	// В раскрое наоборот: две детали одного размера с разным контуром режутся
	// по-разному, и в одной позиции им нельзя.
	var cutOrder []cutKey
	cutGroups := make(map[cutKey][]dommfg.PartNumber)
	profiles := make(map[cutKey][]dommfg.Point2D)
	purchased := make(map[cutKey]bool)
	joints := make(map[cutKey]jointFlags)

	for _, p := range parts {
		k := groupKey{p.Kind, p.Material, p.Thickness, p.Length, p.Width}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], p.Number)

		ck := cutKey{k, profileID(p.Profile)}
		if _, ok := cutGroups[ck]; !ok {
			cutOrder = append(cutOrder, ck)
		}
		cutGroups[ck] = append(cutGroups[ck], p.Number)
		// Контур публикуется только если его габарит совпадает с габаритом
		// позиции. Проверка стоит здесь, в точке выхода, а не только в
		// decompose: у косоура заготовка пересчитывается позже, чем снимается
		// контур тела, и контур всего марша вдвое длиннее одной пластины.
		// Контур не той детали хуже отсутствия контура — он правдоподобен.
		if len(p.Profile) > 0 && profiles[ck] == nil &&
			dommfg.ProfilesMatchBounds(p.Profile, p.Length.Millimeters(), p.Width.Millimeters(), 0.5) {
			profiles[ck] = p.Profile
		}
		if p.Purchased {
			purchased[ck] = true
		}
		j := joints[ck]
		j.count = p.JointCount
		j.onWidth = p.JointOnWidth
		j.tongue = j.tongue || p.HasTongue
		j.mortise = j.mortise || p.HasMortise
		joints[ck] = j
	}

	bom := dommfg.BOM{Lines: make([]dommfg.BOMLine, 0, len(order))}
	for i, k := range order {
		nums := groups[k]
		bom.Lines = append(bom.Lines, dommfg.BOMLine{
			Number:       i + 1,
			PartNumber:   nums[0],
			Description:  kindDescription(k.kind),
			MaterialCode: k.material,
			Thickness:    k.thickness,
			Quantity:     dommfg.Quantity(len(nums)),
			Length:       k.length,
			Width:        k.width,
		})
	}

	// Покупные детали (ограждение, крепёж) идут в спецификацию, но не в
	// карту раскроя: листа под них не режем. Раньше buildBOM клал каждую
	// деталь и туда, и сюда, и Nest падал с «no stock sheet for material
	// STAINLESS-304» — покупные приходят готовыми, а не из листа.
	cut := dommfg.CutList{Items: make([]dommfg.CutItem, 0, len(cutOrder))}
	for _, ck := range cutOrder {
		if purchased[ck] {
			continue
		}
		nums := cutGroups[ck]
		j := joints[ck]
		cut.Items = append(cut.Items, dommfg.CutItem{
			PartNumber:   nums[0],
			MaterialCode: ck.material,
			Thickness:    ck.thickness,
			Length:       ck.length,
			Width:        ck.width,
			Quantity:     dommfg.Quantity(len(nums)),
			Profile:      profiles[ck],
			HasTongue:    j.tongue,
			HasMortise:   j.mortise,
			JointOnWidth: j.onWidth,
			JointCount:   j.count,
		})
	}
	return bom, cut
}
