package cad

import (
	"bufio"
	"fmt"
	"io"

	kerngeo "stairplatform/internal/geometry"
)

// ДОКУМЕНТ РЕЗА (Фаза 0.2).
//
// Что здесь и почему такой формат. Лазерная резка идёт по контуру детали на
// плоскости листа: внешний контур плюс каждое отверстие внутри. Прежний
// writeDXF писал 3DFACE — треугольники меша, то есть дамп картинки. Таким
// файлом нельзя резать: лазер получил бы объёмные грани вместо плоского
// контура, а отверстий в нём не было бы вовсе.
//
// Формат — ASCII DXF R12, как и прежний (EDR-0022 §3.3), но правильный по
// существу:
//
//	внешний контур детали  → LWPOLYLINE, замкнутая
//	отверстие-окружность  → CIRCLE
//	отверстие иной формы  → LWPOLYLINE, замкнутая
//
// Про CIRCLE. Владелец потребовал «честную дугу», и это правильно: отверстие
// в документе должно быть окружностью, а не многоугольником. Подмена
// выполняется только там, где многоугольник является верным приближением
// окружности (kerngeo.CircleFromPoints проверяет и прогиб рёбер, и то, что
// вершины лежат на одной окружности). Там, где прогиб больше погрешности
// построения — например у отверстий покупного фланца — остаётся
// многоугольник, иначе документ описал бы деталь, которой нет.
//
// Отверстия, полностью лежащие ЗА пределами детали, пропускаются: это
// технологические отверстия и вырезы под рельсы в полях заготовки, лазер их
// не ведёт.

// cuttingPart — деталь в документе реза.
type cuttingPart struct {
	// Number — номер детали (стикер). Пишется в слой номеров, чтобы
	// оператор видел на экране станка, что именно вырезается.
	Number string
	// Material — код материала: детали из разных листов в одном файле не
	// смешиваются, но пометка в слое остаётся.
	Material string
	// Outer — внешний контур детали в её собственных осях раскроя, мм.
	Outer []kerngeo.Point2D
	// Holes — контуры отверстий в тех же осях.
	Holes [][]kerngeo.Point2D
	// DXF — координаты для файла: деталь сдвинута в свободное поле листа.
	DXF    kerngeo.Point2D
	Length float64
	Width  float64
}

// writeCuttingDXF — документ реза: контуры деталей с раскладкой на лист.
//
// parts — уже разложенные по листам детали (результат раскроя). Каждая
// сдвигается на своё место в поле листа, поэтому координаты в файле — это
// координаты НА ЛИСТЕ, а не детали в системе координат изделия: оператор
// загружает файл и режет по нему, не зная ничего об изделии.
func writeCuttingDXF(w io.Writer, parts []cuttingPart) error {
	bw := bufio.NewWriter(w)
	if _, err := bw.WriteString("0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n"); err != nil {
		return err
	}
	if _, err := bw.WriteString("0\nSECTION\n2\nENTITIES\n"); err != nil {
		return err
	}
	for _, p := range parts {
		if len(p.Outer) < 3 {
			continue
		}
		layer := layerFor(p.Material)
		if err := writePolyline(bw, layer, p.Outer, p.DXF, true); err != nil {
			return err
		}
		for _, h := range p.Holes {
			if !insideContour(h, p.Outer) {
				continue
			}
			if c, ok := kerngeo.CircleFromPoints(h); ok {
				if err := writeCircle(bw, layer, c, p.DXF); err != nil {
					return err
				}
				continue
			}
			if err := writePolyline(bw, layer, h, p.DXF, true); err != nil {
				return err
			}
		}
		// Номер детали отдельной сущностью на своём слое: лазер его не режет,
		// а станок показывает текст на экране, и оператор сверяет.
		if err := writeText(bw, "НOМЕРА", p.Number, p.DXF.X, p.DXF.Y); err != nil {
			return err
		}
	}
	if _, err := bw.WriteString("0\nENDSEC\n0\nEOF\n"); err != nil {
		return err
	}
	return bw.Flush()
}

// writePolyline — замкнутая ломаная LWPOLYLINE. Все точки лежат в плоскости
// детали, поэтому координата Z не пишется.
func writePolyline(w *bufio.Writer, layer string, pts []kerngeo.Point2D, off kerngeo.Point2D, closed bool) error {
	if len(pts) < 2 {
		return nil
	}
	// Флаг 1 в коде 70 = «замкнута». Для незамкнутой ломаной флаг 0.
	flag := 0
	if closed {
		flag = 1
	}
	if _, err := fmt.Fprintf(w, "0\nLWPOLYLINE\n8\n%s\n90\n%d\n70\n%d\n", layer, len(pts), flag); err != nil {
		return err
	}
	for _, p := range pts {
		if _, err := fmt.Fprintf(w, "10\n%.4f\n20\n%.4f\n", p.X+off.X, p.Y+off.Y); err != nil {
			return err
		}
	}
	return nil
}

// writeCircle — окружность CIRCLE. Это честная дуга: одна сущность вместо
// двух десятков отрезков, и радиус в файле ровно тот, по которому считали
// диаметр детали.
func writeCircle(w *bufio.Writer, layer string, c kerngeo.Circle2D, off kerngeo.Point2D) error {
	_, err := fmt.Fprintf(w, "0\nCIRCLE\n8\n%s\n10\n%.4f\n20\n%.4f\n40\n%.6f\n",
		layer, c.Center.X+off.X, c.Center.Y+off.Y, c.Radius)
	return err
}

// writeText — текстовая сущность для номера детали. Слой отдельный, чтобы
// при подготовке файла к резке его можно было отключить одной кнопкой.
func writeText(w *bufio.Writer, layer, value string, x, y float64) error {
	_, err := fmt.Fprintf(w,
		"0\nTEXT\n8\n%s\n10\n%.4f\n20\n%.4f\n40\n%.2f\n1\n%s\n",
		layer, x, y, textHeightMM, value)
	return err
}

// insideContour — лежит ли кольцо целиком внутри внешнего контура.
//
// Точка внутри многоугольника проверяется лучом по знаку пересечений
// (правило чётности). Кольцо берётся целиком: если хоть одна вершина вне
// детали, отверстие считается технологическим и в рез не идёт.
func insideContour(ring []kerngeo.Point2D, outer []kerngeo.Point2D) bool {
	if len(ring) < 3 || len(outer) < 3 {
		return false
	}
	for _, p := range ring {
		if !pointInContour(p, outer) {
			return false
		}
	}
	return true
}

// pointInContour — точка внутри замкнутого контура. Направление луча
// ЗАФИКСИРОВАНО осью X: правило чётности даёт один и тот же ответ при
// любом обходе контура, а выбор луча «по направлению» сделал бы результат
// зависящим от порядка вершин (ADR-0007).
func pointInContour(p kerngeo.Point2D, contour []kerngeo.Point2D) bool {
	inside := false
	for i, j := 0, len(contour)-1; i < len(contour); j, i = i, i+1 {
		pi, pj := contour[i], contour[j]
		if (pi.Y > p.Y) != (pj.Y > p.Y) {
			// Пересечение ребра с горизонталью y = p.Y.
			xCross := pi.X + (p.Y-pi.Y)*(pj.X-pi.X)/(pj.Y-pi.Y)
			if p.X < xCross {
				inside = !inside
			}
		}
	}
	return inside
}

// layerFor — слой по материалу. Нужен для того, чтобы оператор мог видеть
// на экране станка, какая деталь из какого листа, и не поставить кусок
// дерева на место стального.
func layerFor(material string) string {
	switch material {
	case "", "STEEL-S235", "STEEL-304":
		return "СТАЛЬ"
	case "WOOD-OAK":
		return "ДУБ"
	case "WOOD-SOFT":
		return "СОСНА"
	case "PLASTIC":
		return "ПЛАСТИК"
	}
	return "ПРОЧЕЕ"
}

// textHeightMM — высота номера детали в документе реза. Число на детали
// 50×50 мм должно читаться с экрана станка и с бумаги.
const textHeightMM = 5.0
