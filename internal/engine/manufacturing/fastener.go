package manufacturing

import (
	"fmt"
	"strings"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	kerngeo "stairplatform/internal/geometry"
)

// КРЕПЁЖ В ПРОИЗВОДСТВЕ (MNT-0001).
//
// Правило простое и единственное: одно отверстие в пятке — один комплект
// крепежа. Отверстия уже просверлены в геометрии (geometry/comb_heels.go) и
// приезжают в деталь числом (Part.HoleCount), поэтому смета не может
// разойтись с моделью: сколько отверстий вырезано лазером, столько же
// штук крепежа и попадёт в смету.
//
// Что за крепёж на каком узле (решение владельца 2026-10-08):
//
//	пятка 1 (под ступенью), пятка 2 и пластина 1-й ступени (под подступенком)
//	  → глухарь 6х35, по одному на отверстие;
//	пятка пола и пятка выхода на перекрытие
//	  → глухарь 10х60, по одному на отверстие;
//	  → плюс дюбель 10х60 на то же отверстие, ЕСЛИ основание бетонное.
//	    В дерево дюбель не нужен, и в смете его не будет.
//
// Диаметр 6 мм для обоймы выреза не случаен: отверстия пятки 1 и пятки 2
// Ø10 (CombFrame.HoleDiameter), и глухарь 6 мм в них садится с зазором под
// головку.

// extraFastener — дополнительный крепёж по объекту: код позиции каталога и
// количество. Хранится в конфигурации лестницы и добавляется в смету сверх
// автоматического крепежа по отверстиям.
type extraFastener struct {
	Code     string
	Quantity dommfg.Quantity
}

// FastenerDemandFor считает потребность в крепеже по набору твёрдых тел.
//
// registry — каталог позиций (для названий и вида); nil берётся встроенный.
// unknownCode — сюда попадают коды доп. крепежа, которых нет в каталоге:
// они не роняют расчёт (справочник редактируется в админке), а просто не
// попадают в смету, и вызывающий может показать предупреждение.
func FastenerDemandFor(cfg *engineering.StairConfiguration, solids []*kerngeo.Solid,
	reg *dommfg.FastenerRegistry, unknownCode func(string)) ([]dommfg.FastenerDemand, error) {
	if cfg == nil {
		return nil, fmt.Errorf("manufacturing: configuration is required")
	}
	if reg == nil {
		reg = dommfg.DefaultFastenerRegistry()
	}

	// Отверстия считаются ПО РОЛИ ТЕЛА, а не по типу детали.
	//
	// Это принципиально. У детали в спецификации сохраняются только
	// толщина и два габарита в плоскости, поэтому пятка пола (пластина
	// лежит плашмя: 270×50×4) и пятка выхода на перекрытие (пластина стоит
	// вертикально: 150×50×4) в спецификации НЕРАЗЛИЧИМЫ — обе идут как
	// ANC-nn, и ориентация пластины при переходе «тело → деталь» теряется.
	// Роль тела различает их однозначно и не даёт перепутать, где пол, а где
	// потолок: ошибиться тут — значит положить в смету дюбели туда, где
	// крепят в дерево.
	var treadRiser, floor, ceiling int
	for _, s := range solids {
		if s == nil {
			continue
		}
		n := s.Holes()
		if n <= 0 {
			continue
		}
		switch s.Role() {
		case "heel", "web", "first_web":
			// Пятка 1 держит ступень на посадке гребёнки, пятка 2 и
			// пластина 1-й ступени — подступенок.
			treadRiser += n
		case "anchor":
			floor += n
		case "top_anchor":
			ceiling += n
		}
	}

	acc := newDemandAccumulator(reg)
	// Пятки подступенка и ступени.
	acc.add(dommfg.ScrewTreadRiserCode, dommfg.Quantity(treadRiser), dommfg.PurposeTreadRiser)
	// Крепление в пол и в перекрытие: глухарь на каждое отверстие плюс
	// дюбель, если основание бетонное.
	acc.addAnchoring(dommfg.ScrewAnchorCode, cfg.FloorFixing.OrDefault(), floor, dommfg.PurposeFloor)
	acc.addAnchoring(dommfg.ScrewAnchorCode, cfg.CeilingFixing.OrDefault(), ceiling, dommfg.PurposeCeiling)

	// Дополнительный крепёж по объекту.
	for _, ex := range parseExtraFasteners(cfg.ExtraFasteners) {
		if _, ok := reg.Find(ex.Code); !ok && unknownCode != nil {
			unknownCode(ex.Code)
			continue
		}
		acc.add(ex.Code, ex.Quantity, dommfg.PurposeObject)
	}
	return acc.demands(), nil
}

// parseExtraFasteners разбирает список доп. крепежа из конфигурации.
//
// Формат элемента — либо код позиции («SCREW-8x80»), либо код и количество
// через «@» («SCREW-8x80@12»). Множитель нужен потому, что «двенадцать
// глухарей» и «один глухарь» — разные позиции сметы.
//
// Разделитель количества — именно «@», а не «x» или «*». Раньше стоял
// LastIndexAny(s, "x*"), и на коде «PLUG-8x80» он находил «x» ВНУТРИ размера:
// получалось code=«PLUG-8», qty=80. Дюбель 8х80 уезжал из сметы, а вместо
// него в сумму попадал мусор — и всё это тихо, потому что несуществующий
// код пропускается без ошибки.
func parseExtraFasteners(raw []string) []extraFastener {
	out := make([]extraFastener, 0, len(raw))
	for _, s := range raw {
		if s == "" {
			continue
		}
		code, qty := s, 1
		if i := strings.LastIndex(s, "@"); i > 0 {
			if n := atoi(s[i+1:]); n > 0 {
				code, qty = s[:i], n
			}
		}
		if code == "" {
			continue
		}
		out = append(out, extraFastener{Code: code, Quantity: dommfg.Quantity(qty)})
	}
	return out
}

// atoi разбирает неотрицательное целое; мусор на конце даёт 0.
func atoi(s string) int {
	n := 0
	if s == "" {
		return 0
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
		if n > 1_000_000 {
			return 0
		}
	}
	return n
}

// demandAccumulator собирает потребность, склеивая одинаковые коды.
type demandAccumulator struct {
	reg   *dommfg.FastenerRegistry
	order []string
	byKey map[string]*dommfg.FastenerDemand
}

func newDemandAccumulator(reg *dommfg.FastenerRegistry) *demandAccumulator {
	return &demandAccumulator{reg: reg, byKey: map[string]*dommfg.FastenerDemand{}}
}

// add добавляет количество по коду. Назначения склеиваются через «+», чтобы
// в смете было видно, что глухарь 10х60 нужен и на пол, и на перекрытие.
func (a *demandAccumulator) add(code string, qty dommfg.Quantity, purpose string) {
	if qty <= 0 {
		return
	}
	key := code + "|" + purpose
	if d, ok := a.byKey[key]; ok {
		d.Quantity += qty
		return
	}
	f, ok := a.reg.Find(code)
	if !ok {
		// Код вне каталога: заводить позицию без названия нельзя, но и
		// терять объект нельзя. Пропускаем — количество не попадёт в цену.
		return
	}
	a.byKey[key] = &dommfg.FastenerDemand{
		Code: code, Name: f.NameRu, Kind: f.Kind, Quantity: qty, Purpose: purpose,
	}
	a.order = append(a.order, key)
}

// addAnchoring добавляет глухарь и, при бетонном основании, дюбель.
func (a *demandAccumulator) addAnchoring(screwCode string, base engineering.FixingBase,
	holes int, purpose string) {
	if holes <= 0 {
		return
	}
	a.add(screwCode, dommfg.Quantity(holes), purpose)
	if base.NeedsPlug() {
		a.add(plugCodeFor(screwCode), dommfg.Quantity(holes), purpose)
	}
}

// plugCodeFor подбирает дюбель к глухарю: тот же диаметр, та же длина.
func plugCodeFor(screwCode string) string {
	// SCREW-10x60 → PLUG-10x60. Если позиции с таким кодом в каталоге нет
	// (глухарь добавили в админке без пары), add() её молча пропустит —
	// дюбель в смете не появится, но и расчёт не упадёт.
	if len(screwCode) > len("SCREW-") && screwCode[:len("SCREW-")] == "SCREW-" {
		return "PLUG-" + screwCode[len("SCREW-"):]
	}
	return "PLUG-10x60"
}

// demands возвращает накопленную потребность в стабильном порядке.
func (a *demandAccumulator) demands() []dommfg.FastenerDemand {
	out := make([]dommfg.FastenerDemand, 0, len(a.order))
	for _, key := range a.order {
		out = append(out, *a.byKey[key])
	}
	return out
}
