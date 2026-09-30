package manufacturing

import (
	"fmt"
	"math"
	"runtime"

	"golang.org/x/sync/errgroup"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
	kerngeo "stairplatform/internal/geometry"
)

// decompose превращает модель в производственные детали: каждое твёрдое
// тело становится Part (MFG-0002 Part Decomposition). Тип детали
// определяется семантической меткой тела (Role, проставляется Geometry
// Engine) или, при её отсутствии, «тонкой» осью ограничивающего
// параллелепипеда (косоур тонок по Y, проступь по Z, подступенок по X).
// Семантическая метка обязательна для повёрнутых маршей (L-образная
// лестница), где ориентация тел зависит от марша. SolidIndex трассирует
// деталь к телу модели (BC-007).
//
// Центральная колонна спиральной лестницы (role "column", EDR-0007)
// обрабатывается особым образом: её диаметр превышает максимальную
// толщину материалов каталога, поэтому деталь представляется развёрткой
// цилиндра — толщиной косоура, длиной H и шириной 2πr (периметр).
func decompose(cfg *engineering.StairConfiguration, model *kerngeo.Compound) ([]dommfg.Part, error) {
	if model == nil {
		return nil, fmt.Errorf("manufacturing: model is required")
	}
	solids := model.Solids()
	if len(solids) == 0 {
		return nil, fmt.Errorf("manufacturing: model has no solids")
	}

	// Материал нужен ДО параллельного разбора: от него зависит, считается ли
	// деталь как лист лазерного раскроя (metal) или как монолит (дерево).
	//
	// Порядок здесь важен. Материал обычно назначается ПОСЛЕ разбора, и даже
	// выводится из толщины (assignMaterial по SupportsThickness) — но тогда
	// круг замыкается: 40 мм подходит стали (2–60), сталь получает
	// 40-миллиметровый «лист», и ступень обходится как стальной блок. Материал
	// берём из конфигурации явно; если он не задан, откатываемся на тот же
	// подбор по толщине, что и раньше, — поведение для таких конфигураций не
	// меняется.
	registry, err := DefaultMaterialRegistry()
	if err != nil {
		return nil, fmt.Errorf("manufacturing: default registry: %w", err)
	}
	// Рама (косоуры, колонна) идёт по материалу каркаса, ступени и
	// подступенки — по материалу ступеней: тот же выбор, что и в engine.go.
	// Если материал не задан, металлом считаем деталь по её модельной
	// толщине: под неё и подбиралась толщина.
	isMetal := func(role string, design float64) bool {
		code := dommfg.MaterialCode(cfg.Material)
		switch role {
		case "tread", "landing", "winder", "riser":
			code = dommfg.MaterialCode(cfg.TreadMaterial)
		}
		if code != "" {
			if m, ok := registry.Find(code); ok {
				return m.Category == "Steel"
			}
		}
		// Материал не задан в конфигурации — подбираем его по толщине, как
		// это делает engine.go при назначении Material деталям.
		guess, err := assignMaterial(registry, design)
		if err != nil {
			return false
		}
		if m, ok := registry.Find(guess); ok {
			return m.Category == "Steel"
		}
		return false
	}

	parts := make([]dommfg.Part, 0, len(solids))
	seq := make(map[dommfg.PartKind]int)

	// Классификация тел (bbox + тип/габариты) независима для каждого тела
	// (чтение неизменяемых тел), поэтому выполняется параллельно по
	// индексу (result-slot, EM-06/B3.1). Нумерация деталей остаётся
	// последовательной: номер Part зависит от порядка тел (seq).
	type spec struct {
		kind                     dommfg.PartKind
		thickness, length, width float64
	}
	specs := make([]spec, len(solids))
	limit := runtime.GOMAXPROCS(0)
	if limit > len(solids) {
		limit = len(solids)
	}
	g := new(errgroup.Group)
	g.SetLimit(limit)
	for i, solid := range solids {
		i, solid := i, solid
		g.Go(func() error {
			bb := kerngeo.SolidBoundingBox(solid)
			ext := [3]float64{
				bb.Max.X - bb.Min.X,
				bb.Max.Y - bb.Min.Y,
				bb.Max.Z - bb.Min.Z,
			}
			var s spec
			switch {
			case solid.Role() == "column":
				// EDR-0007: развёртка колонны (толщина косоура, H×2πr).
				s.kind, s.thickness, s.length, s.width = columnPart(cfg, ext)
			case solid.Role() == "stringer":
				// Косоур — пилообразная пластина: заготовка это полоса
				// «длина марша × шаг ступени», а не габаритный блок.
				s.kind = dommfg.PartStringer
				s.thickness, s.length, s.width = stringerBlank(cfg, ext)
			default:
				s.kind, s.thickness, s.length, s.width = classify(ext, solid.Role())
			}
			// ЛИСТ лазерного раскроя для металла: модельная толщина —
			// конструктивный габарит, а не толщина материала. Единое правило
			// для всех металлических деталей: косоур, ступень, подступенок.
			// Материал определяем ПОСЛЕ расчёта заготовки, подсказывая
			// толщиной детали: если материал не задан в конфигурации, он и
			// раньше выводился из толщины — пусть выводится из той же.
			// Раньше 40-миллиметровая стальная ступень считалась как
			// 40-миллиметровая стальная ступень, то есть 1,3 тонны только на
			// проступи.
			if isMetal(solid.Role(), s.thickness) {
				s.thickness = laserPlateMM(s.thickness)
			}
			specs[i] = s
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("manufacturing: classify: %w", err)
	}

	for i, s := range specs {
		seq[s.kind]++

		mk := func(v float64) (engineering.Length, error) {
			return engineering.NewLength(v)
		}
		thick, err := mk(s.thickness)
		if err != nil {
			return nil, fmt.Errorf("manufacturing: part %d: %w", i, err)
		}
		lng, err := mk(s.length)
		if err != nil {
			return nil, fmt.Errorf("manufacturing: part %d: %w", i, err)
		}
		wid, err := mk(s.width)
		if err != nil {
			return nil, fmt.Errorf("manufacturing: part %d: %w", i, err)
		}
		parts = append(parts, dommfg.Part{
			Number:     partNumber(s.kind, seq[s.kind]),
			Kind:       s.kind,
			Thickness:  thick,
			Length:     lng,
			Width:      wid,
			SolidIndex: i,
		})
	}
	return parts, nil
}

// Лазерный раскрой металла: диапазон толщины листа 3–8 мм (тот же, что
// отдаёт конфигуратор в fieldRules.stepThicknessMM для STEEL-S235).
const (
	laserPlateMinMM = 3.0
	laserPlateMaxMM = 8.0
)

// laserPlateMM приводит модельную толщину детали к толщине ЛИСТА для
// лазерного раскроя металла.
//
// Зачем. В модели толщина детали — это КОНСТРУКТИВНЫЙ габарит, а не толщина
// материала: у стального косоура StringerThickness = 50 мм означает ширину
// сечения, а не 50 мм листа. Если такую цифру отдать в расход материала,
// деталь обходится как стальной блок 50 мм толщиной — и цена металла
// взлетает на порядки (см. TestSteelMassIsPlateNotBlock).
//
// Для металла берётся та же величина, но ограниченная диапазоном выпуска
// листа: пользовательские 3 мм остаются 3 мм, штатные 6 мм остаются 6 мм, а
// конструктивные 50 мм схлопываются в 8 мм — верх выпуска, чтобы цена не
// оказалась занижена. Для неметаллических материалов функция не вызывается:
// доска/плита пилится по всей толщине, там модельная толщина и есть толщина
// материала.
func laserPlateMM(design float64) float64 {
	switch {
	case design < laserPlateMinMM:
		return laserPlateMinMM
	case design > laserPlateMaxMM:
		return laserPlateMaxMM
	default:
		return design
	}
}

// stringerBlank возвращает габариты ЗАГОТОВКИ косоура.
//
// Косоур — пилообразная пластина. Её bbox по высоте равен высоте марша
// (например 2660 мм), но материала в заготовке столько не нужно: пила
// распиливает полосу, и зубья треугольниками уходят в обрез. Развёртка
// пластины — длинная полоса шириной в ШАГ СТУПЕНИ, поэтому заготовка это
// длина марша × высота ступени, а не длина × высота марша.
//
// Раньше здесь брался bbox целиком: для прямого марша 16 ступеней это
// давало заготовку 4050×2660×50 мм на косоур, то есть 1,08 м³ и 8,5 тонны
// стали на марш при ставке 100 ₽/кг. Цена лестницы расходилась с ценой
// цельного дуба в 9,4 раза, и витрина по умолчанию (стальной каркас)
// показывала завышенную в десять раз сумму.
func stringerBlank(cfg *engineering.StairConfiguration, ext [3]float64) (thickness, length, width float64) {
	// Тонкая ось — это ширина сечения косоура в плане; по ней и определяем
	// «длину марша»: у прямого и L-образного марша ось подъёма может быть
	// любой из двух горизонтальных.
	thin, run := ext[1], ext[0]
	if ext[0] < ext[1] {
		thin, run = ext[0], ext[1]
	}
	// Толщина листа для металла ставится общим правилом ниже (у всех
	// металлических деталей одинаково), здесь — модельная толщина секции.
	thickness = thin
	// Ширина заготовки. Два случая:
	//
	// 	• Пластина (металлокаркас): сбоку это глухая стенка, а не пила, и
	//      развёртка — полоса шириной «глубина пластины + толщина». Длину
	//      берём по горизонтальному габариту марша (run), как и раньше:
	//      точная длина по склону отличается на несколько процентов и на
	//      проходимости по листу не сказывается, а вот с честной склонной
	//      длиной марш выше 5.7 м переставал влезать на лист 10400×6200
	//      и упирался в MFG-SHEET. Сварной стык длинного косоура — отдельная
	//      задача, здесь считаем деталь целиком.
	// 	• Гребёнка (дерево): полоса шириной в шаг ступени — зубья уходят в
	//      обрез, как и раньше.
	//
	// Раньше здесь стоял только второй случай, поэтому после перехода
	// металлокаркаса на боковую пластину заготовка считалась в разы
	// меньше реальной: 16-ступенчатый марш брался полосой «длина × шаг
	// ступени» вместо «длина × (300 + толщина)».
	if enggeo.PlateStringerFrame(cfg) {
		thickness = thin
		length = run
		// Ширина заготовки = глубина пластины + толщина ЛИСТА, а не модельная
		// толщина сечения: у металла модельная t — конструктивный габарит, и
		// в материал идёт лист 3–8 мм (то же правило, что по толщине ниже).
		return thickness, length, enggeo.StringerPlateDepthMM + laserPlateMM(thin)
	}
	width = cfg.StepHeight.Millimeters()
	if width <= 0 {
		// Шаг не задан (тестовая/вырожденная конфигурация) — берём высоту
		// марша, то есть прежнее поведение, чтобы не получить нулевую
		// площадь детали.
		width = ext[2]
	}
	length = run
	if width > length {
		length, width = width, length
	}
	return thickness, length, width
}

// columnPart вычисляет габариты развёртки центральной колонны спиральной
// лестницы (EDR-0007): толщина — толщина косоура t, длина — высота подъёма
// H (из bbox: тонкая ось колонны — диаметр, большая — высота), ширина —
// периметр 2πr, где r — радиус колонны (R − W). Длина ≥ ширина инвариант
// Part сохраняется: периметр развёртки не превышает высоты для разумных
// радиусов; если это не так, длина/ширина нормализуются.
func columnPart(cfg *engineering.StairConfiguration, ext [3]float64) (dommfg.PartKind, float64, float64, float64) {
	t := cfg.StringerThickness.Millimeters()
	if t <= 0 {
		t = 30 // безопасный минимум косоура (GEO-STRINGER-THICKNESS)
	}
	h := ext[2] // высота колонны = H
	r := cfg.OuterRadius.Millimeters() - cfg.Width.Millimeters()
	circ := 2 * math.Pi * r
	length, width := h, circ
	if width > length {
		length, width = width, length
	}
	return dommfg.PartColumn, t, length, width
}

// roleKind сопоставляет семантическую метку тела типу детали.
// Площадка (landing) — горизонтальная плита, обрабатывается как проступь
// (PartTread), но получает уникальный номер (TRD).
func roleKind(role string) (dommfg.PartKind, bool) {
	switch role {
	case "stringer":
		return dommfg.PartStringer, true
	case "tread", "landing", "winder":
		return dommfg.PartTread, true
	case "riser":
		return dommfg.PartRiser, true
	case "column":
		return dommfg.PartColumn, true
	default:
		return "", false
	}
}

// classify определяет тип детали по семантической метке (если задана) или
// по габаритам bbox (X, Y, Z) и возвращает тип, толщину (тонкий размер) и
// габариты в плоскости (Length ≥ Width).
func classify(dims [3]float64, role string) (dommfg.PartKind, float64, float64, float64) {
	// «тонкая» ось вычисляется всегда — толщина (тонкий размер) одинакова
	// вне зависимости от типа.
	thin := dims[0]
	axis := 0
	if dims[1] < thin {
		thin = dims[1]
		axis = 1
	}
	if dims[2] < thin {
		thin = dims[2]
		axis = 2
	}
	var a, b float64
	switch axis {
	case 0:
		a, b = dims[1], dims[2]
	case 1:
		a, b = dims[0], dims[2]
	default:
		a, b = dims[0], dims[1]
	}
	if a < b {
		a, b = b, a
	}
	kind, ok := roleKind(role)
	if ok {
		return kind, thin, a, b
	}
	switch axis {
	case 0:
		return dommfg.PartRiser, thin, a, b
	case 1:
		return dommfg.PartStringer, thin, a, b
	default:
		return dommfg.PartTread, thin, a, b
	}
}

// partNumber формирует детерминированный уникальный номер детали:
// STR-nn для косоуров, TRD-nn для проступей, RSR-nn для подступенков,
// CLM-nn для колонны.
func partNumber(kind dommfg.PartKind, seq int) dommfg.PartNumber {
	prefix := "STR"
	switch kind {
	case dommfg.PartStringer:
		prefix = "STR"
	case dommfg.PartTread:
		prefix = "TRD"
	case dommfg.PartRiser:
		prefix = "RSR"
	case dommfg.PartColumn:
		prefix = "CLM"
	}
	return dommfg.PartNumber(fmt.Sprintf("%s-%02d", prefix, seq))
}
