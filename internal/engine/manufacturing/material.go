package manufacturing

import (
	"fmt"
	"sync"

	engmfg "stairplatform/internal/domain/engineering"
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
				// Выпуск лазерного раскроя 3–12 мм (тот же диапазон, что
				// отдаёт конфигуратор в fieldRules.stepThicknessMM).
				//
				// Раньше стояло 2–60, и это позволяло считать сталью любую
				// деталь: 40-миллиметровая ступень или 50-миллиметровый
				// косоур проходили как сталь, а потом и оплачивались как
				// стальной блок. Диапазон сузили до выпуска листа.
				// Верхняя граница 12 мм, а не 8: норматив на листовой косоур
				// (решение владельца 2026-10-05) содержит строки 8 / 10 / 12 мм,
				// и каталог обязан пропускать все три. С границей 8 мм стальной
				// косоур 10 или 12 мм отвергался бы как недопустимый.
				Density: 7850, MinThickness: 3, MaxThickness: engmfg.PlateMaxThicknessMM,
				// Ноль = действует общий MinThickness (3 мм, лист раскроя).
				MinTreadThicknessMm: 0,
				// Энвелоп MFG-0012 — пределы ГАБАРИТА МАРША для этого
				// материала, а не рабочего поля лазерного стола: марш 6000 мм
				// по высоте допустим, его косоур режется кусками. Поле стола
				// 3000×1500 живёт в разделе раскроя (engine/manufacturing.Nest).
				MaxWidthMm: 3000, MaxHeightMm: 6000,
				// У металла фасок нет: прокат/шлифовка дают прямой профиль.
				TreadNoseRadiusMm: 0,
				// Толщина металлической ступени не выбирается: 5 мм
				// (решение владельца 2026-10-05). В калькуляторе выбор убран,
				// и сервер приводит любое присланное значение к этому.
				FixedTreadThicknessMm: 5,
			},
			&dommfg.Material{
				Code: "WOOD-OAK", Name: "Oak Wood", NameRu: "Дуб", Category: "Wood",
				// Верхняя граница 100 мм, а не прежние 60: норматив на
				// деревянный косоур (решение владельца 2026-10-05) требует
				// 80 мм для марша длиннее 4 м и 100 мм при ширине более
				// 1200 мм. С верхней границей 60 мм каталог отвергал бы
				// ровно ту толщину, которую норматив предписывает.
				Density: 700, MinThickness: 20, MaxThickness: engmfg.TimberMaxThickMM,
				// Минимум ступени 40 мм (решение владельца 2026-10-05):
				// под скругление R10 нужна полка, а не фанера. Косоур из той
				// же породы остаётся от 20 мм — это разные детали.
				MinTreadThicknessMm: 40,
				// Энвелоп MFG-0012: крупнейшая плита 9000×4600 покрывает
				// косоур до H≈4550, проступи до W=3000 (плита 6000×3000).
				MaxWidthMm: 3000, MaxHeightMm: 4550,
				// Радиус филёнки на носике — 10 мм у ВСЕХ пород (решение
				// владельца 2026-10-05, чертеж «скругление R10 фрезой»).
				TreadNoseRadiusMm: 10,
			},
			// Фаза 1 «студийный 3D» (витрина, вариант Б): породы и металлы с
			// разной ценой и плотностью. Плотность влияет на массу/стоимость
			// (cost.go), диапазон толщин — на допустимость детали (MFG-0012).
			&dommfg.Material{
				Code: "WOOD-WALNUT", Name: "American Walnut", NameRu: "Орех", Category: "Wood",
				Density: 640, MinThickness: 20, MaxThickness: engmfg.TimberMaxThickMM,
				// Минимум ступени 40 мм (решение владельца 2026-10-05):
				// под скругление R10 нужна полка, а не фанера. Косоур из той
				// же породы остаётся от 20 мм — это разные детали.
				MinTreadThicknessMm: 40,
				MaxWidthMm:          3000, MaxHeightMm: 4550,
				TreadNoseRadiusMm: 10,
			},
			&dommfg.Material{
				Code: "WOOD-ASH", Name: "Ash Wood", NameRu: "Ясень", Category: "Wood",
				Density: 690, MinThickness: 20, MaxThickness: engmfg.TimberMaxThickMM,
				// Минимум ступени 40 мм (решение владельца 2026-10-05):
				// под скругление R10 нужна полка, а не фанера. Косоур из той
				// же породы остаётся от 20 мм — это разные детали.
				MinTreadThicknessMm: 40,
				MaxWidthMm:          3000, MaxHeightMm: 4550,
				TreadNoseRadiusMm: 10,
			},
			&dommfg.Material{
				Code: "WOOD-SOFT", Name: "Softwood (pine)", NameRu: "Сосна", Category: "Wood",
				Density: 520, MinThickness: 20, MaxThickness: engmfg.TimberMaxThickMM,
				// Минимум ступени 40 мм (решение владельца 2026-10-05):
				// под скругление R10 нужна полка, а не фанера. Косоур из той
				// же породы остаётся от 20 мм — это разные детали.
				MinTreadThicknessMm: 40,
				MaxWidthMm:          3000, MaxHeightMm: 4550,
				TreadNoseRadiusMm: 10,
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
