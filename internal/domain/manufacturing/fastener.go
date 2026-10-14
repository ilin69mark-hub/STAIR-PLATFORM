package manufacturing

import (
	"fmt"
	"sort"
	"strings"
)

// КРЕПЁЖ (MNT-0001) — дюбели и глухари.
//
// Зачем отдельная сущность, а не «ещё один материал». Крепёж не бывает
// «за кг»: его покупают поштучно, и в смете он должен лежать отдельной
// строкой с количеством в штуках, а не прятаться в килограммах металла.
// Поэтому здесь своя единица (штука), своя цена (₽/шт) и свой счётчик
// FastenerCount, который в ManufacturingCostDataset был заведён заранее и
// до сих пор оставался нулём.
//
// Количество берётся из ГЕОМЕТРИИ: отверстия в пятках уже просверлены
// (см. geometry/comb_heels.go), и их число едет вместе с телом. Одно
// отверстие — один комплект крепежа.

// FastenerKind — вид крепежа.
type FastenerKind string

const (
	// FastenerScrew — глухарь (саморез по дереву/металлу, без дюбеля).
	FastenerScrew FastenerKind = "screw"
	// FastenerPlug — дюбель для бетона.
	FastenerPlug FastenerKind = "plug"
)

// IsValid проверяет вид крепежа.
func (k FastenerKind) IsValid() bool {
	switch k {
	case FastenerScrew, FastenerPlug:
		return true
	}
	return false
}

// FastenerBase — материал, в который крепёж вворачивают по умолчанию.
//
// Это НЕ engineering.FixingBase, и путать их нельзя. FixingBase описывает, к
// чему крепится САМА лестница (бетон/дерево), и его пустое значение означает
// бетон. Здесь пустое значение означает «не задан», и трактовать его как бетон
// нельзя: позиция без основания получила бы требование дюбеля, которого
// никто не заказывал. Такая подмена нужна была, чтобы втиснуть в каталог
// крепёж ограждения — саморез 5х32 в стойку из нержавейки (S-181).
type FastenerBase string

const (
	// FastenerBaseConcrete — бетон: глухарь вворачивается через дюбель.
	FastenerBaseConcrete FastenerBase = "concrete"
	// FastenerBaseWood — дерево: глухарь вворачивается прямо.
	FastenerBaseWood FastenerBase = "wood"
	// FastenerBaseMetal — металл: стойка ограждения из нержавейки, шпилька.
	FastenerBaseMetal FastenerBase = "metal"
)

// Valid — допустимо ли значение основания крепежа. Пустое допустимо: у
// позиции может не быть основания по умолчанию (например, у доп. крепежа,
// который владелец задаёт объекту).
func (b FastenerBase) Valid() bool {
	switch b {
	case FastenerBaseConcrete, FastenerBaseWood, FastenerBaseMetal, "":
		return true
	}
	return false
}

// NeedsPlug — нужен ли к этому глухарю дюбель. Основание по умолчанию: в
// бетон — да, в дерево и металл — нет. Незаданное основание дюбеля не
// требует (см. FastenerBase).
func (b FastenerBase) NeedsPlug() bool { return b == FastenerBaseConcrete }

// Fastener — позиция каталога крепежа.
type Fastener struct {
	// Code — ключ позиции («SCREW-6x35», «PLUG-10x60»). В смете и в
	// справочнике магазина он же: по нему меняется цена.
	Code string
	// NameRu — название для интерфейса и сметы («Глухарь 6х35»).
	NameRu string
	Kind   FastenerKind
	// DiameterMM / LengthMM — размер, мм. Хранятся отдельно, чтобы
	// админка могла фильтровать и сортировать каталог по размеру.
	DiameterMM float64
	LengthMM   float64
	// Base — материал, в который позицию вворачивают по умолчанию.
	Base FastenerBase
}

// Validate проверяет позицию каталога.
func (f *Fastener) Validate() error {
	if f == nil {
		return fmt.Errorf("manufacturing: fastener is required")
	}
	if strings.TrimSpace(f.Code) == "" {
		return fmt.Errorf("manufacturing: fastener code is required")
	}
	if !f.Kind.IsValid() {
		return fmt.Errorf("manufacturing: fastener %q has unknown kind %q", f.Code, f.Kind)
	}
	if f.DiameterMM <= 0 || f.LengthMM <= 0 {
		return fmt.Errorf("manufacturing: fastener %q has non-positive size", f.Code)
	}
	if !f.Base.Valid() {
		return fmt.Errorf("manufacturing: fastener %q has unknown base %q", f.Code, f.Base)
	}
	return nil
}

// FastenerRegistry — каталог крепежа. Только чтение.
type FastenerRegistry struct {
	items []Fastener
	index map[string]int
}

// NewFastenerRegistry собирает каталог из позиций.
func NewFastenerRegistry(items ...Fastener) (*FastenerRegistry, error) {
	r := &FastenerRegistry{items: make([]Fastener, 0, len(items)), index: make(map[string]int, len(items))}
	for i := range items {
		if err := items[i].Validate(); err != nil {
			return nil, err
		}
		if _, dup := r.index[items[i].Code]; dup {
			return nil, fmt.Errorf("manufacturing: duplicate fastener code %q", items[i].Code)
		}
		r.index[items[i].Code] = len(r.items)
		r.items = append(r.items, items[i])
	}
	return r, nil
}

// Find возвращает позицию каталога по коду.
func (r *FastenerRegistry) Find(code string) (Fastener, bool) {
	if r == nil || code == "" {
		return Fastener{}, false
	}
	i, ok := r.index[code]
	if !ok {
		return Fastener{}, false
	}
	return r.items[i], true
}

// Fasteners возвращает все позиции каталога (в порядке добавления).
func (r *FastenerRegistry) Fasteners() []Fastener {
	if r == nil {
		return nil
	}
	return r.items
}

// Codes возвращает отсортированные коды — для стабильного показа в
// интерфейсе и в выгрузке сметы.
func (r *FastenerRegistry) Codes() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.items))
	for _, f := range r.items {
		out = append(out, f.Code)
	}
	sort.Strings(out)
	return out
}

// Коды крепежа по назначению. Они же — значения по умолчанию, которые
// владелец зафиксировал решением от 2026-10-08.
const (
	// ScrewTreadRiserCode — глухарь 6х35 на пятки подступенка и ступеней.
	// Диаметр 6 мм подобран под отверстия Ø10 пятки 1/2 обоймы выреза
	// (CombFrame.HoleDiameter) с зазором на головку.
	ScrewTreadRiserCode = "SCREW-6x35"
	// ScrewAnchorCode — глухарь 10х60 в пятки пола и выхода на перекрытие.
	ScrewAnchorCode = "SCREW-10x60"
	// PlugAnchorCode — дюбель для бетона 10х60 к нему же. В дерево
	// дюбель не нужен, поэтому в смете появляется только при креплении
	// в бетон.
	PlugAnchorCode = "PLUG-10x60"
	// ScrewRailPostCode — саморез 5х32 крепления ограждения: 2 штуки на
	// стойку (решение владельца 2026-10-13). В стойку Ø42,4 из
	// нержавейки дюбель не нужен — это металл, а не бетон, поэтому позиция
	// заведена отдельной строкой, а не рядом 8/10 × 50…150.
	ScrewRailPostCode = "SCREW-5x32"
)

// ScrewRailPostQtyPerPost — сколько саморезов крепления ограждения уходит
// на одну стойку. Решение владельца 2026-10-13: 2 штуки.
//
// В коде геометрии это же число отдаётся как RailScrewsPerPost, чтобы
// правило крепления и правило построения стойки жили в одном месте.
const ScrewRailPostQtyPerPost = 2

// defaultFastenerLengthsMM — ряд длин дюбелей и глухарей, который владелец
// попросил держать в каталоге (решение 2026-10-08). Ряд неполный по
// сравнению с тем, что бывает в магазине, и дополняется прямо в админке:
// каталог редактируемый, этот список — только стартовый набор.
var defaultFastenerLengthsMM = []float64{50, 60, 70, 80, 90, 100, 120, 130, 140, 150}

// defaultFastenerDiametersMM — диаметры стержня: 6 мм на пятки обоймы
// выреза, 8 и 10 мм на крепление к полу и перекрытию.
var defaultFastenerDiametersMM = []float64{8, 10}

// DefaultFastenerRegistry — встроенный каталог крепежа.
//
// Цены здесь НЕ хранятся: цена позиции — настройка магазина (прайс), как
// и цена материала за кг. Встроенная цена появляется только как запасной
// вариант в движке ценообразования (DefaultFastenerPrice), чтобы расчёт не
// падал на пустом справочнике.
func DefaultFastenerRegistry() *FastenerRegistry {
	//nolint:prealloc // список задаётся литералом и нигде не дополняется;
	// переписывать его в make+append ради одной строки — ухудшение читаемости.
	items := []Fastener{
		{
			Code: ScrewTreadRiserCode, NameRu: "Глухарь 6х35", Kind: FastenerScrew,
			DiameterMM: 6, LengthMM: 35, Base: FastenerBaseWood,
		},
		{
			// Крепление ограждения. Длина 32 мм подобрана под стойку
			// Ø42,4/1,5: длиннее — вылезет наружу через стенку трубы.
			Code: ScrewRailPostCode, NameRu: "Саморез 5х32 (ограждение)", Kind: FastenerScrew,
			DiameterMM: 5, LengthMM: 32, Base: FastenerBaseMetal,
		},
	}
	// Ряд 8/10 × 50…150: дюбели для бетона и глухари к ним.
	//
	// Глухарь 10х60 и дюбель 10х60 (крепление пола и перекрытия) НЕ
	// заводятся отдельными строками: они получаются из этого же ряда. Раньше
	// они были в списке вместе с рядом, коды совпадали, и NewFastenerRegistry
	// возвращал ошибку «duplicate fastener code» — а DefaultFastenerRegistry
	// на неё подменялся ПУСТЫМ каталогом. Потребность в крепеже при этом
	// молча становилась нулевой, и смета показывала ноль штук, хотя отверстия
	// в пятках были просверлены.
	for _, d := range defaultFastenerDiametersMM {
		for _, l := range defaultFastenerLengthsMM {
			code := fastenerCodeFor(d, l)
			size := formatSize(d, l)
			items = append(items,
				Fastener{
					Code: "PLUG-" + code, NameRu: "Дюбель для бетона " + size,
					Kind: FastenerPlug, DiameterMM: d, LengthMM: l, Base: FastenerBaseConcrete,
				},
				Fastener{
					Code: "SCREW-" + code, NameRu: "Глухарь " + size,
					Kind: FastenerScrew, DiameterMM: d, LengthMM: l, Base: FastenerBaseConcrete,
				},
			)
		}
	}
	// Реестр собран из инвариантных констант, поэтому ошибка здесь означает
	// потерю инварианта в коде (например, повтор кода в списке выше).
	// Подменить её нечем — возвращаем каталог с одной гарантированной
	// позицией, чтобы расчёт не падал, и оставляем след: без этого такой
	// баг выглядел бы как «крепёж не нужен».
	reg, err := NewFastenerRegistry(items...)
	if err != nil {
		fallback, ferr := NewFastenerRegistry(items[0])
		if ferr != nil {
			return &FastenerRegistry{index: map[string]int{}}
		}
		return fallback
	}
	return reg
}

// fastenerCodeFor — код позиции по размеру: 10, 60 → «10x60».
func fastenerCodeFor(d, l float64) string {
	return fmt.Sprintf("%gx%g", d, l)
}

// formatSize — размер для названия: 10, 60 → «10х60» (кириллическая «х»,
// как в прайсах на стройбазах).
func formatSize(d, l float64) string {
	return trimFloat(d) + "х" + trimFloat(l)
}

// trimFloat печатает число без служебных нулей: 60 → «60», 62.5 → «62,5».
//
// Раньше здесь было `fmt.Sprintf("%.1f")` с обрезанием только хвоста «0», и
// 10.0 превращалось в «10.» — в смете позиция называлась «Глухарь 10.х60.».
// Обрезается и точка: она тоже служебная.
func trimFloat(v float64) string {
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".")
	if s == "" {
		return "0"
	}
	return strings.Replace(s, ".", ",", 1)
}

// FastenerDemand — потребность в крепеже по назначению: сколько чего и
// зачем. Одна строка = один код крепежа на узел (или на объект целиком,
// если Purpose задан как FixingPurposeObject).
type FastenerDemand struct {
	Code     string // код каталога
	Name     string // название для сметы
	Kind     FastenerKind
	Quantity Quantity // штуки
	// Purpose — назначение («пятки подступенка и ступени», «крепление в
	// пол», …). Показывается в смете, чтобы видеть, за что платим.
	Purpose string
}

// Назначения крепежа — показываются в смете и в карточке расчёта.
const (
	PurposeTreadRiser = "пятки подступенка и ступени"
	PurposeFloor      = "крепление в пол"
	PurposeCeiling    = "крепление в перекрытие"
	PurposeObject     = "по объекту"
	// PurposeRailFlange — анкерные отверстия фланцев опоры ограждения.
	// Отдельное назначение, хотя код тот же, что у пяток подступенка:
	// глухарь 6х35 в пятке ступени и во фланце — разные работы, и в смете
	// должно быть видно, за что платим.
	PurposeRailFlange = "анкерные отверстия фланцев ограждения"
	// PurposeRailPost — крепление поручня к стойкам ограждения.
	PurposeRailPost = "крепление поручня к стойкам"
)
