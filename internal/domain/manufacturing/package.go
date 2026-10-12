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
	Nesting *NestingResult
	// Fasteners — потребность в крепеже (MNT-0001): что и сколько штук
	// ввернуть. Считается по отверстиям пяток и по доп. крепежу объекта.
	//
	// Это физическая величина (штуки), а не стоимость: цены живут в ставках
	// расчёта, как и цены материалов за кг.
	Fasteners []FastenerDemand
	// Sealant — потребность в герметике (CONF-SEALANT): площадь швов,
	// толщина слоя, объём и картриджи. nil, если герметик не считался.
	Sealant *SealantDemand
	// SealantZones — площади швов по зонам для детализации сметы.
	SealantZones SealantZones
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

	if p.Nesting == nil {
		return fmt.Errorf("manufacturing: package has no nesting result")
	}
	if err := p.Nesting.Validate(); err != nil {
		return err
	}
	// Раскрой содержит только НЕпокупные детали: покупные приходят готовыми,
	// листа под них не режем. Поэтому сверяем число раскроенных деталей с
	// числом деталей пакета за вычетом покупных (ограждение, крепёж).
	cuttable := 0
	for _, part := range p.Parts {
		if !part.Purchased {
			cuttable++
		}
	}
	if int(p.Nesting.PartCount) != cuttable {
		return fmt.Errorf("manufacturing: nesting parts (%d) must match cuttable parts (%d of %d)",
			p.Nesting.PartCount, cuttable, len(p.Parts))
	}
	for _, sheet := range p.Nesting.Sheets {
		for _, pl := range sheet.Placed {
			if !seenNumbers[pl.PartNumber] {
				return fmt.Errorf("manufacturing: nesting references unknown part %q", pl.PartNumber)
			}
		}
	}
	return nil
}
