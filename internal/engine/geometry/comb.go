package geometry

import (
	"fmt"
	"math"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// Металлокаркасный узел по чертежу прямого марша (freecad-out/ГЕОМЕТРИЯ.md,
// 2026-10-05). Здесь то, чего нет в общем коде марша: профиль косоура-гребёнки
// со спиной, ряд отверстий по формуле, овалы регулировки и пять пяток выреза.
//
// КООРДИНАТЫ (ADR-0008): X — направление подъёма, Y — ширина, Z — высота.
// Косоур A стоит на Y ∈ [0, t], косоур B на Y ∈ [w−t, w] — по краям марша.

// combArcToleranceMM — допуск дуги овального паза регулировки, мм (S-172).
//
// Здесь допуск жёстче, чем у остальных кривых: площадь паза входит в расчёт
// раскроя и цены, и «на глаз» её не проверить. 0,055 мм при типовом овале
// 30×12 (радиус дуги 6 мм) даёт 12 интервалов — ровно столько же, сколько
// давала прежняя константа combArcSegments, и ту же площадь 329,0 мм² против
// точных 329,1. Поменяется размер овала — сегменты пересчитаются сами.
const combArcToleranceMM = 0.055

// combStringerProfile — профиль косоура-гребёнки: сверху идёт посадочная
// линия ступеней, между посадками поднимаются зубья, спина уходит вниз на
// spine по нормали к линии подъёма.
//
// Профиль замкнут, вершин 2n+3. Верхний срез вертикален на X = n·b — там стоит
// пятка выхода. Пятка стоит на полу: нижнее ребро параллельно посадкам, его
// левый конец сходит в пол по вертикали от передней грани.
func combStringerProfile(n int, b, h, st, spine float64) ([]kerngeo.Point3, error) {
	if n < 1 {
		return nil, fmt.Errorf("geometry: comb stringer needs at least 1 step, got %d", n)
	}
	if b <= kerngeo.Precision || h <= kerngeo.Precision || st <= kerngeo.Precision || spine <= kerngeo.Precision {
		return nil, fmt.Errorf("geometry: comb stringer parameters must be positive")
	}
	l := math.Hypot(b, h)
	firstZ := h - st
	pts := []kerngeo.Point3{kerngeo.NewPoint3(0, 0, 0), kerngeo.NewPoint3(0, 0, firstZ)}
	for k := 0; k < n-1; k++ {
		pts = append(pts,
			kerngeo.NewPoint3(float64(k+1)*b, 0, float64(k+1)*h-st),
			kerngeo.NewPoint3(float64(k+1)*b, 0, float64(k+2)*h-st))
	}
	topX := float64(n) * b
	topZ := float64(n)*h - st
	pts = append(pts, kerngeo.NewPoint3(topX, 0, topZ))
	// Спина идёт ПАРАЛЛЕЛЬНО линии посадок (уклон h/b) и упирается в пол:
	// вертикальное расстояние между посадкой и спиной = spine / cos угла,
	// отсюда точка пересечения спины с полом, а по ней — верхний конец среза.
	floorX := combFloorX(n, b, h, st, spine)
	backTop := (h / b) * (topX - floorX)
	pts = append(pts,
		kerngeo.NewPoint3(topX, 0, backTop),
		kerngeo.NewPoint3(floorX, 0, 0))
	_ = l
	return pts, nil
}

// combFloorX — точка, где спина гребёнки встаёт на пол. Спина параллельна
// линии посадок и отстоит от неё на spine по нормали; вертикальный разрыв
// spine / cos угла, и из него получается X.
func combFloorX(n int, b, h, st, spine float64) float64 {
	l := math.Hypot(b, h)
	dropV := spine * l / b
	_ = n
	return math.Max(0, (dropV-(h-st))*b/h)
}

// CombBlankMM — размеры заготовки косоура-гребёнки для раскроя: длина вдоль
// склона и ширина по нормали к линии подъёма.
func CombBlankMM(n int, b, h, st, spine float64) (length, width float64) {
	l := math.Hypot(b, h)
	floorX := combFloorX(n, b, h, st, spine)
	topX := float64(n) * b
	topZ := float64(n)*h - st
	backTop := (h / b) * (topX - floorX)
	along := func(x, z float64) float64 { return x*b/l + z*h/l }
	perp := func(x, z float64) float64 { return -x*h/l + z*b/l }
	// Длина заготовки вдоль склона — от самой нижней точки до самой верхней.
	hi := math.Max(along(topX, topZ), along(topX, backTop))
	lo := math.Min(along(0, 0), along(0, h-st))
	length = hi - lo
	// Ширина — расстояние от линии посадок до спины (это и есть spine).
	width = perp(0, h-st) - perp(floorX, 0)
	return length, width
}

// HoleRow — ряд отверстий Ød по длине детали. Отступ от края edgeK·d, шаг ряда
// не меньше stride·pitchK·d («через одно»), фактический шаг пересчитывается на
// равномерный. center — добавить отверстие ровно посередине, если оно не
// слишком близко к соседнему (решение владельца для пятки 2).
//
// Формула, а не вбитые числа: длина детали меняется вместе с габаритом
// лестницы, и ряд должен пересчитаться сам.
func HoleRow(length, d, edgeK, pitchK float64, stride int, center bool) (margin, pitch float64, xs []float64) {
	margin = edgeK * d
	if stride <= 0 {
		stride = 1
	}
	step := float64(stride) * pitchK * d
	if step <= kerngeo.Precision {
		step = 3 * d
	}
	n := int(math.Floor((length-2*margin)/step)) + 1
	if n < 2 {
		n = 2
	}
	pitch = (length - 2*margin) / float64(n-1)
	xs = make([]float64, 0, n+1)
	for i := 0; i < n; i++ {
		xs = append(xs, margin+pitch*float64(i))
	}
	if center && len(xs) > 1 {
		c := length / 2
		far := true
		for _, x := range xs {
			if math.Abs(c-x) <= d {
				far = false
				break
			}
		}
		if far {
			xs = append(xs, c)
			sortFloats(xs)
		}
	}
	return margin, pitch, xs
}

func sortFloats(v []float64) {
	for i := 1; i < len(v); i++ {
		x := v[i]
		j := i - 1
		for j >= 0 && v[j] > x {
			v[j+1] = v[j]
			j--
		}
		v[j+1] = x
	}
}

// SlotTail — хвост регулировочного овала: ±(длина − диаметр)/2 от центра.
func SlotTail(slotLen, d float64) float64 { return (slotLen - d) / 2 }

// circleRing — окружность отверстия в плоскости, заданной осями u и v.
func circleRing(center kerngeo.Point3, u, v kerngeo.Vector3, radius float64, segments int) []kerngeo.Point3 {
	out := make([]kerngeo.Point3, segments)
	for i := 0; i < segments; i++ {
		a := 2 * math.Pi * float64(i) / float64(segments)
		out[i] = center.
			Add(u.Scale(radius * math.Cos(a))).
			Add(v.Scale(radius * math.Sin(a)))
	}
	return out
}

// stadiumRing — овал длиной length вдоль оси u и шириной width вдоль оси v:
// две дуги радиуса width/2 плюс прямые участки между ними.
func stadiumRing(q Quality, center kerngeo.Point3, u, v kerngeo.Vector3, length, width float64) []kerngeo.Point3 {
	r := width / 2
	half := length/2 - r
	if half < 0 {
		half = 0
	}
	var out []kerngeo.Point3
	arc := func(offU float64, from, to float64) {
		segs := kerngeo.ArcSegments(r, math.Abs(to-from), q.arcTol(combArcToleranceMM))
		for i := 0; i <= segs; i++ {
			a := from + (to-from)*float64(i)/float64(segs)
			out = append(out, center.
				Add(u.Scale(offU+r*math.Cos(a))).
				Add(v.Scale(r*math.Sin(a))))
		}
	}
	arc(half, -math.Pi/2, math.Pi/2)
	arc(-half, math.Pi/2, 3*math.Pi/2)
	return out
}

// anchorHoles — раскладка отверстий пятки пола и пятки выхода: ПЕРВОЕ —
// продольный овал (регулировка вдоль детали), ПОСЛЕДНЕЕ — поперечный
// (регулировка по ширине пятки), всё между ними — круглые. Форма задаётся
// НОМЕРОМ, а не позицией: отверстий может быть сколько угодно.
func anchorHoles(q Quality, spec *engineering.CombFrame, xs []float64,
	origin kerngeo.Point3, along, across kerngeo.Vector3) [][]kerngeo.Point3 {
	holes := make([][]kerngeo.Point3, 0, len(xs))
	for i, x := range xs {
		p := origin.Add(along.Scale(x))
		switch {
		case i == 0:
			holes = append(holes, stadiumRing(q, p, along, across, spec.SlotLength, spec.AnchorHoleDiameter))
		case i == len(xs)-1:
			holes = append(holes, stadiumRing(q, p, across, along, spec.SlotLength, spec.AnchorHoleDiameter))
		default:
			holes = append(holes, circleRing(p, along, across, spec.AnchorHoleDiameter/2, spec.HoleSegments))
		}
	}
	return holes
}
