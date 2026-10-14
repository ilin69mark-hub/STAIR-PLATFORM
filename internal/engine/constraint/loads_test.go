package constraint

import (
	"testing"

	"stairplatform/internal/domain/engineering"
)

// loads_test.go — нагрузка на поручень по СП 20.13330.2016 п. 8.2.6.
//
// Тест сверяет не с кодом, а с текстом пункта. Единственная защита от
// незаметной пороги этих чисел — сверка каждого с нормой, поэтому
// значения продублированы здесь из п. 8.2.6, а не взяты из реализации.

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

// TestRailingLoadDesignNeedsGammaF — расчётное значение не выдаётся, пока
// γf не подтверждён. Это главная защита нагрузки: по п. 4.2 СП 20
// расчётное = нормативное × γf, и принять γf = 1 молча — значит
// занизить нагрузку на лестничное ограждение в полтора раза без единого
// предупреждения.
func TestRailingLoadDesignNeedsGammaF(t *testing.T) {
	l := RailingLoadFor(engineering.PurposeHouse)
	if l.GammaF != 0 {
		t.Fatalf("γf = %v: значение должно остаться неподтверждённым, "+
			"пока п. 8.2.7 не сверен по тексту", l.GammaF)
	}
	if got := l.DesignKNPerM(); got != 0 {
		t.Errorf("расчётное значение = %v при неподтверждённом γf, должно быть 0", got)
	}
	// После появления подтверждённого γf расчётное обязано считаться.
	withGamma := l
	withGamma.GammaF = 1.2
	if got := withGamma.DesignKNPerM(); got < 0.599 || got > 0.601 {
		t.Errorf("расчётное = %v, при 0,5 кН/м и γf 1,2 ожидалось 0,6", got)
	}
}

// TestRailingLoadNotesKeepUnassignedCases — величины 1,5 и 0,3 кН/м из
// п. 8.2.6 б) и г) никому не назначены. Это осознанный пробел, а не
// забывчивость: назначения «спортзал» и «обслуживающая площадка» в
// проекте нет. Тест следит, чтобы причина пробела была записана в
// Note и не потерялась при правках.
func TestRailingLoadNotesKeepUnassignedCases(t *testing.T) {
	for _, p := range []engineering.BuildingPurpose{
		engineering.PurposePublic, engineering.PurposeIndustrial,
	} {
		note := RailingLoadFor(p).Note
		for _, want := range []string{"1,5", "0,3"} {
			if !contains(note, want) {
				t.Errorf("%s: в Note нет %q — неприменимые случаи п. 8.2.6 "+
					"б) и г) должны быть названы явно: %s", p, want, note)
			}
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
