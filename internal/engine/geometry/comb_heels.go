package geometry

import (
	"fmt"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// ПЯТКИ ВЫРЕЗА — пять деталей обоймы, которых в движке марша не было
// (решение владельца 2026-10-05, чертёж freecad-out/ГЕОМЕТРИЯ.md).
//
// Зачем они: стальная ступень и подступенок крепятся к косоуру не напрямую —
// между ними стоят закладные детали. Пятка 1 держит ступень на посадке
// гребёнки, пятка 2 одевает вертикальный срез зуба. Вместе с подступенком
// они образуют Г-образную обойму выреза. Плюс две пятки крепления: косоур к
// полу и к перекрытию второго этажа — в них отверстия РЕГУЛИРОВОЧНЫЕ (овалы
// двух направлений), потому что при монтаже косоур ставят с допуском.
//
// Все пятки — плоские детали из листа CombFrame.HeelThickness с отверстиями;
// режутся лазером вместе с контуром, отдельного сверления нет.

// combHeelSpec — размеры обоймы выреза, общие для всех пяти деталей.
type combHeelSpec struct {
	w, t      float64 // ширина марша, толщина косоура
	n         int     // ступеней
	b, h, st  float64 // шаг, подъём, толщина ступени
	heelT     float64 // толщина пяток
	heelW     float64 // ширина пятки
	inset     float64 // отступ пятки 1 от зуба и подступенка
	gap       float64 // зазор между пятками
	firstWebW float64 // ширина пластины 1-й ступени
	firstWebH float64 // высота пластины 1-й ступени
	firstWebZ float64 // низ пластины 1-й ступени
	topZ      float64 // верх последней ступени
	anchorL   float64 // длина пятки пола
}

// combHeelSpecOf собирает размеры обоймы из конфигурации и спецификации узла.
func combHeelSpecOf(cfg *engineering.StairConfiguration) combHeelSpec {
	sp := cfg.CombOrDefault()
	return combHeelSpec{
		w:         cfg.Width.Millimeters(),
		t:         cfg.StringerThickness.Millimeters(),
		n:         cfg.StepCount,
		b:         cfg.TreadDepth.Millimeters(),
		h:         cfg.StepHeight.Millimeters(),
		st:        cfg.StepThickness.Millimeters(),
		heelT:     sp.HeelThickness,
		heelW:     sp.HeelWidth,
		inset:     sp.HeelInset,
		gap:       sp.HeelGap,
		firstWebW: sp.HeelWidth - 2*sp.HeelInset,
		firstWebH: firstWebHeight(cfg, sp),
		firstWebZ: sp.HeelThickness + sp.HeelGap,
		topZ:      float64(cfg.StepCount)*cfg.StepHeight.Millimeters() - cfg.StepThickness.Millimeters(),
		anchorL:   cfg.TreadDepth.Millimeters(),
	}
}

// firstWebHeight — высота пластины 1-й ступени: снизу над пяткой пола, сверху
// под пяткой 1 первой ступени, с зазором gap с каждой стороны.
func firstWebHeight(cfg *engineering.StairConfiguration, sp *engineering.CombFrame) float64 {
	seatZ := cfg.StepHeight.Millimeters() - cfg.StepThickness.Millimeters()
	return seatZ - 2*sp.HeelThickness - 2*sp.HeelGap
}

// webHeight — длина пятки 2: подъём ступени минус толщина пяток и два зазора
// (снизу до пятки 1 своей ступени, сверху до пятки 1 следующей).
func webHeight(cfg *engineering.StairConfiguration, sp *engineering.CombFrame) float64 {
	return cfg.StepHeight.Millimeters() - sp.HeelThickness - 2*sp.HeelGap
}

// combHeelSolid — пятка 1: плашмя под посадкой ступени, широкая грань
// 270×50 вверх к ступени, торец по X упирается в стенку зуба (заднюю стенку
// выреза). Отступ inset с каждой стороны от зуба и от подступенка.
func combHeelSolid(cfg *engineering.StairConfiguration, sp *engineering.CombFrame,
	k int, hy0 float64) (*kerngeo.Solid, error) {
	s := combHeelSpecOf(cfg)
	heelL := s.b - 2*s.inset
	x0 := float64(k)*s.b + s.inset
	zTop := float64(k+1)*s.h - s.st
	zBot := zTop - s.heelT
	plate := profileXY(x0, x0+heelL, hy0, hy0+s.heelW, zBot)
	_, _, xs := HoleRow(heelL, sp.HoleDiameter, sp.HoleEdgeK, sp.HolePitchK, sp.HoleStride, false)
	holes := make([][]kerngeo.Point3, 0, len(xs))
	for _, x := range xs {
		holes = append(holes, circleRing(
			kerngeo.NewPoint3(x0+x, hy0+s.heelW/2, zBot),
			kerngeo.NewVector3(1, 0, 0), kerngeo.NewVector3(0, 1, 0),
			sp.HoleDiameter/2, sp.HoleSegments))
	}
	solid, err := kerngeo.ExtrudeWithHoles(plate, holes, kerngeo.NewVector3(0, 0, 1), s.heelT)
	if err != nil {
		return nil, fmt.Errorf("geometry: heel %d: %w", k+1, err)
	}
	// Отверстия пятки 1 держат ступень на посадке: крепёж идёт сквозь проступь
	// в эти отверстия, поэтому их число — это и есть число крепежей ступени.
	return solid.WithRole("heel").WithHoles(len(holes)), nil
}

// combWebSolid — пятка 2: вдоль вертикального реза зуба, тонким ребром в
// проём к зубу, стоит в Г-обойме рядом с подступенком. Грань в плоскости зуба.
//
// Параметра Quality здесь нет намеренно: число сегментов отверстий приходит
// из спецификации (sp.HoleSegments), а не из режима качества. Пять соседних
// функций его принимают, эти три — нет, и это расхождение выглядит как
// ошибка только потому, что сигнатуры соседние.
func combWebSolid(cfg *engineering.StairConfiguration, sp *engineering.CombFrame,
	k int, hy0 float64) (*kerngeo.Solid, error) {
	s := combHeelSpecOf(cfg)
	webH := webHeight(cfg, sp)
	xTooth := float64(k+1) * s.b
	zBot := float64(k+1)*s.h - s.st + s.heelT + s.gap
	plate := profileYZAt(xTooth, zBot, zBot+webH, hy0, hy0+s.heelW)
	_, _, zs := HoleRow(webH, sp.HoleDiameter, sp.HoleEdgeK, sp.HolePitchK, sp.HoleStride, true)
	holes := make([][]kerngeo.Point3, 0, len(zs))
	for _, z := range zs {
		holes = append(holes, circleRing(
			kerngeo.NewPoint3(xTooth, hy0+s.heelW/2, zBot+z),
			kerngeo.NewVector3(0, 0, 1), kerngeo.NewVector3(0, 1, 0),
			sp.HoleDiameter/2, sp.HoleSegments))
	}
	solid, err := kerngeo.ExtrudeWithHoles(plate, holes, kerngeo.NewVector3(1, 0, 0), s.heelT)
	if err != nil {
		return nil, fmt.Errorf("geometry: web %d: %w", k+1, err)
	}
	// Пятка 2 держит подступенок: крепёж подступенка идёт в эти отверстия.
	return solid.WithRole("web").WithHoles(len(holes)), nil
}

// combAnchorSolid — пятка пола: плашмя на полу в нулевой точке касания
// лестницы с полом, длина во всю посадку (торцам упираться не на что).
// Отверстия регулировочные: первое — продольный овал, последнее — поперечный.
func combAnchorSolid(q Quality, cfg *engineering.StairConfiguration, sp *engineering.CombFrame,
	hy0 float64) (*kerngeo.Solid, error) {
	s := combHeelSpecOf(cfg)
	plate := profileXY(0, s.anchorL, hy0, hy0+s.heelW, 0)
	_, _, xs := HoleRow(s.anchorL, sp.AnchorHoleDiameter, sp.HoleEdgeK, sp.HolePitchK, sp.HoleStride, false)
	holes := anchorHoles(q, sp, xs,
		kerngeo.NewPoint3(0, hy0+s.heelW/2, 0),
		kerngeo.NewVector3(1, 0, 0), kerngeo.NewVector3(0, 1, 0))
	solid, err := kerngeo.ExtrudeWithHoles(plate, holes, kerngeo.NewVector3(0, 0, 1), s.heelT)
	if err != nil {
		return nil, fmt.Errorf("geometry: anchor: %w", err)
	}
	// Пятка пола: одно отверстие — один комплект крепежа в пол
	// (дюбель + глухарь в бетон, только глухарь в дерево).
	return solid.WithRole("anchor").WithHoles(len(holes)), nil
}

// combTopAnchorSolid — пятка выхода на второй этаж: вертикально у самого
// конца марша, грань вровень с крайней вертикальной гранью косоура, ничего за
// неё не выходит. Опущена на (толщина пятки 1 + зазор), поэтому до пятки 1
// сверху остаётся тот же зазор, что в обойме выреза.
func combTopAnchorSolid(q Quality, cfg *engineering.StairConfiguration, sp *engineering.CombFrame,
	hy0 float64) (*kerngeo.Solid, error) {
	s := combHeelSpecOf(cfg)
	webH := webHeight(cfg, sp)
	topZ := s.topZ
	drop := s.heelT + s.gap
	zTop := topZ - drop
	x0 := float64(s.n)*s.b - s.heelT
	plate := profileYZAt(x0, zTop-webH, zTop, hy0, hy0+s.heelW)
	// Ряд с центральным отверстием: у пятки выхода их три — овал, круглое
	// посередине, овал (чертёж 2026-10-05).
	_, _, zs := HoleRow(webH, sp.AnchorHoleDiameter, sp.HoleEdgeK, sp.HolePitchK, sp.HoleStride, true)
	origin := kerngeo.NewPoint3(x0, hy0+s.heelW/2, zTop-webH)
	holes := anchorHoles(q, sp, zs, origin,
		kerngeo.NewVector3(0, 0, 1), kerngeo.NewVector3(0, 1, 0))
	solid, err := kerngeo.ExtrudeWithHoles(plate, holes, kerngeo.NewVector3(1, 0, 0), s.heelT)
	if err != nil {
		return nil, fmt.Errorf("geometry: top anchor: %w", err)
	}
	// Пятка выхода на перекрытие: столько же комплектов крепежа, сколько
	// отверстий.
	return solid.WithRole("top_anchor").WithHoles(len(holes)), nil
}

// combFirstWebSolid — пластина 1-й ступени: ВНУТРИ косоара, между косоурами
// под первой ступенью, вплотную к подступенку №1 и с зазором до соседних
// пяток. По решению владельца 2026-10-05 на 5 мм меньше с каждой стороны по
// ширине, чем остальные пятки.
func combFirstWebSolid(cfg *engineering.StairConfiguration, sp *engineering.CombFrame,
	hy0 float64) (*kerngeo.Solid, error) {
	s := combHeelSpecOf(cfg)
	plate := profileYZAt(0, s.firstWebZ, s.firstWebZ+s.firstWebH, hy0, hy0+s.firstWebW)
	_, _, zs := HoleRow(s.firstWebH, sp.HoleDiameter, sp.HoleEdgeK, sp.HolePitchK, sp.HoleStride, true)
	holes := make([][]kerngeo.Point3, 0, len(zs))
	for _, z := range zs {
		holes = append(holes, circleRing(
			kerngeo.NewPoint3(0, hy0+s.firstWebW/2, s.firstWebZ+z),
			kerngeo.NewVector3(0, 0, 1), kerngeo.NewVector3(0, 1, 0),
			sp.HoleDiameter/2, sp.HoleSegments))
	}
	solid, err := kerngeo.ExtrudeWithHoles(plate, holes, kerngeo.NewVector3(1, 0, 0), s.heelT)
	if err != nil {
		return nil, fmt.Errorf("geometry: first web: %w", err)
	}
	// Пластина 1-й ступени — подкладка подступенка №1: крепёж подступенка.
	return solid.WithRole("first_web").WithHoles(len(holes)), nil
}

// Профили плит всегда ПЛОСКИЕ и лежат в плоскости вытягивания: лежащая
// плита — это прямоугольник XY и вытягивание по +Z на толщину листа;
// вертикальная плита — прямоугольник YZ и вытягивание по +X. Контуры
// отверстий обязаны лежать В ТОЙ ЖЕ плоскости, что и профиль: стенка
// отверстия строится как ребро профиля, вытянутое на ту же величину.

// profileXY — прямоугольный профиль в плоскости XY на отметке z.
func profileXY(x0, x1, y0, y1, z float64) []kerngeo.Point3 {
	return []kerngeo.Point3{
		kerngeo.NewPoint3(x0, y0, z), kerngeo.NewPoint3(x1, y0, z),
		kerngeo.NewPoint3(x1, y1, z), kerngeo.NewPoint3(x0, y1, z),
	}
}

// profileYZ — прямоугольный профиль в плоскости YZ (x = 0), сдвинутый по X
// в начало координат вытягивания.
func profileYZAt(x, z0, z1, y0, y1 float64) []kerngeo.Point3 {
	return []kerngeo.Point3{
		kerngeo.NewPoint3(x, y0, z0), kerngeo.NewPoint3(x, y1, z0),
		kerngeo.NewPoint3(x, y1, z1), kerngeo.NewPoint3(x, y0, z1),
	}
}
