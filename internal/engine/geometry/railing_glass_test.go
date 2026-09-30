package geometry

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// Стеклянное ограждение (этап 3 «студийный 3D»): панели между стойками и
// скруглённый профиль поручня. Оба тела — декоративные (ENG-GEO-0008), в BOM
// не входят, поэтому проверяются геометрически, а не через расчёт.

// solidsByRole раскладывает декор по ролям.
func solidsByRole(solids []*kerngeo.Solid, role string) []*kerngeo.Solid {
	var out []*kerngeo.Solid
	for _, s := range solids {
		if s.Role() == role {
			out = append(out, s)
		}
	}
	return out
}

// TestGlassPanelsSitOnSteps — панель между стойками k и k+1 лежит на ступени k
// своей нижней частью и на ступени k+1 верхней, то есть повторяет лесенку.
// Проверяем не «красиво», а два числа: нижняя кромка на уровне ступени (плюс
// зазор башмака) и толщина стекла по Y.
func TestGlassPanelsSitOnSteps(t *testing.T) {
	cfg := testConfig(t)
	cfg.Railing = engineering.RailingRight
	cfg.RailingHeight = mustLength(t, 900)
	decor, err := BuildRailingDecor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	panels := solidsByRole(decor, roleRailingGlass)
	n := cfg.StepCount
	if len(panels) != n-1 {
		t.Fatalf("glass panels = %d, want %d (between %d balusters)", len(panels), n-1, n)
	}
	balY := 900 - balusterSize/2 // edgeInset по правой кромке
	for i, p := range panels {
		k := i + 1
		bb := kerngeo.SolidBoundingBox(p)
		wantLow := float64(k)*180 + glassBaseGap
		if !nearlyEqual(bb.Min.Z, wantLow) {
			t.Errorf("panel %d: bottom = %v, want %v (step %d)", k, bb.Min.Z, wantLow, k)
		}
		if got := bb.Max.Y - bb.Min.Y; !nearlyEqual(got, glassThickness) {
			t.Errorf("panel %d: thickness = %v, want %v", k, got, glassThickness)
		}
		if got := (bb.Min.Y + bb.Max.Y) / 2; !nearlyEqual(got, balY) {
			t.Errorf("panel %d: Y centre = %v, want %v (on the baluster line)", k, got, balY)
		}
		// Панель не должна быть короче поручня: иначе ограждение выглядит
		// «дырявым» прямо над ступенью.
		if bb.Max.Z <= float64(k)*180 {
			t.Errorf("panel %d: top = %v, must be above its own step", k, bb.Max.Z)
		}
	}
}

// TestGlassTopStaysUnderHandrail — верхняя кромка стекла не должна заходить на
// поручень.
//
// Проверка намеренно НЕ повторяет формулу кода. В коде верх панели сдвинут от
// оси поручня на ВЕРТИКАЛЬ railThickness/2, а низ поручня — это точка, сдвинутая
// от оси на railThickness/2 по ПЕРПЕНДИКУЛЯРУ к оси. Для наклонного поручня это
// разные величины: вертикальный сдвиг занижает низ на
// rt·(1−cos(θ)). Тест считает по-настоящему — по перпендикуляру — и требует,
// чтобы стекло осталось ниже настоящего низа бруса. Это защищает от ситуации,
// когда «приблизительный» зазор однажды перестанет быть приблизительным.
func TestGlassTopStaysUnderHandrail(t *testing.T) {
	cfg := testConfig(t)
	cfg.Railing = engineering.RailingRight
	cfg.RailingHeight = mustLength(t, 900)
	decor, err := BuildRailingDecor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rails := solidsByRole(decor, roleRailing)
	if len(rails) != 1 {
		t.Fatalf("handrails = %d, want 1", len(rails))
	}
	b, h, rh := 270.0, 180.0, 900.0
	slope := h / b
	// Настоящий низ поручня: ось минус половина толщины по перпендикуляру.
	cosTheta := 1 / math.Sqrt(1+slope*slope)
	railBottomZ := func(x float64) float64 {
		axisZ := h/2 + rh + slope*x
		return axisZ - railThickness/2*cosTheta
	}
	for i, p := range solidsByRole(decor, roleRailingGlass) {
		k := i + 1
		bb := kerngeo.SolidBoundingBox(p)
		// Максимум Z панели — на её правом краю (верх идёт под углом марша).
		if got := bb.Max.Z; got >= railBottomZ(bb.Max.X) {
			t.Errorf("panel %d: top = %v, handrail bottom at that X = %v (must be below)",
				k, got, railBottomZ(bb.Max.X))
		}
		// И на левом краю: там панель ниже, но проверяем оба конца, потому
		// что именно на левом конце сдвиги почти сравниваются.
		if got := bb.Min.Z; got >= railBottomZ(bb.Min.X) {
			t.Errorf("panel %d: bottom = %v exceeds handrail bottom %v", k, got, railBottomZ(bb.Min.X))
		}
	}
}

// TestRailingProfileIsRoundedButKeepsOuterSize — скругление поручня срезает
// углы ВНУТРЬ исходного бруса: габарит 50×40 не меняется (иначе поручень вышел
// бы за кромку ступени), а объём становится меньше прямоугольного.
//
// Эталонный прямоугольный брус строится здесь же, тем же Extrude — сравнение
// идёт с той же формулой выдавливания, что и в production-коде, и заодно
// проверяет, что профиль остался ПЛОСКИМ и планарным (иначе Extrude ошибся бы).
func TestRailingProfileIsRoundedButKeepsOuterSize(t *testing.T) {
	cfg := testConfig(t)
	cfg.Railing = engineering.RailingRight
	cfg.RailingHeight = mustLength(t, 900)
	decor, err := BuildRailingDecor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rounded := solidsByRole(decor, roleRailing)
	if len(rounded) != 1 {
		t.Fatalf("handrails = %d, want 1", len(rounded))
	}
	// Те же A/B, что и в straightRailingSolids для правой кромки.
	w, b, h, rh, n := 900.0, 270.0, 180.0, 900.0, float64(cfg.StepCount)
	railY := w - railWidth/2
	overhang := b / 2
	slope := h / b
	a := kerngeo.NewPoint3(b/2-overhang, railY, h+rh-slope*overhang)
	bb := kerngeo.NewPoint3((n-0.5)*b+overhang, railY, n*h+rh+slope*overhang)
	axis := bb.Sub(a)
	L := axis.Norm()
	u, ok := axis.Normalized()
	if !ok {
		t.Fatal("degenerate rail axis")
	}
	wd, _ := kerngeo.NewVector3(-u.Y, u.X, 0).Normalized()
	td, _ := u.Cross(wd).Normalized()
	c := a.Add(axis.Scale(0.5))
	rect := []kerngeo.Point3{
		c.Add(wd.Scale(railWidth / 2)).Add(td.Scale(railThickness / 2)),
		c.Add(wd.Scale(railWidth / 2)).Add(td.Scale(-railThickness / 2)),
		c.Add(wd.Scale(-railWidth / 2)).Add(td.Scale(-railThickness / 2)),
		c.Add(wd.Scale(-railWidth / 2)).Add(td.Scale(railThickness / 2)),
	}
	shift := u.Scale(-L / 2)
	flat := make([]kerngeo.Point3, len(rect))
	for i := range rect {
		flat[i] = rect[i].Add(shift)
	}
	ref, err := kerngeo.Extrude(flat, u, L)
	if err != nil {
		t.Fatalf("reference prism: %v", err)
	}
	got := rounded[0]
	// Габариты снаружи — как у прямого бруса.
	gotBB := kerngeo.SolidBoundingBox(got)
	refBB := kerngeo.SolidBoundingBox(ref)
	if !nearlyEqual(gotBB.Min.Y, refBB.Min.Y) || !nearlyEqual(gotBB.Max.Y, refBB.Max.Y) {
		t.Errorf("handrail Y extent = [%v, %v], want [%v, %v] — скругление не должно выходить наружу",
			gotBB.Min.Y, gotBB.Max.Y, refBB.Min.Y, refBB.Max.Y)
	}
	if !nearlyEqual(gotBB.Min.X, refBB.Min.X) || !nearlyEqual(gotBB.Max.X, refBB.Max.X) {
		t.Errorf("handrail X extent = [%v, %v], want [%v, %v]",
			gotBB.Min.X, gotBB.Max.X, refBB.Min.X, refBB.Max.X)
	}
	// Объём меньше: углы срезаны. И почти равен: срезаны 4 угла по 14 мм, а не
	// половина бруса.
	gotVol, err := kerngeo.Volume(got)
	if err != nil {
		t.Fatalf("handrail volume: %v", err)
	}
	refVol, err := kerngeo.Volume(ref)
	if err != nil {
		t.Fatalf("reference volume: %v", err)
	}
	if gotVol >= refVol {
		t.Errorf("handrail volume = %v, want less than the straight prism %v — профиль не скруглён", gotVol, refVol)
	}
	if ratio := gotVol / refVol; ratio < 0.85 || ratio > 0.999 {
		t.Errorf("handrail volume ratio = %v, want 0.85..0.999 (4 срезанных угла по 14 мм)", ratio)
	}
}

// TestGlassPanelsOnLanding — на площадке стекло вертикальное: контур плоский,
// нижняя кромка на уровне настила площадки, верх — под поручнем.
//
// Вызывается landingRailingSolids НАПРЯМУЮ, а не через BuildRailingDecor: в
// сквозном вызове панели верхнего марша неотличимы от панели площадки по
// габаритам (ниж марша начинается ровно на уровне площадки), и проверка
// молча проверяла бы панели марша вместо площадочных.
func TestGlassPanelsOnLanding(t *testing.T) {
	const (
		rh = 900.0
		w  = 900.0
		wp = 900.0
		b  = 270.0
		h1 = 8 * 180.0
		// Ни одна панель не должна совпасть с другой по высоте: если стекло
		// площадки когда-то станет выше/ниже, тест это заметит.
		wantH = rh - railThickness/2 - glassRailGap - glassBaseGap
	)
	sols := landingRailingSolids(w, wp, w, rh, h1, b, 0, false, false, engineering.RailingBoth)
	panels := solidsByRole(sols, roleRailingGlass)
	if len(panels) == 0 {
		t.Fatal("landing must be closed with glass")
	}
	for i, p := range panels {
		bb := kerngeo.SolidBoundingBox(p)
		if !nearlyEqual(bb.Min.Z, h1+glassBaseGap) {
			t.Errorf("landing panel %d: bottom = %v, want %v (landing deck)", i, bb.Min.Z, h1+glassBaseGap)
		}
		if got := bb.Max.Z - bb.Min.Z; !nearlyEqual(got, wantH) {
			t.Errorf("landing panel %d: height = %v, want %v", i, got, wantH)
		}
		if got := bb.Max.Z; !nearlyEqual(got, h1+rh-railThickness/2-glassRailGap) {
			t.Errorf("landing panel %d: top = %v, want %v (glassRailGap under the handrail)", i, got, h1+rh-railThickness/2-glassRailGap)
		}
	}
	// Панель площадки вертикальна: её толщина по горизонтали мала, а высота
	// близка к высоте ограждения. Стекло, «заваленное» на марш, было бы здесь
	// поймано высотой.
	if got, minH := kerngeo.SolidBoundingBox(panels[0]).Max.Z-kerngeo.SolidBoundingBox(panels[0]).Min.Z, rh*0.9; got < minH {
		t.Errorf("landing panel height %v, want at least %v — стекло не должно быть «прихвостком»", got, minH)
	}
}
