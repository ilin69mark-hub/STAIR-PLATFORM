package manufacturing

import (
	"fmt"
	"regexp"
	"testing"
)

// «КУДА СТАВИТЬ» — гейты строки Where.
//
// Наклейка несёт номер (STR-01), а понять по нему место нельзя. Where
// отвечает на вопрос монтажника «STR-01 — это куда». Здесь проверяется, что
// строка есть у каждой детали, что она называет существующее место и что
// одинаковые места у разных наклеек не повторяются там, где не должны.

// whereStepRE — «ступень N» в любом месте строки.
var whereStepRE = regexp.MustCompile(`ступень (\d+)`)

// TestWhereIsPresentOnEveryModelPart — у каждой детали из тел модели есть
// строка «куда».
//
// Пустая строка означает «место не вычислено». Для плоских деталей модели
// её быть не должно: центр тела известен всегда, и вывести из него строку —
// дело формулы, а не догадки. Пропуск здесь означал бы наклейку без места,
// которую монтажник клеит на глаз.
func TestWhereIsPresentOnEveryModelPart(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatalf("Manufacture: %v", err)
	}
	empty := 0
	for _, p := range pkg.Parts {
		if p.Purchased {
			continue
		}
		if p.Where == "" {
			empty++
			t.Errorf("%s: нет строки «куда»", p.Number)
		}
	}
	t.Logf("деталей каркаса со строкой «куда»: %d", len(pkg.Parts)-empty)
}

// TestWhereStepsAreInRange — номера ступеней в строках принадлежат изделию.
//
// Строка выводится из центра тела, и если формула «номер = высота / подъём»
// съехала, она назовёт ступень, которой нет: «ступень 19» на марше из 15.
// Монтажник по такой строке деталь не найдёт.
func TestWhereStepsAreInRange(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatalf("Manufacture: %v", err)
	}
	for _, p := range pkg.Parts {
		m := whereStepRE.FindStringSubmatch(p.Where)
		if m == nil {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(m[1], "%d", &n); err != nil {
			t.Errorf("%s: номер ступени %q не разбирается", p.Number, m[1])
			continue
		}
		if n < 1 || n > cfg.StepCount {
			t.Errorf("%s: ступень %d при %d ступенях в марше", p.Number, n, cfg.StepCount)
		}
	}
}

// TestWhereSplitPiecesDiffer — куски одной разрезанной детали различаются
// строкой.
//
// Без номера куска две наклейки «косоур А» неотличимы, и монтажник ставит их
// в произвольном порядке. Стык «шип-паз» при этом не сойдётся: паз в нижнем
// куске, шип в верхнем.
func TestWhereSplitPiecesDiffer(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatalf("Manufacture: %v", err)
	}
	split := map[string][]string{}
	for _, p := range pkg.Parts {
		if p.JointCount <= 1 {
			continue
		}
		base := string(p.Number)
		split[base] = append(split[base], p.Where)
	}
	if len(split) == 0 {
		t.Skip("в пакете нет разрезанных деталей")
	}
	for num, wheres := range split {
		seen := map[string]bool{}
		for _, w := range wheres {
			if seen[w] {
				t.Errorf("%s: одинаковая строка «куда» у двух кусков: %q", num, w)
			}
			seen[w] = true
			if !regexp.MustCompile(`кусок \d+ из \d+`).MatchString(w) {
				t.Errorf("%s: в строке нет номера куска: %q", num, w)
			}
		}
	}
}

// TestWhereSidesAreOnlyAB — стороны называются только «А» и «Б».
//
// Сторона выводится из половины ширины марша. Любое третье значение
// означало бы, что формула вышла за согласованный словарь, и монтажник
// услышит «сторона В», которой нет на чертеже.
func TestWhereSidesAreOnlyAB(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatalf("Manufacture: %v", err)
	}
	sideRE := regexp.MustCompile(`сторона ([А-Я])|косоур ([А-Я])`)
	for _, p := range pkg.Parts {
		for _, m := range sideRE.FindAllStringSubmatch(p.Where, -1) {
			for _, s := range m[1:] {
				if s != "" && s != "А" && s != "Б" {
					t.Errorf("%s: сторона %q, допустимы только А и Б: %q", p.Number, s, p.Where)
				}
			}
		}
	}
}

// TestWhereSpotChecks — выборочные строки на эталонной конфигурации.
//
// Структурные тесты выше ловят классы ошибок, но не конкретный сдвиг. Здесь
// зафиксированы правильные строки: если формула «номер = высота / подъём»
// снова даст из детали 1 деталь 4 (как при отладке, когда считалось по X),
// тест это покажет.
func TestWhereSpotChecks(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatalf("Manufacture: %v", err)
	}
	byNum := map[string]string{}
	for _, p := range pkg.Parts {
		byNum[string(p.Number)] = p.Where
	}
	for num, want := range map[string]string{
		"TRD-01": "ступень 1",
		"HLT-01": "ступень 1, сторона А",
		"ANC-01": "пол, сторона А",
	} {
		got, ok := byNum[num]
		if !ok {
			t.Errorf("детали %s нет в пакете", num)
			continue
		}
		if got != want {
			t.Errorf("%s: строка %q, want %q", num, got, want)
		}
	}
}
