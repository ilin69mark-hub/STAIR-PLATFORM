package manufacturing_test

import (
	"strings"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
	engmfg "stairplatform/internal/engine/manufacturing"
)

// mechanicals_test.go — S-237: база прочностных характеристик.
//
// Проверяем не «есть ли числа», а «честно ли ведёт себя отсутствие»:
//   - характеристики могут отсутствовать (StatusUnknown) — это законно;
//   - числа без источника НЕ считаются подтверждёнными;
//   - неполный набор — Partial, а не Known;
//   - заполненное сверяется с текстами СП 16.13330.2017 и СП 64.13330.2017
//     до числа, а дыры нормы (нет E для дуба и ясеня) остаются дырами.

// TestMechanicalsStatus — три состояния набора различаются.
func TestMechanicalsStatus(t *testing.T) {
	cases := []struct {
		name string
		m    *dommfg.Mechanicals
		want dommfg.MechanicalStatus
	}{
		{"нет набора", nil, dommfg.MechUnknown},
		{"пустой набор", &dommfg.Mechanicals{}, dommfg.MechUnknown},
		{"числа без источника", &dommfg.Mechanicals{
			YieldMPa: 235, ElasticModulusMPa: 206000, SafetyFactor: 1.1,
		}, dommfg.MechUnknown},
		{"источник есть, предел не задан", &dommfg.Mechanicals{
			Source: "СП 16.13330", ElasticModulusMPa: 206000, SafetyFactor: 1.1,
		}, dommfg.MechPartial},
		{"полный набор с источником", &dommfg.Mechanicals{
			YieldMPa: 235, ElasticModulusMPa: 206000, SafetyFactor: 1.1,
			Source: "СП 16.13330",
		}, dommfg.MechKnown},
	}
	for _, c := range cases {
		if got := c.m.Status(); got != c.want {
			t.Errorf("%s: статус %q, ждали %q", c.name, got, c.want)
		}
		if (c.m.Known()) != (c.want == dommfg.MechKnown) {
			t.Errorf("%s: Known()=%v при статусе %q", c.name, c.m.Known(), got(c.m))
		}
	}
}

// TestDeviationsAreRecorded — решения владельца, выходящие за пределы
// нормы, обязаны быть записаны в Mechanicals.Source, а не остаться в
// переписке.
//
// Сейчас таких решений два: дуб как основной материал лестницы вопреки
// п. 5.1 СП 64, и Ø42,4 вопреки сортаменту ГОСТ 11068-81. Оба оформлены
// по одному правилу — если отклонение исчезнет из Source, расчёт
// прочности потом выдаст себя за нормативно проверенный, а это ровно то
// самое, ради чего набор с неизвестными полями вообще не считается
// известным.
func TestDeviationsAreRecorded(t *testing.T) {
	reg, err := engmfg.DefaultMaterialRegistry()
	if err != nil {
		t.Fatalf("каталог материалов: %v", err)
	}
	want := map[dommfg.MaterialCode][]string{
		// п. 5.1 СП 64: твёрдые лиственные — только нагели и детали.
		"WOOD-OAK": {"п. 5.1"},
		// сортамент ГОСТ 11068-81: стенка 1,5 нормирована, 42,4 — нет.
		"STAINLESS-304": {"ОТКЛОНЕНИЕ", "42,4", "42,0"},
	}
	for _, m := range reg.Materials() {
		for _, needle := range want[m.Code] {
			if m.Mechanicals == nil {
				t.Errorf("%s: набор пуст, а отклонение должно быть записано", m.Code)
				break
			}
			if !strings.Contains(m.Mechanicals.Source, needle) {
				t.Errorf("%s: в Source нет %q — отклонение от нормы не "+
					"зафиксировано: %s", m.Code, needle, m.Mechanicals.Source)
			}
		}
	}
}

func got(m *dommfg.Mechanicals) dommfg.MechanicalStatus { return m.Status() }

// TestMechanicalsValidateRejectsNonsense — мусорные значения отвергаются,
// отсутствие — нет.
func TestMechanicalsValidateRejectsNonsense(t *testing.T) {
	if err := (*dommfg.Mechanicals)(nil).Validate("STEEL-C245"); err != nil {
		t.Fatalf("отсутствие характеристик не ошибка: %v", err)
	}
	bad := []*dommfg.Mechanicals{
		{YieldMPa: -1},
		{ElasticModulusMPa: -1000},
		{SafetyFactor: 0.5, Source: "СП 16.13330", YieldMPa: 235, ElasticModulusMPa: 206000},
	}
	for i, m := range bad {
		if err := m.Validate("STEEL-C245"); err == nil {
			t.Errorf("мусор %d принят как характеристики: %+v", i, m)
		}
	}
}

// TestDefaultMaterialsStrengthMatchesNorms — наполнение S-237b сверяется с
// официальными текстами СП 16.13330.2017 (ред. 27.06.2023) и СП 64.13330.2017
// (ред. 28.12.2023), а не с тем, что лежит в коде.
//
// Тест жёсткий намеренно: значения переписаны из таблиц норм вручную, и
// единственная защита от их незаметной порчи — сверка каждого числа с тем,
// что написано в документе. При правке таблиц СП тест ОБЯЗАН сломаться:
// это сигнал перепроверить наполнение, а не обновить ожидания под себя.
func TestDefaultMaterialsStrengthMatchesNorms(t *testing.T) {
	reg, err := engmfg.DefaultMaterialRegistry()
	if err != nil {
		t.Fatalf("каталог материалов: %v", err)
	}
	if len(reg.Materials()) == 0 {
		t.Fatal("каталог пуст")
	}

	// want — числа ровно из текстов норм. Расхождение = ошибка наполнения.
	type spec struct {
		status   dommfg.MechanicalStatus
		yield    float64
		tensile  float64
		modulus  float64
		bearing  float64
		shear    float64
		safety   float64
		embedded bool
	}
	want := map[dommfg.MaterialCode]spec{
		// СП 16.13330.2017, табл. В.3 (С245, 2,0–20 мм): Ryn=245, Run=370.
		// E=2,06·10⁵ — прил. Б табл. Б.1. γm=1,05 — табл. 3.
		"STEEL-C245": {dommfg.MechKnown, 245, 370, 206000, 0, 0, 1.05, false},
		// ГОСТ 11068-81, табл. 4, марка 08Х18Н10Т (аналог AISI 304):
		// σт=216, σв=530. E и γm стандартом не даны → partial.
		"STAINLESS-304": {dommfg.MechPartial, 216, 530, 0, 0, 0, 0, false},
		// СП 64.13330.2017, табл. 3 поз. 1а/2а/4а/5а сорт 1 × mдл 0,66
		// (режим В, табл. 4); E0,ср=10 ГПа — прил. В.2.
		"WOOD-SOFT": {dommfg.MechKnown, 13.86, 9.9, 10000, 2.97, 1.78, 0, true},
		// Дуб: те же значения сосны × mп табл. 5 (1,3 вдоль, 2 поперёк,
		// 1,3 скалывание). E в СП 64 нет → partial.
		"WOOD-OAK": {dommfg.MechPartial, 18.02, 12.87, 0, 5.94, 2.32, 0, true},
		"WOOD-ASH": {dommfg.MechPartial, 18.02, 12.87, 0, 5.94, 2.85, 0, true},
		// Орех в СП 64 не упомянут ни разу — характеристик быть не может.
		"WOOD-WALNUT": {dommfg.MechUnknown, 0, 0, 0, 0, 0, 0, false},
	}

	for _, m := range reg.Materials() {
		spec, filled := want[m.Code]
		if !filled {
			// Материал вне наполнения обязан быть честно пустым.
			if m.Mechanicals != nil {
				t.Errorf("%s: нет сверки с нормой, но характеристики заполнены: %+v",
					m.Code, m.Mechanicals)
			}
			continue
		}
		if spec.status == dommfg.MechUnknown {
			// Нормы для материала нет вообще — набор обязан быть пустым,
			// а не заполненным «по аналогии» с соседней породой.
			if m.Mechanicals != nil {
				t.Errorf("%s: в норме материала нет, а набор заполнен: %+v",
					m.Code, m.Mechanicals)
			}
			continue
		}
		if m.Mechanicals == nil {
			t.Errorf("%s: по тексту нормы значения есть, а набор пуст", m.Code)
			continue
		}
		g := m.Mechanicals
		if g.Status() != spec.status {
			t.Errorf("%s: статус %q, по нормам ждали %q", m.Code, g.Status(), spec.status)
		}
		for _, c := range []struct {
			name     string
			got, exp float64
		}{
			{"изгиб/предел текучести", g.YieldMPa, spec.yield},
			{"растяжение", g.TensileMPa, spec.tensile},
			{"модуль упругости", g.ElasticModulusMPa, spec.modulus},
			{"смятие поперёк", g.BearingPerpMPa, spec.bearing},
			{"скалывание вдоль", g.ShearAlongMPa, spec.shear},
			{"γm", g.SafetyFactor, spec.safety},
		} {
			if d := c.got - c.exp; d > 0.005 || d < -0.005 {
				t.Errorf("%s: %s = %v, по тексту нормы %v", m.Code, c.name, c.got, c.exp)
			}
		}
		if g.SafetyFactorEmbedded != spec.embedded {
			t.Errorf("%s: SafetyFactorEmbedded=%v, ждали %v", m.Code, g.SafetyFactorEmbedded, spec.embedded)
		}
		if g.Source == "" {
			t.Errorf("%s: набор заполнен без источника", m.Code)
		}
	}
}

// TestMechanicalsNoFalseKnownWithoutSource — страховка от главной беды:
// набор с числами, но без источника обязан остаться Unknown. Заполненные
// каталогом значения здесь не проверяются, только само правило.
func TestMechanicalsNoFalseKnownWithoutSource(t *testing.T) {
	m := &dommfg.Mechanicals{
		YieldMPa: 245, ElasticModulusMPa: 206000, SafetyFactor: 1.05,
	}
	if m.Known() {
		t.Fatal("числа без источника не должны считаться известными")
	}
}

// TestWoodHardwoodHasNoElasticModulus — модуль упругости дуба и ясеня в
// СП 64 НЕ ПРИВЕДЁН. Тест фиксирует именно эту дыру: если кто-то «доберёт
// E» из сосны или из справочника, тест упадёт и правку придётся обосновать
// новым источником.
func TestWoodHardwoodHasNoElasticModulus(t *testing.T) {
	reg, err := engmfg.DefaultMaterialRegistry()
	if err != nil {
		t.Fatalf("каталог материалов: %v", err)
	}
	for _, m := range reg.Materials() {
		if m.Code != "WOOD-OAK" && m.Code != "WOOD-ASH" {
			continue
		}
		if m.Mechanicals.ElasticModulusMPa != 0 {
			t.Errorf("%s: E=%v — в СП 64 модуль упругости для этой породы "+
				"не приведён, значение подставлено без источника",
				m.Code, m.Mechanicals.ElasticModulusMPa)
		}
		if m.Mechanicals.Known() {
			t.Errorf("%s: набор без E не может быть известным — расчёт по "+
				"деформациям пойдёт по выдуманному модулю", m.Code)
		}
	}
}
