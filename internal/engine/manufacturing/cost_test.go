package manufacturing

import (
	"reflect"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

func TestPrepareCostMetrics(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatal(err)
	}
	ds, err := PrepareCost(pkg, mustMaterials(t), DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}

	// 98 деталей вместо 96: каждый из двух косоуров длиннее поля лазерного
	// стола 3000 мм (заготовка 4845 мм) и режется на 2 куска со сварным
	// стыком «шип-паз». Плюс припуск на шипы: суммарная длина заготовок
	// косоура выросла на 2 × 8 мм.
	if ds.PartCount != 98 {
		t.Fatalf("part count = %d, want 98", ds.PartCount)
	}
	if ds.OperationCount != 196 {
		t.Fatalf("operation count = %d, want 196 (98 деталей × 2)", ds.OperationCount)
	}
	// Крепёж по отверстиям пяток (MNT-0001, 2026-10-08):
	//   пятка 1 под ступенью  30 шт × 4 отв. = 120
	//   пятка 2 под подступенком 28 шт × 3 отв. =  84
	//   пластина 1-й ступени  2 шт × 3 отв. =    6   → 210 на ступени и подступенки
	//   пятка пола            2 шт × 4 отв. =    8   → глухарь + дюбель
	//   пятка перекрытия       2 шт × 3 отв. =    6   → глухарь + дюбель
	// Итого 238 штук.
	//
	// Плюс крепёж ограждения (решение владельца 2026-10-13), на марше одна
	// сторона и четыре стойки:
	//   анкерные отверстия фланцев  4 стойки × 3 отв. = 12 (глухарь 6х35)
	//   крепление поручня            4 стойки × 2      =  8 (саморез 5х32)
	// Итого 238 + 20 = 258 штук.
	if ds.FastenerCount != 258 {
		t.Fatalf("fastener count = %d, want 258", ds.FastenerCount)
	}
	// Позиций сметы на крепёж 7: пять старых (глухарь и дюбель 10х60 на пол
	// и на перекрытие, глухарь 6х35 на пятки) и две новые — фланцы и стойки.
	if len(ds.FastenerConsumption) != 7 {
		t.Fatalf("fastener consumption = %d позиций, want 7", len(ds.FastenerConsumption))
	}
	// операции: 98 деталей × 2 (Cutting + Finishing).
	if ds.OperationPlan == nil || len(ds.OperationPlan.Parts) != 98 {
		t.Fatalf("operation plan must contain 98 part routes, got %d",
			len(ds.OperationPlan.Parts))
	}
	if len(ds.MaterialConsumption) != 1 || ds.MaterialConsumption[0].MaterialCode != "STEEL-C245" {
		t.Fatalf("consumption = %+v, want single STEEL-C245", ds.MaterialConsumption)
	}

	// Метрики производственного набора. Значения — после починки заготовки
	// деталей: заготовка это лист лазерного раскроя, а не габаритный блок.
	//
	// Масса проверяется не «точным числом», а правдоподобием: именно точное
	// число раньше держало баг. 16-ступенчатый стальной марш с проступями,
	// подступенками и косоуром весит сотни килограммов; тест закреплял
	// 10 364 кг, потому что косоур брался блоком 4050×2660×50 мм. По цене
	// 100 ₽/кг это 1,4 млн ₽ материала и 2,6 млн ₽ с наценкой — в девять с
	// половиной раз выше цельного дубового марша.
	// Значения пересчитаны под раскрой по полю лазерного стола (шпатка
	// технолога 2026-10-05): лист 6000×1500 вместо прежних 6000×3000 и
	// 10400×6200, поэтому площадь листов выросла с 21,1 до 27,0 млн мм² и
	// утилизация упала. Машинное время выросло из-за контура сварного стыка
	// «шип-паз» на каждом куске косоура.
	//
	// Объём и масса выросли ещё и потому, что заготовка косоура теперь
	// толщиной 12 мм, а не 8: конструктивная толщина 50 мм сжимается к
	// ВЕРХУ выпуска листа, а выпуск STEEL-C245 расширен до PlateMaxThicknessMM
	// вместе с нормативом на листовой косоур (строки 8 / 10 / 12 мм).
	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{"part area", ds.PartArea, 9.93514372841643e+06},
		{"sheet area", ds.SheetArea, 2.7e+07},
		{"waste area", ds.WasteArea, 1.7064856271583572e+07},
		{"volume", ds.Volume, 1.1381564474099741e+08},
		{"surface area", ds.SurfaceArea, 2.1083390155106205e+07},
		{"waste percent", ds.WastePercent, 1.7064856271583572e+07 / 2.7e+07},
		{"utilization", ds.Utilization, 9.93514372841643e+06 / 2.7e+07},
		{"machine time", ds.EstimatedMachineTime, 276.47894210751053},
		{"labor time", ds.EstimatedLaborTime, 294},
		{"production time", ds.EstimatedProductionTime, 570.47894210751053},
	}
	for _, c := range checks {
		if !nearlyEqual(c.got, c.want) {
			t.Fatalf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	// Правдоподобие массы: 16-ступенчатый стальной марш — сотни кг. Верхняя
	// граница 1200 кг оставляет запас на заготовку, но отсекает возврат
	// ошибки с габаритным блоком (там было бы тонны). При листе 12 мм эталон
	// даёт 893 кг — до границы ещё запас, но при выпуске выше 12 мм проверку
	// придётся пересмотреть.
	if ds.Mass < 250 || ds.Mass > 1200 {
		t.Fatalf("mass = %v kg — неправдоподобно для 16 ступеней: проверь заготовку деталей (лист, а не блок)", ds.Mass)
	}
	if err := ds.Validate(); err != nil {
		t.Fatalf("dataset must validate: %v", err)
	}
}

func TestPrepareCostConsumption(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatal(err)
	}
	ds, err := PrepareCost(pkg, mustMaterials(t), DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	c := ds.MaterialConsumption[0]
	// масса потребления по материалу = суммарная масса изделия.
	if !nearlyEqual(c.Mass, ds.Mass) {
		t.Fatalf("consumption mass = %v, want %v", c.Mass, ds.Mass)
	}
	// расход листа = сумма площадей листов из раскроя.
	if !nearlyEqual(c.SheetArea, ds.SheetArea) {
		t.Fatalf("consumption sheet area = %v, want %v", c.SheetArea, ds.SheetArea)
	}
	// площадь деталей по каталогу = сумма площадей из CutList.
	var fromCutList float64
	for _, item := range pkg.CutList.Items {
		fromCutList += float64(item.Quantity) * item.Length.Millimeters() * item.Width.Millimeters()
	}
	if !nearlyEqual(c.PartArea, fromCutList) {
		t.Fatalf("consumption part area = %v, want %v", c.PartArea, fromCutList)
	}
}

func TestPrepareCostDeterminism(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := PrepareCost(pkg, mustMaterials(t), DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	b, err := PrepareCost(pkg, mustMaterials(t), DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("cost preparation must be deterministic")
	}
}

func TestPrepareCostErrors(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := PrepareCost(nil, mustMaterials(t), DefaultMachineRates()); err == nil {
		t.Fatal("nil package must be rejected")
	}
	if _, err := PrepareCost(pkg, nil, DefaultMachineRates()); err == nil {
		t.Fatal("nil material registry must be rejected")
	}
	// материал деталей отсутствует в реестре.
	empty, err := dommfg.NewMaterialRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareCost(pkg, empty, DefaultMachineRates()); err == nil {
		t.Fatal("material missing from registry must be rejected")
	}
	// площадь по CutList превышает площадь листов раскроя.
	bad := &dommfg.ManufacturingPackage{}
	*bad = *pkg
	bad.CutList.Items[1].Quantity = 9999
	if _, err := PrepareCost(bad, mustMaterials(t), DefaultMachineRates()); err == nil {
		t.Fatal("cut list exceeding nesting sheets must be rejected")
	}
}
