package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
)

// fakeCalc — in-memory реализация порта stairCalculator для юнит-тестов.
type fakeCalc struct {
	optByFlight map[engineering.FlightType]*stair.OptimizeResult
	validateErr error
	seen        []engineering.FlightType
}

func (f *fakeCalc) Calculate(context.Context, stair.Config, stair.Options) (*stair.Result, error) {
	return nil, errors.New("not implemented in fake")
}

func (f *fakeCalc) Optimize(_ context.Context, cfg stair.Config, _ stair.Options, _ stair.OptimizeRequest) (*stair.OptimizeResult, error) {
	o, ok := f.optByFlight[cfg.Flight]
	if !ok {
		return &stair.OptimizeResult{Valid: false}, nil
	}
	f.seen = append(f.seen, cfg.Flight)
	return o, nil
}

func (f *fakeCalc) ValidateConfig(stair.Config) error { return f.validateErr }

// optResult конструирует оптимум заданного марша с ценой и комфортом.
// Для non-spiral шаг комфорта в Response считаем из геометрии: h=180,
// b = comfort − 2h — тогда comfortStepOf совпадёт с переданным comfort.
func optResult(flight engineering.FlightType, priceMajor float64, comfort float64) *stair.OptimizeResult {
	tread := comfort - 360 // 2*180
	res := &stair.Result{
		Price: &domprc.PriceBreakdown{
			Currency:   domprc.CurrencyRUB,
			FinalPrice: domprc.NewMoney(int64(priceMajor*100 + 0.5)),
		},
	}
	switch flight {
	case engineering.FlightStraight:
		res.Flight = solver.FlightResult{
			StepCount: 15, StepHeight: engineering.Length(180), TreadDepth: engineering.Length(tread),
		}
	case engineering.FlightLShape:
		res.LShape = &solver.LShapeResult{
			StepCount: 15, StepHeight: engineering.Length(180), TreadDepth: engineering.Length(tread),
		}
	case engineering.FlightUShape:
		res.UShape = &solver.UShapeResult{
			StepCount: 16, StepHeight: engineering.Length(169), TreadDepth: engineering.Length(tread),
		}
	case engineering.FlightSpiral:
		res.Spiral = &solver.SpiralResult{StepCount: 14, StepHeight: engineering.Length(193), ComfortStep: comfort}
	default:
		res.Flight = solver.FlightResult{
			StepCount: 15, StepHeight: engineering.Length(180), TreadDepth: engineering.Length(tread),
		}
	}
	return &stair.OptimizeResult{
		Valid:       true,
		Target:      stair.TargetPrice,
		Objective:   priceMajor,
		BestResult:  res,
		ComfortStep: comfort,
	}
}

func testServiceWithCalc(calc stairCalculator) *Service {
	return NewService(calc, nil)
}

func TestDesignRecommendCheapestFlight(t *testing.T) {
	calc := &fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{
		engineering.FlightStraight: optResult(engineering.FlightStraight, 100, 630),
		engineering.FlightLShape:   optResult(engineering.FlightLShape, 140, 628),
		engineering.FlightUShape:   optResult(engineering.FlightUShape, 150, 635),
		engineering.FlightSpiral:   optResult(engineering.FlightSpiral, 200, 640),
	}}
	svc := testServiceWithCalc(calc)

	res, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if res.Kind != KindDesign {
		t.Fatalf("kind = %q, want design", res.Kind)
	}
	if !strings.Contains(res.Response.Recommendation, "прямой марш") {
		t.Fatalf("recommendation %q should mention straight flight", res.Response.Recommendation)
	}
	if res.Response.Recommendation == "" {
		t.Fatal("recommendation must be non-empty")
	}
	if res.Commentary == "" {
		t.Fatal("commentary must be non-empty")
	}
	// Рекомендация — самый дешёвый вариант.
	if len(res.Response.Alternatives) != 3 {
		t.Fatalf("alternatives = %d, want 3", len(res.Response.Alternatives))
	}
	// Перебор всех четырёх типов без сбоя (спираль без радиуса — R=2W).
	if len(calc.seen) != 4 {
		t.Fatalf("evaluated %d flight types, want 4", len(calc.seen))
	}
}

func TestDesignComfortPriority(t *testing.T) {
	calc := &fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{
		// Прямой марш с отклонением комфорта больше, но дешевле.
		engineering.FlightStraight: optResult(engineering.FlightStraight, 90, 700),
		// L-образный с идеальным комфортом, но дороже.
		engineering.FlightLShape: optResult(engineering.FlightLShape, 150, 630),
		engineering.FlightUShape: optResult(engineering.FlightUShape, 160, 700),
		engineering.FlightSpiral: optResult(engineering.FlightSpiral, 200, 700),
	}}
	svc := testServiceWithCalc(calc)

	res, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config:      stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
		Preferences: DesignPreferences{Priority: PriorityComfort},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !strings.Contains(res.Response.Recommendation, "L-образный") {
		t.Fatalf("comfort priority should recommend L-shape, got %q", res.Response.Recommendation)
	}
}

func TestDesignUnknownPriorityInvalid(t *testing.T) {
	svc := testServiceWithCalc(&fakeCalc{})
	_, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config:      stair.Config{Width: 900, Height: 2700},
		Preferences: DesignPreferences{Priority: DesignPriority("metal")},
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestDesignInvalidConfig(t *testing.T) {
	svc := testServiceWithCalc(&fakeCalc{validateErr: errors.New("bad width")})
	_, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config: stair.Config{Width: -5, Height: 2700},
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestDesignNoFeasible(t *testing.T) {
	svc := testServiceWithCalc(&fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{}})
	_, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config: stair.Config{Width: 900, Height: 2700},
	})
	if !errors.Is(err, ErrNoFeasible) {
		t.Fatalf("err = %v, want ErrNoFeasible", err)
	}
}

func TestAskUnknownKind(t *testing.T) {
	svc := testServiceWithCalc(&fakeCalc{})
	_, err := svc.Ask(context.Background(), "t1", "u1", Kind("nope"), nil)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestAskRejectsEmptyTenant(t *testing.T) {
	svc := testServiceWithCalc(&fakeCalc{})
	_, err := svc.Ask(context.Background(), "", "u1", KindDesign, DesignRequest{})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

// --- Engineering assistant (D2) ----

// engResult конструирует полный результат расчёта (реализует Calculate).
func engResult(flight engineering.FlightType, h, b float64, blocking bool) *stair.Result {
	res := &stair.Result{
		Price: &domprc.PriceBreakdown{
			Currency:   domprc.CurrencyRUB,
			FinalPrice: domprc.NewMoney(int64(100 * 100)),
		},
		Validation: validation.Result{Blocking: blocking},
	}
	switch flight {
	case engineering.FlightStraight:
		res.Flight = solver.FlightResult{StepCount: 15, StepHeight: engineering.Length(h), TreadDepth: engineering.Length(b)}
	case engineering.FlightLShape:
		res.LShape = &solver.LShapeResult{StepCount: 15, StepHeight: engineering.Length(h), TreadDepth: engineering.Length(b)}
	case engineering.FlightUShape:
		res.UShape = &solver.UShapeResult{StepCount: 16, StepHeight: engineering.Length(h), TreadDepth: engineering.Length(b)}
	case engineering.FlightSpiral:
		res.Spiral = &solver.SpiralResult{StepCount: 14, StepHeight: engineering.Length(h), ComfortStep: 630}
	default:
		res.Flight = solver.FlightResult{StepCount: 15, StepHeight: engineering.Length(h), TreadDepth: engineering.Length(b)}
	}
	return res
}

// engCalc — порт, у которого Calculate возвращает фиксированный результат,
// а Optimize не используется engineering-экспертом.
type engCalc struct {
	res *stair.Result
	err error
}

func (f *engCalc) Calculate(context.Context, stair.Config, stair.Options) (*stair.Result, error) {
	return f.res, f.err
}
func (f *engCalc) Optimize(context.Context, stair.Config, stair.Options, stair.OptimizeRequest) (*stair.OptimizeResult, error) {
	return nil, errors.New("not used")
}
func (f *engCalc) ValidateConfig(stair.Config) error { return nil }

func TestEngineeringAnalysis(t *testing.T) {
	svc := NewService(&engCalc{res: engResult(engineering.FlightStraight, 180, 270, false)}, nil)
	res, err := svc.Ask(context.Background(), "t1", "u1", KindEngineering, AnalysisRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if res.Kind != KindEngineering {
		t.Fatalf("kind = %q", res.Kind)
	}
	if res.Response.Rating != 1 {
		t.Fatalf("rating = %v, want 1 for compliant config", res.Response.Rating)
	}
	if len(res.Response.Findings) != 0 {
		t.Fatalf("findings = %v, want none", res.Response.Findings)
	}
	if res.Commentary == "" {
		t.Fatal("commentary must be non-empty")
	}
}

func TestEngineeringBlockingValidation(t *testing.T) {
	svc := NewService(&engCalc{res: &stair.Result{Validation: validation.Result{Blocking: true}}}, nil)
	res, err := svc.Ask(context.Background(), "t1", "u1", KindEngineering, AnalysisRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if res.Response.Rating != 0 {
		t.Fatalf("rating = %v, want 0 on blocking", res.Response.Rating)
	}
	if !strings.Contains(res.Response.Recommendation, "блокирующ") {
		t.Fatalf("recommendation should mention blocking validation, got %q", res.Response.Recommendation)
	}
}

type stubBackend struct {
	text string
	err  error
}

func (s stubBackend) Infer(context.Context, Prompt) (*Answer, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &Answer{Text: s.text}, nil
}

// --- Manufacturing assistant (D3) ----

// mfgCalc — порт с фиксированным производственным пакетом.
type mfgCalc struct {
	res *stair.Result
	err error
}

func (f *mfgCalc) Calculate(context.Context, stair.Config, stair.Options) (*stair.Result, error) {
	return f.res, f.err
}
func (f *mfgCalc) Optimize(context.Context, stair.Config, stair.Options, stair.OptimizeRequest) (*stair.OptimizeResult, error) {
	return nil, errors.New("not used")
}
func (f *mfgCalc) ValidateConfig(stair.Config) error { return nil }

func mfgResult(util, waste float64) *stair.Result {
	return &stair.Result{
		Flight: solver.FlightResult{StepCount: 15, StepHeight: engineering.Length(180), TreadDepth: engineering.Length(270)},
		Price: &domprc.PriceBreakdown{
			Currency:   domprc.CurrencyRUB,
			FinalPrice: domprc.NewMoney(int64(100 * 100)),
		},
		Package: &dommfg.ManufacturingPackage{
			Parts: []dommfg.Part{
				{Number: "P-01", Material: "STEEL-S235"},
				{Number: "P-02", Material: "STEEL-S235"},
			},
			BOM:     dommfg.BOM{Lines: []dommfg.BOMLine{{Number: 1}}},
			CutList: dommfg.CutList{Items: []dommfg.CutItem{{PartNumber: "P-01"}}},
			Nesting: &dommfg.NestingResult{
				Sheets:      []dommfg.SheetLayout{{}},
				PartCount:   2,
				PartArea:    1e6,
				SheetArea:   1e6 / util,
				WasteArea:   1e6 / util * waste,
				Utilization: util,
			},
		},
		Cost: &dommfg.ManufacturingCostDataset{
			PartCount: 2, EstimatedMachineTime: 60, EstimatedLaborTime: 40,
			EstimatedProductionTime: 100,
		},
	}
}

func TestManufacturingAnalysis(t *testing.T) {
	svc := NewService(&mfgCalc{res: mfgResult(0.9, 0.08)}, nil)
	res, err := svc.Ask(context.Background(), "t1", "u1", KindManufacturing, AnalysisRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if res.Kind != KindManufacturing {
		t.Fatalf("kind = %q", res.Kind)
	}
	for _, f := range res.Response.Findings {
		if f.Severity == "warning" {
			t.Fatalf("unexpected warning for good config: %+v", f)
		}
	}
	if res.Response.Rating < 0.9 {
		t.Fatalf("rating = %v, want ≥ 0.9", res.Response.Rating)
	}
	if !strings.Contains(res.Response.Recommendation, "готова") {
		t.Fatalf("recommendation should say ready, got %q", res.Response.Recommendation)
	}
}

func TestManufacturingLowUtilization(t *testing.T) {
	svc := NewService(&mfgCalc{res: mfgResult(0.5, 0.45)}, nil)
	res, err := svc.Ask(context.Background(), "t1", "u1", KindManufacturing, AnalysisRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !hasWarning(res.Response.Findings, "раскрой") {
		t.Fatalf("expected utilization warning, got %+v", res.Response.Findings)
	}
	if res.Response.Rating >= 0.7 {
		t.Fatalf("rating = %v, want low for bad utilization", res.Response.Rating)
	}
	if len(res.Response.Suggestions) == 0 {
		t.Fatal("expected suggestions for low utilization")
	}
}

func TestManufacturingBlocked(t *testing.T) {
	res := &stair.Result{
		Validation: validation.Result{Blocking: true},
		Flight:     solver.FlightResult{StepCount: 15, StepHeight: engineering.Length(180), TreadDepth: engineering.Length(270)},
	}
	svc := NewService(&mfgCalc{res: res}, nil)
	out, err := svc.Ask(context.Background(), "t1", "u1", KindManufacturing, AnalysisRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if out.Response.Rating != 0 {
		t.Fatalf("rating = %v, want 0 on blocked", out.Response.Rating)
	}
}

// --- Pricing assistant (D4) ----

// prcCalc — порт с фиксированным PriceBreakdown.
type prcCalc struct {
	res *stair.Result
	err error
}

func (f *prcCalc) Calculate(context.Context, stair.Config, stair.Options) (*stair.Result, error) {
	return f.res, f.err
}
func (f *prcCalc) Optimize(context.Context, stair.Config, stair.Options, stair.OptimizeRequest) (*stair.OptimizeResult, error) {
	return nil, errors.New("not used")
}
func (f *prcCalc) ValidateConfig(stair.Config) error { return nil }

func prcResult(material, machine, labor, overhead, margin, final int64) *stair.Result {
	cur := domprc.CurrencyRUB
	pc := domprc.NewMoney(material + machine + labor + overhead)
	return &stair.Result{
		Flight: solver.FlightResult{StepCount: 15, StepHeight: engineering.Length(180), TreadDepth: engineering.Length(270)},
		Price: &domprc.PriceBreakdown{
			Currency:       cur,
			Material:       domprc.NewMoney(material),
			Machine:        domprc.NewMoney(machine),
			Labor:          domprc.NewMoney(labor),
			Overhead:       domprc.NewMoney(overhead),
			ProductionCost: pc,
			Margin:         domprc.NewMoney(margin),
			FinalPrice:     domprc.NewMoney(final),
		},
		Cost: &dommfg.ManufacturingCostDataset{PartArea: 1e6, Mass: 100},
	}
}

func TestPricingAnalysisBalanced(t *testing.T) {
	// Материал 40%, мажжа 15% — внутри порогов.
	svc := NewService(&prcCalc{res: prcResult(4000, 2000, 2000, 1000, 1500, 11500)}, nil)
	res, err := svc.Ask(context.Background(), "t1", "u1", KindPricing, AnalysisRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if res.Kind != KindPricing {
		t.Fatalf("kind = %q", res.Kind)
	}
	for _, f := range res.Response.Findings {
		if f.Severity == "warning" {
			t.Fatalf("unexpected warning: %+v", f)
		}
	}
	if res.Response.Rating <= 0.8 {
		t.Fatalf("rating = %v, want high for balanced price", res.Response.Rating)
	}
	if !strings.Contains(res.Response.Recommendation, "сбалансирована") {
		t.Fatalf("recommendation should say balanced, got %q", res.Response.Recommendation)
	}
}

func TestPricingMaterialHeavy(t *testing.T) {
	// Материал 80% себестоимости → warning + suggestion.
	svc := NewService(&prcCalc{res: prcResult(8000, 500, 500, 1000, 1000, 11000)}, nil)
	res, err := svc.Ask(context.Background(), "t1", "u1", KindPricing, AnalysisRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !hasWarning(res.Response.Findings, "материалы") {
		t.Fatalf("expected material-heavy warning, got %+v", res.Response.Findings)
	}
	if len(res.Response.Suggestions) == 0 {
		t.Fatal("expected suggestions for material-heavy price")
	}
}

func TestPricingThinMargin(t *testing.T) {
	// Маржа 5% → warning.
	svc := NewService(&prcCalc{res: prcResult(3000, 1000, 1000, 500, 282, 5762)}, nil)
	res, err := svc.Ask(context.Background(), "t1", "u1", KindPricing, AnalysisRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !hasWarning(res.Response.Findings, "маржа") {
		t.Fatalf("expected thin-margin warning, got %+v", res.Response.Findings)
	}
}

func TestPricingBlocked(t *testing.T) {
	res := &stair.Result{
		Validation: validation.Result{Blocking: true},
		Flight:     solver.FlightResult{StepCount: 15, StepHeight: engineering.Length(180), TreadDepth: engineering.Length(270)},
	}
	svc := NewService(&prcCalc{res: res}, nil)
	out, err := svc.Ask(context.Background(), "t1", "u1", KindPricing, AnalysisRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if out.Response.Rating != 0 {
		t.Fatalf("rating = %v, want 0 on blocked", out.Response.Rating)
	}
}

func hasWarning(findings []Finding, element string) bool {
	for _, f := range findings {
		if f.Element == element && f.Severity == "warning" {
			return true
		}
	}
	return false
}

func TestRouterPrimaryUsed(t *testing.T) {
	r := NewModelRouter(stubBackend{text: "from llm"}, localComment)
	ans, err := r.Infer(context.Background(), &intent{Kind: "design", UserTask: "k", Context: "c"}, &Response{Recommendation: "r"})
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if ans.Text != "from llm" {
		t.Fatalf("text = %q, want from llm", ans.Text)
	}
}

func TestRouterFallbackOnPrimaryFail(t *testing.T) {
	r := NewModelRouter(stubBackend{err: errors.New("api down")}, localComment)
	resp := &Response{Recommendation: "Рекомендуется прямой марш"}
	ans, err := r.Infer(context.Background(), &intent{Kind: "design", UserTask: "k", Context: "c"}, resp)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if !strings.Contains(ans.Text, "прямой марш") {
		t.Fatalf("fallback text should contain recommendation, got %q", ans.Text)
	}
}

func TestRouterLocalOnly(t *testing.T) {
	r := NewModelRouter(nil, localComment)
	ans, err := r.Infer(context.Background(), &intent{Kind: "design"}, &Response{Recommendation: "x"})
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if ans.Text == "" {
		t.Fatal("local-only router must return non-empty text")
	}
}

// ---- OpenAI backend ----

func TestOpenAIInfer(t *testing.T) {
	var gotModel, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "первичный комментарий"}},
			},
		})
	}))
	defer srv.Close()

	o, err := NewOpenAI(OpenAIConfig{BaseURL: srv.URL, APIKey: "secret", Model: "gpt-test"})
	if err != nil {
		t.Fatalf("NewOpenAI: %v", err)
	}
	ans, err := o.Infer(context.Background(), Prompt{UserTask: "задача", Context: "контекст"})
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if ans.Text != "первичный комментарий" {
		t.Fatalf("text = %q", ans.Text)
	}
	if gotModel != "gpt-test" || gotAuth != "Bearer secret" {
		t.Fatalf("model=%q auth=%q", gotModel, gotAuth)
	}
}

func TestOpenAIErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	o, err := NewOpenAI(OpenAIConfig{BaseURL: srv.URL, Model: "m"})
	if err != nil {
		t.Fatalf("NewOpenAI: %v", err)
	}
	if _, err := o.Infer(context.Background(), Prompt{UserTask: "t", Context: "c"}); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestOpenAIServiceIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "LLM-комментарий"}},
			},
		})
	}))
	defer srv.Close()

	calc := &fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{
		engineering.FlightStraight: optResult(engineering.FlightStraight, 100, 630),
	}}
	svc := NewService(calc, nil)
	o, _ := NewOpenAI(OpenAIConfig{BaseURL: srv.URL, Model: "m"})
	svc = svc.WithPrimaryBackend(o)

	res, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if res.Commentary != "LLM-комментарий" {
		t.Fatalf("commentary = %q, want LLM-комментарий", res.Commentary)
	}
}
