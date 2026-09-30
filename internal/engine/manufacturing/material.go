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
	err  error
}

// DefaultMaterialRegistry возвращает встроенный детерминированный каталог
// материалов (MFG-0005) для MVP-05. Возвращается разделяемый неизменяемый
// экземпляр; вызывающий не должен его изменять. Ошибка — вместо паники
// (P2-12): реестр строит из инвариантных констант и может не собраться
// только при потере инвариантов → сообщаем вызывающему, не валим процесс.
func DefaultMaterialRegistry() (*dommfg.MaterialRegistry, error) {
	defaultMaterialRegistry.once.Do(func() {
		reg, err := dommfg.NewMaterialRegistry(
			&dommfg.Material{
				Code: "STEEL-S235", Name: "Structural Steel S235", NameRu: "Сталь S235", Category: "Steel",
				// Выпуск лазерного раскроя 3–8 мм (тот же диапазон, что
				// отдаёт конфигуратор в fieldRules.stepThicknessMM).
				//
				// Раньше стояло 2–60, и это позволяло считать сталью любую
				// деталь: 40-миллиметровая ступень или 50-миллиметровый
				// косоур проходили как сталь, а потом и оплачивались как
				// стальной блок. Диапазон сузили до выпуска: деталь толще
				// 8 мм из стали не существует.
				Density: 7850, MinThickness: 3, MaxThickness: 8,
				// Энвелоп MFG-0012: крупнейший лист 10400×6200 покрывает
				// косоур до H≈6000, проступи до ширины W=3000 (лист 6000×3000).
				MaxWidthMm: 3000, MaxHeightMm: 6000,
				// У металла фасок нет: прокат/шлифовка дают прямой профиль.
				TreadNoseRadiusMm: 0,
			},
			&dommfg.Material{
				Code: "WOOD-OAK", Name: "Oak Wood", NameRu: "Дуб", Category: "Wood",
				Density: 700, MinThickness: 20, MaxThickness: 60,
				// Энвелоп MFG-0012: крупнейшая плита 9000×4600 покрывает
				// косоур до H≈4550, проступи до W=3000 (плита 6000×3000).
				MaxWidthMm: 3000, MaxHeightMm: 4550,
				// Дуб — самая твёрдая порода каталога, скругление держит
				// чисто; 8 мм — типовой радиус филёнки на носике.
				TreadNoseRadiusMm: 8,
			},
			// Фаза 1 «студийный 3D» (витрина, вариант Б): породы и металлы с
			// разной ценой и плотностью. Плотность влияет на массу/стоимость
			// (cost.go), диапазон толщин — на допустимость детали (MFG-0012).
			&dommfg.Material{
				Code: "WOOD-WALNUT", Name: "American Walnut", NameRu: "Орех", Category: "Wood",
				Density: 640, MinThickness: 20, MaxThickness: 60,
				MaxWidthMm: 3000, MaxHeightMm: 4550,
				// Орех плотнее дуба и темнее — на нём скругление заметнее,
				// поэтому радиус чуть больше.
				TreadNoseRadiusMm: 10,
			},
			&dommfg.Material{
				Code: "WOOD-ASH", Name: "Ash Wood", NameRu: "Ясень", Category: "Wood",
				Density: 690, MinThickness: 20, MaxThickness: 60,
				MaxWidthMm: 3000, MaxHeightMm: 4550,
				TreadNoseRadiusMm: 8,
			},
			&dommfg.Material{
				Code: "WOOD-SOFT", Name: "Softwood (pine)", NameRu: "Сосна", Category: "Wood",
				Density: 520, MinThickness: 20, MaxThickness: 60,
				MaxWidthMm: 3000, MaxHeightMm: 4550,
				// Сосна крошится на скруглении: маленький радиус, иначе
				// фрезер снимает щепу.
				TreadNoseRadiusMm: 5,
			},
		)
		if err != nil {
			defaultMaterialRegistry.err = err
			return
		}
		defaultMaterialRegistry.reg = reg
	})
	return defaultMaterialRegistry.reg, defaultMaterialRegistry.err
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
	registry, err := DefaultMaterialRegistry()
	if err != nil {
		return "", fmt.Errorf("manufacturing: default registry: %w", err)
	}
	return assignMaterial(registry, thickness)
}
