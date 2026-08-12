package manufacturing

import (
	"stairplatform/internal/domain/engineering"
)

// Quantity — положительное целое количество деталей (BC-007).
type Quantity int

// BOMLine — строка спецификации: группа геометрически одинаковых деталей
// (одинаковый тип, материал и габариты). PartNumber — первая деталь группы,
// трассируемая к Parts.
type BOMLine struct {
	Number       int
	PartNumber   PartNumber
	Description  string
	MaterialCode MaterialCode
	Thickness    engineering.Length
	Quantity     Quantity
	Length       engineering.Length
	Width        engineering.Length
}

// BOM — спецификация изделия (MFG-0001). Порядок строк детерминирован.
type BOM struct {
	Lines []BOMLine
}

// CutItem — позиция карты раскроя: прямоугольник листового материала
// (длина × ширина × толщина) в количестве Quantity.
type CutItem struct {
	PartNumber   PartNumber
	MaterialCode MaterialCode
	Thickness    engineering.Length
	Length       engineering.Length
	Width        engineering.Length
	Quantity     Quantity
}

// CutList — карта раскроя (MFG-0001): прямоугольники, вырезаемые из
// листового материала.
type CutList struct {
	Items []CutItem
}
