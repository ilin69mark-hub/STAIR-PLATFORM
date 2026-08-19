package http

import (
	"math"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	engprc "stairplatform/internal/engine/pricing"
	"stairplatform/internal/engine/solver"
)

// All DTOs use mm / degrees / mm³ / mm² / mm и руб (старшие единицы валюты)
// при передачи наружу; внутренние расчёты — минорные единицы (PRC-0013).

// calculateRequest — запрос расчёта лестницы (POST /api/v1/stairs:calculate).
type calculateRequest struct {
	WidthMM             float64   `json:"width_mm"`
	HeightMM            float64   `json:"height_mm"`
	Flight              string    `json:"flight"`
	StepHeightMM        float64   `json:"step_height_mm"`
	StringerThicknessMM float64   `json:"stringer_thickness_mm"`
	StepThicknessMM     float64   `json:"step_thickness_mm"`
	Riser               bool      `json:"riser,omitempty"`
	ClearanceMM         float64   `json:"clearance_mm"`
	RailingHeightMM     float64   `json:"railing_height_mm"`
	ComfortStepMM       float64   `json:"comfort_step_mm,omitempty"`
	LandingWidthMM      float64   `json:"landing_width_mm,omitempty"`
	LowerStepCount      int       `json:"lower_step_count,omitempty"`
	OuterRadiusMM       float64   `json:"outer_radius_mm,omitempty"`
	Material            string    `json:"material,omitempty"`
	Rates               *ratesDTO `json:"rates,omitempty"`
}

// ratesDTO — опциональное переопределение ставок цены (в руб/натуральной
// единице); пустые поля наследуются от DefaultRates.
type ratesDTO struct {
	MaterialPerKg struct {
		STEEL_S235 float64 `json:"STEEL-S235"`
		ALUM_5083  float64 `json:"ALUM-5083"`
		WOOD_OAK   float64 `json:"WOOD-OAK"`
	} `json:"material_per_kg_rub"`
	MachinePerHour  float64 `json:"machine_per_hour_rub,omitempty"`
	LaborPerHour    float64 `json:"labor_per_hour_rub,omitempty"`
	OverheadPercent float64 `json:"overhead_percent,omitempty"`
	MarginPercent   float64 `json:"margin_percent,omitempty"`
	DiscountPercent float64 `json:"discount_percent,omitempty"`
	TaxPercent      float64 `json:"tax_percent,omitempty"`
}

// optimizeRequest — запрос оптимизации конфигурации (POST /api/v1/stairs:optimize).
// Поля конфигурации совпадают с calculateRequest; дополнение — целевая
// метрика и опциональные границы поиска (0/не указано → авто).
type optimizeRequest struct {
	calculateRequest
	Target            string   `json:"target,omitempty"`         // price|cost|material
	Maximize          bool     `json:"maximize,omitempty"`       // true → максимум цели
	StepCountMin      *int     `json:"step_count_min,omitempty"` // границы числа ступеней
	StepCountMax      *int     `json:"step_count_max,omitempty"`
	ComfortStepMinMM  *float64 `json:"comfort_step_min_mm,omitempty"` // границы шага комфорта
	ComfortStepMaxMM  *float64 `json:"comfort_step_max_mm,omitempty"`
	ComfortStepGridMM *float64 `json:"comfort_step_grid_mm,omitempty"` // шаг сетки (мм)
}

// optimizeResponse — итог оптимизации (EDR-0032 §3.4).
type optimizeResponse struct {
	Valid     bool             `json:"valid"`
	Evaluated int              `json:"evaluated"`
	Target    string           `json:"target"`
	Objective float64          `json:"objective"`
	Best      *optimizeBestDTO `json:"best,omitempty"`
}

// optimizeBestDTO — лучшая найденная конфигурация и её полный расчёт.
type optimizeBestDTO struct {
	StepCount      int               `json:"step_count"`
	StepHeightMM   float64           `json:"step_height_mm"`
	TreadDepthMM   float64           `json:"tread_depth_mm"`
	LowerStepCount int               `json:"lower_step_count,omitempty"`
	ComfortStepMM  float64           `json:"comfort_step_mm"`
	Result         calculateResponse `json:"result"`
}

// validationIssueDTO — запись отчёта валидации (EDR-0003).
type validationIssueDTO struct {
	ID       string  `json:"id,omitempty"`
	Code     string  `json:"code"`
	Severity string  `json:"severity"`
	Element  string  `json:"element"`
	Message  string  `json:"message"`
	Value    float64 `json:"value,omitempty"`
	Min      float64 `json:"min,omitempty"`
	Max      float64 `json:"max,omitempty"`
	Fix      string  `json:"fix,omitempty"`
	// Param — поле конструктора, которое надо поправить; Guide — понятное
	// описание блокировки и решения; Suggestions — готовые проходящие
	// нормы варианты (заполняются советником advisor).
	Param       string          `json:"param,omitempty"`
	Guide       string          `json:"guide,omitempty"`
	Suggestions []suggestionDTO `json:"suggestions,omitempty"`
}

// suggestionDTO — готовый вариант конфигурации, проходящий нормы
// (советник advisor). По нему фронтенд собирает параметры и повторяет
// расчёт (кнопка «Применить»).
type suggestionDTO struct {
	StepCount      int     `json:"step_count"`
	LowerStepCount int     `json:"lower_step_count,omitempty"`
	StepHeightMm   float64 `json:"step_height_mm"`
	TreadDepthMm   float64 `json:"tread_depth_mm"`
	AngleDeg       float64 `json:"angle_deg"`
	// OuterRadiusMm и WidthMm — для спиральных вариантов (EDR-0007).
	OuterRadiusMm float64 `json:"outer_radius_mm,omitempty"`
	WidthMm       float64 `json:"width_mm,omitempty"`
}

// validationDTO — итог валидации конфигурации.
type validationDTO struct {
	Valid    bool                 `json:"valid"`
	Blocking bool                 `json:"blocking"`
	Issues   []validationIssueDTO `json:"issues"`
}

// flightDTO — результат Solver (EDR-0001). Производственные параметры
// (StepThicknessMm, RailingHeightMm, Riser) — эхо конфигурации, по которому
// фронтенд рисует 2D-профиль «как посчитано» (BC-002).
type flightDTO struct {
	StepCount       int     `json:"step_count"`
	StepHeightMm    float64 `json:"step_height_mm"`
	TreadDepthMm    float64 `json:"tread_depth_mm"`
	RunMm           float64 `json:"run_mm"`
	StringerMm      float64 `json:"stringer_mm"`
	AngleDeg        float64 `json:"angle_deg"`
	WidthMm         float64 `json:"width_mm"`
	StepThicknessMm float64 `json:"step_thickness_mm"`
	RailingHeightMm float64 `json:"railing_height_mm"`
	Riser           bool    `json:"riser"`
	StringerThicknessMm float64 `json:"stringer_thickness_mm"`
}

// configEcho — производственные параметры конфигурации, дублируемые в
// flight-ответы для 2D-рендера (BC-002).
type configEcho struct {
	StepThicknessMm   float64
	RailingHeightMm   float64
	Riser             bool
	StringerThicknessMm float64
}

func flightEcho(r stair.Result) configEcho {
	return configEcho{
		StepThicknessMm:   r.StepThickness.Millimeters(),
		RailingHeightMm:   r.RailingHeight.Millimeters(),
		Riser:             r.Riser,
		StringerThicknessMm: r.StringerThickness.Millimeters(),
	}
}

// lshapeDTO — результат Solver для L-образной лестницы (EDR-0005).
type lshapeDTO struct {
	StepCount       int     `json:"step_count"`
	LowerStepCount  int     `json:"lower_step_count"`
	UpperStepCount  int     `json:"upper_step_count"`
	StepHeightMm    float64 `json:"step_height_mm"`
	TreadDepthMm    float64 `json:"tread_depth_mm"`
	AngleDeg        float64 `json:"angle_deg"`
	LowerHeightMm   float64 `json:"lower_height_mm"`
	UpperHeightMm   float64 `json:"upper_height_mm"`
	LowerRunMm      float64 `json:"lower_run_mm"`
	UpperRunMm      float64 `json:"upper_run_mm"`
	LowerStringerMm float64 `json:"lower_stringer_mm"`
	UpperStringerMm float64 `json:"upper_stringer_mm"`
	LandingWidthMm  float64 `json:"landing_width_mm"`
	WidthMm         float64 `json:"width_mm"`
	StepThicknessMm float64 `json:"step_thickness_mm"`
	RailingHeightMm float64 `json:"railing_height_mm"`
	Riser           bool    `json:"riser"`
	StringerThicknessMm float64 `json:"stringer_thickness_mm"`
}

// ushapeDTO — результат Solver для П-образной лестницы (EDR-0006).
type ushapeDTO struct {
	StepCount       int     `json:"step_count"`
	LowerStepCount  int     `json:"lower_step_count"`
	UpperStepCount  int     `json:"upper_step_count"`
	StepHeightMm    float64 `json:"step_height_mm"`
	TreadDepthMm    float64 `json:"tread_depth_mm"`
	AngleDeg        float64 `json:"angle_deg"`
	LowerHeightMm   float64 `json:"lower_height_mm"`
	UpperHeightMm   float64 `json:"upper_height_mm"`
	LowerRunMm      float64 `json:"lower_run_mm"`
	UpperRunMm      float64 `json:"upper_run_mm"`
	LowerStringerMm float64 `json:"lower_stringer_mm"`
	UpperStringerMm float64 `json:"upper_stringer_mm"`
	LandingWidthMm  float64 `json:"landing_width_mm"`
	WidthMm         float64 `json:"width_mm"`
	StepThicknessMm float64 `json:"step_thickness_mm"`
	RailingHeightMm float64 `json:"railing_height_mm"`
	Riser           bool    `json:"riser"`
	StringerThicknessMm float64 `json:"stringer_thickness_mm"`
}

// spiralDTO — результат Solver для спиральной лестницы (EDR-0007).
type spiralDTO struct {
	StepCount       int     `json:"step_count"`
	StepHeightMm    float64 `json:"step_height_mm"`
	OuterRadiusMm   float64 `json:"outer_radius_mm"`
	ColumnRadiusMm  float64 `json:"column_radius_mm"`
	WalkRadiusMm    float64 `json:"walk_radius_mm"`
	InnerTreadMm    float64 `json:"inner_tread_mm"`
	WalkTreadMm     float64 `json:"walk_tread_mm"`
	OuterTreadMm    float64 `json:"outer_tread_mm"`
	AngleDeg        float64 `json:"angle_deg"`
	AngularStepDeg  float64 `json:"angular_step_deg"`
	ArcLengthMm     float64 `json:"arc_length_mm"`
	ComfortStepMm   float64 `json:"comfort_step_mm"`
	AngularTotalDeg float64 `json:"angular_total_deg"`
	WidthMm         float64 `json:"width_mm"`
	StepThicknessMm float64 `json:"step_thickness_mm"`
	RailingHeightMm float64 `json:"railing_height_mm"`
	Riser           bool    `json:"riser"`
	StringerThicknessMm float64 `json:"stringer_thickness_mm"`
}

// geometryIssueDTO — запись валидации геометрии (ENG-GEO-0018).
type geometryIssueDTO struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Element  string `json:"element"`
	Message  string `json:"message"`
}

// geometryDTO — измерения модели (ENG-GEO-0013).
type geometryDTO struct {
	SolidCount     int                `json:"solid_count"`
	VolumeMm3      float64            `json:"volume_mm3"`
	SurfaceAreaMm2 float64            `json:"surface_area_mm2"`
	BBox           bboxDTO            `json:"bbox"`
	Issues         []geometryIssueDTO `json:"issues"`
}

type bboxDTO struct {
	Min pointDTO `json:"min"`
	Max pointDTO `json:"max"`
}

type pointDTO struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// partDTO — производственная деталь (MFG-0002).
type partDTO struct {
	Number      string  `json:"number"`
	Kind        string  `json:"kind"`
	Material    string  `json:"material"`
	ThicknessMm float64 `json:"thickness_mm"`
	LengthMm    float64 `json:"length_mm"`
	WidthMm     float64 `json:"width_mm"`
	SolidIndex  int     `json:"solid_index"`
}

// bomLineDTO — строка спецификации (MFG-0001).
type bomLineDTO struct {
	Number      int     `json:"number"`
	PartNumber  string  `json:"part_number"`
	Description string  `json:"description"`
	Material    string  `json:"material"`
	ThicknessMm float64 `json:"thickness_mm"`
	Quantity    int     `json:"quantity"`
	LengthMm    float64 `json:"length_mm"`
	WidthMm     float64 `json:"width_mm"`
}

// cutItemDTO — позиция карты раскроя (MFG-0001).
type cutItemDTO struct {
	PartNumber  string  `json:"part_number"`
	Material    string  `json:"material"`
	ThicknessMm float64 `json:"thickness_mm"`
	LengthMm    float64 `json:"length_mm"`
	WidthMm     float64 `json:"width_mm"`
	Quantity    int     `json:"quantity"`
}

// placedPartDTO — деталь, размещённая на листе (MFG-0012).
type placedPartDTO struct {
	PartNumber string  `json:"part_number"`
	LengthMm   float64 `json:"length_mm"`
	WidthMm    float64 `json:"width_mm"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
}

// sheetLayoutDTO — заполненный лист (MFG-0012).
type sheetLayoutDTO struct {
	Material    string          `json:"material"`
	ThicknessMm float64         `json:"thickness_mm"`
	LengthMm    float64         `json:"length_mm"`
	WidthMm     float64         `json:"width_mm"`
	Placed      []placedPartDTO `json:"placed"`
}

// nestingDTO — результат раскроя (MFG-0012).
type nestingDTO struct {
	Sheets       []sheetLayoutDTO `json:"sheets"`
	PartCount    int              `json:"part_count"`
	PartAreaMm2  float64          `json:"part_area_mm2"`
	SheetAreaMm2 float64          `json:"sheet_area_mm2"`
	WasteAreaMm2 float64          `json:"waste_area_mm2"`
	Utilization  float64          `json:"utilization"`
}

// manufacturingDTO — полный производственный пакет (BC-007).
type manufacturingDTO struct {
	Parts   []partDTO    `json:"parts"`
	BOM     []bomLineDTO `json:"bom"`
	CutList []cutItemDTO `json:"cut_list"`
	Nesting nestingDTO   `json:"nesting"`
}

// costComponentDTO — элемент стоимости (PRC-0004) в рублях.
type costComponentDTO struct {
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	AmountRub float64 `json:"amount_rub"`
	Source    string  `json:"source"`
}

// pricingDTO — цена проекта (PRC-0004) в рублях.
type pricingDTO struct {
	Currency          string             `json:"currency"`
	MaterialRub       float64            `json:"material_rub"`
	MachineRub        float64            `json:"machine_rub"`
	LaborRub          float64            `json:"labor_rub"`
	OverheadRub       float64            `json:"overhead_rub"`
	ProductionCostRub float64            `json:"production_cost_rub"`
	MarginRub         float64            `json:"margin_rub"`
	DiscountRub       float64            `json:"discount_rub"`
	PreTaxRub         float64            `json:"pre_tax_rub"`
	TaxRub            float64            `json:"tax_rub"`
	FinalPriceRub     float64            `json:"final_price_rub"`
	Lines             []costComponentDTO `json:"lines"`
}

// calculateResponse — полный результат конвейера.
type calculateResponse struct {
	Validation    validationDTO    `json:"validation"`
	Flight        flightDTO        `json:"flight"`
	LShape        *lshapeDTO       `json:"lshape,omitempty"`
	UShape        *ushapeDTO       `json:"ushape,omitempty"`
	Spiral        *spiralDTO       `json:"spiral,omitempty"`
	Geometry      geometryDTO      `json:"geometry"`
	Manufacturing manufacturingDTO `json:"manufacturing"`
	Pricing       pricingDTO       `json:"pricing"`
}

// ---- converters: domain/engine → DTO ----

func toValidationResult(r *stair.Result) validationDTO {
	issues := make([]validationIssueDTO, 0, len(r.Validation.Issues))
	for _, i := range r.Validation.Issues {
		dto := validationIssueDTO{
			ID: i.ID, Code: string(i.Code), Severity: string(i.Severity),
			Element: i.Element, Message: i.Message, Value: i.Value,
			Min: i.Min, Max: i.Max, Fix: i.Fix,
			Param: i.Param, Guide: i.Guide,
		}
		if len(i.Suggestions) > 0 {
			dto.Suggestions = make([]suggestionDTO, 0, len(i.Suggestions))
			for _, s := range i.Suggestions {
				dto.Suggestions = append(dto.Suggestions, suggestionDTO{
					StepCount: s.StepCount, LowerStepCount: s.LowerStepCount,
					StepHeightMm: s.StepHeightMm, TreadDepthMm: s.TreadDepthMm,
					AngleDeg: s.AngleDeg, OuterRadiusMm: s.OuterRadiusMm, WidthMm: s.WidthMm,
				})
			}
		}
		issues = append(issues, dto)
	}
	return validationDTO{Valid: r.Validation.Valid, Blocking: r.Validation.Blocking, Issues: issues}
}

func toFlight(r stair.Result) flightDTO {
	e := flightEcho(r)
	return flightDTO{
		StepCount: r.Flight.StepCount, StepHeightMm: r.Flight.StepHeight.Millimeters(),
		TreadDepthMm: r.Flight.TreadDepth.Millimeters(), RunMm: r.Flight.Run.Millimeters(),
		StringerMm: r.Flight.Stringer.Millimeters(), AngleDeg: r.Flight.Angle.Degrees(),
		StepThicknessMm: e.StepThicknessMm, RailingHeightMm: e.RailingHeightMm, Riser: e.Riser,
		StringerThicknessMm: e.StringerThicknessMm,
	}
}

func toLShape(l *solver.LShapeResult, e configEcho) *lshapeDTO {
	if l == nil {
		return nil
	}
	return &lshapeDTO{
		StepCount: l.StepCount, LowerStepCount: l.LowerStepCount, UpperStepCount: l.UpperStepCount,
		StepHeightMm: l.StepHeight.Millimeters(), TreadDepthMm: l.TreadDepth.Millimeters(),
		AngleDeg: l.Angle.Degrees(), LowerHeightMm: l.LowerHeight.Millimeters(),
		UpperHeightMm: l.UpperHeight.Millimeters(), LowerRunMm: l.LowerRun.Millimeters(),
		UpperRunMm: l.UpperRun.Millimeters(), LowerStringerMm: l.LowerStringer.Millimeters(),
		UpperStringerMm: l.UpperStringer.Millimeters(), LandingWidthMm: l.LandingWidth.Millimeters(),
		StepThicknessMm: e.StepThicknessMm, RailingHeightMm: e.RailingHeightMm, Riser: e.Riser,
		StringerThicknessMm: e.StringerThicknessMm,
	}
}

func toUShape(u *solver.UShapeResult, e configEcho) *ushapeDTO {
	if u == nil {
		return nil
	}
	return &ushapeDTO{
		StepCount: u.StepCount, LowerStepCount: u.LowerStepCount, UpperStepCount: u.UpperStepCount,
		StepHeightMm: u.StepHeight.Millimeters(), TreadDepthMm: u.TreadDepth.Millimeters(),
		AngleDeg: u.Angle.Degrees(), LowerHeightMm: u.LowerHeight.Millimeters(),
		UpperHeightMm: u.UpperHeight.Millimeters(), LowerRunMm: u.LowerRun.Millimeters(),
		UpperRunMm: u.UpperRun.Millimeters(), LowerStringerMm: u.LowerStringer.Millimeters(),
		UpperStringerMm: u.UpperStringer.Millimeters(), LandingWidthMm: u.LandingWidth.Millimeters(),
		StepThicknessMm: e.StepThicknessMm, RailingHeightMm: e.RailingHeightMm, Riser: e.Riser,
		StringerThicknessMm: e.StringerThicknessMm,
	}
}

func toSpiral(s *solver.SpiralResult, e configEcho) *spiralDTO {
	if s == nil {
		return nil
	}
	return &spiralDTO{
		StepCount:       s.StepCount,
		StepHeightMm:    s.StepHeight.Millimeters(),
		OuterRadiusMm:   s.OuterRadius.Millimeters(),
		ColumnRadiusMm:  s.ColumnRadius.Millimeters(),
		WalkRadiusMm:    s.WalkRadius.Millimeters(),
		InnerTreadMm:    s.InnerTread.Millimeters(),
		WalkTreadMm:     s.WalkTread.Millimeters(),
		OuterTreadMm:    s.OuterTread.Millimeters(),
		AngleDeg:        s.Angle.Degrees(),
		AngularStepDeg:  s.AngularStep * 180 / math.Pi,
		ArcLengthMm:     s.ArcLength.Millimeters(),
		ComfortStepMm:   s.ComfortStep,
		AngularTotalDeg: s.AngularTotal * 180 / math.Pi,
		StepThicknessMm: e.StepThicknessMm, RailingHeightMm: e.RailingHeightMm, Riser: e.Riser,
		StringerThicknessMm: e.StringerThicknessMm,
	}
}

func toGeometry(r stair.Result) geometryDTO {
	bb := r.Measurement.BoundingBox
	issues := make([]geometryIssueDTO, 0, len(r.GeometryIssues))
	for _, i := range r.GeometryIssues {
		issues = append(issues, geometryIssueDTO{Code: i.Code, Severity: string(i.Severity), Element: i.Element, Message: i.Message})
	}
	return geometryDTO{
		SolidCount: r.Measurement.SolidCount, VolumeMm3: r.Measurement.Volume,
		SurfaceAreaMm2: r.Measurement.SurfaceArea,
		BBox: bboxDTO{
			Min: pointDTO{X: bb.Min.X, Y: bb.Min.Y, Z: bb.Min.Z},
			Max: pointDTO{X: bb.Max.X, Y: bb.Max.Y, Z: bb.Max.Z},
		},
		Issues: issues,
	}
}

func toManufacturing(p *dommfg.ManufacturingPackage) manufacturingDTO {
	parts := make([]partDTO, 0, len(p.Parts))
	for _, pt := range p.Parts {
		parts = append(parts, partDTO{
			Number: string(pt.Number), Kind: string(pt.Kind), Material: string(pt.Material),
			ThicknessMm: pt.Thickness.Millimeters(), LengthMm: pt.Length.Millimeters(),
			WidthMm: pt.Width.Millimeters(), SolidIndex: pt.SolidIndex,
		})
	}
	bom := make([]bomLineDTO, 0, len(p.BOM.Lines))
	for _, l := range p.BOM.Lines {
		bom = append(bom, bomLineDTO{
			Number: l.Number, PartNumber: string(l.PartNumber), Description: l.Description,
			Material: string(l.MaterialCode), ThicknessMm: l.Thickness.Millimeters(),
			Quantity: int(l.Quantity), LengthMm: l.Length.Millimeters(), WidthMm: l.Width.Millimeters(),
		})
	}
	cut := make([]cutItemDTO, 0, len(p.CutList.Items))
	for _, c := range p.CutList.Items {
		cut = append(cut, cutItemDTO{
			PartNumber: string(c.PartNumber), Material: string(c.MaterialCode),
			ThicknessMm: c.Thickness.Millimeters(), LengthMm: c.Length.Millimeters(),
			WidthMm: c.Width.Millimeters(), Quantity: int(c.Quantity),
		})
	}
	sheets := make([]sheetLayoutDTO, 0, len(p.Nesting.Sheets))
	for _, s := range p.Nesting.Sheets {
		placed := make([]placedPartDTO, 0, len(s.Placed))
		for _, pp := range s.Placed {
			placed = append(placed, placedPartDTO{
				PartNumber: string(pp.PartNumber), LengthMm: pp.Length.Millimeters(),
				WidthMm: pp.Width.Millimeters(), X: pp.X, Y: pp.Y,
			})
		}
		sheets = append(sheets, sheetLayoutDTO{
			Material: string(s.MaterialCode), ThicknessMm: s.Thickness.Millimeters(),
			LengthMm: s.Length.Millimeters(), WidthMm: s.Width.Millimeters(), Placed: placed,
		})
	}
	return manufacturingDTO{
		Parts: parts, BOM: bom, CutList: cut,
		Nesting: nestingDTO{
			Sheets: sheets, PartCount: int(p.Nesting.PartCount),
			PartAreaMm2: p.Nesting.PartArea, SheetAreaMm2: p.Nesting.SheetArea,
			WasteAreaMm2: p.Nesting.WasteArea, Utilization: p.Nesting.Utilization,
		},
	}
}

func toPricing(b *domprc.PriceBreakdown) pricingDTO {
	lines := make([]costComponentDTO, 0, len(b.Lines))
	for _, l := range b.Lines {
		lines = append(lines, costComponentDTO{
			Name: l.Name, Category: string(l.Category),
			AmountRub: l.Amount.Major(b.Currency), Source: l.Source,
		})
	}
	return pricingDTO{
		Currency:    b.Currency.Code,
		MaterialRub: b.Material.Major(b.Currency), MachineRub: b.Machine.Major(b.Currency),
		LaborRub: b.Labor.Major(b.Currency), OverheadRub: b.Overhead.Major(b.Currency),
		ProductionCostRub: b.ProductionCost.Major(b.Currency), MarginRub: b.Margin.Major(b.Currency),
		DiscountRub: b.Discount.Major(b.Currency), PreTaxRub: b.PreTax.Major(b.Currency),
		TaxRub: b.Tax.Major(b.Currency), FinalPriceRub: b.FinalPrice.Major(b.Currency),
		Lines: lines,
	}
}

// ---- converters: request → application ----

func toConfig(req calculateRequest) (stair.Config, error) {
	cfg := stair.Config{
		Width:             engineering.Length(req.WidthMM),
		Height:            engineering.Length(req.HeightMM),
		Flight:            engineering.FlightType(req.Flight),
		StepHeight:        engineering.Length(req.StepHeightMM),
		StringerThickness: engineering.Length(req.StringerThicknessMM),
		StepThickness:     engineering.Length(req.StepThicknessMM),
		Riser:             req.Riser,
		Clearance:         engineering.Length(req.ClearanceMM),
		RailingHeight:     engineering.Length(req.RailingHeightMM),
		LandingWidth:      engineering.Length(req.LandingWidthMM),
		LowerStepCount:    req.LowerStepCount,
		OuterRadius:       engineering.Length(req.OuterRadiusMM),
		Material:          dommfg.MaterialCode(req.Material),
	}
	return cfg, nil
}

func toOptions(req calculateRequest) (stair.Options, error) {
	opts := stair.Options{}
	if req.ComfortStepMM > 0 {
		opts.ComfortStep = req.ComfortStepMM
	}
	if req.Rates == nil {
		return opts, nil
	}
	r := req.Rates
	rates := engprc.DefaultRates()
	for code, price := range map[dommfg.MaterialCode]float64{
		"STEEL-S235": r.MaterialPerKg.STEEL_S235,
		"ALUM-5083":  r.MaterialPerKg.ALUM_5083,
		"WOOD-OAK":   r.MaterialPerKg.WOOD_OAK,
	} {
		if price > 0 {
			m, err := domprc.CurrencyRUB.FromMajor(price)
			if err != nil {
				return opts, err
			}
			rates.Material[code] = m
		}
	}
	if r.MachinePerHour > 0 {
		m, err := domprc.CurrencyRUB.FromMajor(r.MachinePerHour)
		if err != nil {
			return opts, err
		}
		rates.MachinePerHour = m
	}
	if r.LaborPerHour > 0 {
		m, err := domprc.CurrencyRUB.FromMajor(r.LaborPerHour)
		if err != nil {
			return opts, err
		}
		rates.LaborPerHour = m
	}
	for pct, dst := range map[float64]*domprc.Rate{
		r.OverheadPercent: &rates.OverheadPercent,
		r.MarginPercent:   &rates.MarginPercent,
		r.DiscountPercent: &rates.DiscountPercent,
		r.TaxPercent:      &rates.TaxPercent,
	} {
		if pct > 0 {
			rt, err := domprc.NewRate(pct)
			if err != nil {
				return opts, err
			}
			*dst = rt
		}
	}
	opts.Rates = &rates
	return opts, nil
}

// toOptimizeRequest собирает параметры поиска из DTO. Не указанные границы
// (nil) остаются нулевыми — app-слой выводит их автоматически.
func toOptimizeRequest(req optimizeRequest) stair.OptimizeRequest {
	oreq := stair.OptimizeRequest{Target: stair.OptimizeTarget(req.Target), Maximize: req.Maximize}
	if req.StepCountMin != nil {
		oreq.StepCountMin = *req.StepCountMin
	}
	if req.StepCountMax != nil {
		oreq.StepCountMax = *req.StepCountMax
	}
	if req.ComfortStepMinMM != nil {
		oreq.ComfortStepMin = *req.ComfortStepMinMM
	}
	if req.ComfortStepMaxMM != nil {
		oreq.ComfortStepMax = *req.ComfortStepMaxMM
	}
	if req.ComfortStepGridMM != nil {
		oreq.ComfortStepGrid = *req.ComfortStepGridMM
	}
	return oreq
}

// toOptimizeResponse собирает итог оптимизации для ответа (EDR-0032 §3.4).
func toOptimizeResponse(out *stair.OptimizeResult) optimizeResponse {
	resp := optimizeResponse{
		Valid:     out.Valid,
		Evaluated: out.Evaluated,
		Target:    string(out.Target),
		Objective: out.Objective,
	}
	if !out.Valid || out.BestResult == nil {
		return resp
	}
	m := stepMetrics(out.BestResult)
	resp.Best = &optimizeBestDTO{
		StepCount:      m.stepCount,
		StepHeightMM:   m.stepHeight,
		TreadDepthMM:   m.treadDepth,
		LowerStepCount: m.lowerStepCount,
		ComfortStepMM:  out.ComfortStep,
		Result:         toResponse(out.BestResult),
	}
	return resp
}

// stepMetrics — (n, h, b, n1) результата расчёта по типу марша.
type stepMetricsResult struct {
	stepCount      int
	stepHeight     float64
	treadDepth     float64
	lowerStepCount int
}

func stepMetrics(res *stair.Result) stepMetricsResult {
	var m stepMetricsResult
	switch {
	case res.LShape != nil:
		m.stepCount = res.LShape.StepCount
		m.stepHeight = res.LShape.StepHeight.Millimeters()
		m.treadDepth = res.LShape.TreadDepth.Millimeters()
		m.lowerStepCount = res.LShape.LowerStepCount
	case res.UShape != nil:
		m.stepCount = res.UShape.StepCount
		m.stepHeight = res.UShape.StepHeight.Millimeters()
		m.treadDepth = res.UShape.TreadDepth.Millimeters()
		m.lowerStepCount = res.UShape.LowerStepCount
	case res.Spiral != nil:
		m.stepCount = res.Spiral.StepCount
		m.stepHeight = res.Spiral.StepHeight.Millimeters()
		m.treadDepth = res.Spiral.WalkTread.Millimeters()
	default:
		m.stepCount = res.Flight.StepCount
		m.stepHeight = res.Flight.StepHeight.Millimeters()
		m.treadDepth = res.Flight.TreadDepth.Millimeters()
	}
	return m
}
