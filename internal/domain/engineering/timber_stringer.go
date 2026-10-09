package engineering

import "strconv"

// Норматив на деревянный косоур-балку (решение владельца 2026-10-05).
//
// Таблица норматива:
//
//	Длина марша по проекции | Ширина марша | Мин. толщина | Заготовка (ширина/высота) | Мин. глубина тела
//	до 2.0 м                | до 0.9 м     | 40 мм        | 250 мм                   | не менее 120 мм
//	2.0 – 3.0 м             | до 1.0 м     | 50 мм        | 250 – 300 мм             | не менее 130 – 150 мм
//	3.0 – 4.0 м             | 1.0 – 1.2 м  | 60 мм        | 300 мм                   | не менее 150 – 160 мм
//	более 4.0 м             | более 1.2 м  | 70 и более   | 350 мм                   | требуется промежуточная опора
//
// Уточнения владельца к таблице:
//
//   - последняя полоса (> 4.0 м) — толщина 80 мм, а не «70 и более»;
//   - при ширине марша более 1200 мм толщина 100 мм в любой полосе;
//   - промежуточная опора для марша длиннее 4 м — ВНЕДРЕНА ПОЗЖЕ, поэтому
//     здесь она только помечена, но НЕ блокирует расчёт.
//
// ТАБЛИЦА ПРИМЕНЯЕТСЯ ТОЛЬКО К ДЕРЕВЯННОМУ КОСОУРУ. У листового металлокаркаса
// своя толщина (3–8 мм, лазерный раскрой) и никакой балки нет.
//
// Длина марша — именно ПО ПРОЕКЦИИ (горизонтальная проекция, Run), а не длина
// по склону: так она указана в нормативе и так её видит пользователь в плане.

// TimberStringerBand — одна нормативная полоса деревянного косоура.
type TimberStringerBand struct {
	// RunMaxMM — верхняя граница длины марша по проекции. Ноль означает
	// «последняя полоса, верхней границы нет».
	RunMaxMM float64
	// RunLabel — подпись полосы для текста подсказки: «до 2.0 м».
	RunLabel string
	// WidthLabel — типичная ширина марша для этой полосы («до 0.9 м»).
	WidthLabel string
	// MinThicknessMM — минимальная толщина косоура в полосе.
	MinThicknessMM float64
	// BlankMinMM / BlankMaxMM — полная ширина/высота заготовки-балки.
	// BlankMaxMM = 0, если верхней границы нет.
	BlankMinMM float64
	BlankMaxMM float64
	// MinBodyMM — минимальная глубина «тела» балки после выпила гребёнки.
	// Ноль означает, что вместо числа требуется опора (NeedsSupport).
	MinBodyMM float64
	// NeedsSupport — требуется промежуточная опора (столб). Помечается, но не
	// блокирует: опоры владелец вводит позже (см. комментарий выше).
	NeedsSupport bool
}

// TimberStringerBands — нормативные полосы по возрастанию длины марша.
// Порядок значим: полоса выбирается первым, у которой RunMaxMM не меньше
// длины марша, поэтому диапазоны обязаны идти по возрастанию и без дыр.
var TimberStringerBands = []TimberStringerBand{
	{
		RunMaxMM: 2000, RunLabel: "до 2.0 м", WidthLabel: "до 0.9 м",
		MinThicknessMM: 40,
		BlankMinMM:     250,
		MinBodyMM:      120,
	},
	{
		RunMaxMM: 3000, RunLabel: "2.0 – 3.0 м", WidthLabel: "до 1.0 м",
		MinThicknessMM: 50,
		BlankMinMM:     250, BlankMaxMM: 300,
		MinBodyMM: 130,
	},
	{
		RunMaxMM: 4000, RunLabel: "3.0 – 4.0 м", WidthLabel: "1.0 – 1.2 м",
		MinThicknessMM: 60,
		BlankMinMM:     300,
		MinBodyMM:      150,
	},
	{
		RunMaxMM: 0, RunLabel: "более 4.0 м", WidthLabel: "более 1.2 м",
		// Уточнение владельца: 80 мм, а не «70 и более».
		MinThicknessMM: 80,
		BlankMinMM:     350,
		NeedsSupport:   true,
	},
}

// Ширина марша, выше которой толщина косоура поднимается до 100 мм в любой
// полосе (уточнение владельца 2026-10-05).
const (
	TimberWideWidthMM = 1200
	TimberWideThickMM = 100
	TimberMaxThickMM  = 100
)

// TimberStringerBandFor возвращает нормативную полосу для марша длиной runMM
// по проекции. При runMM ≤ 0 берётся первая полоса (самый короткий марш).
func TimberStringerBandFor(runMM float64) TimberStringerBand {
	for _, b := range TimberStringerBands {
		if b.RunMaxMM <= 0 || runMM <= b.RunMaxMM {
			return b
		}
	}
	last := TimberStringerBands[len(TimberStringerBands)-1]
	return last
}

// MinTimberStringerThicknessMM — минимальная толщина деревянного косоура для
// марша длиной runMM по проекции и шириной widthMM.
//
// Ширина марша влияет на толщину поверх полосы: при ширине более 1200 мм
// требуется 100 мм независимо от длины. Иначе берётся минимум полосы.
func MinTimberStringerThicknessMM(runMM, widthMM float64) float64 {
	if widthMM > TimberWideWidthMM {
		return TimberWideThickMM
	}
	return TimberStringerBandFor(runMM).MinThicknessMM
}

// TimberStringerSpec — сводное требование к деревянному косоуру: полоса
// норматива плюс действующий минимум толщины с учётом ширины марша.
type TimberStringerSpec struct {
	Band             TimberStringerBand
	RunMM            float64
	WidthMM          float64
	MinThicknessMM   float64
	ThicknessByWidth bool
}

// TimberStringerSpecFor собирает требование по длине и ширине марша.
func TimberStringerSpecFor(runMM, widthMM float64) TimberStringerSpec {
	band := TimberStringerBandFor(runMM)
	min := band.MinThicknessMM
	byWidth := false
	if widthMM > TimberWideWidthMM {
		min = TimberWideThickMM
		byWidth = true
	}
	return TimberStringerSpec{
		Band:             band,
		RunMM:            runMM,
		WidthMM:          widthMM,
		MinThicknessMM:   min,
		ThicknessByWidth: byWidth,
	}
}

// BlankLabel — заготовка полосы словами: «250 мм», «250–300 мм».
func (b TimberStringerBand) BlankLabel() string {
	if b.BlankMaxMM <= 0 {
		return formatMM(b.BlankMinMM)
	}
	return formatMM(b.BlankMinMM) + "–" + formatMM(b.BlankMaxMM)
}

// BodyLabel — глубина тела словами: «не менее 120 мм» либо «требуется
// промежуточная опора», когда числа в нормативе нет.
func (b TimberStringerBand) BodyLabel() string {
	if b.NeedsSupport {
		return "требуется промежуточная опора (столб)"
	}
	return "не менее " + formatMM(b.MinBodyMM) + " мм"
}

// formatMM — число миллиметров без дробной части: «250».
func formatMM(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
