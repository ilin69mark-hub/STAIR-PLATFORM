package stair_test

import (
	"context"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
)

// SOLVER-03 (forensic 2026-09-27) — landing_depth_mm не должен теряться.
//
// БЫЛО: SolveCheckedLShape вызывал res.Apply(cfg) ДО чтения cfg.LandingDepth.
// Apply пишет `cfg.LandingDepth = r.LandingDepth`, а решатель это поле не
// заполняет, поэтому оно обнулялось, и следующая строка читала 0. Дальше
// «при 0 — равна ширине марша» превращало ЛЮБОЙ вход в 900 мм:
//
//	вход 1200 → ответ 900      вход 1600 → ответ 900
//
// Пользовательский параметр принимался DTO, клался в конфиг — и молча
// исчезал. Площадка всегда была квадратной W×W.
//
// СТАЛО: эффективная глубина читается до Apply, а в конфиг кладётся
// эффективное значение.
func TestSOLVER003_LandingDepthIsHonored(t *testing.T) {
	s := stair.NewService()
	for _, tc := range []struct{ in, want float64 }{
		{0, 900},   // доменное умолчание: 0 → ширина марша
		{900, 900}, // ровно ширина марша
		{1100, 1100},
		{1400, 1400},
		{1800, 1800},
	} {
		c := stair.Config{
			Width: engineering.Length(900), Height: engineering.Length(2700),
			Flight: engineering.FlightLShape, StepHeight: engineering.Length(180),
			StringerThickness: engineering.Length(50), StepThickness: engineering.Length(40),
			Clearance: engineering.Length(2500), RailingHeight: engineering.Length(900),
			LandingWidth: engineering.Length(1200), LandingDepth: engineering.Length(tc.in),
			LowerStepCount: 7, RoomWidth: engineering.Length(6000), RoomLength: engineering.Length(3000),
		}
		res, err := s.Calculate(context.Background(), c, stair.Options{})
		if err != nil {
			t.Fatalf("Calculate(landing_depth=%.0f): %v", tc.in, err)
		}
		if res.LShape == nil {
			t.Fatalf("landing_depth=%.0f: LShape=nil", tc.in)
		}
		if got := res.LShape.LandingDepth.Millimeters(); got != tc.want {
			t.Errorf("SOLVER-03: вход landing_depth_mm=%.0f → результат %.0f, ожидалось %.0f",
				tc.in, got, tc.want)
		}
	}
}

// TestSOLVER003_LandingDepthBelowFlightWidthIsRejected — после починки
// значение доходит до доменной валидации, которая требует ld >= Width
// (площадка должна перекрывать поворот по всей ширине марша). Правило
// было написано, но НИКОГДА не срабатывало по той же причине, что и SOLVER-03:
// проверялось обнулённое значение. Теперь оно живое, и негодный вход
// отвергается явно вместо молчаливой подмены.
func TestSOLVER003_LandingDepthBelowFlightWidthIsRejected(t *testing.T) {
	s := stair.NewService()
	c := stair.Config{
		Width: engineering.Length(900), Height: engineering.Length(2700),
		Flight: engineering.FlightLShape, StepHeight: engineering.Length(180),
		StringerThickness: engineering.Length(50), StepThickness: engineering.Length(40),
		Clearance: engineering.Length(2500), RailingHeight: engineering.Length(900),
		LandingWidth: engineering.Length(1200), LandingDepth: engineering.Length(600),
		LowerStepCount: 7, RoomWidth: engineering.Length(6000), RoomLength: engineering.Length(3000),
	}
	res, err := s.Calculate(context.Background(), c, stair.Options{})
	if err != nil {
		// Ошибка — тоже корректный исход: вход отвергнут.
		return
	}
	if !res.Validation.Blocking {
		t.Error("landing_depth_mm=600 при width_mm=900 обязан отвергаться, а не подменяться на 900")
	}
}
