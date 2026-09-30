package geometry

import (
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// Регрессии на перила площадки, найденные forensic-аудитом 2026-09-27.
//
// GEOM-02: проход к нижнему маршу вычислялся по ГЛУБИНЕ площадки
// (LandingDepth) вместо ШИРИНЫ марша. Пока LandingDepth всегда обнулялся
// решателем (SOLVER-03) и равен был Width, подмена была незаметна. Обе
// правки сделаны вместе: починка одной без другой превратила бы латентный
// дефект в живой.
//
// GEOM-03: для П-марша с левым поворотом перила площадки строились с x0 = l1
// (= n1·b ≈ 1890 мм) при том, что сама площадка строится с landingX0 = 0.
// Комментарий в коде утверждал «x0 остаётся 0», но условие проверяло только
// L-марш: перила уезжали на 1865 мм от площадки.

// g3xUShapeConfig — П-марш площадочного типа, изолированный на перилах площадки.
func g3xUShapeConfig(width float64, left bool) *engineering.StairConfiguration {
	dir := engineering.TurnRight
	if left {
		dir = engineering.TurnLeft
	}
	return &engineering.StairConfiguration{
		Width: engineering.Length(width), Height: engineering.Length(2700),
		Flight: engineering.FlightUShape, StepHeight: engineering.Length(180),
		StringerThickness: engineering.Length(50), StepThickness: engineering.Length(40),
		Clearance: engineering.Length(2500), RailingHeight: engineering.Length(900),
		TreadDepth: engineering.Length(270), StepCount: 15,
		LandingWidth: engineering.Length(width), LowerStepCount: 7,
		TurnKind: engineering.TurnPlatform, Direction: dir,
		RailingLanding: engineering.RailingLeft,
		RailingLower:   engineering.RailingNone, RailingUpper: engineering.RailingNone,
	}
}

// TestGEOM03_LandingRailingMatchesLanding — перила площадки обязаны лежать
// НА площадке, а не рядом с ней.
func TestGEOM03_LandingRailingMatchesLanding(t *testing.T) {
	for _, left := range []bool{false, true} {
		c := g3xUShapeConfig(900, left)
		_, _, lx0, ly := uShapePlatformTransforms(c)
		landing, err := buildLanding(900, ly, lx0, 7*180, 40, roundNone, 0)
		if err != nil {
			t.Fatalf("buildLanding: %v", err)
		}
		lb := kerngeo.SolidBoundingBox(landing)

		sols, err := buildLURNailing(c, 900)
		if err != nil {
			t.Fatalf("buildLURNailing: %v", err)
		}
		if len(sols) == 0 {
			t.Errorf("U-shape left=%v: перила площадки не построены", left)
			continue
		}
		rb := kerngeo.BoundingBox(kerngeo.NewCompound(sols...))

		// Допуск — половина толщины профиля перил: перила строятся по краю
		// плиты и лежат на её границе, а не внутри.
		const tol = 60.0
		if d := rb.Min.X - lb.Min.X; d > tol || d < -tol {
			t.Errorf("U-shape left=%v: перила смещены по minX на %.0f мм (площадка minX=%.0f, перила minX=%.0f)",
				left, d, lb.Min.X, rb.Min.X)
		}
		if d := rb.Max.X - lb.Max.X; d > tol || d < -tol {
			t.Errorf("U-shape left=%v: перила смещены по maxX на %.0f мм (площадка maxX=%.0f, перила maxX=%.0f)",
				left, d, lb.Max.X, rb.Max.X)
		}
		if d := rb.Min.Y - lb.Min.Y; d > tol || d < -tol {
			t.Errorf("U-shape left=%v: перила смещены по minY на %.0f мм", left, d)
		}
		if d := rb.Max.Y - lb.Max.Y; d > tol || d < -tol {
			t.Errorf("U-shape left=%v: перила смещены по maxY на %.0f мм", left, d)
		}
	}
}

// TestGEOM02_LandingPassWidthFollowsFlight — ограждение участка площадки за
// пределами прохода к нижнему маршу обязано считаться от ШИРИНЫ марша.
// До фикса проход брался по глубине площадки: при ld > wp сегмент исчезал
// целиком (ограждение площадки пропадало молча), а при Width < ld < Wp
// оставалась неогороженная полоса.
func TestGEOM02_LandingPassWidthFollowsFlight(t *testing.T) {
	const (
		flightW = 900.0 // ширина марша
		wp      = 1000.0
	)
	// ld меньше wp — ограждение внешнего участка обязано существовать.
	for _, ld := range []float64{flightW, 950} {
		sols := landingRailingSolids(ld, wp, flightW, 900, 7*180, 270, 0, false, false, engineering.RailingBoth)
		if len(sols) == 0 {
			t.Errorf("ld=%.0f: перила площадки не построены — проход ошибочно съел контур", ld)
			continue
		}
		rb := kerngeo.BoundingBox(kerngeo.NewCompound(sols...))
		// Проход по ширине марша = 900, значит ограждение идёт от y=900 до y=wp=1000.
		if rb.Max.Y < wp-tol2 || rb.Min.Y > flightW+tol2 {
			t.Errorf("ld=%.0f: ограждение участка [%.0f..%.0f] ожидалось вне прохода [0..%.0f], получено Y[%.0f..%.0f]",
				ld, flightW, wp, flightW, rb.Min.Y, rb.Max.Y)
		}
	}
	// ld >= wp: площадка не шире прохода, ограждать нечего вдоль этой кромки,
	// но контур в целом строиться обязан.
	for _, ld := range []float64{wp, 1400} {
		sols := landingRailingSolids(ld, wp, flightW, 900, 7*180, 270, 0, false, false, engineering.RailingBoth)
		if len(sols) == 0 {
			t.Errorf("ld=%.0f: перила площадки не построены вовсе", ld)
		}
	}
}

const tol2 = 60.0
