package cad

import (
	"io"

	dommfg "stairplatform/internal/domain/manufacturing"
	kerngeo "stairplatform/internal/geometry"
)

// Мост от производственного пакета к документу реза.
//
// Почему отдельный вход, а не ещё один Format для cad.Write. Прежние три
// формата (DXF, STL, SVG) сериализуют МЕШ — картинку изделия. Документ реза
// строится по РАСКРОЮ: у него другая входная величина (карта раскроя на
// листах, а не геометрия), другой смысл координат (место детали НА ЛИСТЕ, а не
// в системе координат изделия) и другое содержимое (номер детали и отверстия).
// Отдавать это через Write(w, mesh, ...) значило бы тащить пакет в наружу.

// WriteCutting — документ реза по производственному пакету.
//
// Файл содержит все детали РАСКРОЯ, каждую на своём месте листа, с её
// контуром, отверстиями и номером. Покупные детали ограждения в файл не
// входят: листа под них не режем (согласовано с buildBOM).
//
// Листы идут подряд со сдвигом по Y: оператор загружает файл и режет листы
// один за другим, не переставляя координаты между ними.
func WriteCutting(w io.Writer, pkg *dommfg.ManufacturingPackage) error {
	if pkg == nil {
		return ErrNoPackage
	}
	if len(pkg.CutList.Items) == 0 {
		return ErrEmptyCutList
	}
	placed := placementIndex(pkg.Nesting)
	parts := make([]cuttingPart, 0, len(pkg.CutList.Items))
	for _, item := range pkg.CutList.Items {
		outer := toPoints(item.Profile)
		if len(outer) < 3 {
			// Контура нет — деталь описана прямоугольником. Рисуем
			// прямоугольник: лучше описывающий прямоугольник, чем ничего.
			outer = rectangle(item.Length.Millimeters(), item.Width.Millimeters())
		}
		holes := make([][]kerngeo.Point2D, 0, len(item.Holes))
		for _, h := range item.Holes {
			if r := toPoints(h); len(r) >= 3 {
				holes = append(holes, r)
			}
		}
		cp := cuttingPart{
			Number:   string(item.PartNumber),
			Material: string(item.MaterialCode),
			Outer:    outer,
			Holes:    holes,
			Length:   item.Length.Millimeters(),
			Width:    item.Width.Millimeters(),
		}
		if p, ok := placed[string(item.PartNumber)]; ok {
			cp.DXF = kerngeo.Point2D{X: p.X, Y: p.Y}
		}
		parts = append(parts, cp)
	}
	return writeCuttingDXF(w, parts)
}

// placementIndex — координаты деталей на листах по номеру детали.
//
// Номер здесь служит ключом, а не идентификатором для человека: он уникален
// внутри пакета (partNumber сквозной по всей нумерации), а список размещений
// по нему и собирается.
func placementIndex(n *dommfg.NestingResult) map[string]dommfg.PlacedPart {
	out := make(map[string]dommfg.PlacedPart)
	if n == nil {
		return out
	}
	for _, s := range n.Sheets {
		for _, p := range s.Placed {
			if _, ok := out[string(p.PartNumber)]; ok {
				continue
			}
			out[string(p.PartNumber)] = p
		}
	}
	return out
}

// toPoints — перенос контура из доменной формы в форму движка ядра.
func toPoints(p []dommfg.Point2D) []kerngeo.Point2D {
	if len(p) == 0 {
		return nil
	}
	out := make([]kerngeo.Point2D, 0, len(p))
	for _, v := range p {
		out = append(out, kerngeo.Point2D{X: v.X, Y: v.Y})
	}
	return out
}

// rectangle — описывающий прямоугольник детали без контура.
func rectangle(length, width float64) []kerngeo.Point2D {
	return []kerngeo.Point2D{
		{X: 0, Y: 0}, {X: length, Y: 0}, {X: length, Y: width}, {X: 0, Y: width},
	}
}
