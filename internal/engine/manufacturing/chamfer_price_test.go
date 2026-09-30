package manufacturing

import (
	"context"
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	enggeo "stairplatform/internal/engine/geometry"
	engprc "stairplatform/internal/engine/pricing"
)

// Скругление носа — работа, и она обязана быть оплачена. Здесь считается
// главный вопрос фазы 0: РАСТЁТ ЛИ ЦЕНА на ОДНОЙ И ТОЙ ЖЕ конфигурации при
// включённой фаске.
//
// Сравнивать лестницы из разных материалов бесполезно: разница в цене металла
// или древесины перекрывает стоимость обработки. Поэтому здесь меняется
// ровно одно — радиус скругления.

func chamferFinalPrice(t *testing.T, cfg *engineering.StairConfiguration, radius float64) (final int64, laborMinor float64, datasetLabor float64) {
	t.Helper()
	cp := *cfg
	cp.TreadNoseRadiusMM = engineering.Length(radius)
	// TreadMaterial: ступени из дуба — фаска положена по материалу ступеней.
	cp.Material = "STEEL-S235"
	cp.TreadMaterial = "WOOD-OAK"
	cp.Riser = true

	gen, err := enggeo.Generate(context.Background(), &cp)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(&cp, gen)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := DefaultMaterialRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ds, err := PrepareCost(pkg, reg, DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	price, err := engprc.Price(ds, engprc.DefaultRates())
	if err != nil {
		t.Fatal(err)
	}
	return price.FinalPrice.Minor(), float64(price.Labor.Minor()), ds.EstimatedLaborTime
}

func TestChamferRaisesPriceOnSameConfig(t *testing.T) {
	cfg := testConfig(t)
	without, _, laborWithout := chamferFinalPrice(t, cfg, 0)
	with, _, laborWith := chamferFinalPrice(t, cfg, 8)

	if with <= without {
		t.Fatalf("price with chamfer %d must exceed price without %d", with, without)
	}

	// Дельта времени = 15 ступеней × (наладка + 900 мм / подача по дубу).
	rates := DefaultMachineRates()
	wantMin := float64(15) * (rates.MillSetupMin + 900/rates.MillFeed["WOOD-OAK"])
	if d := laborWith - laborWithout; math.Abs(d-wantMin) > 1e-6 {
		t.Fatalf("labor delta = %v min, want %v min (15 treads × (setup + 900/feed))", d, wantMin)
	}

	// Дельта в деньгах = дельта труда с наценками: накладные, маржа, скидка, НДС.
	pr := engprc.DefaultRates()
	perHour := pr.LaborPerHour.Major(pr.Currency)
	wantMoney := wantMin / 60 * perHour
	wantMoney *= 1 + pr.OverheadPercent.Percent()
	wantMoney *= 1 + pr.MarginPercent.Percent()
	wantMoney *= 1 - pr.DiscountPercent.Percent()
	wantMoney *= 1 + pr.TaxPercent.Percent()
	gotMoney := float64(with-without) / 100
	if rel := absRel(gotMoney, wantMoney); rel > 0.01 {
		t.Fatalf("price delta = %.2f ₽, want about %.2f ₽ (relative error %.4f)", gotMoney, wantMoney, rel)
	}
	t.Logf("фаска +8 мм: цена %d → %d (+%.0f ₽), труд +%.2f мин на 15 ступеней",
		without, with, gotMoney, wantMin)
}

func TestNoChamferMeansNoPriceChange(t *testing.T) {
	// Радиус 0 (металл) — цена обязана совпасть с расчётом без скругления
	// бит в бит: поле не должно влиять на цену само по себе.
	cfg := testConfig(t)
	a, laborA, _ := chamferFinalPrice(t, cfg, 0)
	b, laborB, _ := chamferFinalPrice(t, cfg, 0)
	if a != b || laborA != laborB {
		t.Fatalf("price must be deterministic: %d/%v vs %d/%v", a, laborA, b, laborB)
	}
}

func absRel(got, want float64) float64 {
	if want == 0 {
		return 1
	}
	d := got - want
	if d < 0 {
		d = -d
	}
	return d / want
}
