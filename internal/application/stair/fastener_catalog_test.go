package stair

import (
	"context"
	"errors"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	engprc "stairplatform/internal/engine/pricing"
)

// Сквозная проверка: позиция крепежа, добавленная магазином в админке
// (000048), доходит до сметы.
//
// Почему тест именно здесь, а не в application/store. Там проверено, что
// позиция попадает в каталог и в прайс. Здесь — что каталог tenant'а реально
// доезжает до расчёта. Между этими двумя точками был зазор: справочник
// читался магазином, а движок считал по встроенному каталогу всегда. Позиция
// была видна в админке и не была видна в смете — ровно тот молчаливый обрыв,
// который стоил денег в S-179.

// catalogOnly — источник магазина: отдаёт заданный каталог крепежа и
// дописывает в ставки цену своей позиции.
//
// Отвечает и за каталог, и за ставки — ровно как store.Service в проде.
// Разделение было бы обманом теста: позиция с ценой в каталоге, но без ставки,
// попала бы в смету нулевой строкой, и «дошла до сметы» ничего не значило бы.
type catalogOnly struct {
	reg   *dommfg.FastenerRegistry
	price int64
}

func (c catalogOnly) FastenerRegistry(context.Context, string) (*dommfg.FastenerRegistry, error) {
	return c.reg, nil
}

func (c catalogOnly) ResolveRates(_ context.Context, _ string) (*engprc.Rates, error) {
	rates := engprc.DefaultRates()
	for _, f := range c.reg.Fasteners() {
		rates.Fastener[f.Code] = domprc.NewMoney(c.price)
	}
	return &rates, nil
}

// catalogWithCode — встроенный каталог плюс одна позиция: так выглядит
// каталог tenant'а, которому владелец добавил крепёж через админку.
func catalogWithCode(t *testing.T, code, name string) *dommfg.FastenerRegistry {
	t.Helper()
	base := dommfg.DefaultFastenerRegistry().Fasteners()
	items := append([]dommfg.Fastener(nil), base...)
	items = append(items, dommfg.Fastener{
		Code: code, NameRu: name, Kind: dommfg.FastenerScrew,
		DiameterMM: 6, LengthMM: 16, Base: dommfg.FastenerBaseWood,
	})
	reg, err := dommfg.NewFastenerRegistry(items...)
	if err != nil {
		t.Fatalf("NewFastenerRegistry: %v", err)
	}
	return reg
}

// TestCustomFastenerReachesEstimate — своя позиция, добавленная в админке,
// доходит до сметы ПО ЦЕНЕ, а не строкой с нулём.
func TestCustomFastenerReachesEstimate(t *testing.T) {
	const code = "SCREW-6x16"
	svc := NewServiceWithStore(
		catalogOnly{reg: catalogWithCode(t, code, "Глухарь 6х16 (свой)"), price: 800},
		catalogOnly{reg: catalogWithCode(t, code, "Глухарь 6х16 (свой)"), price: 800},
	)

	cfg := referenceConfig()
	cfg.Railing = "none"
	cfg.ExtraFasteners = []string{code + "@6"}
	// TenantID обязателен: без контекста справочник магазина не читается
	// вовсе (см. TestCatalogResolutionSkippedWithoutTenant).
	res, err := svc.Calculate(context.Background(), cfg, Options{TenantID: "t-1"})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	// 6 штук по 8 ₽ = 48 ₽ = 4800 копеек.
	for _, l := range res.Price.Lines {
		if l.Source != "fastener:"+code+",qty:6,purpose:по объекту" {
			continue
		}
		if l.Amount.Minor() == 0 {
			t.Fatalf("своя позиция попала в смету строкой с нулём: %s", l.Name)
		}
		return
	}
	t.Fatalf("своя позиция %q не попала в смету.\nИсточники строк: %v",
		code, fastenerSources(res))
}

// TestCustomFastenerWithoutRateStaysVisible — позиция посчитана по отверстиям,
// но ставки по её коду нет: строка остаётся НУЛЕВОЙ и видимой.
//
// Не ошибка, а контракт «разрыв виден»: сумма не растёт, но менеджер видит
// в смете «6 шт», которые никто не посчитал в деньгах. Раньше здесь стоял
// continue, и строка исчезала вместе с расхождением.
func TestCustomFastenerWithoutRateStaysVisible(t *testing.T) {
	const code = "SCREW-6x16"
	reg := catalogWithCode(t, code, "Глухарь 6х16 (свой)")
	// Ставки — встроенные: кода своей позиции в них нет.
	svc := NewServiceWithStore(noRates{}, catalogOnly{reg: reg})

	cfg := referenceConfig()
	cfg.Railing = "none"
	cfg.ExtraFasteners = []string{code + "@6"}
	res, err := svc.Calculate(context.Background(), cfg, Options{TenantID: "t-1"})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	for _, l := range res.Price.Lines {
		if l.Source == "fastener:"+code+",qty:6,purpose:по объекту" {
			if l.Amount.Minor() != 0 {
				t.Fatalf("ставки нет, но строка не нулевая: %d", l.Amount.Minor())
			}
			return
		}
	}
	t.Fatalf("строка без ставки исчезла из сметы — расхождение не видно.\nИсточники: %v",
		fastenerSources(res))
}

// noRates — ставки магазина недоступны, действуют встроенные.
type noRates struct{}

func (noRates) ResolveRates(context.Context, string) (*engprc.Rates, error) {
	return nil, errors.New("store: rates unavailable")
}

// TestCustomFastenerNotCountedWithoutCatalog — позиция, которой нет в
// каталоге, в смету не попадает и расчёт не роняет. Это не дефект, а
// контракт: движок не знает названия такой позиции.
func TestCustomFastenerNotCountedWithoutCatalog(t *testing.T) {
	const code = "SCREW-6x16"
	// Каталог ВСТРОЕННЫЙ: позиции в нём нет.
	svc := NewServiceWithStore(nil, catalogOnly{reg: dommfg.DefaultFastenerRegistry()})

	cfg := referenceConfig()
	cfg.Railing = "none"
	cfg.ExtraFasteners = []string{code + "@6"}
	res, err := svc.Calculate(context.Background(), cfg, Options{TenantID: "t-1"})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	for _, l := range res.Price.Lines {
		if l.Source == "fastener:"+code+",qty:6,purpose:по объекту" {
			t.Fatalf("позиция %q без каталога попала в смету: %s", code, l.Name)
		}
	}
}

// TestStoreCatalogFailureKeepsEstimate — сбой справочника не роняет расчёт:
// считаем по встроенному каталогу. Иначе недоступность базы магазина означала
// бы невозможность посчитать лестницу.
func TestStoreCatalogFailureKeepsEstimate(t *testing.T) {
	svc := NewServiceWithStore(nil, failingCatalog{})
	cfg := referenceConfig()
	cfg.Railing = "none"
	res, err := svc.Calculate(context.Background(), cfg, Options{TenantID: "t-1"})
	if err != nil {
		t.Fatalf("сбой справочника уронил расчёт: %v", err)
	}
	if res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("цена не посчитана: расчёт вернулся пустым")
	}
}

// TestCatalogResolutionSkippedWithoutTenant — без tenant-контекста справочник
// магазина не читается: внутренние вызовы (перебор, ассистент, graphql) не
// должны ходить в базу магазина.
func TestCatalogResolutionSkippedWithoutTenant(t *testing.T) {
	cat := &countingCatalog{}
	svc := NewServiceWithStore(nil, cat)
	if _, err := svc.Calculate(context.Background(), referenceConfig(), Options{}); err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if cat.calls != 0 {
		t.Errorf("справочник магазина прочитан %d раз без tenant-контекста, want 0", cat.calls)
	}
}

// failingCatalog — справочник магазина недоступен.
type failingCatalog struct{}

func (failingCatalog) FastenerRegistry(context.Context, string) (*dommfg.FastenerRegistry, error) {
	return nil, context.DeadlineExceeded
}

// countingCatalog — считает обращения, чтобы поймать лишний поход в базу.
type countingCatalog struct{ calls int }

func (c *countingCatalog) FastenerRegistry(context.Context, string) (*dommfg.FastenerRegistry, error) {
	c.calls++
	return nil, nil
}

// fastenerSources — список источников строк сметы, для сообщения об ошибке.
func fastenerSources(res *Result) []string {
	out := make([]string, 0, len(res.Price.Lines))
	for _, l := range res.Price.Lines {
		if len(l.Source) >= 8 && l.Source[:8] == "fastener" {
			out = append(out, l.Source)
		}
	}
	return out
}
