package constraint

import (
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
)

// loads_test.go — нагрузки по СП 20.13330.2016 (ред. 14.12.2023).
//
// Тесты сверяют не с кодом, а с текстом пункта. Единственная защита от
// незаметной порчи этих чисел — сверка каждого с нормой, поэтому значения
// продублированы здесь из документа, а не взяты из реализации.

func TestRailingLoadMatchesSP20826(t *testing.T) {
	// п. 8.2.6: а) жилые здания, дошкольные организации, дома отдыха,
	// санатории, больницы и другие лечебные учреждения — 0,5 кН/м;
	// б) трибуны и спортивные залы — 1,5 кН/м;
	// в) другие здания и помещения — 0,8 кН/м или по заданию;
	// г) обслуживающие площадки, мостики, ограждения крыш — 0,3 кН/м.
	cases := []struct {
		purpose engineering.BuildingPurpose
		want    float64
		clause  string
		why     string
	}{
		{engineering.PurposeHouse, 0.5, "п. 8.2.6 а)", "жилое — п. 8.2.6 а)"},
		{engineering.PurposeFlat, 0.5, "п. 8.2.6 а)", "МКД тоже жилое здание"},
		{engineering.PurposePublic, 0.8, "п. 8.2.6 в)", "общественное — «другие здания»"},
		{engineering.PurposeIndustrial, 0.8, "п. 8.2.6 в)", "производственное — «другие здания»"},
	}
	for _, c := range cases {
		got := RailingLoadFor(c.purpose)
		if got.NormativeKNPerM != c.want {
			t.Errorf("%s: нормативное %v кН/м, по п. 8.2.6 нужно %v (%s)",
				c.purpose, got.NormativeKNPerM, c.want, c.why)
		}
		if got.Clause != c.clause {
			t.Errorf("%s: пункт %q, ожидался %q", c.purpose, got.Clause, c.clause)
		}
	}
}

// TestRailingLoadNeverZero — нагрузка не может оказаться нулевой ни при
// каком назначении, включая пустое и мусорное. Ноль в RailingLoad
// означает «данных нет», и он обязан быть виден, а не прятаться в
// значении, которое выглядит как расчёт.
func TestRailingLoadNeverZero(t *testing.T) {
	for _, p := range []engineering.BuildingPurpose{
		"", "house", "flat", "public", "industrial", "мусор",
	} {
		l := RailingLoadFor(p)
		if !l.Known() || l.NormativeKNPerM <= 0 {
			t.Errorf("назначение %q: нагрузка не задана (%+v)", p, l)
		}
		if l.Doc == "" || l.Clause == "" {
			t.Errorf("назначение %q: нагрузка без источника", p)
		}
	}
}

// TestRailingLoadDesignNeedsGammaF — расчётное считается только с
// подтверждённым γf.
//
// Изначально γf был 0 и DesignKNPerM() давал 0: формула п. 8.2.7 в
// HTML-выжимке вырезается. Значение подтверждено по PDF-изданию (третий
// абзац п. 8.2.7: γf = 1,2), и теперь гейт проверяет обратное — что
// расчётное равно нормативному × 1,2, а не просто нормативному, и что
// подмена по-прежнему невозможна.
func TestRailingLoadDesignNeedsGammaF(t *testing.T) {
	l := RailingLoadFor(engineering.PurposeHouse)
	if l.GammaF != 1.2 {
		t.Fatalf("γf = %v, по п. 8.2.7 (абзац 3) СП 20 нужно 1,2", l.GammaF)
	}
	if got := l.DesignKNPerM(); got < 0.599 || got > 0.601 {
		t.Errorf("расчётное = %v, при 0,5 кН/м и γf 1,2 ожидалось 0,6", got)
	}
	noGamma := l
	noGamma.GammaF = 0
	if got := noGamma.DesignKNPerM(); got != 0 {
		t.Errorf("без γf расчётное = %v, должно быть 0", got)
	}
	noNorm := l
	noNorm.NormativeKNPerM = 0
	if got := noNorm.DesignKNPerM(); got != 0 {
		t.Errorf("без нормативного расчётное = %v, должно быть 0", got)
	}
}

// TestRailingLoadNotesKeepUnassignedCases — величины 1,5 и 0,3 кН/м из
// п. 8.2.6 б) и г) никому не назначены. Это осознанный пробел, а не
// забывчивость: назначения «спортзал» и «обслуживаемая площадка» в
// проекте нет. Тест следит, чтобы причина пробела была записана в
// Note и не потерялась при правках.
func TestRailingLoadNotesKeepUnassignedCases(t *testing.T) {
	for _, p := range []engineering.BuildingPurpose{
		engineering.PurposePublic, engineering.PurposeIndustrial,
	} {
		note := RailingLoadFor(p).Note
		for _, want := range []string{"1,5", "0,3"} {
			if !strings.Contains(note, want) {
				t.Errorf("%s: в Note нет %q — неприменимые случаи п. 8.2.6 "+
					"б) и г) должны быть названы явно: %s", p, want, note)
			}
		}
	}
}

// TestStepLoadMatchesTable83 — нагрузка на марш по табл. 8.3 СП 20.
//
// поз. 12 «вестибюли, фойе, коридоры, лестницы …, примыкающие к
// помещениям»: а) позиции 1, 2, 3 — 3,0 кПа; б) позиции 4, 5, 6, 11, 13 —
// 4,0 кПа; в) позиция 7 (трибуны) — 5,0 кПа. Жильё тянет на поз. 1,
// поэтому 3,0; общественное и производственное — на 12 б), поэтому 4,0.
func TestStepLoadMatchesTable83(t *testing.T) {
	cases := []struct {
		purpose engineering.BuildingPurpose
		want    float64
		clause  string
	}{
		{engineering.PurposeHouse, 3.0, "поз. 12 а)"},
		{engineering.PurposeFlat, 3.0, "поз. 12 а)"},
		{engineering.PurposePublic, 4.0, "поз. 12 б)"},
		{engineering.PurposeIndustrial, 4.0, "поз. 12 б)"},
	}
	for _, c := range cases {
		got := StepLoadFor(c.purpose)
		if !got.Known() || got.NormativeKPa != c.want {
			t.Errorf("%s: нормативное %v кПа, по табл. 8.3 нужно %v",
				c.purpose, got.NormativeKPa, c.want)
		}
		if want := "табл. 8.3, " + c.clause; got.Clause != want {
			t.Errorf("%s: пункт %q, ожидался %q", c.purpose, got.Clause, want)
		}
		if got.DesignKPa() <= 0 {
			t.Errorf("%s: расчётное не считается при заполненных данных", c.purpose)
		}
	}
}

// TestGammaFDistributedThreshold — порог 2,0 кПа из п. 8.2.7 (абзац 2):
// 1,3 при нормативном значении менее 2,0 кПа, 1,2 при 2,0 кПа и более.
// Граница проверяется с обеих сторон: значение ровно 2,0 кПа уже даёт
// 1,2, а не 1,3.
func TestGammaFDistributedThreshold(t *testing.T) {
	cases := []struct {
		kpa  float64
		want float64
		why  string
	}{
		{1.5, 1.3, "квартира, поз. 1 — менее 2,0 кПа"},
		{1.999, 1.3, "чуть ниже порога — ещё 1,3"},
		{2.0, 1.2, "ровно порог — уже 1,2"},
		{3.0, 1.2, "лестница в жилом, поз. 12 а) — 1,2"},
		{4.0, 1.2, "лестница, поз. 12 б) — 1,2"},
		{0, 0, "нет нагрузки — нет и коэффициента"},
	}
	for _, c := range cases {
		if got := gammaFDistributed(c.kpa); got != c.want {
			t.Errorf("%v кПа: γf = %v, нужно %v (%s)", c.kpa, got, c.want, c.why)
		}
	}
}

// TestStepConcentratedLoadMatchesSP20 — сосредоточенная нагрузка на
// лестницу: п. 8.3.4 а) 1,5 кН, п. 8.3.5 γf = 1,2, п. 8.3.1 — пятно не
// более 10 см. Размер пятна проверяется так же, как и величина:
// «размазать по ступени» — значит снять проверку, ради которой она есть.
func TestStepConcentratedLoadMatchesSP20(t *testing.T) {
	l := StepConcentratedLoad()
	if l.NormativeKN != 1.5 {
		t.Errorf("нормативное %v кН, по п. 8.3.4 а) нужно 1,5", l.NormativeKN)
	}
	if l.PatchSideMM != 100 {
		t.Errorf("площадка %v мм, по п. 8.3.1 допускается не более 100", l.PatchSideMM)
	}
	if l.GammaF != 1.2 {
		t.Errorf("γf = %v, по п. 8.3.5 нужно 1,2", l.GammaF)
	}
	if got := l.DesignKN(); got < 1.799 || got > 1.801 {
		t.Errorf("расчётное = %v, при 1,5 кН и γf 1,2 ожидалось 1,8", got)
	}
	// Терраса и балкон дают 1,0, трапы 0,5 — разница почти в полтора
	// раза, и оговорка обязана называть их, иначе их возьмут по умолчанию.
	for _, needle := range []string{"1,0", "0,5"} {
		if !strings.Contains(l.Note, needle) {
			t.Errorf("в Note нет %q — случаи п. 8.3.4 б) и в) должны быть названы", needle)
		}
	}
}

// TestSelfWeightFactorMatchesTable71 — γf для веса конструкций по
// табл. 7.1: металл 1,05, дерево 1,1. Неизвестный материал обязан давать
// ноль, а не металлические 1,05 «на всякий случай»: разница в 5 % к
// постоянной нагрузке меняет итоговое сочетание.
func TestSelfWeightFactorMatchesTable71(t *testing.T) {
	cases := []struct {
		category string
		want     float64
	}{
		{"Steel", 1.05},
		{"Wood", 1.1},
		{"", 0},
		{"Бетон", 0},
	}
	for _, c := range cases {
		if got := SelfWeightFactor(c.category); got != c.want {
			t.Errorf("категория %q: γf = %v, по табл. 7.1 нужно %v", c.category, got, c.want)
		}
	}
}

// TestCombinationShortTermMatchesSP20 — коэффициенты сочетаний
// кратковременных нагрузок по п. 6.4: 1,0 / 0,9 / 0,8 / 0,7, индексация
// с единицы. Ноль означает «кратковременных нагрузок нет» и обязан
// отличаться от 1,0.
func TestCombinationShortTermMatchesSP20(t *testing.T) {
	want := []float64{0, 1.0, 0.9, 0.8, 0.7}
	for i, w := range want {
		if got := CombinationShortTerm(i); got != w {
			t.Errorf("ψt(%d) = %v, по п. 6.4 нужно %v", i, got, w)
		}
	}
	if got := CombinationShortTerm(5); got != 0 {
		t.Errorf("ψt(5) = %v, за пределами четырёх кратковременных нужно 0", got)
	}
}
