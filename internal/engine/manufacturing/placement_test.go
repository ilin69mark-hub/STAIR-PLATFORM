package manufacturing

import (
	"math"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
	kerngeo "stairplatform/internal/geometry"
)

// Положение детали в сборке (Фаза 0.1) — гейты согласованности.
//
// Главная опасность здесь не «нет данных», а «данные есть, но разные».
// Профиль детали и её положение приходят из одной профильной рамки, и если
// это когда-нибудь перестанет быть правдой, чертёж детали напечатается в
// одном месте, а на сборке она встанет в другом. Выглядит это будет как
// ошибка раскроя, а не как ошибка печати.

// TestPlacementMatchesProfileAxes — оси положения совпадают с осями профиля.
//
// Это главный гейт. Если Placement и Profile когда-нибудь разойдутся по
// системе координат, контур раскроя перестанет соответствовать детали на
// сборке, и тест поймает это до появления чертежей.
func TestPlacementMatchesProfileAxes(t *testing.T) {
	solid := plateWithHole(t)
	frame, ok := kerngeo.NewProfileFrame(solid)
	if !ok {
		t.Fatal("профильная рамка не построилась")
	}
	pl, ok := kerngeo.SolidPlacement(solid)
	if !ok {
		t.Fatal("положение не вычислилось")
	}
	if !nearVec(pl.U, frame.U) || !nearVec(pl.V, frame.V) || !nearVec(pl.N, frame.N) {
		t.Fatalf("положение и рамка разошлись:\n  Placement U=%+v V=%+v N=%+v\n  frame     U=%+v V=%+v N=%+v",
			pl.U, pl.V, pl.N, frame.U, frame.V, frame.N)
	}
	if !nearPoint(pl.Origin, frame.Origin) {
		t.Fatalf("начало координат разошлось: %+v против %+v", pl.Origin, frame.Origin)
	}
}

// TestPlacementAxesOrthonormal — оси положения ортонормированы и
// взаимно перпендикулярны.
//
// Если они не ортонормированы, формула «мир = Origin + u·U + v·V» даёт не
// точку, а смазанное пятно: деталь на сборочном чертеже окажется
// деформированной, и чем дальше от начала координат, тем сильнее.
func TestPlacementAxesOrthonormal(t *testing.T) {
	solid := plateWithHole(t)
	pl, ok := kerngeo.SolidPlacement(solid)
	if !ok {
		t.Fatal("положение не вычислилось")
	}
	for name, v := range map[string]kerngeo.Vector3{"U": pl.U, "V": pl.V, "N": pl.N} {
		if n := norm(v); math.Abs(n-1) > 1e-9 {
			t.Errorf("ось %s не единичная: длина %.12f", name, n)
		}
	}
	if d := dot(pl.U, pl.V); math.Abs(d) > 1e-9 {
		t.Errorf("U и V не перпендикулярны: скалярное произведение %.12f", d)
	}
	if d := dot(pl.U, pl.N); math.Abs(d) > 1e-9 {
		t.Errorf("U и N не перпендикулярны: скалярное произведение %.12f", d)
	}
	if d := dot(pl.V, pl.N); math.Abs(d) > 1e-9 {
		t.Errorf("V и N не перпендикулярны: скалярное произведение %.12f", d)
	}
}

// TestProfileRoundTripsThroughPlacement — профиль, переведённый в мир и
// обратно, восстанавливается точно.
//
// Это проверка формулы «мир = Origin + u·U + v·V» и, главное, того, что
// начало координат лежит в плоскости той же крышки, с которой взят контур.
// При ошибке на толщину детали (4 мм у пятки, 8 мм у косоура) деталь на
// сборочном чертеже встала бы выше своего места — и это выглядело бы как
// брак монтажа, а не как ошибка чертежа.
func TestProfileRoundTripsThroughPlacement(t *testing.T) {
	solid := plateWithHole(t)
	pl, ok := kerngeo.SolidPlacement(solid)
	if !ok {
		t.Fatal("положение не вычислилось")
	}
	prof, ok := kerngeo.SolidProfile2D(solid)
	if !ok || len(prof) < 3 {
		t.Fatal("профиль не найден")
	}
	for i, p := range prof {
		w := pl.WorldAt(p.X, p.Y)
		// Обратное преобразование: снимаем Origin и раскладываем по U и V.
		dx, dy, dz := w.X-pl.Origin.X, w.Y-pl.Origin.Y, w.Z-pl.Origin.Z
		u := dx*pl.U.X + dy*pl.U.Y + dz*pl.U.Z
		v := dx*pl.V.X + dy*pl.V.Y + dz*pl.V.Z
		if math.Abs(u-p.X) > 1e-9 || math.Abs(v-p.Y) > 1e-9 {
			t.Fatalf("вершина %d профиля (%.4f, %.4f) не восстановилась: "+
				"получили (%.9f, %.9f)", i, p.X, p.Y, u, v)
		}
	}
	// Точки профиля лежат в ОДНОЙ плоскости — плоскости крышки.
	first := pl.WorldAt(prof[0].X, prof[0].Y)
	nx, ny, nz := pl.N.X, pl.N.Y, pl.N.Z
	d0 := (first.X-pl.Origin.X)*nx + (first.Y-pl.Origin.Y)*ny + (first.Z-pl.Origin.Z)*nz
	for i, p := range prof {
		w := pl.WorldAt(p.X, p.Y)
		d := (w.X-pl.Origin.X)*nx + (w.Y-pl.Origin.Y)*ny + (w.Z-pl.Origin.Z)*nz
		if math.Abs(d-d0) > 1e-9 {
			t.Fatalf("вершина %d вне плоскости крышки: смещение %.9f против %.9f", i, d, d0)
		}
	}
	// Начало координат лежит в плоскости ТОЙ ЖЕ крышки, с которой взят
	// контур, поэтому смещение нулевое. Раньше начало жёстко закреплялось на
	// нижней крышке, и для тела, чей контур попался на верхней, весь профиль
	// отображался в мир на толщину детали выше своего места.
	if math.Abs(d0) > 1e-9 {
		t.Errorf("начало координат не в плоскости крышки контура: смещение %.9f мм, want 0", d0)
	}
}

// TestPlacementRoundTripInDecompose — положение доходит до детали в пакете.
//
// Заполнять Placement в Part — половина дела: если поле теряется по дороге
// (забыли в конструкторе, обнулили при копировании), чертеж получит пустые
// места, и это будет выглядеть как «деталь без номера».
func TestPlacementRoundTripInDecompose(t *testing.T) {
	cfg := testConfig(t)
	res := genResult(t, cfg)
	parts, err := decompose(cfg, res.Model)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	known, unknown := 0, 0
	for _, p := range parts {
		if p.Placement.Known {
			known++
			// Единичные оси обязаны быть единичными и после переноса в домен.
			if n := normVec(p.Placement.U); math.Abs(n-1) > 1e-9 {
				t.Fatalf("деталь %s: ось U не единичная после переноса: %.12f", p.Number, n)
			}
			continue
		}
		unknown++
	}
	if known == 0 {
		t.Fatal("ни у одной детали нет положения — Placement не заполняется вовсе")
	}
	if unknown != 0 {
		t.Errorf("деталей без положения: %d из %d. Для плоских деталей модели "+
			"их быть не должно", unknown, len(parts))
	}
	t.Logf("деталей с положением: %d из %d", known, len(parts))
}

// TestPurchasedPartsHaveNoPlacement — у покупных деталей ограждения
// положения НЕТ, и это должно быть видно, а не молчаливый ноль.
//
// Тела ограждения строятся как декор и в Model не входят
// (appendRailingMesh), поэтому разложение по телам модели их не видит.
// Если бы Placement молча остался нулевым, деталь нарисовалась бы в начале
// координат — и это выглядело бы как ошибка раскроя, а не как «не знаю».
func TestPurchasedPartsHaveNoPlacement(t *testing.T) {
	cfg := testConfig(t)
	parts := purchasedRailingParts(cfg.StepCount, cfg.TreadDepth.Millimeters(),
		railLengthMM(cfg), cfg.RailingHeight.Millimeters(), enggeo.RailingSideCount(cfg), 1)
	if len(parts) == 0 {
		t.Skip("конфигурация без ограждения")
	}
	for _, p := range parts {
		if p.Placement.Known {
			t.Errorf("покупная деталь %s заявила положение, которого у неё нет", p.Number)
		}
	}
}

// ---- вспомогательное ----

// plateWithHole — плоская деталь 100×40×4 с отверстием Ø10 в (30, 20).
//
// Строится НАСТОЯЩИМ публичным API ядра (ExtrudeWithHoles), а не
// тестовой подделкой: проверка положения имеет смысл только на той же
// геометрии, из которой положение и берётся на производстве.
func plateWithHole(t *testing.T) *kerngeo.Solid {
	t.Helper()
	rect := func(cx, cy, w, h float64) []kerngeo.Point3 {
		return []kerngeo.Point3{
			kerngeo.NewPoint3(cx-w/2, cy-h/2, 0),
			kerngeo.NewPoint3(cx+w/2, cy-h/2, 0),
			kerngeo.NewPoint3(cx+w/2, cy+h/2, 0),
			kerngeo.NewPoint3(cx-w/2, cy+h/2, 0),
		}
	}
	hole := rect(30, 20, 10, 10)
	s, err := kerngeo.ExtrudeWithHoles(
		rect(50, 20, 100, 40), [][]kerngeo.Point3{hole},
		kerngeo.NewVector3(0, 0, 1), 4)
	if err != nil {
		t.Fatalf("ExtrudeWithHoles: %v", err)
	}
	return s
}

func norm(v kerngeo.Vector3) float64 { return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z) }

func normVec(v dommfg.Vec3) float64 { return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z) }

func dot(a, b kerngeo.Vector3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }

func nearVec(a, b kerngeo.Vector3) bool {
	return math.Abs(a.X-b.X) < 1e-12 && math.Abs(a.Y-b.Y) < 1e-12 && math.Abs(a.Z-b.Z) < 1e-12
}

func nearPoint(a, b kerngeo.Point3) bool {
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 && math.Abs(a.Z-b.Z) < 1e-9
}
