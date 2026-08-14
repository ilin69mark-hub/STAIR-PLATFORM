package manufacturing

import (
	"fmt"
	"math"
	"runtime"

	"golang.org/x/sync/errgroup"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
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
			if solid.Role() == "column" {
				// EDR-0007: развёртка колонны (толщина косоура, H×2πr).
				s.kind, s.thickness, s.length, s.width = columnPart(cfg, ext)
			} else {
				s.kind, s.thickness, s.length, s.width = classify(ext, solid.Role())
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
	case "tread", "landing":
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
	case dommfg.PartTread:
		prefix = "TRD"
	case dommfg.PartRiser:
		prefix = "RSR"
	case dommfg.PartColumn:
		prefix = "CLM"
	}
	return dommfg.PartNumber(fmt.Sprintf("%s-%02d", prefix, seq))
}
