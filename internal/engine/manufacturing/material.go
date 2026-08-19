package manufacturing

import (
	"fmt"
	"sync"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// defaultMaterialRegistry — общий неизменяемый каталог материалов, созданный
// один раз (EM-06, B2.3). Каталог доступен только на чтение (Find, Materials),
// поэтому его можно безопасно разделять между запросами вместо построения
// с нуля на каждый вызов Calculate.
var defaultMaterialRegistry struct {
	once sync.Once
	reg  *dommfg.MaterialRegistry
}

// DefaultMaterialRegistry возвращает встроенный детерминированный каталог
// материалов (MFG-0005) для MVP-05. Возвращается разделяемый неизменяемый
// экземпляр; вызывающий не должен его изменять.
func DefaultMaterialRegistry() *dommfg.MaterialRegistry {
	defaultMaterialRegistry.once.Do(func() {
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
		defaultMaterialRegistry.reg = reg
	})
	return defaultMaterialRegistry.reg
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

// DefaultMaterialForThickness назначает материал из встроенного каталога
// (MFG-0005) по толщине детали — та же политика, что в Manufacture.
// Используется советником для оценки раскроя кандидатов.
func DefaultMaterialForThickness(thickness float64) (dommfg.MaterialCode, error) {
	return assignMaterial(DefaultMaterialRegistry(), thickness)
}
