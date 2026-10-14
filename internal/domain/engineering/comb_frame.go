package engineering

// CombFrame — спецификация металлокаркасного узла: косоур-гребёнка, пятки
// выреза и отверстия в них (BC-002: геометрия считается только из параметров).
//
// Спецификация пришла из чертежа прямого марша на металлокаркасе
// (freecad-out/ГЕОМЕТРИЯ.md, 2026-10-05). Значения по умолчанию — рабочие и
// утверждённые владельцем; переопределение нужно только когда чертёж меняют.
type CombFrame struct {
	// HeelThickness — толщина ВСЕХ пяток обоймы выреза (лист, мм).
	HeelThickness float64
	// HeelWidth — ширина пятки в проём марша от внутренней грани косоура (мм).
	HeelWidth float64
	// HeelInset — отступ пятки 1 от зуба и от подступенка (мм).
	HeelInset float64
	// HeelGap — зазор между соседними пятками (мм).
	HeelGap float64
	// HoleDiameter — диаметр отверстий пятки 1 и пятки 2 (мм).
	HoleDiameter float64
	// HoleEdgeK — поле от края детали до центра отверстия в долях диаметра.
	HoleEdgeK float64
	// HolePitchK — минимальный шаг между центрами в долях диаметра.
	HolePitchK float64
	// HoleStride — «через одно»: берётся каждое второе отверстие.
	HoleStride int
	// AnchorHoleDiameter — диаметр отверстий пятки пола и пятки выхода (мм).
	AnchorHoleDiameter float64
	// SlotLength — длина регулировочного овала (мм), ширина равна диаметру.
	SlotLength float64
	// NosingLength — свес носа ступени за заднюю грань подступенка (мм).
	NosingLength float64
	// TopStepExtend — добор последней ступени до конца косоура (мм).
	TopStepExtend float64
	// StringerSpine — глубина полки гребёнки по нормали к линии подъёма (мм).
	StringerSpine float64
	// HoleSegments — число точек на окружность отверстия. Влияет только на
	// гладкость кромки в 3D, не на объём (площадь многоугольника сходится к
	// кругу); 24 — компромисс между гладкостью и размером меша.
	HoleSegments int
}

// CombFrameDefaults — утверждённая спецификация металлокаркаса (чертёж
// 2026-10-05). Норма владельца: подъём ступени 150…180, глубина проступи
// 270…300; при подъёме 175 шаг 280 и проступь 300.
func CombFrameDefaults() *CombFrame {
	return &CombFrame{
		HeelThickness:      4,
		HeelWidth:          50,
		HeelInset:          5,
		HeelGap:            5,
		HoleDiameter:       10,
		HoleEdgeK:          2,
		HolePitchK:         3,
		HoleStride:         2,
		AnchorHoleDiameter: 12,
		SlotLength:         30,
		NosingLength:       20,
		TopStepExtend:      40,
		StringerSpine:      300,
		HoleSegments:       24,
	}
}

// CombOrDefault — спецификация узла из конфигурации либо утверждённая по
// умолчанию. Нулевое значение поля означает «взять умолчание», поэтому
// частично заполненная спецификация тоже работает.
func (c *StairConfiguration) CombOrDefault() *CombFrame {
	d := CombFrameDefaults()
	if c == nil || c.Comb == nil {
		return d
	}
	s := *c.Comb
	pick := func(v, def float64) float64 {
		if v > 0 {
			return v
		}
		return def
	}
	s.HeelThickness = pick(s.HeelThickness, d.HeelThickness)
	s.HeelWidth = pick(s.HeelWidth, d.HeelWidth)
	s.HeelInset = pick(s.HeelInset, d.HeelInset)
	s.HeelGap = pick(s.HeelGap, d.HeelGap)
	s.HoleDiameter = pick(s.HoleDiameter, d.HoleDiameter)
	s.HoleEdgeK = pick(s.HoleEdgeK, d.HoleEdgeK)
	s.HolePitchK = pick(s.HolePitchK, d.HolePitchK)
	if s.HoleStride <= 0 {
		s.HoleStride = d.HoleStride
	}
	s.AnchorHoleDiameter = pick(s.AnchorHoleDiameter, d.AnchorHoleDiameter)
	s.SlotLength = pick(s.SlotLength, d.SlotLength)
	s.NosingLength = pick(s.NosingLength, d.NosingLength)
	s.TopStepExtend = pick(s.TopStepExtend, d.TopStepExtend)
	s.StringerSpine = pick(s.StringerSpine, d.StringerSpine)
	if s.HoleSegments < 6 {
		s.HoleSegments = d.HoleSegments
	}
	return &s
}
