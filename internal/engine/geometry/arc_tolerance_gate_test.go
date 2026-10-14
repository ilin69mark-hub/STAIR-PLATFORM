package geometry

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// ДОПУСК ДУГИ ОБЯЗАН ВЫПОЛНЯТЬСЯ (Фаза 0.3).
//
// Что проверяет. Число сегментов на окружность должно выводиться из
// объявленного допуска отклонения, а не задаваться константой «достаточно».
// Разница не косметическая:
//
//	константа 10 сегментов на Ø9  → отклонение 0,2202 мм
//	по допуску 0,19 мм            → нужно 15 сегментов, отклонение 0,1886 мм
//
// Обещание «допуск 0,19 мм» было написано в railingTubeToleranceMM, но его
// никто не проверял, и две окружности его нарушали. Детали ограждения
// покупные и лазером не режутся, поэтому денег тут не было — но обещание,
// которое не выполняется, вредно тем, что на него ссылаются.

// circleDeviation — фактическое отклонение многоугольника от окружности.
func circleDeviation(radius float64, segments int) float64 {
	return kerngeo.ArcChordDeviationMM(radius, 2*math.Pi, segments)
}

// TestCircleRailingRespectsTolerance — окружность ограждения, построенная по
// допуску, укладывается в этот допуск при любом радиусе.
func TestCircleRailingRespectsTolerance(t *testing.T) {
	const tol = railTubeToleranceMM
	for _, r := range []float64{2.5, 4.5, 6, 10, 21.2, 25.65, 44} {
		ring := circleRailing(ProductionQuality, kerngeo.NewPoint3(0, 0, 0),
			kerngeo.NewVector3(1, 0, 0), kerngeo.NewVector3(0, 1, 0), r)
		got := circleDeviation(r, len(ring))
		if got > tol+1e-12 {
			t.Errorf("R=%g: отклонение %.6f мм при допуске %.2f (сегментов %d)",
				r, got, tol, len(ring))
		}
		// Все точки лежат на окружности — радиус не «уплывает».
		for i, p := range ring {
			d := math.Hypot(p.X, p.Y)
			if math.Abs(d-r) > 1e-9 {
				t.Fatalf("R=%g, точка %d вне окружности: расстояние %.12f", r, i, d)
			}
		}
	}
}

// TestDisplayQualityIsCoarserButStillRounded — режим показа даёт меньше
// сегментов, но не превращает отверстие в угол.
func TestDisplayQualityIsCoarserButStillRounded(t *testing.T) {
	const r = 4.5
	prod := circleRailing(ProductionQuality, kerngeo.NewPoint3(0, 0, 0),
		kerngeo.NewVector3(1, 0, 0), kerngeo.NewVector3(0, 1, 0), r)
	disp := circleRailing(DisplayQuality, kerngeo.NewPoint3(0, 0, 0),
		kerngeo.NewVector3(1, 0, 0), kerngeo.NewVector3(0, 1, 0), r)
	if len(disp) >= len(prod) {
		t.Errorf("режим показа не легче: %d сегментов против %d", len(disp), len(prod))
	}
	if len(disp) < kerngeo.MinArcSegments {
		t.Errorf("режим показа превратил отверстие в угол: %d сегментов", len(disp))
	}
}

// TestFlangeAnchorHolesMeetDeclaredTolerance — анкерные отверстия цанги
// действительно укладываются в railingTubeToleranceMM.
//
// Отдельный тест на конкретную деталь, потому что именно здесь стояла
// константа hseg = 10 и именно её никто не проверял.
func TestFlangeAnchorHolesMeetDeclaredTolerance(t *testing.T) {
	got := circleDeviation(anchorD/2, len(circleRailing(ProductionQuality,
		kerngeo.NewPoint3(0, 0, 0), kerngeo.NewVector3(1, 0, 0), kerngeo.NewVector3(0, 1, 0),
		anchorD/2)))
	if got > railTubeToleranceMM {
		t.Errorf("анкерное отверстие Ø%g: отклонение %.4f мм при допуске %.2f",
			anchorD, got, railTubeToleranceMM)
	}
	t.Logf("анкерное отверстие Ø%g: %d сегментов, отклонение %.4f мм",
		anchorD, 15, got)
}

// TestCapTopOpeningMeetsDeclaredTolerance — верхнее отверстие колпака
// Ø51,3 тоже укладывается в допуск.
//
// Здесь была константа hseg = 16 и отклонение 0,4929 мм — вдвое больше
// прежнего DefaultArcToleranceMM. Отверстие крупное и видно сбоку.
func TestCapTopOpeningMeetsDeclaredTolerance(t *testing.T) {
	got := circleDeviation(capTopID/2, len(circleRailing(ProductionQuality,
		kerngeo.NewPoint3(0, 0, 0), kerngeo.NewVector3(1, 0, 0), kerngeo.NewVector3(0, 1, 0),
		capTopID/2)))
	if got > railTubeToleranceMM {
		t.Errorf("отверстие колпака Ø%g: отклонение %.4f мм при допуске %.2f",
			capTopID, got, railTubeToleranceMM)
	}
	t.Logf("отверстие колпака Ø%g: отклонение %.4f мм", capTopID, got)
}

// TestCombHoleSegmentsMeetCombTolerance — отверстия обоймы выреза, которые
// режутся лазером, укладываются в combArcToleranceMM.
//
// Эти детали НАСТОЯЩИЕ режутся, поэтому допуск здесь денежный: число точек на
// окружность берётся из конфигурации (CombFrame.HoleSegments = 24), и нужно
// убедиться, что оно не разошлось с объявленным допуском. Сейчас укладывается:
// Ø10 → 0,0428 мм, Ø12 → 0,0513 мм при допуске 0,055.
func TestCombHoleSegmentsMeetCombTolerance(t *testing.T) {
	sp := engineering.CombFrameDefaults()
	for _, c := range []struct {
		name   string
		diamMM float64
	}{
		{"отверстие пятки", sp.HoleDiameter},
		{"отверстие анкера", sp.AnchorHoleDiameter},
	} {
		d := circleDeviation(c.diamMM/2, sp.HoleSegments)
		if d > combArcToleranceMM {
			t.Errorf("%s Ø%g при %d сегментах: отклонение %.4f мм при допуске %.3f",
				c.name, c.diamMM, sp.HoleSegments, d, combArcToleranceMM)
		}
		t.Logf("%s Ø%g: %d сегментов, отклонение %.4f мм (допуск %.3f)",
			c.name, c.diamMM, sp.HoleSegments, d, combArcToleranceMM)
	}
}
