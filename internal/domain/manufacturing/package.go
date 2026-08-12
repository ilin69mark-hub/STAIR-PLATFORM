package manufacturing

import (
	"fmt"
)

// ManufacturingPackage — aggregate root Manufacturing (BC-007): комплект
// производственных данных для одной валидной ревизии модели.
type ManufacturingPackage struct {
	Parts   []Part
	BOM     BOM
	CutList CutList
}

// Validate проверяет инварианты Manufacturing (BC-007): наличие деталей,
// уникальные номера деталей, валидный тип и обязательный материал,
// положительные размеры и количества, трассируемость BOM к Parts.
func (p *ManufacturingPackage) Validate() error {
	if p == nil {
		return fmt.Errorf("manufacturing: package is required")
	}
	if len(p.Parts) == 0 {
		return fmt.Errorf("manufacturing: package has no parts")
	}

	seenNumbers := make(map[PartNumber]bool, len(p.Parts))
	for i, part := range p.Parts {
		if part.Number == "" {
			return fmt.Errorf("manufacturing: part %d has empty number", i)
		}
		if seenNumbers[part.Number] {
			return fmt.Errorf("manufacturing: duplicate part number %q", part.Number)
		}
		seenNumbers[part.Number] = true
		if !part.Kind.IsValid() {
			return fmt.Errorf("manufacturing: part %q has invalid kind %q", part.Number, part.Kind)
		}
		if part.Material == "" {
			return fmt.Errorf("manufacturing: part %q has no material", part.Number)
		}
		if part.Thickness.Millimeters() <= 0 {
			return fmt.Errorf("manufacturing: part %q has non-positive thickness", part.Number)
		}
		if part.Length.Millimeters() <= 0 || part.Length.Millimeters() < part.Width.Millimeters() {
			return fmt.Errorf("manufacturing: part %q has invalid dimensions", part.Number)
		}
		if part.SolidIndex < 0 {
			return fmt.Errorf("manufacturing: part %q has invalid solid index", part.Number)
		}
	}

	if len(p.BOM.Lines) == 0 {
		return fmt.Errorf("manufacturing: BOM has no lines")
	}
	for i, line := range p.BOM.Lines {
		if line.Number != i+1 {
			return fmt.Errorf("manufacturing: BOM line numbers must be sequential")
		}
		if line.Quantity <= 0 {
			return fmt.Errorf("manufacturing: BOM line %d has non-positive quantity", line.Number)
		}
		if !seenNumbers[line.PartNumber] {
			return fmt.Errorf("manufacturing: BOM line %d references unknown part %q", line.Number, line.PartNumber)
		}
	}
	return nil
}
