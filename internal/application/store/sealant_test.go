package store

import (
	"context"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	engprc "stairplatform/internal/engine/pricing"
)

// engprc_DefaultSealantPriceRub — встроенная ставка герметика из движка.
// Тест сверяется с ней, а не с зашитым 500: если владелец поменяет ставку,
// тест должен падать на НОВУЮ цифру, а не молча проверять устаревшую.
const engprc_DefaultSealantPriceRub = engprc.DefaultSealantPriceRub

// sealantTestTenant — произвольный tenant: тесты правят только его прайс.
const sealantTestTenant = "t-1"

// Тесты прайса герметика (CONF-SEALANT) — полный аналог прайса крепежа.

// TestSealantPricesListCatalogWithBuiltInPrice — список прайса содержит ВСЕ
// позиции каталога, даже если магазин задал цену только для части.
//
// Это и есть смысл хранения цен отдельно от каталога: каталог задаёт код
// движком, магазин может переопределить цену. Наличие позиции в прайсе не
// должно зависеть от того, трогал её владелец или нет.
func TestSealantPricesListCatalogWithBuiltInPrice(t *testing.T) {
	svc := NewService(newFakeRepo())
	got, err := svc.SealantPrices(context.Background(), sealantTestTenant)
	if err != nil {
		t.Fatalf("SealantPrices: %v", err)
	}
	want := len(dommfg.DefaultSealantRegistry().Sealants())
	if len(got) != want {
		t.Fatalf("позиций в прайсе = %d, want %d (весь каталог)", len(got), want)
	}
	for _, p := range got {
		if p.PricePerCartridgeRub != engprc_DefaultSealantPriceRub {
			t.Errorf("%s: цена %d, want встроенная %d", p.Code, p.PricePerCartridgeRub, engprc_DefaultSealantPriceRub)
		}
		if p.Overridden {
			t.Errorf("%s: Overridden=true без правки магазином", p.Code)
		}
		if p.VolumeML <= 0 || p.NameRu == "" {
			t.Errorf("%s: описание из каталога не пришло: %+v", p.Code, p)
		}
	}
}

// TestSetSealantPriceOverridesCalculationRate — цена магазина доходит до
// расчёта. Без этого шага правка цены в админке была бы молчаливым
// no-op: страница показывала бы новую цифру, а смета считала по встроенной.
func TestSetSealantPriceOverridesCalculationRate(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	if err := svc.SetSealantPrice(context.Background(), sealantTestTenant,
		dommfg.DefaultSealantCode, 950, "u-1"); err != nil {
		t.Fatalf("SetSealantPrice: %v", err)
	}
	rates, err := svc.ResolveRates(context.Background(), sealantTestTenant)
	if err != nil {
		t.Fatalf("ResolveRates: %v", err)
	}
	// Прайс хранит рубли, ставки — копейки (Money), поэтому 950 ₽ = 95000.
	if want := domprc.Money(950 * 100); rates.Sealant[dommfg.DefaultSealantCode] != want {
		t.Fatalf("ставка герметика = %d копеек, want %d", rates.Sealant[dommfg.DefaultSealantCode], want)
	}
}

// TestSetSealantPriceRejectsZeroAndUnknown — нулевая цена и несуществующий
// код отклоняются (DOM-007: иначе опечатка в имени поля дала бы «бесплатный
// герметик» и ответ 200).
func TestSetSealantPriceRejectsZeroAndUnknown(t *testing.T) {
	svc := NewService(newFakeRepo())
	if err := svc.SetSealantPrice(context.Background(), sealantTestTenant, dommfg.DefaultSealantCode, 0, "u-1"); err == nil {
		t.Error("нулевая цена герметика должна отклоняться")
	}
	if err := svc.SetSealantPrice(context.Background(), sealantTestTenant, "SEALANT-НЕТ-ТАКОГО", 500, "u-1"); err == nil {
		t.Error("неизвестный код герметика должен отклоняться")
	}
}

// TestDeleteSealantPriceRestoresBuiltIn — сброс возвращает встроенную ставку.
func TestDeleteSealantPriceRestoresBuiltIn(t *testing.T) {
	svc := NewService(newFakeRepo())
	ctx := context.Background()
	if err := svc.SetSealantPrice(ctx, sealantTestTenant, dommfg.DefaultSealantCode, 950, "u-1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSealantPrice(ctx, sealantTestTenant, dommfg.DefaultSealantCode); err != nil {
		t.Fatal(err)
	}
	got, err := svc.SealantPrices(ctx, sealantTestTenant)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if p.Code != dommfg.DefaultSealantCode {
			continue
		}
		if p.Overridden {
			t.Fatal("после сброса позиция не должна оставаться переопределённой")
		}
		if p.PricePerCartridgeRub != engprc_DefaultSealantPriceRub {
			t.Fatalf("цена = %d, want встроенная %d", p.PricePerCartridgeRub, engprc_DefaultSealantPriceRub)
		}
		return
	}
	t.Fatalf("позиция %s пропала из прайса", dommfg.DefaultSealantCode)
}
