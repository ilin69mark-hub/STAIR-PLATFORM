package manufacturing

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/engine/scheduler"
)

// DefaultKerf — ширина реза по умолчанию (MFG-0006): 3 мм между деталями.
const DefaultKerf = 3.0

// DefaultStockSheetRegistry возвращает встроенный детерминированный каталог
// стандартных листов (MFG-0012) для MVP-05: листы для материалов каталога
// DefaultMaterialRegistry. Крупные листы — для косоуров, стандартные —
// для проступей/подступенков. Возвращается разделяемый неизменяемый экземпляр;
// вызывающий не должен его изменять.
func DefaultStockSheetRegistry() *dommfg.StockSheetRegistry {
	defaultStockSheetRegistry.once.Do(func() {
		reg, err := dommfg.NewStockSheetRegistry(
			&dommfg.StockSheet{MaterialCode: "STEEL-S235", Length: sheetLength(6000), Width: sheetLength(3000)},
			&dommfg.StockSheet{MaterialCode: "STEEL-S235", Length: sheetLength(2500), Width: sheetLength(1250)},
			// Крупные листы (энвелоп H ≤ 6000 мм, MFG-0012): худший косоур
			// прямого марша = прогон ≈1.732·H × (H+heel 50). Для H=6000 это
			// ≈10200×6050 → покрывается листом 10400×6200; средний 8000×4600
			// дешевле для H до ~4550 (pickSheet берёт наименьший по площади).
			&dommfg.StockSheet{MaterialCode: "STEEL-S235", Length: sheetLength(8000), Width: sheetLength(4600)},
			&dommfg.StockSheet{MaterialCode: "STEEL-S235", Length: sheetLength(10400), Width: sheetLength(6200)},
			&dommfg.StockSheet{MaterialCode: "ALUM-5083", Length: sheetLength(3000), Width: sheetLength(1500)},
			// Крупные алюминиевые листы (выбор материала конструктора, MFG-0012):
			// 6000×3000 — типовые марши (H ≤ ~2950 мм), 9000×4600 — до H ≈ 4550 мм.
			&dommfg.StockSheet{MaterialCode: "ALUM-5083", Length: sheetLength(6000), Width: sheetLength(3000)},
			&dommfg.StockSheet{MaterialCode: "ALUM-5083", Length: sheetLength(9000), Width: sheetLength(4600)},
			// Дуб (выбор материала конструктора, MFG-0012): стандартный лист
			// 2500×600 — для проступей/подступенков; крупные плиты 6000×3000
			// покрывают типовые марши (H ≤ ~2950 мм), 9000×4600 — до H ≈ 4550 мм
			// (ширина листа ≥ H+heel). pickSheet берёт наименьший по площади.
			&dommfg.StockSheet{MaterialCode: "WOOD-OAK", Length: sheetLength(2500), Width: sheetLength(600)},
			&dommfg.StockSheet{MaterialCode: "WOOD-OAK", Length: sheetLength(2500), Width: sheetLength(1250)},
			&dommfg.StockSheet{MaterialCode: "WOOD-OAK", Length: sheetLength(6000), Width: sheetLength(3000)},
			&dommfg.StockSheet{MaterialCode: "WOOD-OAK", Length: sheetLength(9000), Width: sheetLength(4600)},
		)
		if err != nil {
			panic(fmt.Sprintf("manufacturing: default stock sheet registry: %v", err))
		}
		defaultStockSheetRegistry.reg = reg
	})
	return defaultStockSheetRegistry.reg
}

var defaultStockSheetRegistry struct {
	once sync.Once
	reg  *dommfg.StockSheetRegistry
}

func sheetLength(mm float64) engineering.Length {
	l, err := engineering.NewLength(mm)
	if err != nil {
		panic(fmt.Sprintf("manufacturing: stock sheet length %v: %v", mm, err))
	}
	return l
}

// Rect — прямоугольник детали в плоскости раскроя (мм). Length ≥ Width
// нормализуется внутри функций раскроя.
type Rect struct {
	Length float64
	Width  float64
}

// FeasibilityError — ошибка раскроя: в каталоге листов (MFG-0012) нет
// листа, вмещающего самую крупную деталь группы с учётом kerf. Параметры
// позволяют прикладному слою построить понятное пользователю сообщение.
type FeasibilityError struct {
	PartLen float64
	PartWid float64
	Kerf    float64
}

func (e *FeasibilityError) Error() string {
	return fmt.Sprintf("manufacturing: no stock sheet fits %v×%v mm (kerf %v)", e.PartLen, e.PartWid, e.Kerf)
}

// SheetFeasible проверяет, что хотя бы один лист каталога для материала
// вмещает все прямоугольники с учётом kerf. Предикат не раскладывает
// детали (упрощение для советника): достаточно, чтобы самый крупный
// прямоугольник помещался на какой-либо лист реестра.
func SheetFeasible(registry *dommfg.StockSheetRegistry, material dommfg.MaterialCode, kerf float64, rects ...Rect) bool {
	cut := make([]cutRect, 0, len(rects))
	for _, r := range rects {
		l, w := r.Length, r.Width
		if l < w {
			l, w = w, l
		}
		cut = append(cut, cutRect{length: l, width: w})
	}
	_, ok := pickSheetFit(registry, material, cut, kerf)
	return ok
}

// LargestStockSheet возвращает габариты самого крупного листа каталога
// для материала (максимумы по длине и ширине в отдельности). Используется
// для человекочитаемых сообщений советника.
func LargestStockSheet(registry *dommfg.StockSheetRegistry, material dommfg.MaterialCode) (length, width float64, ok bool) {
	if registry == nil {
		return 0, 0, false
	}
	for _, s := range registry.SheetsFor(material) {
		l, w := s.Length.Millimeters(), s.Width.Millimeters()
		if l > length {
			length = l
		}
		if w > width {
			width = w
		}
		ok = true
	}
	return length, width, ok
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
	if len(order) == 0 {
		return result, nil
	}

	// Per-material Nest (B3, EDR-0034 §3.3): группы (материал, толщина)
	// раскладываются независимо (pickSheet + nestGroup), листы каждой группы
	// записываются в слот i; сборка идёт в порядке order — тот же результат,
	// что и последовательный обход (детерминизм ADR-0003).
	groupLayouts := make([][]dommfg.SheetLayout, len(order))
	err := scheduler.New(0).Execute(context.Background(), len(order), func(i int) error {
		k := order[i]
		rects := groups[k]
		sheet, err := pickSheet(registry, k.material, rects, kerf)
		if err != nil {
			return err
		}
		groupLayouts[i] = nestGroup(rects, sheet, k.thickness, kerf)
		return nil
	})
	if err != nil {
		return nil, err
	}

	for i := range order {
		for _, l := range groupLayouts[i] {
			result.Sheets = append(result.Sheets, l)
			result.PartCount += dommfg.Quantity(len(l.Placed))
			result.SheetArea += l.Length.Millimeters() * l.Width.Millimeters()
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
	best, ok := pickSheetFit(registry, material, rects, kerf)
	if !ok {
		maxLen, maxWid := 0.0, 0.0
		for _, r := range rects {
			if r.length > maxLen {
				maxLen = r.length
			}
			if r.width > maxWid {
				maxWid = r.width
			}
		}
		return nil, &FeasibilityError{PartLen: maxLen, PartWid: maxWid, Kerf: kerf}
	}
	return best, nil
}

// pickSheetFit возвращает наименьший подходящий лист и признак наличия
// такого листа. Общая логика для pickSheet и SheetFeasible.
func pickSheetFit(registry *dommfg.StockSheetRegistry, material dommfg.MaterialCode, rects []cutRect, kerf float64) (*dommfg.StockSheet, bool) {
	sheets := registry.SheetsFor(material)
	if len(sheets) == 0 {
		return nil, false
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
	return best, best != nil
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
