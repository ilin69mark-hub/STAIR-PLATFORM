package manufacturing

import (
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
	// Признаки стыков накапливаем по группе: в строке спецификации косоуров
	// стоят все 4 куска, и шип есть не у каждого — но карта раскроя должна
	// показать, что стык в этой позиции есть.
	joints := make(map[groupKey]jointFlags)
	for _, p := range parts {
		k := groupKey{p.Kind, p.Material, p.Thickness, p.Length, p.Width}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], p.Number)
		j := joints[k]
		j.count = p.JointCount
		j.onWidth = p.JointOnWidth
		j.tongue = j.tongue || p.HasTongue
		j.mortise = j.mortise || p.HasMortise
		joints[k] = j
	}

	bom := dommfg.BOM{Lines: make([]dommfg.BOMLine, 0, len(order))}
	cut := dommfg.CutList{Items: make([]dommfg.CutItem, 0, len(order))}
	for i, k := range order {
		nums := groups[k]
		quantity := dommfg.Quantity(len(nums))
		bom.Lines = append(bom.Lines, dommfg.BOMLine{
			Number:       i + 1,
			PartNumber:   nums[0],
			Description:  kindDescription(k.kind),
			MaterialCode: k.material,
			Thickness:    k.thickness,
			Quantity:     quantity,
			Length:       k.length,
			Width:        k.width,
		})
		j := joints[k]
		cut.Items = append(cut.Items, dommfg.CutItem{
			PartNumber:   nums[0],
			MaterialCode: k.material,
			Thickness:    k.thickness,
			Length:       k.length,
			Width:        k.width,
			Quantity:     quantity,
			HasTongue:    j.tongue,
			HasMortise:   j.mortise,
			JointOnWidth: j.onWidth,
			JointCount:   j.count,
		})
	}
	return bom, cut
}
