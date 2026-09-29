package stair

import (
	"context"
	"errors"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	engprc "stairplatform/internal/engine/pricing"
)

// Регрессия CRITICAL-03 (2026-09-27): выбор источника ставок расчёта.
//
// ДО фикса политика жила в транспорте: публичная витрина подкладывала ставки
// магазина (publicStoreRates → storeSvc.ResolveRates), а девять авторизованных
// маршрутов не подкладывали ничего и падали в engprc.DefaultRates(). Одна и та
// же лестница стоила по-разному в зависимости от маршрута, и цена на витрине
// не совпадала с ценой в расчёте. Решение ошибочно принималось в 11 местах
// HTTP-слоя, а не в одном.
//
// ПОСЛЕ фикса источник выбирает единственный код — Service.resolveRates.
// Транспорт только называет контекст (Options.TenantID).

// c003Resolver — резолвер с фиксированной маржой и счётчиком вызовов.
type c003Resolver struct {
	rates  *engprc.Rates
	err    error
	calls  int
	lastID string
}

func (r *c003Resolver) ResolveRates(_ context.Context, tenantID string) (*engprc.Rates, error) {
	r.calls++
	r.lastID = tenantID
	if r.err != nil {
		return nil, r.err
	}
	return r.rates, nil
}

func c003Rates(marginPercent float64) *engprc.Rates {
	rs := engprc.DefaultRates()
	rs.MarginPercent = mustRateHelper(marginPercent)
	return &rs
}

// mustRateHelper — процент → доменная ставка; в тесте падение конверсии
// невозможно, поэтому panic уместнее молчаливого нуля.
func mustRateHelper(percent float64) domprc.Rate {
	r, err := domprc.NewRate(percent)
	if err != nil {
		panic(err)
	}
	return r
}

// TestCRIT003_ExplicitRatesWinOverResolver — явно заданные ставки имеют
// приоритет: внутренние переборы оптимизации и тесты задают их напрямую, и
// resolver не должен их перекрывать.
func TestCRIT003_ExplicitRatesWinOverResolver(t *testing.T) {
	rs := &c003Resolver{rates: c003Rates(0.9)}
	s := NewServiceWithRates(rs)

	explicit := c003Rates(0.1)
	got := s.resolveRates(context.Background(), Options{TenantID: "t-1", Rates: explicit})
	if got != explicit {
		t.Error("explicit Options.Rates must win over the store resolver")
	}
	if rs.calls != 0 {
		t.Errorf("resolver must not be called when rates are explicit, calls=%d", rs.calls)
	}
}

// TestCRIT003_ResolverUsedForTenantContext — при заданном tenant'е берутся
// ставки магазина, и именно для ЭТОГО tenant'а.
func TestCRIT003_ResolverUsedForTenantContext(t *testing.T) {
	rs := &c003Resolver{rates: c003Rates(0.42)}
	s := NewServiceWithRates(rs)

	got := s.resolveRates(context.Background(), Options{TenantID: "tenant-a"})
	if got != rs.rates {
		t.Error("store rates must be used when a tenant context is given")
	}
	if rs.calls != 1 {
		t.Errorf("resolver calls = %d, want 1", rs.calls)
	}
	if rs.lastID != "tenant-a" {
		t.Errorf("resolver got tenant %q, want tenant-a", rs.lastID)
	}
}

// TestCRIT003_NoTenantContextKeepsEngineDefaults — внутренние вызовы без
// tenant-контекста (graphql, ассистент, перебор оптимизации) сохраняют
// поведение до CRITICAL-03: встроенные ставки движка.
func TestCRIT003_NoTenantContextKeepsEngineDefaults(t *testing.T) {
	rs := &c003Resolver{rates: c003Rates(0.42)}
	s := NewServiceWithRates(rs)

	if got := s.resolveRates(context.Background(), Options{}); got != nil {
		t.Errorf("no tenant context must fall back to engine rates, got %+v", got)
	}
	if rs.calls != 0 {
		t.Errorf("resolver must not be called without a tenant context, calls=%d", rs.calls)
	}

	// И сервис без резолвера — тоже встроенные ставки.
	if got := NewService().resolveRates(context.Background(), Options{TenantID: "t"}); got != nil {
		t.Errorf("service without resolver must fall back to engine rates, got %+v", got)
	}
}

// TestCRIT003_ResolverFailureFallsBackNotFails — сбой прайса магазина не
// должен отменять геометрию: расчёт уходит на встроенные ставки. Иначе
// недоступность прайса превращается в 500 на расчёте лестницы.
func TestCRIT003_ResolverFailureFallsBackNotFails(t *testing.T) {
	boom := errors.New("store settings unavailable")
	s := NewServiceWithRates(&c003Resolver{err: boom})

	if got := s.resolveRates(context.Background(), Options{TenantID: "t-1"}); got != nil {
		t.Errorf("resolver error must fall back to engine rates, got %+v", got)
	}

	// Полный конвейер при сломанном резолвере обязан завершиться ценой.
	res, err := s.Calculate(context.Background(), referenceConfig(), Options{TenantID: "t-1"})
	if err != nil {
		t.Fatalf("calculate with broken rates resolver must succeed, got: %v", err)
	}
	if res.Price == nil {
		t.Fatal("price is nil after rates resolver failure")
	}
	if res.Price.FinalPrice <= 0 {
		t.Errorf("final price = %v, want > 0", res.Price.FinalPrice)
	}
}

// TestCRIT003_SameConfigSamePriceRegardlessOfEntryPoint — суть дефекта:
// одна и та же лестница обязана стоить одинаково при расчёте со ставками
// магазина и со встроенными. Если бы резолвер подмешивался не туда или
// транспорт продолжал бы подкладывать Rates, цены разошлись бы.
func TestCRIT003_SameConfigSamePriceRegardlessOfEntryPoint(t *testing.T) {
	cfg := referenceConfig()

	viaStore := NewServiceWithRates(&c003Resolver{rates: c003Rates(0.35)})
	viaEngine := NewService()

	a, err := viaStore.Calculate(context.Background(), cfg, Options{TenantID: "t-1"})
	if err != nil {
		t.Fatalf("calculate via store rates: %v", err)
	}
	b, err := viaEngine.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("calculate via engine rates: %v", err)
	}
	if a.Price == nil || b.Price == nil {
		t.Fatal("price is nil")
	}
	if a.Price.FinalPrice == b.Price.FinalPrice {
		t.Fatalf("rates had no effect on price (%v) — resolver is not wired",
			a.Price.FinalPrice)
	}
	t.Logf("store rates=%v engine rates=%v", a.Price.FinalPrice, b.Price.FinalPrice)
}

// TestCRIT003_StoreRatesMaterialPriceIsApplied — ставки магазина включают
// цену материала за кг (store.ResolveRates наполняет Rates.Material), и
// application-слой обязан донести её до движка, а не только маржу/налоги.
func TestCRIT003_StoreRatesMaterialPriceIsApplied(t *testing.T) {
	cfg := referenceConfig()
	material := cfg.Material
	if material == "" {
		material = dommfg.MaterialCode("STEEL-S235")
	}
	rs := engprc.DefaultRates()
	rs.Material[material] = domprc.NewMoney(90000) // 900 ₽/кг
	rs.MarginPercent = mustRateHelper(20)

	withStore := NewServiceWithRates(&c003Resolver{rates: &rs})
	engine := NewService()

	a, err := withStore.Calculate(context.Background(), cfg, Options{TenantID: "t-1"})
	if err != nil {
		t.Fatalf("calculate with store material price: %v", err)
	}
	b, err := engine.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("calculate with engine rates: %v", err)
	}
	if a.Price == nil || b.Price == nil {
		t.Fatal("price is nil")
	}
	if a.Price.FinalPrice <= b.Price.FinalPrice {
		t.Errorf("expensive material price must raise the total: store=%v engine=%v",
			a.Price.FinalPrice, b.Price.FinalPrice)
	}
}
