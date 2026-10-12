package geometry_test

import (
	"context"
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/geometry"
	"stairplatform/internal/engine/solver"
	kerngeo "stairplatform/internal/geometry"
)

func previewCfg(t *testing.T, flight engineering.FlightType, w, h float64) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(engineering.Length(w), engineering.Length(h), flight)
	if err != nil {
		t.Fatal(err)
	}
	r, err := solver.Solve(cfg.Height, engineering.Length(180))
	if err != nil {
		t.Fatal(err)
	}
	r.Apply(cfg)
	cfg.StringerThickness = engineering.Length(40)
	cfg.StepThickness = engineering.Length(60)
	cfg.ApproachSpace = engineering.Length(1000)
	cfg.LowerStepCount = 7
	cfg.LandingWidth = engineering.Length(900)
	cfg.LandingDepth = engineering.Length(900)
	// Радиус скругления носа по умолчанию НУЛЕВОЙ — тогда скругления нет и
	// огрублять нечего. Ставим типовой R10, как в реальном изделии.
	cfg.TreadNoseRadiusMM = engineering.Length(10)
	return cfg
}

// TestPreviewMeshIsLighter — облегчённый меш легче производственного, и
// именно для этого он существует.
func TestPreviewMeshIsLighter(t *testing.T) {
	for _, flight := range []engineering.FlightType{
		engineering.FlightStraight, engineering.FlightLShape, engineering.FlightUShape,
	} {
		cfg := previewCfg(t, flight, 900, 2700)
		prod, err := geometry.Generate(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		disp, err := geometry.GeneratePreviewMesh(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		pt, dt := len(prod.Mesh.Triangles), len(disp.Mesh.Triangles)
		pv, dv := len(prod.Mesh.Vertices), len(disp.Mesh.Vertices)
		if dt >= pt {
			t.Errorf("%v: меш отображения не легче — треугольников %d против %d", flight, dt, pt)
		}
		if dv >= pv {
			t.Errorf("%v: вершин отображения не меньше — %d против %d", flight, dv, pv)
		}
		t.Logf("%-8v треугольников %d → %d (−%.0f%%), вершин %d → %d (−%.0f%%)",
			flight, pt, dt, float64(pt-dt)/float64(pt)*100,
			pv, dv, float64(pv-dv)/float64(pv)*100)
	}
}

// TestPreviewMeshCarriesNoMeasurements — главное правило S-165: из облегчённой
// сборки не берутся числа. Если кто-то начнёт брать отсюда объём или цену,
// витрина и админка разойдутся по цене — тот самый класс ошибок, который
// в проекте случался дважды. Проверяем явно.
func TestPreviewMeshCarriesNoMeasurements(t *testing.T) {
	cfg := previewCfg(t, engineering.FlightStraight, 900, 2700)
	disp, err := geometry.GeneratePreviewMesh(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if disp.Measurement.Volume != 0 || disp.Measurement.SurfaceArea != 0 {
		t.Errorf("облегчённая сборка вернула измерения: объём %v, площадь %v — их брать нельзя",
			disp.Measurement.Volume, disp.Measurement.SurfaceArea)
	}
	if disp.Measurement.SolidCount != 0 {
		t.Errorf("облегчённая сборка вернула SolidCount %d", disp.Measurement.SolidCount)
	}
	if len(disp.Issues) != 0 {
		t.Errorf("облегчённая сборка вернула issues %v — валидация должна идти по производственной модели", disp.Issues)
	}
}

// TestPreviewMeshMatchesProductionBoundingBox — габарит обязан совпасть с
// производственным: разные плотности, одна и та же лестница. Расхождение
// означало бы, что на витрине рисуется не та лестница, что посчитана.
func TestPreviewMeshMatchesProductionBoundingBox(t *testing.T) {
	for _, flight := range []engineering.FlightType{
		engineering.FlightStraight, engineering.FlightLShape, engineering.FlightUShape,
	} {
		cfg := previewCfg(t, flight, 900, 2700)
		prod, err := geometry.Generate(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		disp, err := geometry.GeneratePreviewMesh(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		// У превью измерений нет намеренно, поэтому габарит считаем по
		// вершинам меша напрямую.
		pbb := meshBBox(prod.Mesh)
		dbb := meshBBox(disp.Mesh)
		if pbb != dbb {
			t.Errorf("%v: габарит меша разошёлся: производственный %+v, отображение %+v", flight, pbb, dbb)
		}
	}
}

// TestPreviewMeshIsDeterministic — ADR-0007: два прогона дают один результат.
func TestPreviewMeshIsDeterministic(t *testing.T) {
	cfg := previewCfg(t, engineering.FlightLShape, 900, 3000)
	first, err := geometry.GeneratePreviewMesh(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 3; run++ {
		again, err := geometry.GeneratePreviewMesh(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if len(again.Mesh.Vertices) != len(first.Mesh.Vertices) ||
			len(again.Mesh.Triangles) != len(first.Mesh.Triangles) {
			t.Fatalf("прогон %d: размеры меша отличаются", run)
		}
		for i := range first.Mesh.Vertices {
			if again.Mesh.Vertices[i] != first.Mesh.Vertices[i] {
				t.Fatalf("прогон %d, вершина %d: %+v != %+v", run, i, again.Mesh.Vertices[i], first.Mesh.Vertices[i])
			}
		}
	}
}

// meshBBox — габарит по вершинам меша. У превью измерений нет намеренно,
// поэтому для проверки «та же лестница» считаем напрямую.
func meshBBox(m *kerngeo.Mesh) [6]float64 {
	if m == nil || len(m.Vertices) == 0 {
		return [6]float64{}
	}
	b := [6]float64{
		m.Vertices[0].X, m.Vertices[0].Y, m.Vertices[0].Z,
		m.Vertices[0].X, m.Vertices[0].Y, m.Vertices[0].Z,
	}
	for _, v := range m.Vertices {
		if v.X < b[0] {
			b[0] = v.X
		}
		if v.Y < b[1] {
			b[1] = v.Y
		}
		if v.Z < b[2] {
			b[2] = v.Z
		}
		if v.X > b[3] {
			b[3] = v.X
		}
		if v.Y > b[4] {
			b[4] = v.Y
		}
		if v.Z > b[5] {
			b[5] = v.Z
		}
	}
	for i := 0; i < 6; i++ {
		b[i] = math.Round(b[i]*1e6) / 1e6
	}
	return b
}
