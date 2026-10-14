package manufacturing

// Параметры лазерного раскроя металла (сводная шпатка технолога от
// 2026-10-05). Два независимых ограничения:
//
//	ПОЛЕ СТОЛА. Рабочее поле лазерного стола 3000×1500 мм. Дальше поля
//	сделать нельзя: деталь длиннее или шире поля режется в несколько
//	кусков со сварным стыком (см. JointSpec).
//
//	ЛИСТ. Покупается лист 6000×1500 мм и режется на ДВЕ заготовки
//	3000×1500 без остатка. Поэтому число купленных листов = ⌈заготовок / 2⌉:
//	второй уровень упаковки вырожден и отдельной задачи не создаёт.
//
// ЗАЗОРЫ И ОТСТУПЫ. Зазор между деталями и отступ от края заготовки
// зависят от толщины листа: толще металл — больше тепла, шире зазор.
// Раньше в раскрое стояла одна константа на все толщины (DefaultKerf = 3 мм),
// и для листа 12 мм это был зазор, втрое меньше нужного.
//
//	Толщина | Назначение          | Отступ от края | Зазор между деталями
//	4 мм    | пятки, опоры        | 8 мм           | 5 мм
//	5 мм    | ступени, пластины   | 10 мм          | 6 мм
//	8 мм    | косоур              | 10 мм          | 10 мм
//	10 мм   | косоур              | 12 мм          | 12 мм
//	12 мм   | косоур              | 15 мм          | 15 мм
//
// Берутся ВЕРХНИЕ границы диапазонов шпатки: зазор — это зона термического
// влияния и теплоотвод, зазор впритык там недопустим.
//
// Таблица относится ТОЛЬКО к металлу, режемому лазером. Дерево режется на
// другом станке и имеет свои листы (woodSheets) и свой зазор.

// LaserFieldLengthMM и LaserFieldWidthMM — рабочее поле лазерного стола, мм.
const (
	LaserFieldLengthMM = 3000.0
	LaserFieldWidthMM  = 1500.0
)

// BlankLengthMM и BlankWidthMM — заготовка: ровно рабочее поле стола.
// Лист режется на две такие заготовки без остатка.
const (
	BlankLengthMM = LaserFieldLengthMM
	BlankWidthMM  = LaserFieldWidthMM
)

// BlanksPerSheet — сколько заготовок выходит из одного купленного листа.
const BlanksPerSheet = 2

// StockSheetLengthMM и StockSheetWidthMM — покупаемый лист металла.
const (
	StockSheetLengthMM = 6000.0
	StockSheetWidthMM  = 1500.0
)

// laserLayout — одна строка шпатки: зазор и отступ для толщины листа.
type laserLayout struct {
	ThicknessMM float64
	// EdgeOffsetMM — отступ от края заготовки до первой детали.
	EdgeOffsetMM float64
	// GapMM — зазор между деталями.
	GapMM float64
}

// laserLayouts — шпатка по возрастанию толщины. Отступ для листов 8/10/12 мм
// в шпатке не задан и принят равным зазору той же толщины.
var laserLayouts = []laserLayout{
	{ThicknessMM: 4, EdgeOffsetMM: 8, GapMM: 5},
	{ThicknessMM: 5, EdgeOffsetMM: 10, GapMM: 6},
	{ThicknessMM: 8, EdgeOffsetMM: 10, GapMM: 10},
	{ThicknessMM: 10, EdgeOffsetMM: 12, GapMM: 12},
	{ThicknessMM: 12, EdgeOffsetMM: 15, GapMM: 15},
}

// fallbackLaserLayout — строка для толщины, которой нет в шпатке. Зазор
// берётся у ближайшей строки сверху, потому что тонкий лист хуже отводит
// тепло, а толстый переносит больше: пропуск вниз опаснее, чем вверх.
func fallbackLaserLayout(thicknessMM float64) laserLayout {
	for _, l := range laserLayouts {
		if thicknessMM <= l.ThicknessMM {
			return l
		}
	}
	return laserLayouts[len(laserLayouts)-1]
}

// laserLayoutFor — строка шпатки для толщины листа.
func laserLayoutFor(thicknessMM float64) laserLayout {
	for _, l := range laserLayouts {
		if absFloat(thicknessMM-l.ThicknessMM) < 0.01 {
			return l
		}
	}
	return fallbackLaserLayout(thicknessMM)
}

// CutGapMM — зазор между деталями для листа толщиной thicknessMM, мм.
// Заменяет прежнюю константу DefaultKerf, которая была одна на все толщины.
func CutGapMM(thicknessMM float64) float64 { return laserLayoutFor(thicknessMM).GapMM }

// CutEdgeOffsetMM — отступ от края заготовки до первой детали для листа
// толщиной thicknessMM, мм.
func CutEdgeOffsetMM(thicknessMM float64) float64 {
	return laserLayoutFor(thicknessMM).EdgeOffsetMM
}

// MaxPartLengthMM — максимальная длина детали, вырезаемой за один проход на
// столе: поле стола минус зазор до соседней детали. Деталь длиннее делится на
// куски со сварным стыком.
func MaxPartLengthMM(thicknessMM float64) float64 {
	return LaserFieldLengthMM - CutGapMM(thicknessMM)
}

// MaxPartWidthMM — максимальная ширина детали: поле стола минус два отступа.
func MaxPartWidthMM(thicknessMM float64) float64 {
	return LaserFieldWidthMM - 2*CutEdgeOffsetMM(thicknessMM)
}

// IsLaserMaterial — металл, режемый лазером, по категории материала (не по
// коду: кодов может быть несколько, а категория одна). Для дерева поле стола
// и шпатка зазоров не применяются: дерево режется на другом станке.
func IsLaserMaterial(m *Material) bool { return m != nil && m.Category == "Steel" }

// absFloat — модуль без импорта math (нужен только для сравнения с таблицей).
func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// ---- Сварной стык кусков детали ----
//
// Деталь длиннее поля стола режется на куски, и куски соединяются внахлёст
// «шип-паз» (сводная шпатка технолога 2026-10-05):
//
//   - в ОДНОМ куске деталь выпиливается: сквозная выемка-паз на торце,
//     ширина JointWidthMM, глубина S + JointRootMM;
//   - в СЛЕДУЮЩЕМ куске на торце есть шип: ширина JointWidthMM − 0.35,
//     длина S − 0.2, на всю толщину;
//   - сборка: шип входит в выемку, внизу остаётся корень JointRootMM под
//     корневой шов, кромки разделаны на 45° с притуплением JointLandMM;
//   - в углах паза — R=1, иначе концентратор напряжений и шип упирается в
//     недорез лазера.
//
// Размеры выводятся из толщины S, поэтому одна спефикация покрывает все
// толщины: для листа 10 мм паз 40×11,5 и шип 39,65×9,8.

// Параметры стыка, не зависящие от толщины.
const (
	// JointWidthMM — номинальная ширина шипа и паза поперёк детали.
	// Для листа 10 мм это 4·S, минимум по шпатке — 30 мм.
	JointWidthMM = 40.0
	// JointRootMM — технологический зазор между дном паза и торцом шипа:
	// его заполняет корневой шов, и детали не должны сжиматься до упора.
	JointRootMM = 1.5
	// JointLandMM — притупление на краю фаски, чтобы первый проход дуги не
	// прожёг металл насквозь.
	JointLandMM = 1.5
	// JointCornerRadiusMM — скругление в углах паза.
	JointCornerRadiusMM = 1.0
	// JointSideClearanceMM — зазор по ширине между шипом и пазом на сторону.
	JointSideClearanceMM = 0.15
	// JointEndClearanceMM — зазор по длине между торцом шипа и пазом.
	JointEndClearanceMM = 0.2
)

// JointSpec — размеры стыка для листа толщиной thicknessMM.
type JointSpec struct {
	ThicknessMM float64
	// SlotWidthMM / SlotDepthMM — паз: номинальная ширина и глубина.
	SlotWidthMM float64
	SlotDepthMM float64
	// TongueWidthMM / TongueLengthMM — шип: уменьшен на зазоры по шпатке,
	// поэтому входит в паз от лёгкого удара киянкой и не люфтит.
	TongueWidthMM  float64
	TongueLengthMM float64
	// OverlapMM — длина нахлёста по торцу детали: сколько материала шипа
	// уходит внутрь соседнего куска. Из неё складывается припуск на стык.
	OverlapMM float64
}

// JointSpecFor собирает размеры стыка для листа толщиной thicknessMM.
func JointSpecFor(thicknessMM float64) JointSpec {
	return JointSpec{
		ThicknessMM:    thicknessMM,
		SlotWidthMM:    JointWidthMM,
		SlotDepthMM:    thicknessMM + JointRootMM,
		TongueWidthMM:  JointWidthMM - 2*JointSideClearanceMM,
		TongueLengthMM: thicknessMM - JointEndClearanceMM,
		OverlapMM:      thicknessMM,
	}
}

// SplitPlan — результат разбиения детали на куски по полю стола.
type SplitPlan struct {
	// Pieces — число кусков (1, если деталь влезла целиком).
	Pieces int
	// PiecesAlongLength / PiecesAlongWidth — по сколько кусков режется вдоль
	// длины и вдоль ширины. Обычно одна из осей равна 1.
	PiecesAlongLength int
	PiecesAlongWidth  int
	// PieceLengthMM / PieceWidthMM — габарит одного куска, мм.
	PieceLengthMM float64
	PieceWidthMM  float64
	// TotalLengthMM — суммарная длина заготовок: исходная длина плюс
	// припуск на шипы (по одному на каждый стык).
	TotalLengthMM float64
	// HasJoints — есть ли сварные стыки (Pieces > 1).
	HasJoints bool
}

// SplitToField разбивает деталь на куски, влезающие в поле лазерного стола, по
// ОБЕИМ осям: длинная полоса режется вдоль длины (обычный случай с косоуром),
// а широкая деталь вроде развёртки обечайки — вдоль ширины, иначе
// она не помещается ни в одно поле.
//
// Куски РАВНЫЕ: короткий хвост съел бы целый ряд заготовки, а равные куски
// пакуются лучше. Делим только когда деталь не влезла целиком — иначе на
// коротком косоуре тратился бы лишний лист.
//
// Суммарный размер заготовок = размер + (n−1)·Overlap по каждой резаной оси:
// каждый шип уходит внутрь соседнего куска и занимает там место, поэтому без
// этого припуса собранная деталь была бы меньше заданной.
func SplitToField(lengthMM, widthMM, thicknessMM float64) SplitPlan {
	overlap := JointSpecFor(thicknessMM).OverlapMM
	nL := splitAxis(lengthMM, MaxPartLengthMM(thicknessMM), overlap)
	nW := splitAxis(widthMM, MaxPartWidthMM(thicknessMM), overlap)
	plan := SplitPlan{
		Pieces:            nL * nW,
		PiecesAlongLength: nL,
		PiecesAlongWidth:  nW,
		PieceLengthMM:     lengthMM,
		PieceWidthMM:      widthMM,
		TotalLengthMM:     lengthMM,
		HasJoints:         nL > 1 || nW > 1,
	}
	if nL > 1 {
		plan.TotalLengthMM = lengthMM + float64(nL-1)*overlap
		plan.PieceLengthMM = plan.TotalLengthMM / float64(nL)
	}
	if nW > 1 {
		plan.PieceWidthMM = (widthMM + float64(nW-1)*overlap) / float64(nW)
	}
	return plan
}

// splitAxis — минимальное число равных кусков, чтобы ось size влезла в предел
// limit с припуском overlap на каждый стык.
func splitAxis(size, limit, overlap float64) int {
	if size <= limit {
		return 1
	}
	n := 1
	for {
		piece := (size + float64(n-1)*overlap) / float64(n)
		if piece <= limit || n > 1000 {
			return n
		}
		n++
	}
}

// BlankCapacity — сколько кусков длиной pieceLenMM и шириной pieceWidMM влезает
// в одну заготовку 3000×1500 при отступе и зазоре листа толщиной thicknessMM.
func BlankCapacity(thicknessMM, pieceLenMM, pieceWidMM float64) int {
	off := CutEdgeOffsetMM(thicknessMM)
	gap := CutGapMM(thicknessMM)
	usableW := LaserFieldWidthMM - 2*off
	usableL := LaserFieldLengthMM - 2*off
	if pieceWidMM+gap <= 0 || pieceLenMM+gap <= 0 {
		return 0
	}
	rows := int((usableW + gap) / (pieceWidMM + gap))
	perRow := int((usableL + gap) / (pieceLenMM + gap))
	if rows < 0 {
		rows = 0
	}
	if perRow < 0 {
		perRow = 0
	}
	return rows * perRow
}

// SheetsForBlanks — сколько купленных листов нужно под blanks заготовок.
func SheetsForBlanks(blanks int) int {
	if blanks <= 0 {
		return 0
	}
	return (blanks + BlanksPerSheet - 1) / BlanksPerSheet
}
