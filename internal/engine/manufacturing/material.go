package manufacturing

import (
	"fmt"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// DefaultMaterialRegistry возвращает встроенный детерминированный каталог
// материалов (MFG-0005) для MVP-05.
func DefaultMaterialRegistry() *dommfg.MaterialRegistry {
	reg, err := dommfg.NewMaterialRegistry(
		&dommfg.Material{
			Code: "STEEL-S235", Name: "Structural Steel S235", Category: "Steel",
			Density: 7850, MinThickness: 2, MaxThickness: 60,
		},
		&dommfg.Material{
			Code: "ALUM-5083", Name: "Aluminum 5083", Category: "Aluminum",
			Density: 2700, MinThickness: 2, MaxThickness: 60,
		},
		&dommfg.Material{
			Code: "WOOD-OAK", Name: "Oak Wood", Category: "Wood",
			Density: 700, MinThickness: 20, MaxThickness: 60,
		},
	)
	if err != nil {
		panic(fmt.Sprintf("manufacturing: default registry: %v", err))
	}
	return reg
}

// assignMaterial назначает материал детали: первый материал каталога,
// поддерживающий толщину (детерминированная политика). Материал является
// обязательным атрибутом детали (MFG-0005).
func assignMaterial(registry *dommfg.MaterialRegistry, thickness float64) (dommfg.MaterialCode, error) {
	for _, m := range registry.Materials() {
		if m.SupportsThickness(thickness) {
			return m.Code, nil
		}
	}
	return "", fmt.Errorf("manufacturing: no material supports thickness %v mm", thickness)
}
