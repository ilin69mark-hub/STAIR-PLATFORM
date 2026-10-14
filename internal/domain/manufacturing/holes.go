package manufacturing

import "math"

// HolesWithinProfile — отверстия, лежащие внутри контура детали.
//
// Зачем фильтр. Отверстия снимаются с тела целиком, вместе с полями и
// вырезами под рельсы в заготовке. В документ реза они попадать не должны:
// лазер ведёт луч по внешнему контуру и по отверстиям ВНУТРИ него. Отверстие
// в поле лазер всё равно не вырежет, а в файле будет выглядеть как дефект
// геометрии.
//
// Отверстие берётся ЦЕЛИКОМ: если хоть одна его вершина вне контура, оно не
// внутреннее. Так ведут себя и CAM-системы — частично вырезанное отверстие в
// поле детали не считается отверстием детали.
//
// Пустой контур означает «деталь описана прямоугольником», и тогда отверстия
// определить нечем: возвращается пустой срез. Выдумывать прямоугольник ради
// проверки означало бы проверять выдумку.
func HolesWithinProfile(holes [][]Point2D, profile []Point2D) [][]Point2D {
	if len(holes) == 0 || len(profile) < 3 {
		return nil
	}
	out := make([][]Point2D, 0, len(holes))
	for _, h := range holes {
		if len(h) < 3 || !ringWithin(h, profile) {
			continue
		}
		out = append(out, h)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ringWithin — всё кольцо внутри контура.
func ringWithin(ring, contour []Point2D) bool {
	for _, p := range ring {
		if !PointInContour(p, contour) {
			return false
		}
	}
	return true
}

// PointInContour — точка внутри замкнутого контура правилом чётности.
//
// Луч ЗАФИКСИРОВАН осью X. Правило чётности не зависит от направления обхода
// контура, но зависит от направления луча: выбирать луч «по направлению» —
// значит сделать ответ зависимым от порядка вершин, а это ломает
// воспроизводимость (ADR-0007). Точки на границе считаются ВНУТРИ, чтобы
// отверстие, построенное ровно по краю детали, не отфильтровалось.
func PointInContour(p Point2D, contour []Point2D) bool {
	if len(contour) < 3 {
		return false
	}
	inside := false
	for i, j := 0, len(contour)-1; i < len(contour); j, i = i, i+1 {
		pi, pj := contour[i], contour[j]
		if (pi.Y > p.Y) != (pj.Y > p.Y) {
			xCross := pi.X + (p.Y-pi.Y)*(pj.X-pi.X)/(pj.Y-pi.Y)
			if p.X < xCross {
				inside = !inside
			}
		}
	}
	// Точка на ребре контура считается внутри: отверстие, построенное по
	// самому краю детали, отверстием детали и является.
	for i, j := 0, len(contour)-1; i < len(contour); j, i = i, i+1 {
		if distanceToSegment(p, contour[i], contour[j]) <= ContourToleranceMM {
			return true
		}
	}
	return inside
}

// ContourToleranceMM — насколько точка может лежать «на ребре» контура и
// считаться на нём. Меньше допуска геометрии (Precision) в тысячу раз:
// контуры строятся с точностью до 1e-9 мм, и проверка должна ловить именно
// настоящее попадание на ребро, а не численный шум.
const ContourToleranceMM = 1e-6

// distanceToSegment — расстояние от точки до отрезка.
func distanceToSegment(p, a, b Point2D) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / l2
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(p.X-(a.X+t*dx), p.Y-(a.Y+t*dy))
}
