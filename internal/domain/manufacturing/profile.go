package manufacturing

// Point2D — вершина плоского контура детали в её собственной системе осей:
// X — вдоль длины, Y — вдоль ширины (S-163).
//
// Тип намеренно продублирован в домене, а не импортирован из
// internal/geometry: доменный слой не зависит от ядра (ADR-0006), а контур
// детали — это данные документа раскроя, а не геометрия сцены.
type Point2D struct {
	X float64
	Y float64
}

// ProfileAreaMM — площадь многоугольника по правилу шнуровки.
func ProfileAreaMM(p []Point2D) float64 {
	if len(p) < 3 {
		return 0
	}
	var a float64
	for i := range p {
		j := (i + 1) % len(p)
		a += p[i].X*p[j].Y - p[j].X*p[i].Y
	}
	if a < 0 {
		a = -a
	}
	return a / 2
}

// ProfileBounds — габариты контура по осям. Нужен, чтобы сходиться с
// габаритом детали: у настоящего контура он равен Length×Width, и это лучшая
// проверка, что контур извлечён правильно, а не подставлен.
func ProfileBounds(p []Point2D) (length, width float64) {
	if len(p) == 0 {
		return 0, 0
	}
	minX, minY := p[0].X, p[0].Y
	maxX, maxY := p[0].X, p[0].Y
	for _, v := range p[1:] {
		if v.X < minX {
			minX = v.X
		}
		if v.X > maxX {
			maxX = v.X
		}
		if v.Y < minY {
			minY = v.Y
		}
		if v.Y > maxY {
			maxY = v.Y
		}
	}
	return maxX - minX, maxY - minY
}

// ProfilesMatchBounds — совпадают ли габариты контура с габаритом детали.
//
// Проверка обязательна перед тем, как отдавать контур в документ раскроя:
// контур, взятый не из той заготовки, хуже отсутствия контура, потому что
// выглядит правдоподобно. Так сработало на косоуре — его тело охватывает весь
// марш (контур 4845 мм), а заготовка это одна пластина (2428 мм).
func ProfilesMatchBounds(p []Point2D, length, width, tol float64) bool {
	if len(p) < 3 {
		return false
	}
	l, w := ProfileBounds(p)
	d := func(a, b float64) float64 {
		if a > b {
			return a - b
		}
		return b - a
	}
	return d(l, length) <= tol && d(w, width) <= tol
}
