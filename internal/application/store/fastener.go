package store

import (
	"context"
	"fmt"
	"sort"

	"stairplatform/internal/domain/manufacturing"
	engprc "stairplatform/internal/engine/pricing"
)

// КРЕПЁЖ В МАГАЗИНЕ (MNT-0001).
//
// Прайс крепежа устроен как прайс материалов, но поштучно и по другому
// справочнику: у материала единица — кг, у крепежа — штука, и в прайс
// попадают не пять материалов, а сорок с лишним позиций дюбелей и
// глухарей.
//
// Позиции берутся из ВСТРОЕННОГО каталога движка, а не из своей копии в
// магазине. Копия была бы второй правдой: добавили позицию в каталог
// движка — она не появилась бы в прайсе, и крепёж в смете молча уехал бы в
// ноль. Здесь магазин может ТОЛЬКО переопределить цену (как и материалы),
// а состав позиций задаёт код.

// FastenerPrice — цена крепежа магазина, ₽ за штуку.
type FastenerPrice struct {
	Code string `json:"code"`
	// NameRu — название позиции из каталога движка. Приходит сюда, чтобы
	// админка не держала вторую копию названий: правится каталог, а таблица
	// цен остаётся просто числами.
	NameRu string `json:"name_ru"`
	// Kind — screw (глухарь) | plug (дюбель).
	Kind string `json:"kind"`
	// DiameterMM / LengthMM — размер позиции. Тоже из каталога: по ним
	// админка группирует и сортирует список, не разбирая код.
	DiameterMM float64 `json:"diameter_mm"`
	LengthMM   float64 `json:"length_mm"`
	// PricePerPcRub — цена в рублях за штуку (не minor-единицы: это
	// витринная величина, как и цена материала за кг).
	PricePerPcRub int64 `json:"price_per_pc_rub"`
	// Overridden — true, если цену задал магазин; false — действует
	// встроенная ставка движка (15 ₽, правится в админке).
	Overridden bool `json:"overridden"`
}

// FastenerPrices возвращает весь прайс крепежа: позиции встроенного
// каталога, у которых цена магазина переопределена, плюс остальные с
// встроенной ценой. Порядок стабильный: сначала переопределённые, потом
// встроенные — по коду (тот же порядок, что у прайса материалов).
func (s *Service) FastenerPrices(ctx context.Context, tenantID string) ([]FastenerPrice, error) {
	overrides, err := s.repo.ListFastenerPrices(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	byCode := make(map[string]int64, len(overrides))
	for _, p := range overrides {
		byCode[p.Code] = p.PricePerPcRub
	}
	defaults := engprc.DefaultRates().Fastener
	out := make([]FastenerPrice, 0, len(defaults))
	for _, f := range manufacturing.DefaultFastenerRegistry().Fasteners() {
		if price, ok := byCode[f.Code]; ok {
			out = append(out, FastenerPrice{
				Code: f.Code, NameRu: f.NameRu, Kind: string(f.Kind),
				DiameterMM: f.DiameterMM, LengthMM: f.LengthMM,
				PricePerPcRub: price, Overridden: true,
			})
		}
	}
	// Встроенные позиции добавляем отдельно: список показывает весь
	// каталог, даже если магазин задал цену только для части.
	for _, f := range manufacturing.DefaultFastenerRegistry().Fasteners() {
		if _, ok := byCode[f.Code]; ok {
			continue
		}
		out = append(out, FastenerPrice{
			Code: f.Code, NameRu: f.NameRu, Kind: string(f.Kind),
			DiameterMM: f.DiameterMM, LengthMM: f.LengthMM,
			PricePerPcRub: rubMajor(defaults[f.Code]), Overridden: false,
		})
	}
	sortFastenerPrices(out)
	return out, nil
}

// SetFastenerPrice сохраняет цену крепежа магазина.
func (s *Service) SetFastenerPrice(ctx context.Context, tenantID, code string, pricePerPcRub int64, updatedBy string) error {
	if tenantID == "" {
		return ErrNotFound
	}
	if _, ok := manufacturing.DefaultFastenerRegistry().Find(code); !ok {
		return fmt.Errorf("%w: неизвестный крепёж %q", ErrInvalid, code)
	}
	// Нулевая цена запрещена по той же причине, что и у материала (DOM-007):
	// опечатка в имени поля дала бы «бесплатный крепёж» и ответ 200.
	if pricePerPcRub <= 0 {
		return fmt.Errorf("%w: цена крепежа должна быть положительной (получено %d)", ErrInvalid, pricePerPcRub)
	}
	return s.repo.SetFastenerPrice(ctx, tenantID, code, pricePerPcRub, updatedBy)
}

// DeleteFastenerPrice возвращает крепёж к встроенной ставке.
func (s *Service) DeleteFastenerPrice(ctx context.Context, tenantID, code string) error {
	if tenantID == "" {
		return ErrNotFound
	}
	return s.repo.DeleteFastenerPrice(ctx, tenantID, code)
}

// sortFastenerPrices даёт стабильный порядок: сначала заданные магазином,
// затем встроенные — по коду позиции.
func sortFastenerPrices(prices []FastenerPrice) {
	sort.Slice(prices, func(i, j int) bool {
		if prices[i].Overridden != prices[j].Overridden {
			return prices[i].Overridden
		}
		return prices[i].Code < prices[j].Code
	})
}