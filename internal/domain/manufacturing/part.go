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
)

// IsValid проверяет корректность типа детали.
func (k PartKind) IsValid() bool {
	switch k {
	case PartStringer, PartTread, PartRiser:
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
}
