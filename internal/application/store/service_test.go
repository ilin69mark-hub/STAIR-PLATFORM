package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
)

// fakeRepo — репозиторий настроек магазина в памяти.
type fakeRepo struct {
	settings  map[string]Settings
	prices    map[string]map[string]int64
	fasteners map[string]map[string]int64
	// fastenerPositions — свои позиции крепежа tenant'а по коду (000048).
	fastenerPositions map[string]map[string]FastenerPosition
	// errPositions — сбой чтения справочника: им проверяется откат на
	// встроенный каталог вместо падения расчёта.
	errPositions error
	sealants     map[string]map[string]int64
	// purchased — закупочные цены по ВИДУ детали, не по номеру экземпляра.
	purchased map[string]map[dommfg.PartKind]int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		settings:          map[string]Settings{},
		prices:            map[string]map[string]int64{},
		fasteners:         map[string]map[string]int64{},
		fastenerPositions: map[string]map[string]FastenerPosition{},
		sealants:          map[string]map[string]int64{},
		purchased:         map[string]map[dommfg.PartKind]int64{},
	}
}

func (f *fakeRepo) GetSettings(_ context.Context, tenantID string) (Settings, error) {
	s, ok := f.settings[tenantID]
	if !ok {
		return Settings{}, ErrNotFound
	}
	return s, nil
}

func (f *fakeRepo) SaveSettings(_ context.Context, tenantID string, s Settings, updatedBy string) error {
	s.UpdatedBy = updatedBy
	f.settings[tenantID] = s
	return nil
}

func (f *fakeRepo) ListMaterialPrices(_ context.Context, tenantID string) ([]MaterialPrice, error) {
	byCode := f.prices[tenantID]
	out := make([]MaterialPrice, 0, len(byCode))
	for code, price := range byCode {
		out = append(out, MaterialPrice{Code: code, PricePerKgRub: price, Overridden: true})
	}
	return out, nil
}

func (f *fakeRepo) SetMaterialPrice(_ context.Context, tenantID, code string, price int64, _ string) error {
	if f.prices[tenantID] == nil {
		f.prices[tenantID] = map[string]int64{}
	}
	f.prices[tenantID][code] = price
	return nil
}

func (f *fakeRepo) DeleteMaterialPrice(_ context.Context, tenantID, code string) error {
	delete(f.prices[tenantID], code)
	return nil
}

func (f *fakeRepo) ListFastenerPrices(_ context.Context, tenantID string) ([]FastenerPrice, error) {
	byCode := f.fasteners[tenantID]
	out := make([]FastenerPrice, 0, len(byCode))
	for code, price := range byCode {
		out = append(out, FastenerPrice{Code: code, PricePerPcRub: price, Overridden: true})
	}
	return out, nil
}

func (f *fakeRepo) SetFastenerPrice(_ context.Context, tenantID, code string, price int64, _ string) error {
	if f.fasteners[tenantID] == nil {
		f.fasteners[tenantID] = map[string]int64{}
	}
	f.fasteners[tenantID][code] = price
	return nil
}

func (f *fakeRepo) DeleteFastenerPrice(_ context.Context, tenantID, code string) error {
	delete(f.fasteners[tenantID], code)
	return nil
}

// ---- свои позиции крепежа (000048) ----

func (f *fakeRepo) ListFastenerPositions(_ context.Context, tenantID string) ([]FastenerPosition, error) {
	if f.errPositions != nil {
		return nil, f.errPositions
	}
	byCode := f.fastenerPositions[tenantID]
	out := make([]FastenerPosition, 0, len(byCode))
	for _, p := range byCode {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeRepo) CreateFastenerPosition(_ context.Context, tenantID string,
	p FastenerPosition, price int64, _ string) error {
	if f.fastenerPositions[tenantID] == nil {
		f.fastenerPositions[tenantID] = map[string]FastenerPosition{}
	}
	f.fastenerPositions[tenantID][p.Code] = p
	if f.fasteners[tenantID] == nil {
		f.fasteners[tenantID] = map[string]int64{}
	}
	f.fasteners[tenantID][p.Code] = price
	return nil
}

func (f *fakeRepo) DeleteFastenerPosition(_ context.Context, tenantID, code string) error {
	delete(f.fastenerPositions[tenantID], code)
	delete(f.fasteners[tenantID], code)
	return nil
}

func (f *fakeRepo) ListSealantPrices(_ context.Context, tenantID string) ([]SealantPrice, error) {
	byCode := f.sealants[tenantID]
	out := make([]SealantPrice, 0, len(byCode))
	for code, price := range byCode {
		out = append(out, SealantPrice{Code: code, PricePerCartridgeRub: price, Overridden: true})
	}
	return out, nil
}

func (f *fakeRepo) SetSealantPrice(_ context.Context, tenantID, code string, price int64, _ string) error {
	if f.sealants[tenantID] == nil {
		f.sealants[tenantID] = map[string]int64{}
	}
	f.sealants[tenantID][code] = price
	return nil
}

func (f *fakeRepo) DeleteSealantPrice(_ context.Context, tenantID, code string) error {
	delete(f.sealants[tenantID], code)
	return nil
}

// Цены покупных деталей хранятся по ВИДУ детали (part_kind), а не по номеру
// экземпляра: прайс по PST-01 и PST-02 построить нельзя.
func (f *fakeRepo) ListPurchasedPrices(_ context.Context, tenantID string) ([]PurchasedPrice, error) {
	byKind := f.purchased[tenantID]
	out := make([]PurchasedPrice, 0, len(byKind))
	for kind, price := range byKind {
		out = append(out, PurchasedPrice{Kind: kind, PricePerPcRub: price, Overridden: true})
	}
	return out, nil
}

func (f *fakeRepo) SetPurchasedPrice(_ context.Context, tenantID, kind string, price int64, _ string) error {
	if f.purchased[tenantID] == nil {
		f.purchased[tenantID] = map[dommfg.PartKind]int64{}
	}
	f.purchased[tenantID][dommfg.PartKind(kind)] = price
	return nil
}

func (f *fakeRepo) DeletePurchasedPrice(_ context.Context, tenantID, kind string) error {
	delete(f.purchased[tenantID], dommfg.PartKind(kind))
	return nil
}

// TestSettingsDefaultsWithoutRow — ненастроенный магазин работает на дефолтах:
// витрина не отдаёт 404 и расчёт считает как до появления модуля.
func TestSettingsDefaultsWithoutRow(t *testing.T) {
	svc := NewService(newFakeRepo())
	got, err := svc.Settings(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if got.Rates.OverheadPercent != 20 || got.Rates.MarginPercent != 30 || got.Rates.TaxPercent != 20 {
		t.Fatalf("параметры расчёта не совпали с дефолтами движка: %+v", got.Rates)
	}
	if got.SEO.DefaultTitle == "" || got.SEO.DefaultDescription == "" {
		t.Fatalf("SEO-дефолты пусты: %+v", got.SEO)
	}
}

// TestUpdateSettingsValidates — проценты вне 0–100 и отрицательные ставки
// отклоняются, валидные сохраняются.
func TestUpdateSettingsValidates(t *testing.T) {
	svc := NewService(newFakeRepo())
	ctx := context.Background()

	bad := DefaultSettings()
	bad.Rates.MarginPercent = 150
	if _, err := svc.UpdateSettings(ctx, "t-1", bad); err == nil {
		t.Fatal("margin 150% должен отклоняться")
	}
	bad = DefaultSettings()
	bad.Rates.MachinePerHourRub = -1
	if _, err := svc.UpdateSettings(ctx, "t-1", bad); err == nil {
		t.Fatal("отрицательная ставка станка должна отклоняться")
	}

	good := DefaultSettings()
	good.Contacts.Phone = "+7 900 000-00-00"
	good.Rates.MarginPercent = 15
	if _, err := svc.UpdateSettings(ctx, "t-1", good); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	got, err := svc.Settings(ctx, "t-1")
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if got.Contacts.Phone != "+7 900 000-00-00" || got.Rates.MarginPercent != 15 {
		t.Fatalf("настройки не сохранены: %+v", got)
	}
}

// TestMaterialPricesMergeDefaults — прайс магазина перекрывает встроенную ставку
// только для заданного материала, остальные остаются встроенными.
func TestMaterialPricesMergeDefaults(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	if err := svc.SetMaterialPrice(ctx, "t-1", "WOOD-OAK", 4500, "u-1"); err != nil {
		t.Fatalf("SetMaterialPrice: %v", err)
	}
	prices, err := svc.MaterialPrices(ctx, "t-1")
	if err != nil {
		t.Fatalf("MaterialPrices: %v", err)
	}
	byCode := map[string]MaterialPrice{}
	for _, p := range prices {
		byCode[p.Code] = p
	}
	oak, ok := byCode["WOOD-OAK"]
	if !ok || oak.PricePerKgRub != 4500 || !oak.Overridden {
		t.Fatalf("цена магазина для WOOD-OAK не применена: %+v", byCode)
	}
	// Каталог MFG-0005: сталь + 4 породы дерева.
	if len(byCode) < 5 {
		t.Fatalf("встроенные материалы потеряны: %d", len(byCode))
	}
	for code, p := range byCode {
		if code == "WOOD-OAK" {
			continue
		}
		if p.Overridden || p.PricePerKgRub <= 0 {
			t.Fatalf("встроенная ставка %s искажена: %+v", code, p)
		}
	}
}

// TestSetMaterialPriceRejectsUnknown — код вне каталога MFG-0005 отклоняется,
// иначе в прайсе появились бы фантомные материалы.
func TestSetMaterialPriceRejectsUnknown(t *testing.T) {
	svc := NewService(newFakeRepo())
	if err := svc.SetMaterialPrice(context.Background(), "t-1", "GOLD-24K", 999999, "u-1"); err == nil {
		t.Fatal("неизвестный материал должен отклоняться")
	}
	if err := svc.SetMaterialPrice(context.Background(), "t-1", "WOOD-OAK", -5, "u-1"); err == nil {
		t.Fatal("отрицательная цена должна отклоняться")
	}
}

// TestDeleteMaterialPriceRestoresDefault — удаление строки возвращает встроенную
// ставку движка.
func TestDeleteMaterialPriceRestoresDefault(t *testing.T) {
	svc := NewService(newFakeRepo())
	ctx := context.Background()
	if err := svc.SetMaterialPrice(ctx, "t-1", "WOOD-OAK", 4500, "u-1"); err != nil {
		t.Fatalf("SetMaterialPrice: %v", err)
	}
	if err := svc.DeleteMaterialPrice(ctx, "t-1", "WOOD-OAK"); err != nil {
		t.Fatalf("DeleteMaterialPrice: %v", err)
	}
	prices, err := svc.MaterialPrices(ctx, "t-1")
	if err != nil {
		t.Fatalf("MaterialPrices: %v", err)
	}
	for _, p := range prices {
		if p.Code == "WOOD-OAK" {
			if p.Overridden {
				t.Fatal("цена магазина не удалена")
			}
			return
		}
	}
	t.Fatal("встроенная ставка WOOD-OAK пропала")
}

// TestResolveRatesAppliesStore — ставки расчёта собираются из дефолтов движка,
// настроек магазина и прайса материалов.
func TestResolveRatesAppliesStore(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	base, err := svc.ResolveRates(ctx, "t-1")
	if err != nil {
		t.Fatalf("ResolveRates: %v", err)
	}

	settings := DefaultSettings()
	settings.Rates.MarginPercent = 10
	settings.Rates.MachinePerHourRub = 5000
	if _, err := svc.UpdateSettings(ctx, "t-1", settings); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if err := svc.SetMaterialPrice(ctx, "t-1", "WOOD-OAK", 5000, "u-1"); err != nil {
		t.Fatalf("SetMaterialPrice: %v", err)
	}

	got, err := svc.ResolveRates(ctx, "t-1")
	if err != nil {
		t.Fatalf("ResolveRates: %v", err)
	}
	if got.MarginPercent.Percent() != 10 {
		t.Fatalf("маржа магазина не применена: %v", got.MarginPercent.Percent())
	}
	if got.MachinePerHour != domprc.NewMoney(settings.Rates.MachinePerHourRub*100) {
		t.Fatalf("ставка станка не применена: %v", got.MachinePerHour)
	}
	if rub := got.Material["WOOD-OAK"].Major(domprc.CurrencyRUB); rub != 5000 {
		t.Fatalf("цена материала не применена: %v", rub)
	}
	if got.Material["STEEL-S235"] != base.Material["STEEL-S235"] {
		t.Fatal("чужая ставка изменилась")
	}
}

// TestPublicSettingsExcludeRates — витринное подмножество не содержит ставок:
// цена материалов приходит из каталога, параметры расчёта — из движка.
func TestPublicSettingsExcludeRates(t *testing.T) {
	settings := DefaultSettings()
	settings.Rates.MarginPercent = 42
	settings.Contacts.Email = "shop@example.com"
	pub := settings.Public()
	if pub.Contacts.Email != "shop@example.com" {
		t.Fatalf("контакты потеряны: %+v", pub.Contacts)
	}
	raw, err := json.Marshal(pub)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"rates", "margin", "machine_per_hour", "discount", "tax"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("в публичных настройках есть %q: %s", forbidden, raw)
		}
	}
}
