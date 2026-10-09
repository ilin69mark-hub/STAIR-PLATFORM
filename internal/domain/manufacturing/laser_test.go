package manufacturing

import (
	"math"
	"testing"
)

// Шпатка технолога 2026-10-05: зазор и отступ зависят от толщины листа, а не
// берутся из одной константы на все толщины (как было: DefaultKerf = 3 мм).
func TestLaserLayoutTable(t *testing.T) {
	cases := []struct {
		thickness float64
		offset    float64
		gap       float64
	}{
		{thickness: 4, offset: 8, gap: 5},
		{thickness: 5, offset: 10, gap: 6},
		{thickness: 8, offset: 10, gap: 10},
		{thickness: 10, offset: 12, gap: 12},
		{thickness: 12, offset: 15, gap: 15},
	}
	for _, c := range cases {
		if got := CutGapMM(c.thickness); got != c.gap {
			t.Errorf("лист %.0f мм: зазор %.0f, ожидалось %.0f", c.thickness, got, c.gap)
		}
		if got := CutEdgeOffsetMM(c.thickness); got != c.offset {
			t.Errorf("лист %.0f мм: отступ %.0f, ожидалось %.0f", c.thickness, got, c.offset)
		}
	}
}

// Зазор обязан РАСТИ с толщиной: толще металл — больше тепла, шире зонa
// термического влияния. Прежняя константа 3 мм для листа 12 мм была втрое
// меньше нужного.
func TestLaserGapGrowsWithThickness(t *testing.T) {
	prev := 0.0
	for _, th := range []float64{4, 5, 8, 10, 12} {
		gap := CutGapMM(th)
		if gap <= prev {
			t.Errorf("лист %.0f мм: зазор %.0f не больше предыдущего %.0f", th, gap, prev)
		}
		prev = gap
	}
	// Зазор всегда положителен: ноль означал бы детали вплотную.
	for _, th := range []float64{1, 3, 4, 5, 6, 8, 10, 12, 20} {
		if CutGapMM(th) <= 0 {
			t.Errorf("лист %.0f мм: зазор %.0f должен быть положительным", th, CutGapMM(th))
		}
		if CutEdgeOffsetMM(th) <= 0 {
			t.Errorf("лист %.0f мм: отступ %.0f должен быть положительным", th, CutEdgeOffsetMM(th))
		}
	}
}

// Поле стола и лист связаны: лист 6000×1500 режется на две заготовки 3000×1500
// без остатка, и это ровно рабочее поле стола.
func TestSheetSplitsIntoTwoBlanksWithoutRemainder(t *testing.T) {
	if BlanksPerSheet != 2 {
		t.Fatalf("BlanksPerSheet = %d, ожидалось 2", BlanksPerSheet)
	}
	if BlankLengthMM*BlanksPerSheet != StockSheetLengthMM {
		t.Errorf("две заготовки по %.0f дают %.0f, лист %.0f — остаток есть",
			BlankLengthMM, BlankLengthMM*BlanksPerSheet, StockSheetLengthMM)
	}
	if BlankWidthMM != StockSheetWidthMM {
		t.Errorf("ширина заготовки %.0f ≠ ширина листа %.0f: лист не режется вдоль ширины",
			BlankWidthMM, StockSheetWidthMM)
	}
	if BlankLengthMM != LaserFieldLengthMM || BlankWidthMM != LaserFieldWidthMM {
		t.Errorf("заготовка %.0f×%.0f не равна полю стола %.0f×%.0f",
			BlankLengthMM, BlankWidthMM, LaserFieldLengthMM, LaserFieldWidthMM)
	}
	for _, blanks := range []int{0, 1, 2, 3, 4, 5, 10} {
		want := 0
		switch {
		case blanks <= 0:
			want = 0
		case blanks <= 2:
			want = 1
		case blanks <= 4:
			want = 2
		default:
			want = (blanks + 1) / 2
		}
		if got := SheetsForBlanks(blanks); got != want {
			t.Errorf("заготовок %d: листов %d, ожидалось %d", blanks, got, want)
		}
	}
}

// Размеры стыка выводятся из толщины, поэтому одна спефикация покрывает все
// листы: шпатка технолога задаёт их формулой от S.
func TestJointSpecFromThickness(t *testing.T) {
	cases := []struct {
		thickness float64
		slotW     float64
		slotD     float64
		tongueW   float64
		tongueL   float64
	}{
		{thickness: 8, slotW: 40, slotD: 9.5, tongueW: 39.7, tongueL: 7.8},
		{thickness: 10, slotW: 40, slotD: 11.5, tongueW: 39.7, tongueL: 9.8},
		{thickness: 12, slotW: 40, slotD: 13.5, tongueW: 39.7, tongueL: 11.8},
	}
	for _, c := range cases {
		s := JointSpecFor(c.thickness)
		if s.SlotWidthMM != c.slotW || s.SlotDepthMM != c.slotD {
			t.Errorf("S=%.0f: паз %.0f×%.0f, ожидалось %.0f×%.0f",
				c.thickness, s.SlotWidthMM, s.SlotDepthMM, c.slotW, c.slotD)
		}
		if !near(s.TongueWidthMM, c.tongueW) || !near(s.TongueLengthMM, c.tongueL) {
			t.Errorf("S=%.0f: шип %.2f×%.2f, ожидалось %.2f×%.2f",
				c.thickness, s.TongueWidthMM, s.TongueLengthMM, c.tongueW, c.tongueL)
		}
		// Шип меньше паза по обеим сторонам — иначе не сойдётся от удара и
		// будет люфтить (шпатка технолога).
		if s.TongueWidthMM >= s.SlotWidthMM {
			t.Errorf("S=%.0f: шип по ширине %.2f не меньше паза %.2f",
				c.thickness, s.TongueWidthMM, s.SlotWidthMM)
		}
		if s.TongueLengthMM >= s.SlotDepthMM {
			t.Errorf("S=%.0f: шип по длине %.2f не меньше паза %.2f",
				c.thickness, s.TongueLengthMM, s.SlotDepthMM)
		}
		// Под дном паза остаётся корень под корневой шов.
		if root := s.SlotDepthMM - s.TongueLengthMM; !near(root, JointRootMM+JointEndClearanceMM) {
			t.Errorf("S=%.0f: корень %.2f, ожидалось %.2f",
				c.thickness, root, JointRootMM+JointEndClearanceMM)
		}
	}
}

// Разбиение: деталь, влезающая в поле, остаётся целой — на коротком косоуре
// лишний лист тратить нельзя.
func TestSplitToFieldKeepsShortPartWhole(t *testing.T) {
	for _, th := range []float64{4, 5, 8, 10, 12} {
		maxPart := MaxPartLengthMM(th)
		p := SplitToField(maxPart, 300, th)
		if p.Pieces != 1 || p.HasJoints {
			t.Errorf("лист %.0f мм: деталь ровно в поле (%.0f) делилась на %d кусков",
				th, maxPart, p.Pieces)
		}
		if !near(p.PieceLengthMM, maxPart) {
			t.Errorf("лист %.0f мм: длина куска %.1f ≠ %.0f", th, p.PieceLengthMM, maxPart)
		}
	}
}

// Разбиение: каждый кусок влезает в поле, и их сумма покрывает исходную
// длину плюс припуск на шипы.
func TestSplitToFieldEveryPieceFits(t *testing.T) {
	for _, th := range []float64{8, 10, 12} {
		maxPart := MaxPartLengthMM(th)
		overlap := JointSpecFor(th).OverlapMM
		for _, length := range []float64{3001, 4000, 5265, 7000, 10000} {
			p := SplitToField(length, 300, th)
			if p.PieceLengthMM > maxPart+1e-9 {
				t.Errorf("S=%.0f, L=%.0f: кусок %.1f длиннее поля %.1f",
					th, length, p.PieceLengthMM, maxPart)
			}
			want := length + float64(p.Pieces-1)*overlap
			if !near(p.TotalLengthMM, want) {
				t.Errorf("S=%.0f, L=%.0f: сумма %.1f, ожидалось %.1f",
					th, length, p.TotalLengthMM, want)
			}
			if p.HasJoints != (p.Pieces > 1) {
				t.Errorf("S=%.0f, L=%.0f: HasJoints=%v при %d кусках",
					th, length, p.HasJoints, p.Pieces)
			}
			// Число кусков — МИНИМАЛЬНОЕ: на лишний кусок тратился бы
			// лишний лист. Проверяем, что при куске на одну штуку меньше
			// деталь уже НЕ влезает в поле.
			if p.Pieces > 1 {
				fewer := p.Pieces - 1
				pieceFewer := (length + float64(fewer-1)*overlap) / float64(fewer)
				if pieceFewer <= maxPart {
					t.Errorf("S=%.0f, L=%.0f: %d кусков, а %d уже хватает (%.1f ≤ %.1f)",
						th, length, p.Pieces, fewer, pieceFewer, maxPart)
				}
			}
		}
	}
}

// Эталон из плана: косоур 5265 мм на листе 8 мм делится на 2 куска по ≈2642 и
// оба косоура (4 куска) влезают в одну заготовку 3000×1500.
func TestSplitReferenceCombStringer(t *testing.T) {
	p := SplitToField(5265, 300, 8)
	if p.Pieces != 2 {
		t.Fatalf("косоур 5265 мм на листе 8 мм: кусков %d, ожидалось 2", p.Pieces)
	}
	// 4 куска по 2642 мм, ширина 300: рядов ⌊1490/310⌋ = 4, в ряду 1.
	if got := BlankCapacity(8, p.PieceLengthMM, 300); got != 4 {
		t.Fatalf("вместимость заготовки %d кусков, ожидалось 4", got)
	}
	// Два косоура = 4 куска = одна заготовка = один лист.
	blanks := 4 / BlankCapacity(8, p.PieceLengthMM, 300)
	if sheets := SheetsForBlanks(blanks); sheets != 1 {
		t.Errorf("два косоура: заготовок %d, листов %d, ожидался 1", blanks, sheets)
	}
}

// Широкая деталь шире поля стола по ШИРИНЕ: её делят поперёк, на половинки
// оболочки. Делить по длине бессмысленно — кусок останется той же ширины и
// всё равно не влезет.
func TestSplitToFieldCutsWidePartAcrossWidth(t *testing.T) {
	const width = 2 * math.Pi * 300 // 1884,96 мм — периметр колонны R=300
	p := SplitToField(2700, width, 8)
	if p.Pieces != 2 {
		t.Fatalf("развёртка 2700×%.0f: кусков %d, ожидалось 2", width, p.Pieces)
	}
	if p.PiecesAlongLength != 1 || p.PiecesAlongWidth != 2 {
		t.Fatalf("оси разбиения = %d×%d, ожидалось 1×2",
			p.PiecesAlongLength, p.PiecesAlongWidth)
	}
	if !near(p.PieceLengthMM, 2700) {
		t.Errorf("длина куска %.1f ≠ 2700: вдоль длины резать было не нужно", p.PieceLengthMM)
	}
	if p.PieceWidthMM > MaxPartWidthMM(8)+1e-9 {
		t.Errorf("ширина куска %.1f > поля %.1f", p.PieceWidthMM, MaxPartWidthMM(8))
	}
	if !p.HasJoints {
		t.Error("разбиение по ширине тоже даёт сварной стык")
	}
}

// Косоур-гребёнка по ширине не режется: поперечный рез отделил бы зубья от
// спинки. Поэтому делится только длинная полоса, а узкая остаётся целой.
func TestSplitToFieldKeepsNarrowPartWhole(t *testing.T) {
	for _, length := range []float64{100, 2500, MaxPartLengthMM(8)} {
		p := SplitToField(length, 300, 8)
		if p.Pieces != 1 || p.HasJoints {
			t.Errorf("L=%.0f, W=300: кусков %d, ожидалась 1 деталь", length, p.Pieces)
		}
	}
}

// Вместимость заготовки растёт с зазором: при более широком зазоре в ряд
// влезает меньше кусков, и расход листов растёт.
func TestBlankCapacityRespectsGap(t *testing.T) {
	// Узкая деталь: зазор почти не влияет на число в ряду, зато на число рядов.
	got8 := BlankCapacity(8, 260, 50)
	if got8 <= 0 {
		t.Fatalf("вместимость заготовки для детали 260×50 = %d", got8)
	}
	if BlankCapacity(12, 260, 50) >= got8 {
		t.Errorf("на листе 12 мм (зазор 15) вместимость %d не меньше, чем на 8 мм (зазор 10) %d",
			BlankCapacity(12, 260, 50), got8)
	}
	// Деталь шире поля не влезает ни в один ряд.
	if BlankCapacity(8, 100, 2000) != 0 {
		t.Errorf("деталь 100×2000 должна давать 0 кусков в заготовке")
	}
}

func near(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.01
}
