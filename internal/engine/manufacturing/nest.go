package manufacturing

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/engine/scheduler"
)

// defaultStockSheetGroups — листы каталога по материалам (MFG-0012). Один и
// тот же размер может принадлежать нескольким материалам: древесина (дуб, орех,
// ясень, сосна) и сталь (S235, кортен) раскраиваются одинаково. Этап 1 добавил
// в каталог орех/ясень/сосна/кортен — без листов для них любой расчёт был
// blocked как «изготовление невозможно».
var defaultStockSheetGroups = []struct {
	Material dommfg.MaterialCode
	Sizes    [][2]float64
}{
	// Сталь: прокат лазерной резки. Лист 6000×1500 (сводная шпатка
	// технолога 2026-10-05) режется на две заготовки 3000×1500 без
	// остатка, а рабочее поле лазерного стола — 3000×1500.
	//
	// Раньше здесь стояли листы до 10400×6200: раскрой клал детали прямо на
	// них и считал по ним стоимость материала. Физически это неверно — лист
	// 10400×6200 на стол 3000×1500 не встанет, его сперва режут на заготовки.
	// Теперь листов ровно один типоразмер, а выбор заготовки внутри него
	// делает Nest (см. laser.go в домене).
	{dommfg.MaterialCode("STEEL-S235"), [][2]float64{
		{dommfg.StockSheetLengthMM, dommfg.StockSheetWidthMM},
	}},
	// Древесина: стандартный лист 2500×600 — для проступей/подступенков;
	// крупные плиты 6000×3000 покрывают типовые марши (H ≤ ~2950 мм),
	// 9000×4600 — до H ≈ 4550 мм (ширина листа ≥ H+heel).
	{dommfg.MaterialCode("WOOD-OAK"), [][2]float64{
		{2500, 600}, {2500, 1250}, {6000, 3000}, {9000, 4600},
	}},
	{dommfg.MaterialCode("WOOD-WALNUT"), [][2]float64{
		{2500, 600}, {2500, 1250}, {6000, 3000}, {9000, 4600},
	}},
	{dommfg.MaterialCode("WOOD-ASH"), [][2]float64{
		{2500, 600}, {2500, 1250}, {6000, 3000}, {9000, 4600},
	}},
	{dommfg.MaterialCode("WOOD-SOFT"), [][2]float64{
		{2500, 600}, {2500, 1250}, {6000, 3000}, {9000, 4600},
	}},
}

// buildDefaultStockSheets материализует листы каталога из мм-констант как
// error-результат (P2-12): ни одного panic на всём пути сборки реестров.
func buildDefaultStockSheets() ([]*dommfg.StockSheet, error) {
	sheets := make([]*dommfg.StockSheet, 0, 32)
	for _, group := range defaultStockSheetGroups {
		for i, dim := range group.Sizes {
			l, errL := engineering.NewLength(dim[0])
			if errL != nil {
				return nil, fmt.Errorf("manufacturing: stock sheet %s#%d length %v: %w", group.Material, i, dim[0], errL)
			}
			w, errW := engineering.NewLength(dim[1])
			if errW != nil {
				return nil, fmt.Errorf("manufacturing: stock sheet %s#%d width %v: %w", group.Material, i, dim[1], errW)
			}
			sheets = append(sheets, &dommfg.StockSheet{
				MaterialCode: group.Material,
				Length:       l,
				Width:        w,
			})
		}
	}
	return sheets, nil
}

// defaultStockSheetRegistry — общий неизменяемый каталог листов, созданный
// один раз (EM-06); ошибка сборки запоминается в err-поле (P2-12).
var defaultStockSheetRegistry struct {
	once sync.Once
	reg  *dommfg.StockSheetRegistry
	err  error
}

// DefaultStockSheetRegistry возвращает встроенный детерминированный каталог
// стандартных листов (MFG-0012) для MVP-05: листы для материалов каталога
// DefaultMaterialRegistry. Возвращается разделяемый неизменяемый экземпляр;
// вызывающий не должен его изменять. Ошибка — вместо паники (P2-12).
func DefaultStockSheetRegistry() (*dommfg.StockSheetRegistry, error) {
	defaultStockSheetRegistry.once.Do(func() {
		sheets, err := buildDefaultStockSheets()
		if err != nil {
			defaultStockSheetRegistry.err = err
			return
		}
		reg, err := dommfg.NewStockSheetRegistry(sheets...)
		if err != nil {
			defaultStockSheetRegistry.err = fmt.Errorf("manufacturing: default stock sheet registry: %w", err)
			return
		}
		defaultStockSheetRegistry.reg = reg
	})
	return defaultStockSheetRegistry.reg, defaultStockSheetRegistry.err
}

// Rect — прямоугольник детали в плоскости раскроя (мм). Length ≥ Width
// нормализуется внутри функций раскроя.
type Rect struct {
	Length float64
	Width  float64
}

// FeasibilityError — ошибка раскроя: деталь не влезает в рабочее поле лазерного
// стола даже после разбиения на куски. Параметры позволяют прикладному слою
// построить понятное пользователю сообщение.
type FeasibilityError struct {
	PartLen float64
	PartWid float64
	// Thickness — толщина листа: от неё зависят зазор и отступ в шпатке.
	Thickness float64
	// FieldLength / FieldWidth — рабочее поле стола, мм.
	FieldLength float64
	FieldWidth  float64
}

func (e *FeasibilityError) Error() string {
	return fmt.Sprintf("manufacturing: part %v×%v mm (thickness %v) does not fit laser field %v×%v mm",
		e.PartLen, e.PartWid, e.Thickness, e.FieldLength, e.FieldWidth)
}

// SheetFeasible проверяет, что прямоугольники влезают в рабочее поле
// лазерного стола с учётом отступа и зазора по толщине листа. Предикат не
// раскладывает детали (упрощение для советника): достаточно, чтобы самый
// крупный прямоугольник помещался.
//
// Каталог листов и код материала больше не нужны: предел задаёт ПОЛЕ СТОЛА
// (3000×1500), а деталь длиннее поля режется кусками со сварным стыком.
func SheetFeasible(thickness float64, rects ...Rect) bool {
	return fitsLaserField(normalizeRects(rects), thickness)
}

// SheetFeasibleFor проверяет вместимость деталей с учётом ТОГО, чем их режут.
// Сталь режется лазером, и её ограничение — рабочее поле стола 3000×1500:
// деталь длиннее поля не режется целиком, её можно только разрезать на куски
// со сварным стыком. Дерево режется на другом станке, для него ограничение —
// каталог листов (там есть форматы до 2500×600 и крупнее).
func SheetFeasibleFor(material dommfg.MaterialCode, thickness float64, rects ...Rect) bool {
	cut := normalizeRects(rects)
	if isLaserGroup(material) {
		return fitsLaserField(cut, thickness)
	}
	return fitsStockSheet(material, cut)
}

// SplittableFeasibleFor — изготовляема ли деталь размером lengthMM × widthMM,
// если её можно разрезать по ДЛИНЕ на куски со сварным стыком.
//
// По длине резать можно: куски стыкуются шип-пазом и дают исходную деталь.
// По ширине — нельзя: у косоура-гребёнки ширина это спинка с зубьями, и
// поперечный рез отделил бы зубья от спинки. Поэтому для стали проверяется
// только ширина против рабочего поля стола.
//
// Дерево не разрезается стыком (другой станок, сварка не предусмотрена), и его
// проверяют против каталога листов целиком.
func SplittableFeasibleFor(material dommfg.MaterialCode, thickness, lengthMM, widthMM float64) bool {
	if isLaserGroup(material) {
		// Раньше здесь нормализовали пару (l, w) так, чтобы длина была не
		// меньше ширины, но результат не использовался: w отбрасывался, а
		// проверялся исходный widthMM. Нормализация была мёртвой, убрана.
		// Проверяется именно ширина, переданная вызывающим, — как сказано в
		// описании функции выше. Тестов на эту функцию нет, поэтому менять
		// здесь поведение на min(l, w) без решения владельца нельзя.
		return widthMM <= dommfg.MaxPartWidthMM(thickness)
	}
	return fitsStockSheet(material, []cutRect{{length: lengthMM, width: widthMM}})
}

// fitsStockSheet — влезает ли самая крупная деталь хотя бы в один лист
// материала из каталога (для дерева: раскрой по станку, не лазер).
func fitsStockSheet(material dommfg.MaterialCode, rects []cutRect) bool {
	registry, err := DefaultStockSheetRegistry()
	if err != nil {
		return true // каталог не собрался — не блокируем расчёт
	}
	sheets := registry.SheetsFor(material)
	if len(sheets) == 0 {
		return true
	}
	maxLen, maxWid := bounds(rects)
	for _, sh := range sheets {
		if sh.Length.Millimeters() >= maxLen+woodGapMM && sh.Width.Millimeters() >= maxWid+woodGapMM {
			return true
		}
	}
	return false
}

// bounds — наибольшие габариты среди деталей.
func bounds(rects []cutRect) (maxLen, maxWid float64) {
	for _, r := range rects {
		if r.length > maxLen {
			maxLen = r.length
		}
		if r.width > maxWid {
			maxWid = r.width
		}
	}
	return maxLen, maxWid
}

// normalizeRects — приводит прямоугольники к виду length ≥ width: деталь можно
// повернуть на 90°, и габарит по двум осям не зависит от того, в какую
// сторону конструктор её нарисовал.
func normalizeRects(rects []Rect) []cutRect {
	cut := make([]cutRect, 0, len(rects))
	for _, r := range rects {
		l, w := r.Length, r.Width
		if l < w {
			l, w = w, l
		}
		cut = append(cut, cutRect{length: l, width: w})
	}
	return cut
}

// fitsLaserField — влезает ли самая крупная деталь в рабочее поле лазерного
// стола с учётом отступа и зазора шпатки.
//
// Раньше проверка шла против КАТАЛОГА ЛИСТОВ (до 10400×6200): деталь 5200 мм
// «влезала», и раскрой её принимал, хотя стол лазерной резки 3000×1500 и
// деталь длиннее поля режется только кусками со сварным стыком.
func fitsLaserField(rects []cutRect, thickness float64) bool {
	maxLen, maxWid := 0.0, 0.0
	for _, r := range rects {
		if r.length > maxLen {
			maxLen = r.length
		}
		if r.width > maxWid {
			maxWid = r.width
		}
	}
	off := dommfg.CutEdgeOffsetMM(thickness)
	return maxLen+2*off <= dommfg.BlankLengthMM && maxWid+2*off <= dommfg.BlankWidthMM
}

// LargestStockSheet возвращает габариты самого крупного листа каталога
// для материала (максимумы по длине и ширине в отдельности). Используется
// для человекочитаемых сообщений советника.
// Возвращает НАИБОЛЬШИЙ по площади реальный лист материала. Раньше длина и
// ширина выбирались независимо, и для набора {8000×4600, 10400×6200} функция
// отдавала несуществующий 10400×6200 в одном куске — в сообщении пользователю
// показывался «макс. лист», которого в каталоге нет.
func LargestStockSheet(registry *dommfg.StockSheetRegistry, material dommfg.MaterialCode) (length, width float64, ok bool) {
	if registry == nil {
		return 0, 0, false
	}
	best := 0.0
	for _, s := range registry.SheetsFor(material) {
		l, w := s.Length.Millimeters(), s.Width.Millimeters()
		if a := l * w; !ok || a > best {
			length, width, best, ok = l, w, a, true
		}
	}
	return length, width, ok
}

// cutRect — прямоугольник раскроя: деталь с длиной length ≥ width (мм).
type cutRect struct {
	partNumber dommfg.PartNumber
	length     float64
	width      float64
	// Признаки сварного стыка «шип-паз» — доезжают до PlacedPart, чтобы
	// карта раскроя показывала монтажнику, какой кусок с каким стыкуется.
	tongue  bool
	mortise bool
	onWidth bool
	joint   int
}

// Nest раскладывает карту раскроя в два шага (MFG-0012, MFG-0006).
//
//  1. ДЕТАЛИ → ЗАГОТОВКИ 3000×1500. Это рабочее поле лазерного стола;
//     раскладка shelf-паккингом с отступом от края и зазором из шпатки
//     по толщине листа.
//  2. ЗАГОТОВКИ → КУПЛЕННЫЕ ЛИСТЫ. Лист 6000×1500 режется на две заготовки
//     без остатка, поэтому листов = ⌈заготовок / 2⌉.
//
// Раньше шаг был один: детали клались прямо на листы до 10400×6200 с kerf = 3 мм
// на все толщины. Лист такого размера на стол не встает, а зазор втрое меньше
// нужного для листа 12 мм.
//
// Группы (материал, толщина) раскладываются независимо и параллельно (B3,
// EDR-0034 §3.3); детали сортируются по убыванию длины — результат
// детерминирован (ADR-0003).
func Nest(cut dommfg.CutList, registry *dommfg.StockSheetRegistry) (*dommfg.NestingResult, error) {
	if len(cut.Items) == 0 {
		return nil, fmt.Errorf("manufacturing: cut list is empty")
	}
	if registry == nil {
		return nil, fmt.Errorf("manufacturing: stock sheet registry is required")
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
				tongue:     item.HasTongue,
				mortise:    item.HasMortise,
				onWidth:    item.JointOnWidth,
				joint:      item.JointCount,
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
		sheet, err := pickSheet(registry, k.material, rects, k.thickness)
		if err != nil {
			return err
		}
		if isLaserGroup(k.material) {
			groupLayouts[i] = blanksToSheets(nestBlanks(rects, k.thickness), sheet, k.thickness)
		} else {
			groupLayouts[i] = nestWood(rects, sheet, k.thickness)
		}
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

// isLaserGroup — металлическая группа раскроя. Только для неё действуют поле
// лазерного стола и шпатка зазоров; дерево режется на другом станке, и его
// раскрой владелец отложил на потом (решение 2026-10-05).
// IsLaserMaterial режется ли материал лазером: его ограничение — рабочее поле
// стола, а не каталог листов.
func IsLaserMaterial(material dommfg.MaterialCode) bool {
	return isLaserGroup(material)
}

func isLaserGroup(material dommfg.MaterialCode) bool {
	registryMat, err := DefaultMaterialRegistry()
	if err != nil {
		return false
	}
	m, ok := registryMat.Find(material)
	return ok && dommfg.IsLaserMaterial(m)
}

// pickSheet возвращает лист, из которого режутся заготовки.
func pickSheet(registry *dommfg.StockSheetRegistry, material dommfg.MaterialCode, rects []cutRect, thickness float64) (*dommfg.StockSheet, error) {
	if isLaserGroup(material) && !fitsLaserField(rects, thickness) {
		maxLen, maxWid := bounds(rects)
		return nil, &FeasibilityError{
			PartLen:     maxLen,
			PartWid:     maxWid,
			Thickness:   thickness,
			FieldLength: dommfg.BlankLengthMM,
			FieldWidth:  dommfg.BlankWidthMM,
		}
	}
	sheets := registry.SheetsFor(material)
	if len(sheets) == 0 {
		return nil, fmt.Errorf("manufacturing: no stock sheet for material %q", material)
	}
	// Для металла поле стола важнее размера листа: типоразмер один
	// (6000×1500), выбирать не из чего.
	if isLaserGroup(material) {
		return sheets[0], nil
	}
	// Для дерева листов несколько, и берётся наименьший ПО ПЛОЩАДИ ИЗ
	// ВМЕЩАЮЩИХ. Проверка вместимости обязательна: без неё для детали
	// 2480×1240 выбирался лист 2500×600 (он меньше по площади, но по ширине
	// не подходит), и раскрой всё равно разложил деталь по двум листам.
	maxLen, maxWid := bounds(rects)
	var best *dommfg.StockSheet
	bestArea := 0.0
	for _, sh := range sheets {
		if sh.Length.Millimeters() < maxLen+woodGapMM || sh.Width.Millimeters() < maxWid+woodGapMM {
			continue
		}
		area := sh.Length.Millimeters() * sh.Width.Millimeters()
		if best == nil || area < bestArea {
			best, bestArea = sh, area
		}
	}
	if best == nil {
		return nil, &FeasibilityError{
			PartLen:     maxLen,
			PartWid:     maxWid,
			Thickness:   thickness,
			FieldLength: sheets[0].Length.Millimeters(),
			FieldWidth:  sheets[0].Width.Millimeters(),
		}
	}
	return best, nil
}

// shelf — открытая полоса заготовки: детали кладутся слева направо на высоте y.
type shelf struct {
	y      float64
	usedX  float64
	height float64
}

// blankLayout — заполненная заготовка 3000×1500. Координаты деталей — внутри
// заготовки; в лист они попадают сдвигом на кратность BlankLengthMM.
type blankLayout struct {
	Placed []dommfg.PlacedPart
}

// nestBlanks размещает прямоугольники на ЗАГОТОВКАХ 3000×1500 (рабочее поле
// лазерного стола) shelf-паккингом.
//
// Отступ от края заготовки и зазор между деталями берутся из шпатки по толщине
// листа (CutEdgeOffsetMM / CutGapMM). Раньше стоял один kerf = 3 мм на все
// толщины, а раскладка шла сразу по купленным листам 6000×3000 — лист такого
// размера на стол 3000×1500 не встает.
func nestBlanks(rects []cutRect, thickness float64) []blankLayout {
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

	off := dommfg.CutEdgeOffsetMM(thickness)
	gap := dommfg.CutGapMM(thickness)
	usableL := dommfg.BlankLengthMM - 2*off
	usableW := dommfg.BlankWidthMM - 2*off

	var blanks []blankLayout
	cur := blankLayout{}
	var open []shelf
	curY := off

	for _, r := range sorted {
		ew := r.length + gap
		eh := r.width + gap

		placed := false
		for i := range open {
			sh := &open[i]
			if sh.usedX+ew <= usableL+off && eh <= sh.height {
				cur.Placed = append(cur.Placed, dommfg.PlacedPart{
					PartNumber:   r.partNumber,
					Length:       engineering.Length(r.length),
					Width:        engineering.Length(r.width),
					X:            off + sh.usedX,
					Y:            sh.y,
					HasTongue:    r.tongue,
					HasMortise:   r.mortise,
					JointOnWidth: r.onWidth,
					JointCount:   r.joint,
				})
				sh.usedX += ew
				placed = true
				break
			}
		}
		if placed {
			continue
		}

		if curY+eh > off+usableW {
			blanks = append(blanks, cur)
			cur = blankLayout{}
			open = open[:0]
			curY = off
		}
		open = append(open, shelf{y: curY, usedX: ew, height: eh})
		cur.Placed = append(cur.Placed, dommfg.PlacedPart{
			PartNumber:   r.partNumber,
			Length:       engineering.Length(r.length),
			Width:        engineering.Length(r.width),
			X:            off,
			Y:            curY,
			HasTongue:    r.tongue,
			HasMortise:   r.mortise,
			JointOnWidth: r.onWidth,
			JointCount:   r.joint,
		})
		curY += eh
	}
	if len(cur.Placed) > 0 {
		blanks = append(blanks, cur)
	}
	return blanks
}

// woodGapMM — зазор между деталями при раскрое ДЕРЕВА. Шпатка технолога
// 2026-10-05 относится к лазерной резке металла; дерево режется на другом
// станке, и его раскрой владелец отложил на потом. До этого решения сохраняем
// прежнее поведение (3 мм на все толщины), чтобы изменение не тронуло ни одну
// деревянную цену.
const woodGapMM = 3.0

// nestWood раскладывает прямоугольники на листы ДЕРЕВА каталога — прежний
// shelf-паккинг с постоянным зазором. Металл сюда не попадает: он идёт через
// nestBlanks по полю лазерного стола.
func nestWood(rects []cutRect, sheet *dommfg.StockSheet, thickness float64) []dommfg.SheetLayout {
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
		ew, eh := r.length+woodGapMM, r.width+woodGapMM
		placed := false
		for i := range open {
			sh := &open[i]
			if sh.usedX+ew <= sheetLen && eh <= sh.height {
				current.Placed = append(current.Placed, dommfg.PlacedPart{
					PartNumber: r.partNumber,
					Length:     engineering.Length(r.length),
					Width:      engineering.Length(r.width),
					X:          sh.usedX, Y: sh.y,
					HasTongue:    r.tongue,
					HasMortise:   r.mortise,
					JointOnWidth: r.onWidth,
					JointCount:   r.joint,
				})
				sh.usedX += ew
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
			X:          0, Y: curY,
			HasTongue:    r.tongue,
			HasMortise:   r.mortise,
			JointOnWidth: r.onWidth,
			JointCount:   r.joint,
		})
		curY += eh
	}
	layouts = append(layouts, current)
	return layouts
}

// blanksToSheets собирает заготовки в купленные листы: в лист 6000×1500 входят
// ровно две заготовки 3000×1500 без остатка (BlanksPerSheet). Детали второй
// заготовки сдвигаются по длине листа на BlankLengthMM.
func blanksToSheets(blanks []blankLayout, sheet *dommfg.StockSheet, thickness float64) []dommfg.SheetLayout {
	if len(blanks) == 0 {
		return nil
	}
	per := dommfg.BlanksPerSheet
	if per < 1 {
		per = 1
	}
	var layouts []dommfg.SheetLayout
	for start := 0; start < len(blanks); start += per {
		layout := dommfg.SheetLayout{
			MaterialCode: sheet.MaterialCode,
			Thickness:    engineering.Length(thickness),
			Length:       sheet.Length,
			Width:        sheet.Width,
		}
		for k := start; k < start+per && k < len(blanks); k++ {
			shift := float64(k-start) * dommfg.BlankLengthMM
			for _, p := range blanks[k].Placed {
				p.X += shift
				layout.Placed = append(layout.Placed, p)
			}
		}
		layouts = append(layouts, layout)
	}
	return layouts
}
