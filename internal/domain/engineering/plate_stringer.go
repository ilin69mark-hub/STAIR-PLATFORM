package engineering

import (
	"strconv"
	"strings"
)

// Норматив на листовой (сварной) косоур металлической лестницы — таблица
// владельца от 2026-10-05:
//
//	Толщина листа | Макс. длина марша по проекции | Допустимая ширина | Мин. глубина тела | Мин. высота заготовки
//	8 мм         | до 2.5 – 3.0 м                  | до 0.9 – 1.0 м   | не менее 120 – 130 | 250
//	10 мм        | 3.0 – 4.0 м                     | до 1.1 – 1.2 м   | не менее 140 – 150 | 300
//	12 мм        | более 4.0 м                     | до 1.3 – 1.4 м   | не менее 160 – 180 | 350
//
// Ключ таблицы — ТОЛЩИНА ЛИСТА (её выбирает пользователь), а длина, ширина,
// глубина тела и высота заготовки — пределы этой толщины. Это обратная к
// нормативу на дерево структура: там ключом был марш, здесь — толщина.
//
// АРИФМЕТИКА ПО ПОЛОСАМ. Владелец записал пределы диапазонами («до 2.5 – 3.0 м»,
// «до 0.9 – 1.0 м»): нижний конец отвечает нагруженному широкому маршу, верхний —
// лёгкому узкому. Для расчёта берётся ВЕРХНИЙ конец, потому что иначе полосы
// разошлись бы дырой: 8 мм покрывает до 2.5 м, а 10 мм начинается с 3.0 м, и
// марш 2.7 м не попал бы ни в одну строку. Сами диапазоны сохранены в
// подписях RunLabel/WidthLabel — калькулятор показывает формулировку владельца
// дословно.
//
// Глубина тела, наоборот, берётся по НИЖНЕМУ концу диапазона (120 / 140 / 160):
// это минимум, на который можно положиться, и сверх него получается запас.
//
// «Высота исходной заготовки» для сварного косоура-гребёнки — это ширина
// заготовки по нормали к линии посадок, то есть CombFrame.StringerSpine.

// PlateStringerBand — одна нормативная строка листового косоура.
type PlateStringerBand struct {
	ThicknessMM float64
	// RunMaxMM — верхняя граница длины марша по проекции. Ноль — без границы.
	RunMaxMM float64
	// RunLabel / WidthLabel — подписи полосы дословно по таблице владельца.
	RunLabel   string
	WidthLabel string
	// WidthMaxMM — предел ширины марша для этой толщины.
	WidthMaxMM float64
	// MinBodyMM — минимальная глубина «тела» балки после выпила гребёнки.
	MinBodyMM float64
	// BodyLabel — подпись диапазона глубины тела («не менее 120–130 мм»).
	BodyLabel string
	// BlankMinMM — минимальная высота исходной заготовки (ширина гребёнки).
	BlankMinMM float64
	// Notes — особенности конструкции из таблицы владельца: показываются в
	// калькуляторе как информация, ничего не блокируют.
	Notes string
}

// PlateStringerBands — нормативные строки по возрастанию толщины листа.
var PlateStringerBands = []PlateStringerBand{
	{
		ThicknessMM: 8,
		RunMaxMM:    3000, RunLabel: "до 2.5 – 3.0 м",
		WidthMaxMM: 1000, WidthLabel: "до 0.9 – 1.0 м",
		MinBodyMM: 120, BodyLabel: "не менее 120–130 мм",
		BlankMinMM: 250,
		Notes:      "Самый популярный вариант для стандартных домашних лестниц.",
	},
	{
		ThicknessMM: 10,
		RunMaxMM:    4000, RunLabel: "3.0 – 4.0 м",
		WidthMaxMM: 1200, WidthLabel: "до 1.1 – 1.2 м",
		MinBodyMM: 140, BodyLabel: "не менее 140–150 мм",
		BlankMinMM: 300,
		Notes: "Повышенная жесткость. Подходит для длинных маршей без промежуточных опор. " +
			"Идеально для открытых лестниц без подступенков.",
	},
	{
		ThicknessMM: 12,
		RunMaxMM:    0, RunLabel: "более 4.0 м",
		WidthMaxMM: 1400, WidthLabel: "до 1.3 – 1.4 м",
		MinBodyMM: 160, BodyLabel: "не менее 160–180 мм",
		BlankMinMM: 350,
		Notes: "Применяется при высоких нагрузках, широких маршах или при сборке мощного " +
			"монокосоура (в виде спаренных пластин с проставками).",
	},
}

// Пределы выпуска листового металла по нормативу владельца. Имена НЕ совпадают
// с функцией MinPlateStringerThicknessMM: константа — «самая тонкая толщина из
// таблицы», функция — «какая толщина нужна под этот марш».
const (
	// PlateMinThicknessMM — самая тонкая толщина листа из таблицы (8 мм).
	PlateMinThicknessMM = 8
	// PlateMaxThicknessMM — самая толстая строка таблицы (12 мм).
	PlateMaxThicknessMM = 12
	// PlateMaxBlankMM — самая высокая заготовка из таблицы (12 мм → 350 мм).
	PlateMaxBlankMM = 350
)

// PlateStringerBandForThickness возвращает нормативную строку по толщине
// листа. Вторая возвращаемая величина — найдена ли строка: толщина 6 мм или 20 мм
// в таблице не значится, и это не ошибка, а отсутствие норматива.
func PlateStringerBandForThickness(thicknessMM float64) (PlateStringerBand, bool) {
	for _, b := range PlateStringerBands {
		if absFloat(thicknessMM-b.ThicknessMM) < 0.1 {
			return b, true
		}
	}
	return PlateStringerBand{}, false
}

// PlateStringerBandForRun возвращает нормативную строку, в которую попадает
// марш длиной runMM по проекции. Нужна для ТЕКСТА подсказки: строка по
// толщине листа и строка по длине марша — разные вещи, и называть марш
// 4050 мм строкой «до 2.5 – 3.0 м» бессмысленно.
func PlateStringerBandForRun(runMM float64) PlateStringerBand {
	best := PlateStringerBands[0]
	for _, b := range PlateStringerBands {
		if b.RunMaxMM <= 0 || runMM <= b.RunMaxMM {
			return b
		}
	}
	return best
}

// MinPlateStringerThicknessMM — минимальная толщина листа для марша длиной
// runMM по проекции и шириной widthMM.
//
// Толщина берётся по ДЛИНЕ марша, а ширина может только поднять её: широкий
// марш на тонком листе нормативом не разрешён. Возвращается толщина из
// таблицы либо PlateMaxThicknessMM для марша за пределами самой толстой строки.
func MinPlateStringerThicknessMM(runMM, widthMM float64) float64 {
	var need float64
	switch {
	case runMM > PlateStringerBands[1].RunMaxMM:
		// 3.0–4.0 м и всё, что длиннее верхней строки, — это 12 мм.
		need = PlateStringerBands[2].ThicknessMM
	case runMM > PlateStringerBands[0].RunMaxMM:
		need = PlateStringerBands[1].ThicknessMM
	default:
		need = PlateStringerBands[0].ThicknessMM
	}
	// Толщина по ширине — ПЕРВАЯ строка, чей предел ширины не меньше заданной.
	// Идти по строкам и брать «ту, чей предел превышен», нельзя: на ширине
	// 1001 мм превышен предел строки 8 мм, но толщина осталась бы 8 мм,
	// хотя марш уже шире того, что эта толщина держит.
	for _, b := range PlateStringerBands {
		if widthMM <= b.WidthMaxMM {
			if b.ThicknessMM > need {
				need = b.ThicknessMM
			}
			break
		}
	}
	// Ширина за пределами самой широкой строки: норматива нет, но и толще
	// 12 мм таблица не описывает — отдаём максимум строки.
	last := PlateStringerBands[len(PlateStringerBands)-1]
	if widthMM > last.WidthMaxMM && last.ThicknessMM > need {
		need = PlateMaxThicknessMM
	}
	return need
}

// PlateStringerSpec — сводное требование к листовому косоуру: нормативная
// строка выбранной толщины плюс требуемая толщина по габаритам марша.
type PlateStringerSpec struct {
	// Band — строка норматива для ВЫБРАННОЙ толщины. ok=false, если толщина
	// в таблице не значится (6 мм, 20 мм).
	Band              PlateStringerBand
	BandFound         bool
	ThicknessMM       float64
	RequiredMM        float64
	RunMM             float64
	WidthMM           float64
	ThicknessTooThin  bool
	WidthBeyondNarrow bool
}

// PlateStringerSpecFor собирает требование по толщине листа и габаритам марша.
func PlateStringerSpecFor(thicknessMM, runMM, widthMM float64) PlateStringerSpec {
	band, found := PlateStringerBandForThickness(thicknessMM)
	required := MinPlateStringerThicknessMM(runMM, widthMM)
	last := PlateStringerBands[len(PlateStringerBands)-1]
	return PlateStringerSpec{
		Band:              band,
		BandFound:         found,
		ThicknessMM:       thicknessMM,
		RequiredMM:        required,
		RunMM:             runMM,
		WidthMM:           widthMM,
		ThicknessTooThin:  thicknessMM+0.1 < required,
		WidthBeyondNarrow: widthMM > last.WidthMaxMM,
	}
}

// BlankLabel — высота заготовки словами: «от 250 мм».
func (b PlateStringerBand) BlankLabel() string {
	return "от " + strconv.FormatFloat(b.BlankMinMM, 'f', -1, 64) + " мм"
}

// absFloat — модуль без импорта math: в этом файле нужен только для сравнения
// толщины с таблицей.
func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// PlateStringerThicknessFor — толщина листа по нормативу для изделия.
//
// runs — длины маршей по проекции (у Г/П-лестницы их два), widthMM — ширина
// марша. Берётся МАКСИМУМ по маршам: у П-лестницы короткий нижний марш не
// должен утягивать толщину вниз, а длинный верхний её требует.
func PlateStringerThicknessFor(runs []float64, widthMM float64) float64 {
	thickness := 0.0
	for _, run := range runs {
		if need := MinPlateStringerThicknessMM(run, widthMM); need > thickness {
			thickness = need
		}
	}
	return thickness
}

// IsPlateFrameMaterial — материал каркаса, для которого косоур это ЛИСТ, а не
// брус: у стали и металла норматива на балку нет.
func IsPlateFrameMaterial(material string) bool {
	return strings.HasPrefix(material, "STEEL") || strings.HasPrefix(material, "METAL")
}

// NormalizePlateStringer приводит стальной косоур к нормативу: толщина листа
// берётся из таблицы по длине и ширине марша, а ширина заготовки гребёнки
// поднимается до высоты исходной заготовки своей строки.
//
// Зачем. В конфигурации поле StringerThickness досталось от деревянной модели,
// где это ШИРИНА СЕЧЕНИЯ БРУСА (30–100 мм). Стальному косоуру-гребёнке это
// число не соответствует: листа в 50 мм не существует, заготовка режется из
// проката 8 / 10 / 12 мм. Пока толщина бралась из выпуска «по потолку», любой
// стальной марш тарифицировался листом 12 мм независимо от длины — короткий
// марш в 2,5 м платил за 12 мм.
//
// Что НЕ трогается:
//
//   - материал не задан явно. Тогда 50 мм — это доска, а не ошибочная цифра:
//     по толщине детали автоназначение выбирает между сталью и пиломатериалом,
//     и подмена сдвинула бы изделие в другую геометрию;
//   - дерево: у него своя таблица (timber_stringer.go), и там 50 мм законно;
//   - ширина гребёнки НЕ уменьшается: если она задана больше норматива, это
//     осознанное решение чертёжника, и резать её до минимума нельзя.
func NormalizePlateStringer(c *StairConfiguration, runs []float64) (thickness, spine float64, changed bool) {
	if c == nil || !IsPlateFrameMaterial(c.Material) || len(runs) == 0 {
		return 0, 0, false
	}
	thickness = PlateStringerThicknessFor(runs, c.Width.Millimeters())
	if thickness <= 0 {
		return 0, 0, false
	}
	band, ok := PlateStringerBandForThickness(thickness)
	if !ok {
		// Толщина из таблицы по построению — значит, строка обязана найтись.
		// Не нашлась — оставляем конфигурацию как есть, лучше лишнее
		// предупреждение, чем молчаливая подмена материала заготовки.
		return thickness, 0, false
	}
	comb := *c.CombOrDefault()
	spine = comb.StringerSpine
	if spine < band.BlankMinMM {
		spine = band.BlankMinMM
	}
	c.StringerThickness = Length(thickness)
	if spine != comb.StringerSpine {
		comb.StringerSpine = spine
		c.Comb = &comb
	}
	return thickness, spine, true
}
