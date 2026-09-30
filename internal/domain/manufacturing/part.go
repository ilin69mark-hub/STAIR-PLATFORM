package manufacturing

import (
	"stairplatform/internal/domain/engineering"
)

// PartKind — тип производственной детали, выведенный из геометрии
// (декомпозиция, MFG-0002).
type PartKind string

const (
	// PartStringer — косоур: тело, тонкое по ширине марша (ось Y).
	PartStringer PartKind = "stringer"
	// PartTread — проступь: тело, тонкое по вертикали (ось Z).
	PartTread PartKind = "tread"
	// PartRiser — подступенок: тело, тонкое по глубине (ось X).
	PartRiser PartKind = "riser"
	// PartColumn — центральная колонна спиральной лестницы (EDR-0007):
	// вертикальный цилиндр, представляемый в раскрое как развёртка
	// (длина = высота H, ширина = периметр 2πr, толщина = косоура t).
	PartColumn PartKind = "column"
)

// IsValid проверяет корректность типа детали.
func (k PartKind) IsValid() bool {
	switch k {
	case PartStringer, PartTread, PartRiser, PartColumn:
		return true
	}
	return false
}

// PartNumber — уникальный номер производственной детали (BC-007).
type PartNumber string

// Part — производственная деталь, декомпозированная из твёрдого тела
// модели. SolidIndex трассирует деталь к геометрии (BC-007: все позиции
// BOM трассируются до геометрии). Length ≥ Width — габариты детали в
// плоскости, Thickness — тонкий размер.
type Part struct {
	Number     PartNumber
	Kind       PartKind
	Material   MaterialCode
	Thickness  engineering.Length // мм — тонкий размер
	Length     engineering.Length // мм — наибольший габарит в плоскости
	Width      engineering.Length // мм — наименьший габарит в плоскости
	SolidIndex int                // трассировка к твёрдому телу модели
	// MillEdgeLengthMM — длина кромки, которую на детали фрезеруют (0 = фрезеровка
	// не нужна). Заполняется из GenerationResult.MillingFeatures, то есть из
	// геометрии: длина ребра — не характеристика заготовки, а объём работы.
	//
	// Заготовка при этом остаётся прямоугольной: скругление носа снимается
	// сверху, поэтому раскрой и закупка листа не меняются, меняется только
	// техмаршрут и время.
	MillEdgeLengthMM float64
	// MillRadiusMM — радиус скругления, мм. В тариф не входит, но хранится
	// рядом с длиной: по детали должно быть видно, ЧТО именно фрезеровали.
	MillRadiusMM float64
}

// NeedsMilling сообщает, требует ли деталь фрезеровной операции.
func (p Part) NeedsMilling() bool { return p.MillEdgeLengthMM > 0 }
