package manufacturing

import (
	"fmt"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	kerngeo "stairplatform/internal/geometry"
)

// decompose превращает модель в производственные детали: каждое твёрдое
// тело становится Part (MFG-0002 Part Decomposition). Тип детали
// определяется «тонкой» осью ограничивающего параллелепипеда (косоур
// тонок по Y, проступь по Z, подступенок по X) — признак выводится из
// геометрии и не зависит от порядка тел. SolidIndex трассирует деталь
// к телу модели (BC-007).
func decompose(model *kerngeo.Compound) ([]dommfg.Part, error) {
	if model == nil {
		return nil, fmt.Errorf("manufacturing: model is required")
	}
	solids := model.Solids()
	if len(solids) == 0 {
		return nil, fmt.Errorf("manufacturing: model has no solids")
	}

	parts := make([]dommfg.Part, 0, len(solids))
	seq := make(map[dommfg.PartKind]int)
	for i, solid := range solids {
		bb := kerngeo.BoundingBox(kerngeo.NewCompound(solid))
		ext := [3]float64{
			bb.Max.X - bb.Min.X,
			bb.Max.Y - bb.Min.Y,
			bb.Max.Z - bb.Min.Z,
		}
		kind, thickness, length, width := classify(ext)
		seq[kind]++

		mk := func(v float64) (engineering.Length, error) {
			return engineering.NewLength(v)
		}
		thick, err := mk(thickness)
		if err != nil {
			return nil, fmt.Errorf("manufacturing: part %d: %w", i, err)
		}
		lng, err := mk(length)
		if err != nil {
			return nil, fmt.Errorf("manufacturing: part %d: %w", i, err)
		}
		wid, err := mk(width)
		if err != nil {
			return nil, fmt.Errorf("manufacturing: part %d: %w", i, err)
		}
		parts = append(parts, dommfg.Part{
			Number:     partNumber(kind, seq[kind]),
			Kind:       kind,
			Thickness:  thick,
			Length:     lng,
			Width:      wid,
			SolidIndex: i,
		})
	}
	return parts, nil
}

// classify определяет тип детали по габаритам bbox (X, Y, Z) и возвращает
// тип, толщину (тонкий размер) и габариты в плоскости (Length ≥ Width).
func classify(dims [3]float64) (dommfg.PartKind, float64, float64, float64) {
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
	var kind dommfg.PartKind
	var a, b float64
	switch axis {
	case 0:
		kind = dommfg.PartRiser // тонок по X (глубина)
		a, b = dims[1], dims[2]
	case 1:
		kind = dommfg.PartStringer // тонок по Y (ширина марша)
		a, b = dims[0], dims[2]
	default:
		kind = dommfg.PartTread // тонок по Z (вертикаль)
		a, b = dims[0], dims[1]
	}
	if a < b {
		return kind, thin, b, a
	}
	return kind, thin, a, b
}

// partNumber формирует детерминированный уникальный номер детали:
// STR-nn для косоуров, TRD-nn для проступей, RSR-nn для подступенков.
func partNumber(kind dommfg.PartKind, seq int) dommfg.PartNumber {
	prefix := "STR"
	switch kind {
	case dommfg.PartTread:
		prefix = "TRD"
	case dommfg.PartRiser:
		prefix = "RSR"
	}
	return dommfg.PartNumber(fmt.Sprintf("%s-%02d", prefix, seq))
}
