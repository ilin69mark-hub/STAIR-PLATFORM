package store

import (
	"context"
	"fmt"
	"sort"

	"stairplatform/internal/domain/manufacturing"
	engprc "stairplatform/internal/engine/pricing"
)

// ПРАЙС ГЕРМЕТИКА (CONF-SEALANT).
//
// Устроен как прайс крепежа (fastener.go) и по той же причине: позиции
// берутся из ВСТРОЕННОГО каталога движка, а магазин может только
// переопределить цену. Своя копия каталога в магазине была бы второй
// правдой — добавили позицию в движке, и в прайсе бы её не оказалось.
//
// Единица — картридж (₽/штука), как и у крепежа: герметик покупают
// картриджами 310 и 600 мл, и обе цены живут в одной карте.

// SealantPrice — цена герметика магазина, ₽ за картридж.
type SealantPrice struct {
	Code string `json:"code"`
	// NameRu, Kind, VolumeML, LayerMM приходят из каталога движка, чтобы
	// админка не держала вторую копию описаний позиций.
	NameRu string `json:"name_ru"`
	Kind   string `json:"kind"`
	// VolumeML — объём картриджа (310 или 600). В смете и в прайсе это одна
	// и та же единица «штука», поэтому цена одна, а объём разный.
	VolumeML float64 `json:"volume_ml"`
	// LayerMM — толщина слоя по умолчанию для этой позиции.
	LayerMM float64 `json:"layer_mm"`
	// DensityKgPerL — плотность: нужна админке для справки и проверки, что
	// позиция не вырождена.
	DensityKgPerL float64 `json:"density_kg_per_l"`
	// PricePerCartridgeRub — цена в рублях за картридж (не minor-единицы:
	// это витринная величина, как и цена материала за кг).
	PricePerCartridgeRub int64 `json:"price_per_cartridge_rub"`
	// Overridden — true, если цену задал магазин; false — действует
	// встроенная ставка движка.
	Overridden bool `json:"overridden"`
}

// SealantPrices возвращает весь прайс герметика: позиции каталога с ценой,
// заданной магазином, плюс остальные со встроенной ценой. Порядок
// стабильный: сначала переопределённые, затем встроенные — по коду (тот же
// порядок, что у прайса материалов и крепежа).
func (s *Service) SealantPrices(ctx context.Context, tenantID string) ([]SealantPrice, error) {
	overrides, err := s.repo.ListSealantPrices(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	byCode := make(map[string]int64, len(overrides))
	for _, p := range overrides {
		byCode[p.Code] = p.PricePerCartridgeRub
	}
	defaults := engprc.DefaultRates().Sealant
	catalog := manufacturing.DefaultSealantRegistry().Sealants()
	out := make([]SealantPrice, 0, len(catalog))
	for _, sl := range catalog {
		price, ok := byCode[sl.Code]
		if !ok {
			price = rubMajor(defaults[sl.Code])
		}
		out = append(out, SealantPrice{
			Code: sl.Code, NameRu: sl.NameRu, Kind: string(sl.Kind),
			VolumeML: sl.VolumeML, LayerMM: sl.DefaultLayerMM, DensityKgPerL: sl.DensityKgPerL,
			PricePerCartridgeRub: price, Overridden: ok,
		})
	}
	sortSealantPrices(out)
	return out, nil
}

// SetSealantPrice сохраняет цену герметика магазина.
func (s *Service) SetSealantPrice(ctx context.Context, tenantID, code string, pricePerCartridgeRub int64, updatedBy string) error {
	if tenantID == "" {
		return ErrNotFound
	}
	if _, ok := manufacturing.DefaultSealantRegistry().Find(code); !ok {
		return fmt.Errorf("%w: неизвестный герметик %q", ErrInvalid, code)
	}
	// Нулевая цена запрещена по той же причине, что и у материалов и
	// крепежа (DOM-007): опечатка в имени поля дала бы «бесплатный
	// герметик» и ответ 200 «успешно сохранено».
	if pricePerCartridgeRub <= 0 {
		return fmt.Errorf("%w: цена герметика должна быть положительной (получено %d)", ErrInvalid, pricePerCartridgeRub)
	}
	return s.repo.SetSealantPrice(ctx, tenantID, code, pricePerCartridgeRub, updatedBy)
}

// DeleteSealantPrice возвращает герметик к встроенной ставке.
func (s *Service) DeleteSealantPrice(ctx context.Context, tenantID, code string) error {
	if tenantID == "" {
		return ErrNotFound
	}
	return s.repo.DeleteSealantPrice(ctx, tenantID, code)
}

// sortSealantPrices даёт стабильный порядок: сначала заданные магазином,
// затем встроенные — по коду позиции.
func sortSealantPrices(prices []SealantPrice) {
	sort.Slice(prices, func(i, j int) bool {
		if prices[i].Overridden != prices[j].Overridden {
			return prices[i].Overridden
		}
		return prices[i].Code < prices[j].Code
	})
}
