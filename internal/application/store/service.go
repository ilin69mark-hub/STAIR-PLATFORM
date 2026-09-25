package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	engprc "stairplatform/internal/engine/pricing"
)

// Service — настройки и прайс магазина поверх репозитория и движковых
// дефолтов. Не зависит от HTTP и БД (ADR-0006).
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService создаёт сервис настроек магазина.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// WithClock подменяет часы (тесты).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// Settings возвращает настройки магазина tenant. Если их нет — дефолты:
// магазин работает сразу после установки, без обязательной настройки.
func (s *Service) Settings(ctx context.Context, tenantID string) (Settings, error) {
	if tenantID == "" {
		return Settings{}, ErrNotFound
	}
	got, err := s.repo.GetSettings(ctx, tenantID)
	if err != nil {
		if isNotFound(err) {
			return DefaultSettings(), nil
		}
		return Settings{}, err
	}
	return got.WithDefaults(), nil
}

// UpdateSettings валидирует и сохраняет настройки магазина.
func (s *Service) UpdateSettings(ctx context.Context, tenantID string, in Settings) (Settings, error) {
	if tenantID == "" {
		return Settings{}, ErrNotFound
	}
	next := in.WithDefaults()
	next.UpdatedAt = s.now().UTC()
	if err := next.Validate(); err != nil {
		return Settings{}, err
	}
	if err := s.repo.SaveSettings(ctx, tenantID, next, next.UpdatedBy); err != nil {
		return Settings{}, err
	}
	return next, nil
}

// PublicSettings возвращает витринное подмножество настроек.
func (s *Service) PublicSettings(ctx context.Context, tenantID string) (PublicSettings, error) {
	got, err := s.Settings(ctx, tenantID)
	if err != nil {
		return PublicSettings{}, err
	}
	return got.Public(), nil
}

// MaterialPrices возвращает цены материалов магазина, дополненные встроенными
// ставками движка: так витрина и расчёт видят одинаковые цены, а Overridden
// показывает администратору, что менял он.
func (s *Service) MaterialPrices(ctx context.Context, tenantID string) ([]MaterialPrice, error) {
	overrides, err := s.repo.ListMaterialPrices(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	byCode := make(map[string]int64, len(overrides))
	for _, p := range overrides {
		byCode[p.Code] = p.PricePerKgRub
	}
	defaults := engprc.DefaultRates().Material
	out := make([]MaterialPrice, 0, len(defaults))
	for code, price := range byCode {
		out = append(out, MaterialPrice{Code: code, PricePerKgRub: price, Overridden: true})
	}
	// Встроенные ставки добавляем отдельно: витрина показывает все материалы
	// каталога, даже если магазин задал цену только для части.
	for code, money := range defaults {
		if _, ok := byCode[string(code)]; ok {
			continue
		}
		out = append(out, MaterialPrice{Code: string(code), PricePerKgRub: rubMajor(money), Overridden: false})
	}
	sortPrices(out)
	return out, nil
}

// SetMaterialPrice сохраняет цену материала магазина.
func (s *Service) SetMaterialPrice(ctx context.Context, tenantID, code string, pricePerKgRub int64, updatedBy string) error {
	if tenantID == "" {
		return ErrNotFound
	}
	if _, ok := engprc.DefaultRates().Material[dommfg.MaterialCode(code)]; !ok {
		return fmt.Errorf("%w: unknown material %q", ErrInvalid, code)
	}
	if pricePerKgRub < 0 {
		return ErrInvalid
	}
	return s.repo.SetMaterialPrice(ctx, tenantID, code, pricePerKgRub, updatedBy)
}

// DeleteMaterialPrice возвращает материал к встроенной ставке.
func (s *Service) DeleteMaterialPrice(ctx context.Context, tenantID, code string) error {
	if tenantID == "" {
		return ErrNotFound
	}
	return s.repo.DeleteMaterialPrice(ctx, tenantID, code)
}

// ResolveRates собирает ставки расчёта для tenant: встроенные значения плюс
// настройки магазина и цены материалов. Функция вызывается на публичном
// расчёте и при расчёте проекта — цена везде одна и та же, а из тела запроса
// публичный клиент цену подставить не может.
func (s *Service) ResolveRates(ctx context.Context, tenantID string) (*engprc.Rates, error) {
	rates := engprc.DefaultRates()
	settings, err := s.Settings(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	applySettingsRates(&rates, settings)
	prices, err := s.MaterialPrices(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, p := range prices {
		rates.Material[dommfg.MaterialCode(p.Code)] = moneyFromRub(p.PricePerKgRub)
	}
	return &rates, nil
}

func applySettingsRates(rates *engprc.Rates, s Settings) {
	if s.Rates.MachinePerHourRub > 0 {
		rates.MachinePerHour = moneyFromRub(s.Rates.MachinePerHourRub)
	}
	if s.Rates.LaborPerHourRub > 0 {
		rates.LaborPerHour = moneyFromRub(s.Rates.LaborPerHourRub)
	}
	if r, err := rate(s.Rates.OverheadPercent); err == nil {
		rates.OverheadPercent = r
	}
	if r, err := rate(s.Rates.MarginPercent); err == nil {
		rates.MarginPercent = r
	}
	if r, err := rate(s.Rates.DiscountPercent); err == nil {
		rates.DiscountPercent = r
	}
	if r, err := rate(s.Rates.TaxPercent); err == nil {
		rates.TaxPercent = r
	}
}

func isNotFound(err error) bool {
	return err == ErrNotFound
}

// rubMajor — цена Money в целых рублях: витринные цены храним в рублях за кг,
// а движок считает в минорных единицах.
func rubMajor(m domprc.Money) int64 {
	return int64(m.Major(domprc.CurrencyRUB))
}

// moneyFromRub переводит рубли в минорные единицы (Money).
func moneyFromRub(rub int64) domprc.Money {
	return domprc.NewMoney(rub * 100)
}

func rate(percent float64) (domprc.Rate, error) {
	return domprc.NewRate(percent)
}

// sortPrices даёт стабильный порядок: сначала заданные магазином, затем
// встроенные — по коду материала.
func sortPrices(prices []MaterialPrice) {
	sort.Slice(prices, func(i, j int) bool {
		if prices[i].Overridden != prices[j].Overridden {
			return prices[i].Overridden
		}
		return prices[i].Code < prices[j].Code
	})
}
