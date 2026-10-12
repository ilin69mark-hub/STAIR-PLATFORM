package manufacturing

import (
	"fmt"
	"math"
)

// ГЕРМЕТИК (CONF-SEALANT).
//
// Расход считается по формуле владельца: V = S × t, где S — площадь в м²,
// t — толщина слоя в мм. При плотности силикона 1,04 г/см³ слой 1 мм на
// 1 м² даёт ≈1,05 кг, то есть ≈1 литр — ровно тот результат, который нужен
// для перевода объёма в картриджи.
//
// Единица закупки — КАРТРИДЖ 310 мл, а не литр: герметик не покупают
// канистрами по 300 мл, а «0,64 литра» в смете нельзя ни купить, ни
// привезти. Поэтому литры — это физика, а картриджи — это то, что попадает в
// спецификацию и в цену.

// SealantKind — вид герметика.
type SealantKind string

const (
	// SealantSilicone — силиконовый герметик (владелец, 2026-10-08).
	SealantSilicone SealantKind = "silicone"
	// SealantPU — полиуретановый. Заведён рядом, потому что на объекте
	// выбирают его чаще силикона, а каталог позиций редактируется владельцем.
	SealantPU SealantKind = "pu"
)

// IsValid проверяет вид герметика.
func (k SealantKind) IsValid() bool {
	switch k {
	case SealantSilicone, SealantPU:
		return true
	}
	return false
}

// Sealant — позиция каталога герметика.
type Sealant struct {
	Code   string
	NameRu string
	Kind   SealantKind
	// VolumeML — объём картриджа (мл). 310 — стандартный силиконовый.
	VolumeML float64
	// DensityKgPerL — плотность, кг/л. Силикон 1,04 (владелец, 2026-10-08).
	DensityKgPerL float64
	// DefaultLayerMM — толщина слоя по умолчанию для этой позиции, мм.
	DefaultLayerMM float64
}

// Validate проверяет позицию каталога.
func (s Sealant) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("manufacturing: sealant code is required")
	}
	if !s.Kind.IsValid() {
		return fmt.Errorf("manufacturing: sealant %q has unknown kind %q", s.Code, s.Kind)
	}
	if s.VolumeML <= 0 {
		return fmt.Errorf("manufacturing: sealant %q has non-positive volume", s.Code)
	}
	if s.DensityKgPerL <= 0 {
		return fmt.Errorf("manufacturing: sealant %q has non-positive density", s.Code)
	}
	if s.DefaultLayerMM <= 0 {
		return fmt.Errorf("manufacturing: sealant %q has non-positive layer", s.Code)
	}
	return nil
}

// SealantRegistry — каталог герметика. Только чтение.
type SealantRegistry struct {
	items []Sealant
	index map[string]int
}

// NewSealantRegistry собирает каталог.
func NewSealantRegistry(items ...Sealant) (*SealantRegistry, error) {
	r := &SealantRegistry{items: make([]Sealant, 0, len(items)), index: make(map[string]int, len(items))}
	for i := range items {
		if err := items[i].Validate(); err != nil {
			return nil, err
		}
		if _, dup := r.index[items[i].Code]; dup {
			return nil, fmt.Errorf("manufacturing: duplicate sealant code %q", items[i].Code)
		}
		r.index[items[i].Code] = len(r.items)
		r.items = append(r.items, items[i])
	}
	return r, nil
}

// Find возвращает позицию каталога по коду.
func (r *SealantRegistry) Find(code string) (Sealant, bool) {
	if r == nil || code == "" {
		return Sealant{}, false
	}
	i, ok := r.index[code]
	if !ok {
		return Sealant{}, false
	}
	return r.items[i], true
}

// Sealants возвращает все позиции каталога.
func (r *SealantRegistry) Sealants() []Sealant {
	if r == nil {
		return nil
	}
	return r.items
}

// Коды позиций каталога герметика. Расход по умолчанию считается по
// DefaultSealantCode; остальные позиции нужны для прайса — владелец вносит
// реальные, которые использует на объектах, и цена каждой правится в
// админке.
//
// Объём вынесен в код позиции (310/600), а не в цену: картридж 600 мл — та
// же единица закупки «штука», но вдвое дороже, и прайс обязан это отражать.
const (
	DefaultSealantCode = "SEALANT-SILICONE-310" // расход по умолчанию
	SealantSilicone600 = "SEALANT-SILICONE-600"
	SealantPU310       = "SEALANT-PU-310"
	SealantPU600       = "SEALANT-PU-600"
)

// CartridgeVolumeML — объём стандартного картриджа, мл.
const CartridgeVolumeML = 310.0

// DefaultSealantLayerMM — толщина слоя по умолчанию, мм. Владелец указал
// 1 мм как базовый слой; для шва между листовыми деталями этого достаточно,
// а «на глаз больше» владелец добавляет доп. картриджами в админке.
const DefaultSealantLayerMM = 1.0

// SealantReservePercent — запас герметика на брак и обрезку, % (владелец,
// 2026-10-08: «запас по силикону +15% всегда»).
//
// Запас ВСЕГДА применяется к автоматическому расходу и никогда — к
// добавленному по объекту: доп. картриджи человек ставит руками под конкретный
// шов, и 15% поверх них были бы враньём в смете.
//
// Применяется ДО перевода в картриджи, а не после: округление вверх до целого
// картриджа само по себе даёт запас, и накидывать 15% поверх уже округлённого
// числа значит платить за второй, никому не нужный запас.
const SealantReservePercent = 15.0

// SiliconeDensityKgPerL — плотность силиконового герметика, кг/л.
// 1,04 г/см³ = 1,04 кг/л (владелец, 2026-10-08).
const SiliconeDensityKgPerL = 1.04

// DefaultSealantRegistry — встроенный каталог герметика.
//
// Позиции придут сюда по мере того, как владелец вносит реальные, которые
// использует на объектах: смена каталога — правка этого списка, без миграции
// и без правки расчёта.
func DefaultSealantRegistry() *SealantRegistry {
	// Плотности: силикон 1,04 кг/л (владелец, 2026-10-08), полиуретан
	// 1,20 кг/л. Плотность входит только в расчёт массы для справки —
	// закупается герметик в картриджах, и на цену она не влияет.
	reg, err := NewSealantRegistry(
		Sealant{
			Code: DefaultSealantCode, NameRu: "Герметик силиконовый 310 мл",
			Kind: SealantSilicone, VolumeML: CartridgeVolumeML,
			DensityKgPerL: SiliconeDensityKgPerL, DefaultLayerMM: DefaultSealantLayerMM,
		},
		Sealant{
			Code: SealantSilicone600, NameRu: "Герметик силиконовый 600 мл",
			Kind: SealantSilicone, VolumeML: 600,
			DensityKgPerL: SiliconeDensityKgPerL, DefaultLayerMM: DefaultSealantLayerMM,
		},
		Sealant{
			Code: SealantPU310, NameRu: "Герметик полиуретановый 310 мл",
			Kind: SealantPU, VolumeML: CartridgeVolumeML,
			DensityKgPerL: 1.20, DefaultLayerMM: DefaultSealantLayerMM,
		},
		Sealant{
			Code: SealantPU600, NameRu: "Герметик полиуретановый 600 мл",
			Kind: SealantPU, VolumeML: 600,
			DensityKgPerL: 1.20, DefaultLayerMM: DefaultSealantLayerMM,
		},
	)
	if err != nil {
		return &SealantRegistry{index: map[string]int{}}
	}
	return reg
}

// SealantDemand — потребность в герметике: сколько картриджей и почему.
type SealantDemand struct {
	Code string
	Name string
	Kind SealantKind
	// AreaM2 — площадь герметизируемых стыков, м².
	AreaM2 float64
	// LayerMM — толщина слоя, мм.
	LayerMM float64
	// VolumeL — объём по формуле V = S × t, БЕЗ запаса, литры.
	VolumeL float64
	// ReservePercent — запас на брак и обрезку, %.
	ReservePercent float64
	// VolumeWithReserveL — объём с запасом, литры. По нему считаются
	// картриджи.
	VolumeWithReserveL float64
	// MassKg — масса по плотности позиции, кг.
	MassKg float64
	// Cartridges — картриджей с ОКРУГЛЕНИЕМ ВВЕРХ: купить 2,08 картриджа
	// нельзя, а купить 2 значит не хватит. Раньше округлялось вниз, и на
	// марше 15 ступеней не хватало целого картриджа.
	Cartridges int
	// AutoCartridges — картриджи по геометрии, с запасом. Именно они идут
	// отдельной строкой сметы.
	AutoCartridges int
	// ExtraCartridges — картриджи, добавленные по объекту. Держатся ОТДЕЛЬНО
	// от автоматических, потому что завтра к ним добавятся перерасход и
	// недорасход, и в смете нужно видеть, сколько лежит на швы, а сколько
	// докуплено руками.
	ExtraCartridges int
	// Purpose — зачем (швы обоймы выреза, торец косоура).
	Purpose string
}

// SealantZones — площади стыков по зонам для показа в смете.
type SealantZones struct {
	// Строки: назначение → площадь м².
	Rows []SealantZoneRow
}

// SealantZoneRow — одна зона герметизации.
type SealantZoneRow struct {
	Purpose string
	AreaM2  float64
}

// SealantDemandFor считает потребность в герметике.
//
// areas — площади стыков из геометрии (geometry.SealantContactAreas).
// layerMM — толщина слоя; 0 → DefaultSealantLayerMM.
// extraCartridges — доп. картриджи по объекту («если нужно больше»): они
// прибавляются к автоматическому расходу, а не заменяют его.
//
// Формула владельца: V = S × t. При S в м² и t в мм объём выходит сразу в
// литрах (1 м² × 1 мм = 1 л), поэтому переводить в кубометры не нужно.
func SealantDemandFor(areas SealantAreaSource, reg *SealantRegistry,
	layerMM float64, extraCartridges int) (*SealantDemand, error) {
	if reg == nil {
		reg = DefaultSealantRegistry()
	}
	s, ok := reg.Find(DefaultSealantCode)
	if !ok {
		return nil, fmt.Errorf("manufacturing: sealant %q not in catalog", DefaultSealantCode)
	}
	if layerMM <= 0 {
		layerMM = DefaultSealantLayerMM
	}
	area := areas.TotalM2()
	if area <= 0 || math.IsNaN(area) || math.IsInf(area, 0) {
		return nil, fmt.Errorf("manufacturing: sealant contact area is invalid (%v m2)", area)
	}
	volume := area * layerMM
	// Запас применяется ДО перевода в картриджи: округление вверх до целого
	// картриджа само по себе даёт запас, и 15% поверх уже округлённого числа
	// означали бы платить за второй, никому не нужный запас.
	withReserve := volume * (1 + SealantReservePercent/100)
	auto := int(math.Ceil(withReserve / (s.VolumeML / 1000)))
	if extraCartridges < 0 {
		extraCartridges = 0
	}
	return &SealantDemand{
		Code: s.Code, Name: s.NameRu, Kind: s.Kind,
		AreaM2: area, LayerMM: layerMM, VolumeL: volume,
		ReservePercent: SealantReservePercent, VolumeWithReserveL: withReserve,
		MassKg:         volume * s.DensityKgPerL,
		Cartridges:     auto + extraCartridges,
		AutoCartridges: auto, ExtraCartridges: extraCartridges,
		Purpose: PurposeSealantJoints,
	}, nil
}

// SealantAreaSource — источник площади стыков для расчёта герметика.
// Интерфейс, а не конкретный тип геометрии: расчёт не должен зависеть от
// пакета геометрии (иначе manufacturing потянет за собой движок).
type SealantAreaSource interface{ TotalM2() float64 }

// SealantZonesFrom — разбивка площади по зонам для детализации сметы.
func SealantZonesFrom(treadHeels, riserWebs, riserFirstWebs, stringerEnds float64) SealantZones {
	mm2ToM2 := 1e-6
	z := SealantZones{}
	add := func(purpose string, mm2 float64) {
		if mm2 <= 0 {
			return
		}
		z.Rows = append(z.Rows, SealantZoneRow{Purpose: purpose, AreaM2: mm2 * mm2ToM2})
	}
	add(PurposeTreadHeel, treadHeels)
	add(PurposeRiserHeel, riserWebs)
	add(PurposeFirstWeb, riserFirstWebs)
	add(PurposeStringerEnd, stringerEnds)
	return z
}

// Назначения герметика — показываются в смете рядом с расходом.
const (
	PurposeTreadHeel     = "ступень к опорной пятке"
	PurposeRiserHeel     = "подступенок к пятке 2"
	PurposeFirstWeb      = "подступенок №1 к пластине 1-й ступени"
	PurposeStringerEnd   = "торец косоура"
	PurposeSealantJoints = "герметизация швов обоймы выреза и торца косоура"
)
