package stair

import (
	"context"
	"testing"

	"stairplatform/internal/domain/engineering"
)

// Разделение материала каркаса и материала ступеней.
//
// ДО разделения одно поле Material описывало все детали лестницы, и это было
// физически невозможно для металлокаркаса с деревянными ступенями: стальной
// косоур 8 мм и проступь из дуба (MinThickness = 20 мм) не помещались в одно
// значение. Валидация отвергала такую конфигурацию с ошибкой «material
// STEEL-S235 does not support thickness 40 mm of ступени».
//
// Тесты ниже фиксируют именно эту ошибку: если разделение откатят или
// склеят обратно, они упадут.

func metalFrameWoodTreadConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		Width:             engineering.Length(900),
		Height:            engineering.Length(2700),
		Flight:            engineering.FlightStraight,
		StepHeight:        engineering.Length(180),
		Riser:             true,
		StringerThickness: engineering.Length(8),  // стальной косоур — тонкий
		StepThickness:     engineering.Length(40), // деревянная проступь — толстая
		RiserThickness:    engineering.Length(8),  // подступенок стальной, тонкий
		Clearance:         engineering.Length(2000),
		RailingHeight:     engineering.Length(900),
		Material:          "STEEL-S235",
		TreadMaterial:     "WOOD-OAK",
	}
}

// TestMetalFrameWoodTread_Accepted — ключевой сценарий: стальной каркас с
// толстым косоуром НЕ ТОЛЩИНОЙ, а стальной косоур 8 мм с деревянной
// проступью 40 мм — валидная конфигурация. До разделения она падала.
func TestMetalFrameWoodTread_Accepted(t *testing.T) {
	svc := NewService()
	res, err := svc.Calculate(context.Background(), metalFrameWoodTreadConfig(t), Options{})
	if err != nil {
		t.Fatalf("стальной каркас + деревянная ступень должен считаться, получено: %v", err)
	}
	if res.Validation.Blocking {
		t.Fatalf("расчёт не должен быть блокирующим: %+v", res.Validation.Issues)
	}
	if res.Price == nil {
		t.Fatal("нет расчёта цены")
	}
}

// TestTreadMaterialRouting — детали разводятся по материалам: косоур из
// каркаса, проступь И ПОДСТУПЕНОК — из материала ступеней. Подступенок
// примыкает к проступи и виден вместе с ней, поэтому «деревянные ступени на
// стальном каркасе» это деревянные подступенки. Если маршрутизация
// сломается, цена станет неверной (дерево посчитается по ставке стали или
// наоборот).
func TestTreadMaterialRouting(t *testing.T) {
	svc := NewService()
	res, err := svc.Calculate(context.Background(), metalFrameWoodTreadConfig(t), Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if res.Package == nil {
		t.Fatal("нет производственного пакета")
	}

	var treadWood, riserWood, stringerSteel, riserSteel int
	for _, p := range res.Package.Parts {
		switch string(p.Kind) {
		case "tread":
			if p.Material == "WOOD-OAK" {
				treadWood++
			}
		case "stringer":
			if p.Material == "STEEL-S235" {
				stringerSteel++
			}
		case "riser":
			if p.Material == "WOOD-OAK" {
				riserWood++
			}
			if p.Material == "STEEL-S235" {
				riserSteel++
			}
		}
	}
	if treadWood == 0 {
		t.Errorf("ни одна проступь не изготовлена из WOOD-OAK — материал ступеней не применяется")
	}
	if stringerSteel == 0 {
		t.Errorf("ни один косоур не изготовлен из STEEL-S235 — материал каркаса не применяется")
	}
	if riserWood == 0 {
		t.Errorf("ни один подступенок не изготовлен из WOOD-OAK — подступенок должен идти по материалу ступеней")
	}
	if riserSteel != 0 {
		t.Errorf("%d подступенков изготовлено из STEEL-S235, хотя ступени деревянные: подступенок идёт по материалу ступеней", riserSteel)
	}

	// Ни одна стальная деталь не должна получить минимальную толщину дуба.
	for _, p := range res.Package.Parts {
		if p.Material == "STEEL-S235" && p.Thickness.Millimeters() > 60 {
			t.Errorf("стальная деталь %q имеет неправдоподобную толщину %.1f мм",
				p.Number, p.Thickness.Millimeters())
		}
	}
}

// TestTreadMaterialInheritsFrame — пустой TreadMaterial наследует материал
// каркаса. Обратная совместимость: все существующие запросы отправляют одно
// поле material, и они обязаны вести себя как раньше.
func TestTreadMaterialInheritsFrame(t *testing.T) {
	svc := NewService()
	cfg := metalFrameWoodTreadConfig(t)
	cfg.TreadMaterial = "" // не задан → наследуется STEEL-S235
	// Толщину ступени тогда тоже надо согласовать с материалом каркаса,
	// иначе это другая ошибка (не тот сценарий).
	cfg.StepThickness = engineering.Length(8)

	res, err := svc.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("Calculate без tread_material: %v", err)
	}
	if res.Package == nil {
		t.Fatal("нет производственного пакета")
	}
	for _, p := range res.Package.Parts {
		if string(p.Kind) == "tread" && p.Material != "STEEL-S235" {
			t.Errorf("проступь без tread_material должна идти по каркасу, получено %q", p.Material)
		}
	}
}

// TestRiserThicknessInheritsStepThickness — пустая толщина подступенка
// наследует толщину ступени. Геометрия до разделения не должна измениться ни
// на миллиметр, иначе поедут раскрой, масса и цена у всех старых проектов.
func TestRiserThicknessInheritsStepThickness(t *testing.T) {
	svc := NewService()
	// Наследование проверяем так: RiserThickness = 0 должно давать ровно тот
	// же результат, что и RiserThickness = StepThickness. Сравнивать с
	// «другой толщиной» бессмысленно — 8 мм стали и 40 мм дуба весят по-
	// разному, и цена ОБЯЗАНА отличаться.
	withRiser := metalFrameWoodTreadConfig(t)
	withRiser.RiserThickness = withRiser.StepThickness // явная толщина = толщина ступени
	withoutRiser := metalFrameWoodTreadConfig(t)
	withoutRiser.RiserThickness = 0

	a, err := svc.Calculate(context.Background(), withRiser, Options{})
	if err != nil {
		t.Fatalf("Calculate с толщиной подступенка: %v", err)
	}
	b, err := svc.Calculate(context.Background(), withoutRiser, Options{})
	if err != nil {
		t.Fatalf("Calculate без толщины подступенка: %v", err)
	}
	if a.Price == nil || b.Price == nil {
		t.Fatal("нет цены")
	}
	if a.Price.FinalPrice != b.Price.FinalPrice {
		t.Errorf("толщина подступенка не наследуется при 0: цена %v vs %v",
			a.Price.FinalPrice, b.Price.FinalPrice)
	}
}

// TestWoodFrameWithWoodTreadStillBlocked — обратная проверка: ДЕРЕВЯННЫЙ
// каркас с толщиной косоура 8 мм по-прежнему отвергается. Разделение не
// должно ослабить проверки каталога MFG-0005 (у дерева MinThickness = 20 мм).
func TestWoodFrameWithWoodTreadStillBlocked(t *testing.T) {
	svc := NewService()
	cfg := metalFrameWoodTreadConfig(t)
	cfg.Material = "WOOD-OAK"
	cfg.TreadMaterial = "WOOD-OAK"

	res, err := svc.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	// Каталог отвергает конфигурацию НЕ ошибкой транспорта, а
	// blocking-issue с конкретными числами (errors.go превращает строки
	// material-ошибок в MFG_MATERIAL). Проверяем именно это состояние.
	if !res.Validation.Blocking {
		t.Fatal("деревянный каркас с косоуром 8 мм должен блокировать расчёт")
	}
	if res.Package != nil {
		t.Error("производственный пакет не должен собираться для заблокированной конфигурации")
	}
}

// TestRiserFollowsTreadMaterial — подступенок идёт по материалу ступеней
// (решение владельца): стальной каркас + дубовые ступени → деревянные
// подступенки. Проверяем и симметричный случай (всё стальное), и то, что
// сохранённая конфигурация со стальным riser_thickness_mm НЕ падает с 422:
// сервер отбрасывает недопустимую для материала ступеней толщину в пользу
// толщины ступени. Иначе все накопленные конфигурации, созданные до смены
// правила, сломались бы разом.
func TestRiserFollowsTreadMaterial(t *testing.T) {
	svc := NewService()

	riserMaterials := func(res *Result) map[string]int {
		out := map[string]int{}
		for _, p := range res.Package.Parts {
			if string(p.Kind) == "riser" {
				out[string(p.Material)]++
			}
		}
		return out
	}

	// Стальной каркас + дубовые ступени, riser_thickness_mm = 6 (сталь) —
	// значение из старых конфигураций. Не должно быть ни 422, ни стальных
	// подступенков.
	cfg := metalFrameWoodTreadConfig(t)
	cfg.RiserThickness = engineering.Length(6)
	res, err := svc.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("старое riser_thickness_mm=6 не должно отвергаться: %v", err)
	}
	if got := riserMaterials(res); got["STEEL-S235"] != 0 {
		t.Errorf("стальных подступенков %d, хотя ступени деревянные: %v", got["STEEL-S235"], got)
	}
	if got := riserMaterials(res); got["WOOD-OAK"] == 0 {
		t.Errorf("нет деревянных подступенков: %v", got)
	}

	// Всё стальное: подступенки стальные, толщина ступени.
	steel := metalFrameWoodTreadConfig(t)
	steel.TreadMaterial = "STEEL-S235"
	steel.StepThickness = engineering.Length(6)
	steel.RiserThickness = engineering.Length(0) // 0 → наследует толщину ступени
	res, err = svc.Calculate(context.Background(), steel, Options{})
	if err != nil {
		t.Fatalf("стальная лестница не посчиталась: %v", err)
	}
	if got := riserMaterials(res); got["STEEL-S235"] == 0 {
		t.Errorf("стальных подступенков нет, хотя всё из стали: %v", got)
	}
}
