package manufacturing

import (
	"fmt"
	"math"
	"sort"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
)

// DefaultKerf — ширина реза по умолчанию (MFG-0006): 3 мм между деталями.
const DefaultKerf = 3.0

// DefaultStockSheetRegistry возвращает встроенный детерминированный каталог
// стандартных листов (MFG-0012) для MVP-05: листы для материалов каталога
// DefaultMaterialRegistry. Крупные листы — для косоуров, стандартные —
// для проступей/подступенков.
func DefaultStockSheetRegistry() *dommfg.StockSheetRegistry {
	reg, err := dommfg.NewStockSheetRegistry(
		&dommfg.StockSheet{MaterialCode: "STEEL-S235", Length: sheetLength(6000), Width: sheetLength(3000)},
		&dommfg.StockSheet{MaterialCode: "STEEL-S235", Length: sheetLength(2500), Width: sheetLength(1250)},
		&dommfg.StockSheet{MaterialCode: "ALUM-5083", Length: sheetLength(3000), Width: sheetLength(1500)},
		&dommfg.StockSheet{MaterialCode: "WOOD-OAK", Length: sheetLength(2500), Width: sheetLength(600)},
	)
	if err != nil {
		panic(fmt.Sprintf("manufacturing: default stock sheet registry: %v", err))
	}
	return reg
}

func sheetLength(mm float64) engineering.Length {
	l, err := engineering.NewLength(mm)
	if err != nil {
		panic(fmt.Sprintf("manufacturing: stock sheet length %v: %v", mm, err))
	}
	return l
}

// cutRect — прямоугольник раскроя: деталь с длиной length ≥ width (мм).
type cutRect struct {
	partNumber dommfg.PartNumber
	length     float64
	width      float64
}

// Nest раскладывает карту раскроя на стандартные листы (MFG-0012,
// MFG-0006): детерминированный shelf-паккинг прямоугольников с учётом
// kerf (ширина реза). Для каждой группы (материал, толщина) выбирается
// наименьший лист каталога, вмещающий самую крупную деталь группы.
// Детали сортируются по убыванию длины — результат детерминирован.
func Nest(cut dommfg.CutList, registry *dommfg.StockSheetRegistry, kerf float64) (*dommfg.NestingResult, error) {
	if len(cut.Items) == 0 {
		return nil, fmt.Errorf("manufacturing: cut list is empty")
	}
	if registry == nil {
		return nil, fmt.Errorf("manufacturing: stock sheet registry is required")
	}
	if math.IsNaN(kerf) || math.IsInf(kerf, 0) || kerf < 0 {
		return nil, fmt.Errorf("manufacturing: kerf must not be negative")
	}

	type groupKey struct {
		material  dommfg.MaterialCode
		thickness float64
	}
	groups := make(map[groupKey][]cutRect)
	var order []groupKey
	for _, item := range cut.Items {
		k := groupKey{item.MaterialCode, item.Thickness.Millimeters()}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		for i := dommfg.Quantity(0); i < item.Quantity; i++ {
			groups[k] = append(groups[k], cutRect{
				partNumber: item.PartNumber,
				length:     item.Length.Millimeters(),
				width:      item.Width.Millimeters(),
			})
		}
	}

	result := &dommfg.NestingResult{}
	for _, k := range order {
		rects := groups[k]
		sheet, err := pickSheet(registry, k.material, rects, kerf)
		if err != nil {
			return nil, err
		}
		layouts := nestGroup(rects, sheet, k.thickness, kerf)
		for _, l := range layouts {
			result.Sheets = append(result.Sheets, l)
			result.PartCount += dommfg.Quantity(len(l.Placed))
			result.SheetArea += sheet.Length.Millimeters() * sheet.Width.Millimeters()
			for _, p := range l.Placed {
				result.PartArea += p.Length.Millimeters() * p.Width.Millimeters()
			}
		}
	}
	result.WasteArea = result.SheetArea - result.PartArea
	if result.SheetArea > 0 {
		result.Utilization = result.PartArea / result.SheetArea
	}
	return result, nil
}

// pickSheet выбирает наименьший лист каталога (по площади, при равенстве —
// первый в порядке реестра), вмещающий самую крупную деталь с учётом kerf.
func pickSheet(registry *dommfg.StockSheetRegistry, material dommfg.MaterialCode, rects []cutRect, kerf float64) (*dommfg.StockSheet, error) {
	sheets := registry.SheetsFor(material)
	if len(sheets) == 0 {
		return nil, fmt.Errorf("manufacturing: no stock sheets for material %q", material)
	}
	maxLen, maxWid := 0.0, 0.0
	for _, r := range rects {
		if r.length > maxLen {
			maxLen = r.length
		}
		if r.width > maxWid {
			maxWid = r.width
		}
	}
	var best *dommfg.StockSheet
	bestArea := math.Inf(1)
	for _, s := range sheets {
		if s.Length.Millimeters() < maxLen+kerf || s.Width.Millimeters() < maxWid+kerf {
			continue
		}
		area := s.Length.Millimeters() * s.Width.Millimeters()
		if area < bestArea {
			bestArea = area
			best = s
		}
	}
	if best == nil {
		return nil, fmt.Errorf("manufacturing: no stock sheet fits %v×%v mm (kerf %v)", maxLen, maxWid, kerf)
	}
	return best, nil
}

// shelf — открытая полоса листа: детали кладутся слева направо на высоте y.
type shelf struct {
	y      float64
	usedX  float64
	height float64
}

// nestGroup размещает прямоугольники на листах одного типоразмера
// shelf-паккингом. Эффективный габарит детали = габарит + kerf (зазор
// между деталями и до кромки листа). Возвращает заполненные листы.
func nestGroup(rects []cutRect, sheet *dommfg.StockSheet, thickness float64, kerf float64) []dommfg.SheetLayout {
	sorted := make([]cutRect, len(rects))
	copy(sorted, rects)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].length != sorted[j].length {
			return sorted[i].length > sorted[j].length
		}
		if sorted[i].width != sorted[j].width {
			return sorted[i].width > sorted[j].width
		}
		return sorted[i].partNumber < sorted[j].partNumber
	})

	sheetLen := sheet.Length.Millimeters()
	sheetWid := sheet.Width.Millimeters()

	newSheet := func() dommfg.SheetLayout {
		return dommfg.SheetLayout{
			MaterialCode: sheet.MaterialCode,
			Thickness:    engineering.Length(thickness),
			Length:       sheet.Length,
			Width:        sheet.Width,
		}
	}

	var layouts []dommfg.SheetLayout
	current := newSheet()
	var open []shelf
	curY := 0.0

	for _, r := range sorted {
		ew := r.length + kerf
		eh := r.width + kerf

		placed := false
		for i := range open {
			s := &open[i]
			if s.usedX+ew <= sheetLen && eh <= s.height {
				current.Placed = append(current.Placed, dommfg.PlacedPart{
					PartNumber: r.partNumber,
					Length:     engineering.Length(r.length),
					Width:      engineering.Length(r.width),
					X:          s.usedX,
					Y:          s.y,
				})
				s.usedX += ew
				placed = true
				break
			}
		}
		if placed {
			continue
		}

		if curY+eh > sheetWid {
			layouts = append(layouts, current)
			current = newSheet()
			open = open[:0]
			curY = 0
		}
		open = append(open, shelf{y: curY, usedX: ew, height: eh})
		current.Placed = append(current.Placed, dommfg.PlacedPart{
			PartNumber: r.partNumber,
			Length:     engineering.Length(r.length),
			Width:      engineering.Length(r.width),
			X:          0,
			Y:          curY,
		})
		curY += eh
	}
	layouts = append(layouts, current)
	return layouts
}
