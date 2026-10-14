package store

import (
	"context"

	dommfg "stairplatform/internal/domain/manufacturing"
	engprc "stairplatform/internal/engine/pricing"
)

// PurchasedPrice — закупочная цена покупной детали ограждения.
//
// Стоимость в рублях за штуку, как цена материала за кг: это витринная
// величина, а не minor-единицы движка.
type PurchasedPrice struct {
	// Kind — вид детали (rail_top, post, rigel, tip, flange, slip, deco,
	// end_cap).
	//
	// Именно вид, а не код позиции и не номер экземпляра: у каждой стойки свой
	// номер в спецификации (PST-01, PST-02, …), и прайс по нему построить
	// нельзя — стойка стоит столько-то независимо от их количества.
	Kind dommfg.PartKind `json:"kind"`
	// CatalogID — устойчивый идентификатор позиции каталога (POST-001 и т. п.).
	// Приходит из движка, чтобы админка не держала вторую копию каталога.
	CatalogID string `json:"catalog_id"`
	// NameRu — человекочитаемое имя из каталога.
	NameRu string `json:"name_ru"`
	// PricePerPcRub — цена в рублях за штуку.
	PricePerPcRub int64 `json:"price_per_pc_rub"`
	// Overridden — true, если цену задал магазин; false — действует встроенная
	// запасная ставка движка.
	Overridden bool `json:"overridden"`
}

// PurchasedPrices — закупочные цены ограждения с учётом встроенных ставок.
//
// Возвращаются ВСЕ позиции каталога, а не только переопределённые: админке
// нужно видеть и те, где действует запасная ставка, иначе она не знает, что
// менять.
func (s *Service) PurchasedPrices(ctx context.Context, tenantID string) ([]PurchasedPrice, error) {
	overrides, err := s.repo.ListPurchasedPrices(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	byKind := make(map[dommfg.PartKind]int64, len(overrides))
	for _, p := range overrides {
		byKind[p.Kind] = p.PricePerPcRub
	}
	defaults := engprc.DefaultRates().Purchased
	catalog := dommfg.PurchasedParts()
	out := make([]PurchasedPrice, 0, len(catalog))
	for _, p := range catalog {
		price, ok := byKind[p.Kind]
		if !ok {
			price = rubMajor(defaults[p.Kind])
		}
		out = append(out, PurchasedPrice{
			Kind: p.Kind, CatalogID: p.CatalogID, NameRu: p.Name,
			PricePerPcRub: price, Overridden: ok,
		})
	}
	return out, nil
}

// SetPurchasedPrice сохраняет цену покупной детали.
func (s *Service) SetPurchasedPrice(ctx context.Context, tenantID string, kind dommfg.PartKind, pricePerPcRub int64, updatedBy string) error {
	if tenantID == "" {
		return ErrNotFound
	}
	// Вид детали должен быть из каталога: иначе в прайсе появится строка,
	// которая никогда не участвует в расчёте.
	if dommfg.PurchasedPartCatalogID(kind) == "" {
		return ErrInvalid
	}
	// Нулевая цена запрещена по той же причине, что и у материала и крепежа:
	// опечатка в поле дала бы «бесплатное ограждение» с ответом 200.
	if pricePerPcRub <= 0 {
		return ErrInvalid
	}
	return s.repo.SetPurchasedPrice(ctx, tenantID, string(kind), pricePerPcRub, updatedBy)
}

// DeletePurchasedPrice возвращает деталь ограждения к встроенной ставке.
func (s *Service) DeletePurchasedPrice(ctx context.Context, tenantID string, kind dommfg.PartKind) error {
	if tenantID == "" {
		return ErrNotFound
	}
	return s.repo.DeletePurchasedPrice(ctx, tenantID, string(kind))
}
