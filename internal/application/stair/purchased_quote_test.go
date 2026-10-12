package stair

import (
	"context"
	"strings"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// S-162: ограждение было в спецификации, но не было в цене.
//
// Диагностика на настоящем расчёте: 29 покупных деталей в пакете и НОЛЬ
// строк про них в смете. То есть менеджер выдавал цену без ограждения, не
// подозревая об этом. Тесты ниже фиксируют исправление: позиции обязаны быть
// видны в смете, даже пока ставок закупки нет.

func purchasedLines(t *testing.T, res *Result) []string {
	t.Helper()
	var out []string
	for _, l := range res.Price.Lines {
		if strings.HasPrefix(l.Source, "purchased:") {
			out = append(out, l.Name)
		}
	}
	return out
}

func TestPurchasedRailingIsVisibleInQuote(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}

	// Сколько покупных деталей в пакете — столько строк обязано быть в смете.
	want := int(dommfg.TotalPurchasedQuantity(res.Cost.PurchasedConsumption))
	if want == 0 {
		t.Fatal("покупных деталей в пакете нет — проверка бессмысленна")
	}
	got := purchasedLines(t, res)
	if len(got) != want {
		t.Fatalf("покупных позиций в пакете %d, а строк в смете %d — ограждение снова выпало из цены", want, len(got))
	}
}

func TestPurchasedDemandCoversAllPurchasedParts(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}

	pkgPurchased := 0
	for _, p := range res.Package.Parts {
		if p.Purchased {
			pkgPurchased++
		}
	}
	got := int(dommfg.TotalPurchasedQuantity(res.Cost.PurchasedConsumption))
	if got != pkgPurchased {
		t.Fatalf("в пакете %d покупных деталей, в потребности %d — часть потерялась", pkgPurchased, got)
	}
	if pkgPurchased < 10 {
		t.Fatalf("ожидалось заметное количество ограждения, получилось %d — проверка ослабла", pkgPurchased)
	}
}

func TestPurchasedWithoutRateStaysVisible(t *testing.T) {
	// Пока закупочных ставок нет, строки должны остаться с нулём — по образцу
	// крепежа. Иначе разрыв снова станет невидимым.
	s := NewService()
	res, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	lines := purchasedLines(t, res)
	if len(lines) == 0 {
		t.Fatal("строк закупки нет вовсе")
	}
	for _, n := range lines {
		if !strings.Contains(n, "Закупка") {
			t.Errorf("строка закупки называется невнятно: %q", n)
		}
	}
}
