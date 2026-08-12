// Package manufacturing implements the Manufacturing Engine (ENG-0002,
// MFG-0001): it converts a validated geometry result into a production
// package — parts, materials, BOM and cut list (ENG-0003). The engine
// consumes the geometry output and never rebuilds geometry; manufacturing
// does not modify the design.
package manufacturing

import (
	"fmt"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
	kerngeo "stairplatform/internal/geometry"
)

// Manufacture преобразует валидный результат Geometry Engine в комплект
// производственных данных (ENG-0003, BC-007): декомпозиция на детали,
// назначение материалов из встроенного каталога (MFG-0005), BOM, карту
// раскроя и раскладку по стандартным листам (MFG-0012). Геометрия не
// пересчитывается. Предусловия: валидная конфигурация и геометрия без
// ошибок валидации (производство выполняется только по валидной ревизии).
// Результат детерминирован.
func Manufacture(cfg *engineering.StairConfiguration, gen *enggeo.GenerationResult) (*dommfg.ManufacturingPackage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("manufacturing: configuration is required")
	}
	if gen == nil || gen.Model == nil {
		return nil, fmt.Errorf("manufacturing: geometry result is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("manufacturing: %w", err)
	}
	for _, issue := range gen.Issues {
		if issue.Severity == kerngeo.SeverityError {
			return nil, fmt.Errorf("manufacturing: invalid geometry: %s: %s", issue.Code, issue.Message)
		}
	}

	parts, err := decompose(gen.Model)
	if err != nil {
		return nil, err
	}
	registry := DefaultMaterialRegistry()
	for i := range parts {
		code, err := assignMaterial(registry, parts[i].Thickness.Millimeters())
		if err != nil {
			return nil, fmt.Errorf("manufacturing: part %q: %w", parts[i].Number, err)
		}
		parts[i].Material = code
	}

	bom, cut := buildBOM(parts)
	nesting, err := Nest(cut, DefaultStockSheetRegistry(), DefaultKerf)
	if err != nil {
		return nil, err
	}
	pkg := &dommfg.ManufacturingPackage{Parts: parts, BOM: bom, CutList: cut, Nesting: nesting}
	if err := pkg.Validate(); err != nil {
		return nil, err
	}
	return pkg, nil
}
