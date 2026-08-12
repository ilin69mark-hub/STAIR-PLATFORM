package manufacturing

import (
	"fmt"
	"math"

	"stairplatform/internal/domain/engineering"
)

// StockSheet — стандартный лист материала (MFG-0012): прямоугольные
// габариты Length × Width (мм, Length ≥ Width). Каталог хранит размеры
// листов; материал и толщина раскраиваемых деталей определяются группой
// раскроя (MaterialCode + Thickness), а не листом.
type StockSheet struct {
	MaterialCode MaterialCode
	Length       engineering.Length
	Width        engineering.Length
}

// Validate проверяет корректность записи листа.
func (s *StockSheet) Validate() error {
	if s == nil {
		return fmt.Errorf("manufacturing: stock sheet is required")
	}
	if s.MaterialCode == "" {
		return fmt.Errorf("manufacturing: stock sheet material must not be empty")
	}
	if math.IsNaN(s.Length.Millimeters()) || math.IsInf(s.Length.Millimeters(), 0) ||
		s.Length.Millimeters() <= 0 || s.Width.Millimeters() <= 0 {
		return fmt.Errorf("manufacturing: stock sheet dimensions must be positive")
	}
	if s.Length.Millimeters() < s.Width.Millimeters() {
		return fmt.Errorf("manufacturing: stock sheet length must not be less than width")
	}
	return nil
}

// StockSheetRegistry — детерминированный in-memory каталог стандартных
// листов (MFG-0012). Каталог неизменяем после создания; порядок листов
// для каждого материала фиксирован.
type StockSheetRegistry struct {
	byMaterial map[MaterialCode][]*StockSheet
	order      []MaterialCode
}

// NewStockSheetRegistry создаёт реестр; дубликаты листов (материал +
// размеры) и невалидные записи отклоняются.
func NewStockSheetRegistry(sheets ...*StockSheet) (*StockSheetRegistry, error) {
	r := &StockSheetRegistry{byMaterial: make(map[MaterialCode][]*StockSheet)}
	for _, s := range sheets {
		if err := s.Validate(); err != nil {
			return nil, err
		}
		for _, existing := range r.byMaterial[s.MaterialCode] {
			if existing.Length.Equals(s.Length) && existing.Width.Equals(s.Width) {
				return nil, fmt.Errorf("manufacturing: duplicate stock sheet for material %q", s.MaterialCode)
			}
		}
		if _, ok := r.byMaterial[s.MaterialCode]; !ok {
			r.order = append(r.order, s.MaterialCode)
		}
		r.byMaterial[s.MaterialCode] = append(r.byMaterial[s.MaterialCode], s)
	}
	return r, nil
}

// SheetsFor возвращает листы материала в порядке реестра.
func (r *StockSheetRegistry) SheetsFor(code MaterialCode) []*StockSheet {
	if r == nil {
		return nil
	}
	return r.byMaterial[code]
}

// PlacedPart — деталь, размещённая на листе (MFG-0012): прямоугольник
// Length × Width с левым нижним углом в точке (X, Y) листа (мм).
// Трассируется к Parts по PartNumber.
type PlacedPart struct {
	PartNumber PartNumber
	Length     engineering.Length
	Width      engineering.Length
	X          float64 // мм — вдоль длины листа
	Y          float64 // мм — вдоль ширины листа
}

// SheetLayout — заполненный лист: размещённые детали.
type SheetLayout struct {
	MaterialCode MaterialCode
	Thickness    engineering.Length
	Length       engineering.Length
	Width        engineering.Length
	Placed       []PlacedPart
}

// NestingResult — результат раскроя (MFG-0012): все детали карты раскроя
// размещены на стандартных листах. Метрики: суммарные площади деталей и
// листов, отходы и утилизация листа (доля площади, занятая деталями).
type NestingResult struct {
	Sheets      []SheetLayout
	PartCount   Quantity
	PartArea    float64 // мм²
	SheetArea   float64 // мм²
	WasteArea   float64 // мм²
	Utilization float64 // 0..1
}

// Validate проверяет инварианты результата раскроя: непустой набор
// листов, корректность метрик и размещение деталей в границах листов.
func (n *NestingResult) Validate() error {
	if n == nil {
		return fmt.Errorf("manufacturing: nesting result is required")
	}
	if len(n.Sheets) == 0 {
		return fmt.Errorf("manufacturing: nesting has no sheets")
	}
	if n.PartCount <= 0 {
		return fmt.Errorf("manufacturing: nesting has no parts")
	}
	if !finite(n.PartArea) || !finite(n.SheetArea) || !finite(n.WasteArea) || !finite(n.Utilization) {
		return fmt.Errorf("manufacturing: nesting metrics are invalid")
	}
	if n.SheetArea <= 0 || n.PartArea > n.SheetArea+engineering.Precision {
		return fmt.Errorf("manufacturing: nesting areas are inconsistent")
	}
	if math.Abs(n.WasteArea-(n.SheetArea-n.PartArea)) > engineering.Precision {
		return fmt.Errorf("manufacturing: nesting waste area is inconsistent")
	}
	if n.Utilization < 0 || n.Utilization > 1+engineering.Precision {
		return fmt.Errorf("manufacturing: nesting utilization is out of range")
	}

	seenParts := 0
	for i, s := range n.Sheets {
		if s.MaterialCode == "" {
			return fmt.Errorf("manufacturing: sheet %d has no material", i)
		}
		if s.Thickness.Millimeters() <= 0 {
			return fmt.Errorf("manufacturing: sheet %d has non-positive thickness", i)
		}
		if s.Length.Millimeters() <= 0 || s.Width.Millimeters() <= 0 {
			return fmt.Errorf("manufacturing: sheet %d has invalid dimensions", i)
		}
		for j, p := range s.Placed {
			if p.PartNumber == "" {
				return fmt.Errorf("manufacturing: sheet %d placed part %d has no number", i, j)
			}
			if p.Length.Millimeters() <= 0 || p.Width.Millimeters() <= 0 {
				return fmt.Errorf("manufacturing: sheet %d placed part %q has invalid dimensions", i, p.PartNumber)
			}
			if p.X < 0 || p.Y < 0 {
				return fmt.Errorf("manufacturing: sheet %d placed part %q has negative position", i, p.PartNumber)
			}
			if p.X+p.Length.Millimeters() > s.Length.Millimeters()+engineering.Precision ||
				p.Y+p.Width.Millimeters() > s.Width.Millimeters()+engineering.Precision {
				return fmt.Errorf("manufacturing: sheet %d placed part %q exceeds sheet bounds", i, p.PartNumber)
			}
		}
		seenParts += len(s.Placed)
	}
	if seenParts != int(n.PartCount) {
		return fmt.Errorf("manufacturing: nesting placed parts (%d) must match part count (%d)", seenParts, int(n.PartCount))
	}
	return nil
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
