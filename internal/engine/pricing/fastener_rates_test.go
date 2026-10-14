package pricing

import (
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// Запасные ставки строятся ПЕРЕБОРОМ каталога, а не захардкоженным
// списком. Это сделано специально: новая позиция каталога обязана получить
// цену автоматически. Возврат к списку — тихая регрессия, которую видно
// только проверкой «цен столько же, сколько позиций».
//
// Цена неположительная или отсутствующая у позиции каталога означает, что
// смета по этой позиции покажет 0 ₽ без ошибки.

// TestFastenerPricesCoverCatalog — у каждой позиции каталога крепежа есть
// запасная цена.
func TestFastenerPricesCoverCatalog(t *testing.T) {
	prices := DefaultFastenerPrices()
	catalog := dommfg.DefaultFastenerRegistry().Fasteners()
	if len(catalog) == 0 {
		t.Fatal("встроенный каталог крепежа пуст — тест бессмысленен")
	}
	if len(prices) != len(catalog) {
		t.Errorf("цен крепежа %d на %d позиций каталога: новые позиции получат нулевую цену молча",
			len(prices), len(catalog))
	}
	for _, f := range catalog {
		p, ok := prices[f.Code]
		if !ok {
			t.Errorf("у позиции крепежа %q нет запасной цены", f.Code)
			continue
		}
		if p <= 0 {
			t.Errorf("у позиции крепежа %q запасная цена неположительна: %d", f.Code, p)
		}
	}
}

// TestSealantPricesCoverCatalog — то же для герметика.
func TestSealantPricesCoverCatalog(t *testing.T) {
	prices := DefaultSealantPrices()
	catalog := dommfg.DefaultSealantRegistry().Sealants()
	if len(catalog) == 0 {
		t.Fatal("встроенный каталог герметика пуст — тест бессмысленен")
	}
	if len(prices) != len(catalog) {
		t.Errorf("цен герметика %d на %d позиций каталога", len(prices), len(catalog))
	}
	for _, s := range catalog {
		if p, ok := prices[s.Code]; !ok || p <= 0 {
			t.Errorf("у позиции герметика %q запасная цена отсутствует или неположительна: %d (ok=%v)",
				s.Code, p, ok)
		}
	}
}

// TestPurchasedPricesCoverCatalog — то же для покупных деталей ограждения.
// Именно здесь стоял ноль: Rates.Purchased не заполнял никто, и ограждение
// в смете стоило 0 ₽ (S-179).
func TestPurchasedPricesCoverCatalog(t *testing.T) {
	prices := DefaultPurchasedPrices()
	catalog := dommfg.PurchasedParts()
	if len(catalog) == 0 {
		t.Fatal("встроенный каталог покупных деталей пуст — тест бессмысленен")
	}
	if len(prices) != len(catalog) {
		t.Errorf("цен покупных деталей %d на %d видов детали", len(prices), len(catalog))
	}
	for _, p := range catalog {
		if price, ok := prices[p.Kind]; !ok || price <= 0 {
			t.Errorf("у вида детали %q запасная цена отсутствует или неположительна: %d (ok=%v)",
				p.Kind, price, ok)
		}
	}
}
