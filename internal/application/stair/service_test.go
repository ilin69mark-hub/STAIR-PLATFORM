package stair

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	"stairplatform/internal/engine/constraint"
	engprc "stairplatform/internal/engine/pricing"
)

func mustLengthHelper(mm float64) engineering.Length {
	l, err := engineering.NewLength(mm)
	if err != nil {
		panic(err)
	}
	return l
}

// referenceConfig — эталонная конфигурация n=15 (совпадает с референсом
// Pricing Engine: H=2700, h0=180, шаг комфорта 630 → h=180, b=270, n=15).
func referenceConfig() Config {
	return Config{
		Width:      mustLengthHelper(900),
		Height:     mustLengthHelper(2700),
		Flight:     engineering.FlightStraight,
		StepHeight: mustLengthHelper(180),
		// Стальной каркас: косоур и ступень — листы лазерного раскроя 3–8 мм.
		StringerThickness: mustLengthHelper(8),
		StepThickness:     mustLengthHelper(6),
		// Материалы заданы явно. Раньше они выводились из толщины: 50 мм
		// подходили стали (2–60), и эталон молча проверял сталь на конфигурации
		// без материала. С выходом стали на выпуск 3–8 мм такой вывод стал
		// давать дуб, и эталон проверял уже не то изделие.
		Material:      "STEEL-S235",
		TreadMaterial: "STEEL-S235",
		Riser:         true,
		Clearance:     mustLengthHelper(2500),
		RailingHeight: mustLengthHelper(1000),
	}
}

// customRates — ставки пользователя (переопределение дефолтов).
func customRates() *engprc.Rates {
	r := engprc.DefaultRates()
	r.Material[dommfg.MaterialCode("STEEL-S235")] = domprc.NewMoney(20000) // 200 ₽/кг
	return &r
}

func TestCalculateValidPipeline(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), referenceConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}

	if !res.Validation.Valid || res.Validation.Blocking {
		t.Fatalf("expected valid configuration, got %+v", res.Validation)
	}
	if res.Flight.StepCount != 15 {
		t.Fatalf("step count = %d, want 15", res.Flight.StepCount)
	}
	if res.Flight.StepHeight.Millimeters() != 180 {
		t.Fatalf("step height = %v, want 180", res.Flight.StepHeight.Millimeters())
	}
	if res.Flight.TreadDepth.Millimeters() != 270 {
		t.Fatalf("tread depth = %v, want 270", res.Flight.TreadDepth.Millimeters())
	}

	if res.Package == nil || len(res.Package.Parts) == 0 {
		t.Fatal("package must contain parts")
	}
	if len(res.Package.BOM.Lines) == 0 {
		t.Fatal("package must contain BOM")
	}
	if len(res.Package.CutList.Items) == 0 {
		t.Fatal("package must contain cut list")
	}
	if res.Package.Nesting == nil || len(res.Package.Nesting.Sheets) == 0 {
		t.Fatal("package must contain nesting result")
	}

	if res.Mesh == nil {
		t.Fatal("preview mesh must be present")
	}
	if len(res.Mesh.Vertices) == 0 || len(res.Mesh.Triangles) == 0 {
		t.Fatal("preview mesh must contain vertices and triangles")
	}

	if res.Cost == nil {
		t.Fatal("cost dataset must be present")
	}
	if res.Price == nil {
		t.Fatal("price breakdown must be present")
	}

	// Финальная цена эталонного конвейера (100 ₽/кг, ставки по умолчанию):
	// стальной марш 16 ступеней из листа 8 мм, 277 814,63 ₽.
	//
	// Два предыдущих значения были неверными по разным причинам. Сначала
	// 3 271 385,47 ₽ — заготовка деталей бралась по габаритному блоку, и
	// металла выходило 10,4 тонны. Потом 234 751,18 ₽ — эталон задавал
	// косоур 50 мм, и она схлопывалась в 8 мм правилом листа; теперь
	// толщина в конфигурации сразу 8 мм, и геометрия честно тоньше.
	// Далее 277 814,63 ₽ — косоур был пилообразной гребёнкой (спинка шириной
	// в толщину металла). Теперь косоур металлокаркаса — боковая пластина
	// 300 мм (копия решения Ниоры): металла в заготовке больше, поэтому
	// цена выше, а вот время лазерной резки меньше — у пластины нет зубьев.
	// Металлическая ступень — всегда 5 мм (решение владельца 2026-10-05),
	// поэтому и подступенок из того же листа 5 мм. Прежние 6 мм уменьшали
	// массу и стоимость на треть процента листа в каждой детали.
	// Далее 310 640,32 ₽ — раскрой переведён на рабочее поле лазерного стола
	// 3000×1500 (шпатка технолога 2026-10-05). Косоур длиннее поля режется на
	// 2 куска со сварным стыком «шип-паз», поэтому деталей 104 вместо 102,
	// а закупка идёт листами 6000×1500.
	//
	// Далее 364 072,52 ₽ — толщина листа и ширина заготовки берутся из
	// НОРМАТИВА листового косоура автоматически (решение владельца
	// 2026-10-06). Проекция марша этого эталона 4050 мм — строка «более
	// 4.0 м», то есть лист 12 мм, а не присланные 8 мм. Заготовка гребёнки
	// для 12 мм должна быть от 350 мм, поэтому поднята с утверждённых 300.
	// Раньше 8 мм молча превращались в 12 мм «по потолку выпуска» — цена
	// случайно совпадала, но короткий марш в 2,5 м платил за 12 мм, а
	// длинный получал 12 мм без опоры на норматив.
	// Плюс крепёж по отверстиям пяток (MNT-0001, 2026-10-08). Итог вырос с
	// 364 072,52 ₽ до 370 421,41 ₽: 238 штук по 15 ₽ = 3 570 ₽, и на них
	// накручиваются накладные (20%), маржа (30%), скидка (5%) и НДС (20%).
	// Плюс герметик (CONF-SEALANT): площадь швов обоймы выреза и торца
	// косоура 0,6922 м² × 1 мм = 0,6922 л → 3 картриджа по 500 ₽ = 1 500 ₽,
	// и на них накручиваются накладные, маржа, скидка и НДС: +2 667,60 ₽.
	if res.Price.FinalPrice.Minor() != 38598241 {
		t.Fatalf("final price = %d, want 38598241", res.Price.FinalPrice.Minor())
	}
	if res.Price.FinalPrice.Major(domprc.CurrencyRUB) != 385982.41 {
		t.Fatalf("final price = %.2f rub, want 385982.41", res.Price.FinalPrice.Major(domprc.CurrencyRUB))
	}
	if res.Price.Sealant.Minor() != 150000 {
		t.Fatalf("sealant cost = %d minor, want 150000 (3 картриджа × 500 ₽)", res.Price.Sealant.Minor())
	}
	if res.Price.Fastener.Minor() != 357000 {
		t.Fatalf("fastener cost = %d minor, want 357000 (238 шт × 15 ₽)", res.Price.Fastener.Minor())
	}
}

func TestCalculateDeterminism(t *testing.T) {
	s := NewService()
	a, err := s.Calculate(context.Background(), referenceConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Calculate(context.Background(), referenceConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("calculate must be deterministic")
	}
}

func TestCalculateCustomRates(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), referenceConfig(), Options{Rates: customRates()})
	if err != nil {
		t.Fatal(err)
	}
	// Стоимость стали удвоена → материал выше дефолтных 113 040 ₽.
	if res.Price.Material.Minor() <= 11304000 {
		t.Fatalf("material with doubled rate must exceed default, got %d", res.Price.Material.Minor())
	}
}

func TestCalculateBlockingValidation(t *testing.T) {
	s := NewService()
	// Целевая высота ступени 10 мм → n=270, h=10 вне диапазона 150-200 → Error.
	cfg := referenceConfig()
	cfg.StepHeight = mustLengthHelper(10)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Blocking {
		t.Fatalf("expected blocking validation, got %+v", res.Validation)
	}
	if res.Validation.Valid {
		t.Fatal("blocking result must not be valid")
	}
	// Конвейер должен остановиться: цена и пакет отсутствуют.
	if res.Price != nil || res.Package != nil {
		t.Fatal("blocking validation must stop the pipeline (no price/package)")
	}
}

// TestCalculateTallFlightSucceeds — прямая лестница на пределе энвелопа
// H=5000 (граница изделия, S-169) должна проходить весь конвейер и выдавать
// валидный результат с ценой, а не блокировку.
//
// Предел снижен с 6000 до 5000 решением владельца 2026-10-12: 6000 был
// верхней границей конверта производства, но в изделии мы столько не делаем.
// Тест проверяет, что ровно на границе всё ещё считается.
func TestCalculateTallFlightSucceeds(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Height = mustLengthHelper(engineering.MaxFloorHeightMM)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("tall flight within envelope must calculate: %v", err)
	}
	if !res.Validation.Valid || res.Validation.Blocking {
		t.Fatalf("expected valid configuration, got %+v", res.Validation)
	}
	if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("tall flight must produce price")
	}
	if res.Package == nil || len(res.Package.Parts) == 0 {
		t.Fatal("tall flight must produce nesting package")
	}
}

// assertBlockingInput проверяет, что результат блокирующий (advisory) с
// русским объяснением и указанным кодом — вместо технической ошибки.
func assertBlockingInput(t *testing.T, res *Result, wantCode constraint.RuleCode, wantSubstring string) {
	t.Helper()
	if res == nil {
		t.Fatal("expected advisory result, got nil")
	}
	if !res.Validation.Blocking || res.Validation.Valid {
		t.Fatalf("expected blocking result, got %+v", res.Validation)
	}
	if len(res.Validation.Issues) == 0 {
		t.Fatalf("expected an issue, got none")
	}
	it := res.Validation.Issues[0]
	if string(it.Code) != string(wantCode) {
		t.Fatalf("issue code = %q, want %q", it.Code, wantCode)
	}
	if it.Param == "" || it.Guide == "" {
		t.Fatalf("issue must carry Param and Guide: %+v", it)
	}
	if wantSubstring != "" && !strings.Contains(it.Guide, wantSubstring) {
		t.Fatalf("guide %q does not contain %q", it.Guide, wantSubstring)
	}
	if res.Price != nil || res.Package != nil {
		t.Fatal("blocked result must not carry price/package")
	}
}

// TestCalculateExceedsMaxHeight — высота за пределами поддерживаемого
// энелопа (6000 мм) возвращается как блокирующая подсказка, а не ошибка.
//
// Код MFG-MATERIAL, а не GEO_HEIGHT: у эталона материал задан явно (сталь), а
// предел стали по высоте — те же 6000 мм, что и глобальный энелоп. Проверка
// материала срабатывает раньше и даёт более точную подсказку с названием
// материала. Предел, общий для всех материалов, проверяется отдельно —
// TestCalculateExceedsMaterialMaxHeight (дуб 4550 мм).
func TestCalculateExceedsMaxHeight(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Height = mustLengthHelper(7000)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.MFG_MATERIAL, "6000 мм")
}

// TestCalculateExceedsMaterialMaxHeight — высота в пределах глобального
// энелопа (6000 мм), но выше предела конкретного материала (у древесины
// 4550 мм), возвращается как блокирующая подсказка по материалу.
// Предел ниже глобального энелопа есть только у древесины.
func TestCalculateExceedsMaterialMaxHeight(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Material = dommfg.MaterialCode("WOOD-OAK")
	// Доска 50 мм: эталонные 8 мм — стальной лист, а дуб выпускается 20–100.
	cfg.StringerThickness = mustLengthHelper(50)
	cfg.Height = mustLengthHelper(5000)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.MFG_MATERIAL, "4550 мм")
}

// TestCalculateExceedsMaterialMaxWidth — ширина марша выше предела
// материала (3000 мм) возвращается как блокирующая подсказка.
func TestCalculateExceedsMaterialMaxWidth(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Material = dommfg.MaterialCode("STEEL-S235")
	cfg.Width = mustLengthHelper(4000)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.MFG_MATERIAL, "3000 мм")
}

// TestCalculateRailingConfig — перила/направления проходят конвейер и
// эхо несут выбранные значения: один выбор для прямого марша, по
// сегментам + поворот для L/U, авто-сторона для спирали (CONF-RAILING).
func TestCalculateRailingConfig(t *testing.T) {
	s := NewService()

	straight := referenceConfig()
	straight.Railing = engineering.RailingBoth
	res, err := s.Calculate(context.Background(), straight, Options{})
	if err != nil {
		t.Fatalf("straight railing: %v", err)
	}
	if res.Validation.Blocking {
		t.Fatalf("straight railing must not block: %+v", res.Validation)
	}
	if res.Railing != engineering.RailingBoth {
		t.Fatalf("expected echoed railing both, got %q", res.Railing)
	}

	lshape := referenceConfig()
	lshape.Flight = engineering.FlightLShape
	lshape.LandingWidth = mustLengthHelper(1000)
	lshape.LowerStepCount = 6
	lshape.RailingLower = engineering.RailingLeft
	lshape.RailingLanding = engineering.RailingBoth
	lshape.RailingUpper = engineering.RailingRight
	lshape.Direction = engineering.TurnRight
	resL, err := s.Calculate(context.Background(), lshape, Options{})
	if err != nil {
		t.Fatalf("L-shape railing: %v", err)
	}
	if resL.Validation.Blocking {
		t.Fatalf("L-shape railing must not block: %+v", resL.Validation)
	}
	if resL.RailingLower != engineering.RailingLeft ||
		resL.RailingLanding != engineering.RailingBoth ||
		resL.RailingUpper != engineering.RailingRight ||
		resL.Direction != engineering.TurnRight {
		t.Fatalf("L-shape railing echo mismatch: got %s/%s/%s dir=%s",
			resL.RailingLower, resL.RailingLanding, resL.RailingUpper, resL.Direction)
	}
}

// TestCalculateInvalidRailingConfig — невалидные значения перил/направлений
// возвращаются как блокирующие подсказки с понятным текстом.
func TestCalculateInvalidRailingConfig(t *testing.T) {
	s := NewService()

	badSide := referenceConfig()
	badSide.Railing = engineering.RailingSide("diagonal")
	res, err := s.Calculate(context.Background(), badSide, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.SAF_RAILING_HEIGHT, "сторон")

	badTurn := referenceConfig()
	badTurn.Flight = engineering.FlightLShape
	badTurn.LandingWidth = mustLengthHelper(1000)
	badTurn.LowerStepCount = 6
	badTurn.Direction = engineering.TurnDirection("up")
	resT, err := s.Calculate(context.Background(), badTurn, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, resT, constraint.GEO_LANDING_WIDTH, "влево или вправо")
}

// TestCalculateWithinMaterialLimits — габариты на границе предела каждого
// материала проходят весь конвейер (не блокируются размерными лимитами):
// макс. высота при типовой ширине и макс. ширина при типовой высоте.
func TestCalculateWithinMaterialLimits(t *testing.T) {
	s := NewService()
	limits := map[dommfg.MaterialCode]struct{ height, width int }{
		"STEEL-S235": {5000, 3000}, // граница изделия S-169 (было 6000)
		"WOOD-OAK":   {4550, 3000},
		"WOOD-SOFT":  {4550, 3000},
	}
	for code, lim := range limits {
		cfgH := referenceConfig()
		cfgH.Material = code
		cfgH.TreadMaterial = code
		// Доска для дерева, лист для стали: эталонные 8 мм дуб не принимает.
		if code != "STEEL-S235" {
			cfgH.StringerThickness = mustLengthHelper(50)
			cfgH.StepThickness = mustLengthHelper(40)
		}
		cfgH.Height = mustLengthHelper(float64(lim.height))
		resH, err := s.Calculate(context.Background(), cfgH, Options{})
		if err != nil {
			t.Fatalf("%s (height limit): input issues must not be hard errors: %v", code, err)
		}
		if resH.Validation.Blocking {
			t.Fatalf("%s: height %d must pass material limit, got %+v",
				code, lim.height, resH.Validation)
		}

		cfgW := referenceConfig()
		cfgW.Material = code
		cfgW.TreadMaterial = code
		// Доска для дерева, лист для стали: эталонные 8 мм дуб не принимает.
		if code != "STEEL-S235" {
			cfgW.StringerThickness = mustLengthHelper(50)
			cfgW.StepThickness = mustLengthHelper(40)
		}
		cfgW.Width = mustLengthHelper(float64(lim.width))
		resW, err := s.Calculate(context.Background(), cfgW, Options{})
		if err != nil {
			t.Fatalf("%s (width limit): input issues must not be hard errors: %v", code, err)
		}
		if resW.Validation.Blocking {
			t.Fatalf("%s: width %d must pass material limit, got %+v",
				code, lim.width, resW.Validation)
		}
	}
}

func TestCalculateInvalidInput(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Height = mustLengthHelper(0)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.GEO_HEIGHT, "больше 0 мм")
}

func TestCalculateComfortStepBoundary(t *testing.T) {
	s := NewService()
	// Шаг комфорта 600 → b = 600 - 2*180 = 240 < 260 (диапазон tread 260-320)
	// → blocking с GEO-TREAD-DEPTH.
	cfg := referenceConfig()
	res, err := s.Calculate(context.Background(), cfg, Options{ComfortStep: 600})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Blocking {
		t.Fatalf("expected blocking for b=240, got %+v", res.Validation)
	}
}

// referenceLShapeConfig — эталонная L-образная конфигурация (H=2700,
// n=15, n1=6, h=180, b=270, W=900, Wp=1000). Ответвление n1=6 даёт
// нижний марш 1620×1080 и верхний 2430×1620.
func referenceLShapeConfig() Config {
	cfg := referenceConfig()
	cfg.Flight = engineering.FlightLShape
	cfg.LandingWidth = mustLengthHelper(1000)
	cfg.LowerStepCount = 6
	return cfg
}

// cuttablePartCount — число деталей, которые реально режутся: без покупных.
// Ограждение (стойки, поручень, ригели, фитинги) покупается готовым и в
// раскрой не идёт, поэтому тесты конвейера считают каркас и ступени.
func cuttablePartCount(pkg *dommfg.ManufacturingPackage) int {
	n := 0
	for _, p := range pkg.Parts {
		if !p.Purchased {
			n++
		}
	}
	return n
}

func TestCalculateLShapePipeline(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), referenceLShapeConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Valid || res.Validation.Blocking {
		t.Fatalf("expected valid configuration, got %+v", res.Validation)
	}
	if res.LShape == nil {
		t.Fatal("l_shape flight must populate LShape result")
	}
	if res.LShape.StepCount != 15 || res.LShape.LowerStepCount != 6 || res.LShape.UpperStepCount != 9 {
		t.Fatalf("split = %d/%d/%d, want 15/6/9",
			res.LShape.StepCount, res.LShape.LowerStepCount, res.LShape.UpperStepCount)
	}
	if res.LShape.LowerHeight.Millimeters() != 1080 || res.LShape.UpperHeight.Millimeters() != 1620 {
		t.Fatalf("flight heights = %v/%v, want 1080/1620",
			res.LShape.LowerHeight.Millimeters(), res.LShape.UpperHeight.Millimeters())
	}
	// прямой марш должен оставаться нулевым.
	if res.Flight.StepCount != 0 {
		t.Fatalf("straight flight must be empty for l_shape, got %+v", res.Flight)
	}
	// полный конвейер: 4 косоура, ступени и подступенки обоих маршей и
	// пятки выреза каждой ступени.
	if res.Package == nil || cuttablePartCount(res.Package) != 103 {
		t.Fatalf("cuttable parts = %d, want 103", cuttablePartCount(res.Package))
	}
	if res.Mesh == nil || len(res.Mesh.Vertices) == 0 {
		t.Fatal("l_shape pipeline must produce preview mesh")
	}
	// Объём: косоур-гребёнка листом 8 мм плюс пятки выреза каждой ступени.
	// Ступени и подступенки — по 5 мм (единственная толщина металла), раньше
	// было 6 мм, поэтому объём ниже прежнего.
	if math.Abs(res.Measurement.Volume-5.571154623639395e+07) > 1 {
		t.Fatalf("volume = %v, want 5.571154623639395e+07", res.Measurement.Volume)
	}
	if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("l_shape pipeline must produce price")
	}
}

func TestCalculateLShapeLandingTooNarrow(t *testing.T) {
	s := NewService()
	// Wp=500 < W=900 → блокирующая подсказка (EDR-0005 §7): площадка
	// должна быть не уже марша.
	cfg := referenceLShapeConfig()
	cfg.LandingWidth = mustLengthHelper(500)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.GEO_LANDING_WIDTH, "меньше ширины марша 900 мм")
}

// referenceUShapeConfig — эталонная П-образная конфигурация (EDR-0006):
// та же арифметика, что и у L-марша (H=2700, n=15, n1=6, h=180, b=270,
// W=900, Wp=1000); верхний марш разворачивается на 180°.
func referenceUShapeConfig() Config {
	cfg := referenceConfig()
	cfg.Flight = engineering.FlightUShape
	cfg.LandingWidth = mustLengthHelper(1000)
	cfg.LowerStepCount = 6
	return cfg
}

// TestCalculateUShapeWinderPipeline проверяет сквозной конвейер П-образной
// лестницы в режиме поворотных ступеней (CONF-TURN-KIND=winder): расчёт
// ветвится на SolveUShapeWinder, геометрия строит веер (роль "winder"),
// площадки нет.
func TestCalculateUShapeWinderPipeline(t *testing.T) {
	s := NewService()
	cfg := referenceUShapeConfig()
	cfg.TurnKind = engineering.TurnWinder
	cfg.WinderCount = 3
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Valid || res.Validation.Blocking {
		t.Fatalf("expected valid configuration, got %+v", res.Validation)
	}
	if res.UShape == nil {
		t.Fatal("u_shape flight must populate UShape result")
	}
	if res.UShape.StepCount != 15 || res.UShape.LowerStepCount != 6 ||
		res.UShape.WinderCount != 3 || res.UShape.UpperStepCount != 6 {
		t.Fatalf("split = %d/%d/%d/%d, want 15/6/3/6",
			res.UShape.StepCount, res.UShape.LowerStepCount,
			res.UShape.WinderCount, res.UShape.UpperStepCount)
	}
	if res.Mesh == nil || len(res.Mesh.Vertices) == 0 {
		t.Fatal("winder pipeline must produce preview mesh")
	}
	// деталей: 4 косоура-гребёнки, ступени/подступенки, поворотные площадки
	// и пятки выреза каждой ступени.
	if res.Package == nil || cuttablePartCount(res.Package) != 87 {
		t.Fatalf("cuttable parts = %d, want 87", cuttablePartCount(res.Package))
	}
	// эхо поворота в результате.
	if res.TurnKind != engineering.TurnWinder || res.WinderCount != 3 {
		t.Fatalf("result echo TurnKind/WinderCount = %q/%d, want winder/3",
			res.TurnKind, res.WinderCount)
	}
}

func TestCalculateUShapePipeline(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), referenceUShapeConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Valid || res.Validation.Blocking {
		t.Fatalf("expected valid configuration, got %+v", res.Validation)
	}
	if res.UShape == nil {
		t.Fatal("u_shape flight must populate UShape result")
	}
	if res.UShape.StepCount != 15 || res.UShape.LowerStepCount != 6 || res.UShape.UpperStepCount != 9 {
		t.Fatalf("split = %d/%d/%d, want 15/6/9",
			res.UShape.StepCount, res.UShape.LowerStepCount, res.UShape.UpperStepCount)
	}
	if res.UShape.LowerHeight.Millimeters() != 1080 || res.UShape.UpperHeight.Millimeters() != 1620 {
		t.Fatalf("flight heights = %v/%v, want 1080/1620",
			res.UShape.LowerHeight.Millimeters(), res.UShape.UpperHeight.Millimeters())
	}
	if res.UShape.LowerRun.Millimeters() != 1620 || res.UShape.UpperRun.Millimeters() != 2430 {
		t.Fatalf("runs = %v/%v, want 1620/2430",
			res.UShape.LowerRun.Millimeters(), res.UShape.UpperRun.Millimeters())
	}
	// прямой марш должен оставаться нулевым.
	if res.Flight.StepCount != 0 {
		t.Fatalf("straight flight must be empty for u_shape, got %+v", res.Flight)
	}
	// полный конвейер: 4 косоура, ступени и подступенки обоих маршей и
	// пятки выреза каждой ступени.
	if res.Package == nil || cuttablePartCount(res.Package) != 103 {
		t.Fatalf("cuttable parts = %d, want 103", cuttablePartCount(res.Package))
	}
	if res.Mesh == nil || len(res.Mesh.Vertices) == 0 {
		t.Fatal("u_shape pipeline must produce preview mesh")
	}
	// Ступени и подступенки — по 5 мм (единственная толщина металла).
	if math.Abs(res.Measurement.Volume-5.754904623635878e+07) > 1 {
		t.Fatalf("volume = %v, want 5.754904623635878e+07", res.Measurement.Volume)
	}
	if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("u_shape pipeline must produce price")
	}
}

func TestCalculateUShapeLandingTooNarrow(t *testing.T) {
	s := NewService()
	// Wp=500 < W=900 → блокирующая подсказка (EDR-0006 §7).
	cfg := referenceUShapeConfig()
	cfg.LandingWidth = mustLengthHelper(500)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.GEO_LANDING_WIDTH, "меньше ширины марша 900 мм")
}

func BenchmarkCalculatePipeline(b *testing.B) {
	s := NewService()
	cfg := referenceConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Calculate(context.Background(), cfg, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestCalculateCancelled(t *testing.T) {
	// Отменённый контекст: конвейер не выполняется, возвращается
	// context.Canceled (B2, EDR-0033 §3.1).
	s := NewService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.Calculate(ctx, referenceConfig(), Options{}); err == nil {
		t.Fatal("expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCalculateMaterialSelection(t *testing.T) {
	s := NewService()
	materials := []dommfg.MaterialCode{"STEEL-S235", "WOOD-OAK", "WOOD-SOFT"}
	// Толщина косоура зависит от материала каркаса: сталь 3–8, дерево
	// 20–100. Один эталон на всех не годится.
	prices := make(map[dommfg.MaterialCode]int64)
	for _, m := range materials {
		cfg := referenceConfig()
		cfg.Material = m
		cfg.TreadMaterial = m
		// Доска для дерева, лист для стали: 8 мм дуб не принимает.
		if m != "STEEL-S235" {
			cfg.StringerThickness = mustLengthHelper(50)
			cfg.StepThickness = mustLengthHelper(40)
		}
		res, err := s.Calculate(context.Background(), cfg, Options{})
		if err != nil {
			t.Fatalf("%s: %v", m, err)
		}
		if !res.Validation.Valid || res.Validation.Blocking {
			t.Fatalf("%s: validation %+v", m, res.Validation)
		}
		if res.Package == nil || len(res.Package.Parts) == 0 {
			t.Fatalf("%s: no parts in package", m)
		}
		for _, p := range res.Package.Parts {
			// Ограждение покупается готовым и всегда из нержавейки AISI 304,
			// независимо от материала каркаса и ступеней — это решение
			// владельца. Материал cfg.Material распространяется на каркас и
			// ступени, но не на покупные детали ограждения.
			if p.Purchased {
				// Заглушка торца — пластик, остальное ограждение — нерж.
				if p.Material != "STAINLESS-304" && p.Material != "PLASTIC" {
					t.Fatalf("%s: purchased part %s material = %s, want STAINLESS-304 or PLASTIC",
						m, p.Number, p.Material)
				}
				continue
			}
			if p.Material != m {
				t.Fatalf("%s: part %s material = %s, want %s", m, p.Number, p.Material, m)
			}
		}
		// Материалы имеют разные плотность/ставку → финальная цена различается.
		prices[m] = res.Price.FinalPrice.Minor()
		if res.Price == nil {
			t.Fatalf("%s: price missing", m)
		}
		// Раскрой выполнен (дерево покрыто крупными листами каталога).
		if res.Package.Nesting == nil || len(res.Package.Nesting.Sheets) == 0 {
			t.Fatalf("%s: nesting missing", m)
		}
	}
	if prices["STEEL-S235"] == prices["WOOD-SOFT"] || prices["STEEL-S235"] == prices["WOOD-OAK"] {
		t.Fatalf("prices must differ across materials, got %v", prices)
	}
}

func TestCalculateMaterialValidation(t *testing.T) {
	s := NewService()

	// Неизвестный материал — блокирующий MFG-MATERIAL.
	cfg := referenceConfig()
	cfg.Material = "TITANIUM-X"
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Blocking || res.Validation.Valid {
		t.Fatalf("unknown material must block, got %+v", res.Validation)
	}
	iss := res.Validation.Issues[0]
	if iss.Code != constraint.MFG_MATERIAL || !strings.Contains(iss.Message, "не найден") {
		t.Fatalf("unexpected issue %+v", iss)
	}

	// Дуб не поддерживает косоур 150 мм — блокирующий MFG-MATERIAL.
	cfg = referenceConfig()
	cfg.Material = "WOOD-OAK"
	// Доска 50 мм: эталонные 8 мм — стальной лист, а дуб выпускается 20–100.
	cfg.StringerThickness = mustLengthHelper(50)
	cfg.StringerThickness = mustLengthHelper(150)
	res, err = s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Blocking || res.Validation.Valid {
		t.Fatalf("wood+150mm stringer must block, got %+v", res.Validation)
	}
	iss = res.Validation.Issues[0]
	if iss.Code != constraint.MFG_MATERIAL {
		t.Fatalf("code = %s, want %s", iss.Code, constraint.MFG_MATERIAL)
	}
	// Верхняя граница дуба — 100 мм, а не 60: норматив на деревянный косоур
	// (решение владельца 2026-10-05) требует 80 мм для марша длиннее 4 м и
	// 100 мм при ширине более 1200 мм, и каталог обязан эти толщины пропускать.
	if !strings.Contains(iss.Guide, "20–100") {
		t.Fatalf("guide must mention wood thickness range, got %q", iss.Guide)
	}

	// Дуб при большом подъёме (H=5000 выше предела дуба 4550 мм) —
	// блокирующая подсказка про предельную высоту материала.
	cfg = referenceConfig()
	cfg.Material = "WOOD-OAK"
	// Доска 50 мм: эталонные 8 мм — стальной лист, а дуб выпускается 20–100.
	cfg.StringerThickness = mustLengthHelper(50)
	cfg.Height = mustLengthHelper(5000)
	res, err = s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Blocking || res.Validation.Valid {
		t.Fatalf("wood + H=5000 must block, got %+v", res.Validation)
	}
	iss = res.Validation.Issues[0]
	if iss.Code != constraint.MFG_MATERIAL {
		t.Fatalf("code = %s, want %s", iss.Code, constraint.MFG_MATERIAL)
	}
	if !strings.Contains(iss.Guide, "4550") {
		t.Fatalf("guide must mention oak max height 4550, got %q", iss.Guide)
	}
}
