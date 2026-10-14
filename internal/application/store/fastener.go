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
	// Каталог берётся ТЕЛЕКАНА позиций tenant'а: свои позиции магазина тоже
	// показываются в прайсе, иначе добавленная позиция была бы видна только
	// в одной половине интерфейса.
	reg, err := s.FastenerRegistry(ctx, tenantID)
	if err != nil {
		reg = manufacturing.DefaultFastenerRegistry()
	}
	out := make([]FastenerPrice, 0, len(defaults))
	for _, f := range reg.Fasteners() {
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
	for _, f := range reg.Fasteners() {
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
	// Код принимается и из встроенного каталога, и из своих позиций: цена
	// меняется у любой позиции, которая попадает в смету.
	reg, err := s.FastenerRegistry(ctx, tenantID)
	if err != nil {
		reg = manufacturing.DefaultFastenerRegistry()
	}
	if _, ok := reg.Find(code); !ok {
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

// ---- Свои позиции крепежа (решение владельца 2026-10-13) ----
//
// Состав каталога был зашит в код. Новая позиция значила правку Go и
// пересборку сервера, а на практике означала, что крепёж просто не попадал в
// смету: движок код вне каталога пропускает молча. Магазин теперь добавляет
// позиции сам, но ТОЛЬКО для себя: справочник привязан к tenant'у.
//
// Своя позиция не может переопределить встроенную: код, совпавший со
// встроенным, отклоняется. Иначе позиция с названием SCREW-6x35 и другим
// размером тихо испортила бы расчёт.

// FastenerPosition — своя позиция крепежа магазина.
//
// Цена — часть позиции (PricePerPcRub), но в базу она пишется отдельно, в
// store_fastener_prices: цена должна жить в одном месте независимо от того,
// встроенная позиция или добавленная.
type FastenerPosition struct {
	Code          string  `json:"code"`
	NameRu        string  `json:"name_ru"`
	Kind          string  `json:"kind"`
	DiameterMM    float64 `json:"diameter_mm"`
	LengthMM      float64 `json:"length_mm"`
	Base          string  `json:"base"`
	PricePerPcRub int64   `json:"price_per_pc_rub"`
}

// FastenerRegistry собирает каталог крепежа tenant'а: встроенные позиции плюс
// добавленные магазином.
//
// nil или ошибка — не повод ронять расчёт: возвращается встроенный каталог.
// Сбой чтения справочника должен оставлять смету на встроенных позициях, а не
// ронять геометрию (та же политика, что у ResolveRates).
func (s *Service) FastenerRegistry(ctx context.Context, tenantID string) (*manufacturing.FastenerRegistry, error) {
	builtin := manufacturing.DefaultFastenerRegistry()
	if tenantID == "" {
		return builtin, nil
	}
	custom, err := s.repo.ListFastenerPositions(ctx, tenantID)
	if err != nil {
		return builtin, fmt.Errorf("%w: %s", ErrNotFound, err)
	}
	if len(custom) == 0 {
		return builtin, nil
	}
	items := append([]manufacturing.Fastener(nil), builtin.Fasteners()...)
	seen := make(map[string]bool, len(items))
	for _, f := range items {
		seen[f.Code] = true
	}
	for _, p := range custom {
		// Код, совпавший со встроенным, пропускается: позиция с чужим
		// размером под именем чужой позиции опаснее, чем её отсутствие.
		if seen[p.Code] {
			continue
		}
		seen[p.Code] = true
		items = append(items, manufacturing.Fastener{
			Code: p.Code, NameRu: p.NameRu,
			Kind:       manufacturing.FastenerKind(p.Kind),
			DiameterMM: p.DiameterMM,
			LengthMM:   p.LengthMM,
			Base:       manufacturing.FastenerBase(p.Base),
		})
	}
	reg, err := manufacturing.NewFastenerRegistry(items...)
	if err != nil {
		return builtin, fmt.Errorf("%w: %s", ErrInvalid, err)
	}
	return reg, nil
}

// FastenerPositions возвращает позиции крепежа, добавленные магазином.
// Встроенный каталог в список не входит: он один на всех и живёт в коде, его
// видно в FastenerPrices.
func (s *Service) FastenerPositions(ctx context.Context, tenantID string) ([]FastenerPosition, error) {
	if tenantID == "" {
		return nil, ErrNotFound
	}
	return s.repo.ListFastenerPositions(ctx, tenantID)
}

// CreateFastenerPosition добавляет свою позицию крепежа и её цену.
func (s *Service) CreateFastenerPosition(ctx context.Context, tenantID string,
	p FastenerPosition, updatedBy string) error {
	if tenantID == "" {
		return ErrNotFound
	}
	if err := validateNewFastenerPosition(p); err != nil {
		return err
	}
	if _, ok := manufacturing.DefaultFastenerRegistry().Find(p.Code); ok {
		return fmt.Errorf("%w: код %q уже есть во встроенном каталоге — меняйте ему цену, а не создавайте позицию",
			ErrInvalid, p.Code)
	}
	existing, err := s.repo.ListFastenerPositions(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	for _, e := range existing {
		if e.Code == p.Code {
			return fmt.Errorf("%w: позиция %q уже добавлена", ErrInvalid, p.Code)
		}
	}
	return s.repo.CreateFastenerPosition(ctx, tenantID, p, p.PricePerPcRub, updatedBy)
}

// DeleteFastenerPosition удаляет свою позицию крепежа вместе с ценой.
//
// Встроенную позицию удалить нельзя: она в коде, и удаление из справочника
// tenant'а её не уберёт — код вернётся из встроенного каталога на следующем
// же чтении, и владелец решит, что удаление сработало, а оно ничего не
// сделало.
func (s *Service) DeleteFastenerPosition(ctx context.Context, tenantID, code string) error {
	if tenantID == "" || code == "" {
		return ErrNotFound
	}
	if _, ok := manufacturing.DefaultFastenerRegistry().Find(code); ok {
		return fmt.Errorf("%w: %q — встроенная позиция, её не удаляют, а скрывают ценой", ErrInvalid, code)
	}
	return s.repo.DeleteFastenerPosition(ctx, tenantID, code)
}

// validateNewFastenerPosition проверяет позицию ДО записи.
//
// Проверки те же, что у домена, но по строкам: позиция приходит из HTTP, и
// ошибка должна быть названием поля, а не «manufacturing: fastener … has
// unknown kind». Границы размера повторяют CHECK миграции 000048 — чтобы
// ошибка пришла из сервиса с внятным текстом, а не из базы.
func validateNewFastenerPosition(p FastenerPosition) error {
	if p.Code == "" {
		return fmt.Errorf("%w: код крепежа обязателен", ErrInvalid)
	}
	if p.NameRu == "" {
		return fmt.Errorf("%w: название крепежа обязательно", ErrInvalid)
	}
	kind := manufacturing.FastenerKind(p.Kind)
	if !kind.IsValid() {
		return fmt.Errorf("%w: неизвестный вид крепежа %q (screw или plug)", ErrInvalid, p.Kind)
	}
	if p.DiameterMM <= 0 || p.DiameterMM > 50 {
		return fmt.Errorf("%w: диаметр должен быть от 0 до 50 мм (получено %g)", ErrInvalid, p.DiameterMM)
	}
	if p.LengthMM <= 0 || p.LengthMM > 400 {
		return fmt.Errorf("%w: длина должна быть от 0 до 400 мм (получено %g)", ErrInvalid, p.LengthMM)
	}
	base := manufacturing.FastenerBase(p.Base)
	if !base.Valid() {
		return fmt.Errorf("%w: неизвестный материал %q (concrete, wood, metal или пусто)", ErrInvalid, p.Base)
	}
	// Нулевая цена запрещена по той же причине, что и у остального прайса
	// (DOM-007): опечатка в поле дала бы «бесплатный крепёж» с ответом 200.
	if p.PricePerPcRub <= 0 {
		return fmt.Errorf("%w: цена крепежа должна быть положительной (получено %d)", ErrInvalid, p.PricePerPcRub)
	}
	return nil
}
