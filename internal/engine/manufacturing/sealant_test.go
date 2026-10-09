package manufacturing

import (
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// Тесты расхода герметика (CONF-SEALANT).
//
// Проверяется формула владельца V = S × t и перевод объёма в картриджи с
// ОКРУГЛЕНИЕМ ВВЕРХ: купить 2,08 картриджа нельзя, а купить 2 — не хватит.

// areas — источник площади для расчёта.
type areas float64

func (a areas) TotalM2() float64 { return float64(a) }

func TestSealantFormulaAndCartridgeRounding(t *testing.T) {
	// 0,6518 м² × 1 мм = 0,6518 л; 0,6518 / 0,31 = 2,10 → 3 картриджа.
	d, err := dommfg.SealantDemandFor(areas(0.6518), nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if d.Cartridges != 3 {
		t.Fatalf("картриджей = %d, want 3 (округление вверх из 2,10)", d.Cartridges)
	}
	if d.VolumeL < 0.6517 || d.VolumeL > 0.6519 {
		t.Fatalf("объём = %.4f л, want 0,6518 (V = S × t)", d.VolumeL)
	}
	if d.VolumeWithReserveL < 0.7495 || d.VolumeWithReserveL > 0.7497 {
		t.Fatalf("объём с запасом = %.4f л, want ≈0,7496 (0,6518 × 1,15)", d.VolumeWithReserveL)
	}
	// Плотность силикона 1,04 кг/л: 0,6518 × 1,04 = 0,678 кг.
	if d.MassKg < 0.677 || d.MassKg > 0.679 {
		t.Fatalf("масса = %.3f кг, want ≈0,678", d.MassKg)
	}
}

func TestSealantLayerMultipliesVolume(t *testing.T) {
	// Слой 3 мм → втрое больше объёма, и картриджей тоже втрое с лишним.
	d, err := dommfg.SealantDemandFor(areas(0.6518), nil, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if d.VolumeL < 1.955 || d.VolumeL > 1.956 {
		t.Fatalf("объём = %.4f л, want ≈1,955 (0,6518 × 3)", d.VolumeL)
	}
	// С запасом 15%: 1,9553 × 1,15 = 2,249 л → /0,31 = 7,25 → 8 картриджей.
	if d.Cartridges != 8 {
		t.Fatalf("картриджей = %d, want 8 (2,249 / 0,31 = 7,25 → 8)", d.Cartridges)
	}
	if d.VolumeWithReserveL < 2.248 || d.VolumeWithReserveL > 2.250 {
		t.Fatalf("объём с запасом = %.4f л, want ≈2,249 (1,9553 × 1,15)", d.VolumeWithReserveL)
	}
	if d.ReservePercent != dommfg.SealantReservePercent {
		t.Fatalf("запас = %.0f%%, want %.0f%%", d.ReservePercent, dommfg.SealantReservePercent)
	}
}

func TestSealantExtraCartridgesAreAdded(t *testing.T) {
	// Доп. картриджи прибавляются к автоматическому расходу, а не заменяют
	// его: это ровно то, что нужно, когда авторасхода не хватило.
	d, err := dommfg.SealantDemandFor(areas(0.6518), nil, 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	if d.Cartridges != 7 {
		t.Fatalf("картриджей = %d, want 7 (3 автоматических + 4 доп.)", d.Cartridges)
	}
	// Запас к добавленным по объекту НЕ применяется: человек поставил их под
	// конкретный шов, и 15% поверх были бы враньём.
	if d.ExtraCartridges != 4 || d.AutoCartridges != 3 {
		t.Fatalf("разбивка: авто=%d доп=%d, want 3 и 4", d.AutoCartridges, d.ExtraCartridges)
	}
}

// TestSealantZeroLayerUsesDefault — пустая толщина означает дефолт 1 мм, а не
// «герметика нет»: иначе расход молча обнулялся бы на проектах без явного
// выбора.
func TestSealantZeroLayerUsesDefault(t *testing.T) {
	d, err := dommfg.SealantDemandFor(areas(0.6518), nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if d.LayerMM != dommfg.DefaultSealantLayerMM {
		t.Fatalf("слой = %v, want дефолт %v", d.LayerMM, dommfg.DefaultSealantLayerMM)
	}
}

// TestSealantZeroAreaIsError — нулевая площадь означает, что швов нет (или
// конфигурация вырождена). Молчаливые нулевые картриджи скрыли бы ошибку,
// поэтому расчёт падает.
func TestSealantZeroAreaIsError(t *testing.T) {
	if _, err := dommfg.SealantDemandFor(areas(0), nil, 0, 0); err == nil {
		t.Fatal("нулевая площадь швов должна давать ошибку, а не нулевой расход")
	}
}

func TestSealantZonesSplitByPurpose(t *testing.T) {
	z := dommfg.SealantZonesFrom(390000, 232400, 12560, 0)
	if len(z.Rows) != 3 {
		t.Fatalf("зон = %d, want 3 (нулевая зона не выводится)", len(z.Rows))
	}
	sum := 0.0
	for _, r := range z.Rows {
		sum += r.AreaM2
	}
	if sum < 0.6349 || sum > 0.6350 {
		t.Fatalf("сумма зон = %.4f м², want 0,635", sum)
	}
}