package assistant

import (
	"context"
	"encoding/json"
	"fmt"

	"stairplatform/internal/application/stair"
	dommfg "stairplatform/internal/domain/manufacturing"
)

// manufacturingExpert — эксперт D3 (EDR-0038): анализ производственной
// готовности конфигурации. Работает по данным конвейера (таблица
// ManufacturingPackage от engmfg.Manufacture и ManufacturingCostDataset):
// утилизация раскроя, отходы, номенклатура материалов, размеры партии
// деталей, оценочное машинное/ручное время. Ничего не пересчитывает сам
// (AI-0000). Целевой порог утилизации листа напластинки — 70%
// (эмпирическая, фиксируется здесь для детерминизма).
type manufacturingExpert struct{}

// пороговые значения экспертизы (эмпирические, детерминированы).
const (
	mfgUtilizationWarn = 0.70 // util < 70% — warning
	mfgWasteWarn       = 0.25 // waste > 25% — warning
	mfgMaterialsMax    = 2    // больше марок листа — совет по консолидации
)

func (manufacturingExpert) analyze(ctx context.Context, tools *Tools, req Request) (*Response, *intent, error) {
	a, ok := req.(AnalysisRequest)
	if !ok {
		return nil, nil, fmt.Errorf("%w: expected AnalysisRequest, got %T", ErrInvalid, req)
	}
	if err := tools.ValidateConfig(a.Config); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	res, err := tools.Calculate(ctx, a.Config, a.Options)
	if err != nil {
		return nil, nil, fmt.Errorf("assistant: manufacturing: %w", err)
	}
	if res == nil {
		return nil, nil, fmt.Errorf("%w: пустой результат расчёта", ErrNoFeasible)
	}

	// Конвейер остановлен на production: производственных данных нет.
	if res.Validation.Blocking || res.Package == nil {
		resp := &Response{
			Recommendation: "Производственная подготовка недоступна: конвейер остановлен валидацией. Исправьте замечания инженерного анализа.",
			Rating:         0,
			Findings:       collectPipelineFindings(res),
			Suggestions:    fixSuggestions(collectPipelineFindings(res)),
			Notes:          []string{"ManufacturingPackage не сформирован."},
		}
		return resp, buildManufacturingIntent(a, res, resp), nil
	}

	findings := mfgFindings(res.Package, res.Cost)
	rating := mfgRating(findings, res.Package)

	resp := &Response{
		Recommendation: mfgRecommendation(findings),
		Rating:         rating,
		Findings:       findings,
		Suggestions:    mfgSuggestions(res.Package),
		Notes:          mfgNotes(res.Package, res.Cost),
	}
	return resp, buildManufacturingIntent(a, res, resp), nil
}

// mfgFindings собирает замечания по раскрою и номенклатуре.
func mfgFindings(pkg *dommfg.ManufacturingPackage, cost *dommfg.ManufacturingCostDataset) []Finding {
	var out []Finding
	if pkg.Nesting == nil {
		out = append(out, Finding{Severity: "warning", Element: "раскрой", Message: "карта раскроя не сформирована"})
		return out
	}
	n := pkg.Nesting
	util := n.Utilization
	if util < mfgUtilizationWarn {
		out = append(out, Finding{
			Severity: "warning", Element: "раскрой",
			Message: fmt.Sprintf("утилизация листа %.0f%% ниже порога %d%%", util*100, int(mfgUtilizationWarn*100)),
		})
	}
	if n.SheetArea > 0 {
		waste := n.WasteArea / n.SheetArea
		if waste > mfgWasteWarn {
			out = append(out, Finding{
				Severity: "warning", Element: "отходы",
				Message: fmt.Sprintf("доля отходов %.0f%% выше порога %d%%", waste*100, int(mfgWasteWarn*100)),
			})
		}
	}
	if len(n.Sheets) == 0 {
		out = append(out, Finding{Severity: "warning", Element: "раскрой", Message: "нет размещённых листов"})
	}
	if len(materialSet(pkg)) > mfgMaterialsMax {
		out = append(out, Finding{
			Severity: "info", Element: "материалы",
			Message: fmt.Sprintf("в изделии %d марок листового материала — рассмотрите консолидацию", len(materialSet(pkg))),
		})
	}
	if len(pkg.BOM.Lines) == 0 {
		out = append(out, Finding{Severity: "info", Element: "спецификация", Message: "спецификация пуста"})
	}
	if cost != nil && cost.EstimatedProductionTime > 0 && cost.EstimatedMachineTime > 0 &&
		cost.EstimatedMachineTime/cost.EstimatedProductionTime > 0.8 {
		out = append(out, Finding{
			Severity: "info", Element: "время",
			Message: fmt.Sprintf("машинное время доминирует (%.0f%% оценочного цикла)", cost.EstimatedMachineTime/cost.EstimatedProductionTime*100),
		})
	}
	return out
}

// mfgRating — оценка производственной эффективности (0..1): основа — доля
// используемой площади листа, штраф за warning-замечания.
func mfgRating(findings []Finding, pkg *dommfg.ManufacturingPackage) float64 {
	base := 0.8
	if pkg.Nesting != nil {
		base = 0.4 + pkg.Nesting.Utilization*0.6
	}
	for _, f := range findings {
		if f.Severity == "warning" {
			base -= 0.15
		}
	}
	return clamp01(base)
}

// mfgRecommendation формулирует главный вывод.
func mfgRecommendation(findings []Finding) string {
	if len(findings) == 0 {
		return "Конфигурация готова к производству: раскрой полноценный, отходы в норме."
	}
	warns := 0
	for _, f := range findings {
		if f.Severity == "warning" {
			warns++
		}
	}
	if warns == 0 {
		return "Конфигурация готова к производству; есть информационные замечания."
	}
	return fmt.Sprintf("Требуется доработка производственной готовности: %d замечания. Смотрите детали ниже.", warns)
}

// mfgSuggestions — рекомендации по производственной подготовке.
func mfgSuggestions(pkg *dommfg.ManufacturingPackage) []Suggestion {
	var out []Suggestion
	if pkg.Nesting != nil && pkg.Nesting.Utilization < mfgUtilizationWarn {
		out = append(out, Suggestion{
			Message:   "Улучшите утилизацию листа: скорректируйте размеры деталей или используйте другую марку/размер стандартного листа.",
			Rationale: "низкая утилизация увеличивает расход материала",
		})
	}
	if len(materialSet(pkg)) > 1 {
		out = append(out, Suggestion{
			Message:   "Рассмотрите унификацию марок листового материала.",
			Rationale: "меньше марок — проще снабжение и закупки",
		})
	}
	return out
}

// mfgNotes — ключевые метрики производственного пакета.
func mfgNotes(pkg *dommfg.ManufacturingPackage, cost *dommfg.ManufacturingCostDataset) []string {
	var out []string
	out = append(out, fmt.Sprintf("Деталей: %d; позиций БОМ: %d; позиций раскроя: %d.",
		len(pkg.Parts), len(pkg.BOM.Lines), len(pkg.CutList.Items)))
	if pkg.Nesting != nil {
		out = append(out, fmt.Sprintf("Листов: %d; утилизация %.0f%%; площадь деталей %.3f м².",
			len(pkg.Nesting.Sheets), pkg.Nesting.Utilization*100, pkg.Nesting.PartArea/1e6))
	}
	if cost != nil {
		out = append(out, fmt.Sprintf("Оценочное время цикла: %.0f мин (машинное %.0f, ручное %.0f); масса %.1f кг.",
			cost.EstimatedProductionTime, cost.EstimatedMachineTime, cost.EstimatedLaborTime, cost.Mass))
	}
	return out
}

// materialSet — множество марок листа в изделии (детерминированный порядок).
func materialSet(pkg *dommfg.ManufacturingPackage) []dommfg.MaterialCode {
	seen := map[dommfg.MaterialCode]bool{}
	var out []dommfg.MaterialCode
	for _, p := range pkg.Parts {
		if !seen[p.Material] {
			seen[p.Material] = true
			out = append(out, p.Material)
		}
	}
	return out
}

// buildManufacturingIntent — материал комментария.
func buildManufacturingIntent(a AnalysisRequest, res *stair.Result, resp *Response) *intent {
	util, waste, parts := "n/a", "n/a", "n/a"
	if res.Package != nil && res.Package.Nesting != nil {
		util = fmt.Sprintf("%.0f%%", res.Package.Nesting.Utilization*100)
		waste = fmt.Sprintf("%.0f%%", res.Package.Nesting.WasteArea/res.Package.Nesting.SheetArea*100)
		parts = fmt.Sprintf("%d", len(res.Package.Parts))
	}
	data, err := json.Marshal(map[string]interface{}{
		"flight":      string(a.Config.Flight),
		"utilization": util,
		"waste":       waste,
		"parts":       parts,
		"materials":   materialSetPtr(res.Package),
		"findings":    resp.Findings,
	})
	if err != nil {
		data = nil
	}
	return &intent{
		Kind:         string(KindManufacturing),
		Instructions: "Ты — технолог производства лестничных конструкций. Объясни, насколько конструкция готова к изготовлению, какие риски раскроя и что улучшить. По-русски, 3–7 предложений, только по фактам контекста.",
		UserTask:     "Оцени производственную готовность конфигурации лестницы.",
		Context:      fmt.Sprintf("Конфигурация: %s. Утилизация листа %s, отходы %s, деталей %s. Замечаний %d.", flightTitle(a.Config.Flight), util, waste, parts, len(resp.Findings)),
		Data:         data,
	}
}

// materialSetPtr — коды марок (для intent); nil-безопасно.
func materialSetPtr(pkg *dommfg.ManufacturingPackage) []string {
	if pkg == nil {
		return nil
	}
	set := materialSet(pkg)
	out := make([]string, 0, len(set))
	for _, m := range set {
		out = append(out, string(m))
	}
	return out
}
