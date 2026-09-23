package assistant

// Eval-прогон (S-135, Этот B): набор Q/A testdata/eval.jsonl против
// детерминированной (не-LLM) части ассистента — Response. Ожидаемые факты
// проверяются как подстроки в сериализованном Response. Порог покрытия —
// доля найденных фактов от общего числа (DoD S-135: ≥ 80%).

import (
	"bufio"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
)

// evalMinCoverage — допустимая доля фактов, найденных в детерминированном
// Response (DoD S-135): агрегат по всему набору.
const evalMinCoverage = 0.8

// evalMinCaseCoverage — нижний порог отдельного кейса: ловит катастрофический
// провал вопроса, не ломая набор из-за единичных выбросов.
const evalMinCaseCoverage = 0.6

// evalCase — строка тестового набора.
type evalCase struct {
	Kind     string             `json:"kind"` // design | engineering
	Q        string             `json:"q"`
	Config   evalConfig         `json:"config"`
	Priority string             `json:"priority,omitempty"`
	Prices   map[string]float64 `json:"prices,omitempty"`
	Comfort  float64            `json:"comfort,omitempty"`
	Geometry *evalGeometry      `json:"geometry,omitempty"`
	Facts    []string           `json:"facts"`
}

type evalConfig struct {
	WidthMM             int    `json:"width_mm"`
	HeightMM            int    `json:"height_mm"`
	Flight              string `json:"flight"`
	OuterRadiusMM       int    `json:"outer_radius_mm"`
	ClearanceMM         int    `json:"clearance_mm,omitempty"`
	StringerThicknessMM int    `json:"stringer_thickness_mm,omitempty"`
	RailingHeightMM     int    `json:"railing_height_mm,omitempty"`
}

// evalGeometry — детерминированная геометрия для kind=engineering.
type evalGeometry struct {
	StepHeightMM float64 `json:"step_height_mm"`
	TreadMM      float64 `json:"tread_mm"`
	AngleDeg     float64 `json:"angle_deg"`
}

// evalDesignCalc — калькулятор design-кейсов: оптимум по ценам из кейса.
type evalDesignCalc struct {
	optByFlight map[engineering.FlightType]*stair.OptimizeResult
}

func (e *evalDesignCalc) Calculate(context.Context, stair.Config, stair.Options) (*stair.Result, error) {
	return nil, errNoCalc
}

func (e *evalDesignCalc) Optimize(_ context.Context, cfg stair.Config, _ stair.Options, _ stair.OptimizeRequest) (*stair.OptimizeResult, error) {
	o, ok := e.optByFlight[cfg.Flight]
	if !ok {
		return &stair.OptimizeResult{Valid: false}, nil
	}
	return o, nil
}

func (e *evalDesignCalc) ValidateConfig(stair.Config) error { return nil }

// evalEngCalc — калькулятор engineering-кейсов: фиксированный Result.
type evalEngCalc struct {
	res *stair.Result
}

func (e *evalEngCalc) Calculate(context.Context, stair.Config, stair.Options) (*stair.Result, error) {
	return e.res, nil
}

func (e *evalEngCalc) Optimize(context.Context, stair.Config, stair.Options, stair.OptimizeRequest) (*stair.OptimizeResult, error) {
	return nil, errNoCalc
}

func (e *evalEngCalc) ValidateConfig(stair.Config) error { return nil }

var errNoCalc = errNoCalcT{}

type errNoCalcT struct{}

func (errNoCalcT) Error() string { return "eval: calculate not in scope" }

// evalResult — результат прогона одного кейса.
type evalResult struct {
	q       string
	covered int
	total   int
	missed  []string
}

func (r evalResult) ok() bool {
	return r.total > 0 && r.covered >= int(math.Ceil(float64(r.total)*evalMinCaseCoverage))
}

// TestEvalDeterministicCoverage — прогон eval-набора и проверка порога.
func TestEvalDeterministicCoverage(t *testing.T) {
	cases := loadEvalCases(t)

	total, covered := 0, 0
	failing := 0
	for i, c := range cases {
		got := runEvalCase(t, c)
		total += got.total
		covered += got.covered
		if !got.ok() {
			failing++
			t.Errorf("eval case %d (%s): coverage %d/%d < %d%%; missed: %v",
				i, c.Q, got.covered, got.total, int(evalMinCoverage*100), got.missed)
		} else {
			t.Logf("ok case %d: %s (%d/%d)", i, c.Q, got.covered, got.total)
		}
	}

	rate := float64(covered) / float64(total)
	t.Logf("eval summary: %d cases, facts %d/%d = %.0f%% (threshold %d%%)",
		len(cases), covered, total, rate*100, int(evalMinCoverage*100))
	if rate < evalMinCoverage {
		t.Fatalf("eval coverage %.0f%% < %d%%, failing cases = %d", rate*100, int(evalMinCoverage*100), failing)
	}
	if len(cases) < 30 {
		t.Fatalf("eval set too small: %d cases, want >= 30", len(cases))
	}
}

func loadEvalCases(t *testing.T) []*evalCase {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "eval.jsonl"))
	if err != nil {
		t.Fatalf("open eval.jsonl: %v", err)
	}
	defer func() { _ = f.Close() }()

	var out []*evalCase
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var c evalCase
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			t.Fatalf("eval.jsonl line %d: %v", line, err)
		}
		if len(c.Facts) == 0 {
			t.Fatalf("eval.jsonl line %d: no facts", line)
		}
		out = append(out, &c)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read eval.jsonl: %v", err)
	}
	return out
}

// runEvalCase выполняет кейс детерминированно (локальный бэкенд, без
// RAG/памяти — их покрывают отдельные тесты) и сверяет факты.
func runEvalCase(t *testing.T, c *evalCase) evalResult {
	t.Helper()
	cfg := c.Config.toStairConfig()

	var res *Result
	var err error
	switch c.Kind {
	case "design":
		calc := &evalDesignCalc{optByFlight: make(map[engineering.FlightType]*stair.OptimizeResult)}
		if len(c.Prices) == 0 {
			t.Fatalf("case %q: design requires prices", c.Q)
		}
		for fl, p := range c.Prices {
			calc.optByFlight[engineering.FlightType(fl)] = optResult(engineering.FlightType(fl), p, c.Comfort)
		}
		svc := NewService(calc, nil)
		res, err = svc.Ask(context.Background(), "t-eval", "u-eval", KindDesign, DesignRequest{
			Config:       cfg,
			Preferences:  DesignPreferences{Priority: DesignPriority(c.Priority)},
			HistoryLimit: -1,
		})
	case "engineering":
		if c.Geometry == nil {
			t.Fatalf("case %q: engineering requires geometry", c.Q)
		}
		g := c.Geometry
		calc := &evalEngCalc{res: &stair.Result{
			Validation: validation.Result{Valid: true},
			Flight: solver.FlightResult{
				StepCount:  15,
				StepHeight: engineering.Length(g.StepHeightMM),
				TreadDepth: engineering.Length(g.TreadMM),
				Angle:      engineering.Angle(g.AngleDeg * math.Pi / 180),
			},
		}}
		svc := NewService(calc, nil)
		res, err = svc.Ask(context.Background(), "t-eval", "u-eval", KindEngineering, AnalysisRequest{
			Config:       cfg,
			HistoryLimit: -1,
		})
	default:
		t.Fatalf("case %q: unknown kind %q", c.Q, c.Kind)
	}
	if err != nil {
		t.Fatalf("case %q: Ask: %v", c.Q, err)
	}

	haystack := serializeResponse(&res.Response)
	out := evalResult{q: c.Q, total: len(c.Facts)}
	for _, f := range c.Facts {
		if strings.Contains(haystack, f) {
			out.covered++
		} else {
			out.missed = append(out.missed, f)
		}
	}
	return out
}

// serializeResponse склеивает ВСЮ детерминированную часть Response в один
// haystack для поиска фактов.
func serializeResponse(r *Response) string {
	var b strings.Builder
	b.WriteString(r.Recommendation)
	b.WriteString("\n")
	for _, f := range r.Findings {
		b.WriteString(f.Message)
		b.WriteString("\n")
	}
	for _, s := range r.Suggestions {
		b.WriteString(s.Message)
		b.WriteString("\n")
	}
	for _, tr := range r.Tradeoffs {
		b.WriteString(tr)
		b.WriteString("\n")
	}
	for _, a := range r.Alternatives {
		b.WriteString(a.Title)
		b.WriteString(" ")
		b.WriteString(a.Reason)
		b.WriteString("\n")
	}
	for _, n := range r.Notes {
		b.WriteString(n)
		b.WriteString("\n")
	}
	return b.String()
}

func (c evalConfig) toStairConfig() stair.Config {
	cfg := stair.Config{
		Width:  engineering.Length(c.WidthMM),
		Height: engineering.Length(c.HeightMM),
		Flight: engineering.FlightType(c.Flight),
	}
	if c.OuterRadiusMM > 0 {
		cfg.OuterRadius = engineering.Length(c.OuterRadiusMM)
	}
	if c.ClearanceMM > 0 {
		cfg.Clearance = engineering.Length(c.ClearanceMM)
	}
	if c.StringerThicknessMM > 0 {
		cfg.StringerThickness = engineering.Length(c.StringerThicknessMM)
	}
	if c.RailingHeightMM > 0 {
		cfg.RailingHeight = engineering.Length(c.RailingHeightMM)
	}
	return cfg
}
