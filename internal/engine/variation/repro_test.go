package variation

import (
	"context"
	"math"
	"strconv"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	enggeo "stairplatform/internal/engine/geometry"
	"stairplatform/internal/engine/validation"

	"stairplatform/internal/geometry"
)

// baseCfg — минимальный корректный конфиг с параметрами по умолчанию;
// тесты доопределяют нужные поля.
func baseCfg() *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width:             engineering.Length(1000),
		Height:            engineering.Length(3000),
		Clearance:         engineering.Length(50),
		RailingHeight:     engineering.Length(900),
		StringerThickness: engineering.Length(40),
		StepThickness:     engineering.Length(40),
		ApproachSpace:     engineering.Length(1000),
	}
}

// hasRoomFit возвращает сообщение room_fit-проблемы исходного конфига, если
// марш не помещается в заданную комнату, иначе "".
func hasRoomFit(t *testing.T, c *engineering.StairConfiguration) string {
	t.Helper()
	issues := buildRoomFit(c)
	for _, iss := range issues {
		return iss.Message
	}
	return ""
}

// buildRoomFit возвращает неблокирующие room_fit-проблемы из движка геометрии.
func buildRoomFit(c *engineering.StairConfiguration) []geometry.ValidationIssue {
	g, err := enggeo.Generate(context.Background(), c)
	if err != nil {
		return nil
	}
	var out []geometry.ValidationIssue
	for _, iss := range g.Issues {
		if iss.Code == "room_fit" {
			out = append(out, iss)
		}
	}
	return out
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestRepro_StraightSteeperApplyClearsRoomFit — регрессия багов витрины:
//  1. вариант «сделать круче» (A) для прямого марша, не помещающегося по
//     длине, должен предлагаться и при применении реально убирать room_fit;
//  2. comfortStepMM / stepHeightMM варианта должны лежать в диапазоне
//     фронтенда (иначе applyVariation падает на validateForm и расчёт
//     «не пересчитывается»);
//  3. другие типы (L/П) должны предлагаться для прямого room_fit, если
//     влезают (раньше из-за LowerStepCount=0 и принудительного StepHeight
//     они вообще не генерировались).
func TestRepro_StraightSteeperApplyClearsRoomFit(t *testing.T) {
	set := constraint.StandardProfile("STANDARD")
	c := baseCfg()
	c.Flight = engineering.FlightStraight
	c.Height = engineering.Length(3000)
	c.Width = engineering.Length(1000)
	c.StepHeight = engineering.Length(180)
	c.TreadDepth = engineering.Length(280)
	c.StepCount = 17
	c.Length = engineering.Length(640)
	c.LandingWidth = engineering.Length(0)
	c.RoomWidth = engineering.Length(5500)
	c.RoomLength = engineering.Length(5500)
	c.ApproachSpace = engineering.Length(1000)

	if msg := hasRoomFit(t, c); msg == "" {
		t.Fatalf("предусловие: ожидали room_fit для исходного прямого марша 5500")
	}

	vars := ForRoomFit(context.Background(), c, set)
	if len(vars) == 0 {
		t.Fatalf("ожидали варианты, получили 0")
	}

	var aVar validation.Variation
	foundA := false
	flightSet := map[string]bool{}
	for _, v := range vars {
		flightSet[v.Config["flight"]] = true
		if cs, ok := v.Config["comfortStepMM"]; ok {
			f, _ := strconv.ParseFloat(cs, 64)
			if f < 600 || f > 640 {
				t.Errorf("вариант %q comfortStepMM=%s вне [600,640] -> validateForm заблокирует применение", v.Title, cs)
			}
		}
		if ss, ok := v.Config["stepHeightMM"]; ok {
			f, _ := strconv.ParseFloat(ss, 64)
			if f < 150 || f > 200 {
				t.Errorf("вариант %q stepHeightMM=%s вне [150,200] -> validateForm заблокирует применение", v.Title, ss)
			}
		}
		if contains(v.Title, "круче") {
			aVar = v
			foundA = true
		}
	}
	if !foundA {
		t.Fatalf("вариант «сделать круче» не предложен; offered flights=%v", flightSet)
	}
	// U должен предлагаться для прямого room_fit, когда он объективно влезает
	// (его длина марша короче, чем у высокого прямого — он использует
	// меньшую высоту ступени через площадку).
	if !flightSet["u_shape"] {
		t.Errorf("П-образный вариант не предложен для прямого room_fit: %v", flightSet)
	}
	if !flightSet["l_shape"] {
		t.Errorf("L-образный вариант не предложен для прямого room_fit: %v", flightSet)
	}

	applied, fits, _, _, _, _ := applyVariantConfigToCfg(t, c, aVar)
	if !fits {
		t.Errorf("после применения «круче» лестница не помещается (fits=false)")
	}
	if msg := hasRoomFit(t, applied); msg != "" {
		t.Errorf("после применения «круче» room_fit остался: %s", msg)
	}
}

// TestRepro_LShapeRoomFitOffersU — для L-образного исходника на большой
// комнате П-образный вариант должен предлагаться, когда он реально влезает.
func TestRepro_LShapeRoomFitOffersU(t *testing.T) {
	set := constraint.StandardProfile("STANDARD")
	for side := 5000; side >= 3600; side -= 200 {
		c := baseCfg()
		c.Flight = engineering.FlightLShape
		c.Height = engineering.Length(3000)
		c.Width = engineering.Length(1000)
		c.StepHeight = engineering.Length(180)
		c.TreadDepth = engineering.Length(280)
		c.LandingWidth = engineering.Length(1000)
		c.LandingDepth = engineering.Length(1000)
		c.LowerStepCount = 9
		c.StepCount = 18
		c.Length = engineering.Length(640)
		c.RoomWidth = engineering.Length(side)
		c.RoomLength = engineering.Length(side)
		c.ApproachSpace = engineering.Length(1000)

		vars := ForRoomFit(context.Background(), c, set)
		flights := map[string]bool{}
		for _, v := range vars {
			flights[v.Config["flight"]] = true
		}
		// проверяем независимо, влезает ли П напрямую
		uCanFit := false
		{
			cu := cloneCfg(c)
			cu.Flight = engineering.FlightUShape
			cu.Width = engineering.Length(1000)
			cu.Length = engineering.Length(640)
			if vs := buildOtherTypes(context.Background(), cu, engineering.FlightUShape, cu.Height, 640, set, float64(side), float64(side), fitEps); len(vs) > 0 && vs[0].Fits {
				uCanFit = true
			}
		}
		if uCanFit && !flights["u_shape"] {
			t.Errorf("комната %dx%d: П-образный объективно влезает, но не предложен (offered=%v)", side, side, flights)
		}
		if !uCanFit && flights["u_shape"] {
			t.Errorf("комната %dx%d: П-образный НЕ влезает, но предложен (offered=%v)", side, side, flights)
		}
		t.Logf("room %dx%d: offered=%v uCanFitDirectly=%v", side, side, flights, uCanFit)
	}
}

func atof(t *testing.T, s string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("parse float %q: %v", s, err)
	}
	return f
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("parse int %q: %v", s, err)
	}
	return n
}

// applyVariantConfigToCfg воспроизводит на уровне движка то, что делает
// фронтенд при клике «применить вариант»: сливает поля варианта в исходный
// конфиг, пересобирает геометрию и проверяет, что марш реально помещается.
// Возвращает итоговый конфиг, fits, размеры bbox и комнаты.
func applyVariantConfigToCfg(t *testing.T, base *engineering.StairConfiguration, v validation.Variation) (*engineering.StairConfiguration, bool, float64, float64, float64, float64) {
	t.Helper()
	cfg := cloneCfg(base)
	if f, ok := v.Config["flight"]; ok {
		cfg.Flight = engineering.FlightType(f)
	}
	if f, ok := v.Config["widthMM"]; ok {
		cfg.Width = engineering.Length(atof(t, f))
	}
	if f, ok := v.Config["landingWidthMM"]; ok {
		cfg.LandingWidth = engineering.Length(atof(t, f))
	}
	if f, ok := v.Config["landingDepthMM"]; ok {
		cfg.LandingDepth = engineering.Length(atof(t, f))
	}
	if f, ok := v.Config["stepHeightMM"]; ok {
		cfg.StepHeight = engineering.Length(atof(t, f))
	}
	if cfg.StepHeight.Millimeters() > 0 {
		cfg.StepCount = int(math.Round(cfg.Height.Millimeters() / cfg.StepHeight.Millimeters()))
	}
	if f, ok := v.Config["comfortStepMM"]; ok {
		// проступь пересчитывается из шага комфорта и высоты ступени, как
		// делает фронтенд при applyVariation.
		comfort := atof(t, f)
		cfg.TreadDepth = engineering.Length(comfort - 2*cfg.StepHeight.Millimeters())
		cfg.Length = engineering.Length(comfort)
	}
	if f, ok := v.Config["lowerStepCountMM"]; ok {
		cfg.LowerStepCount = atoi(t, f)
	}
	if f, ok := v.Config["turnKind"]; ok {
		cfg.TurnKind = engineering.TurnKind(f)
	}
	if f, ok := v.Config["winderCount"]; ok {
		cfg.WinderCount = atoi(t, f)
	}
	g, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Logf("applyVariantConfigToCfg generate err: %v", err)
		return cfg, false, 0, 0, cfg.RoomWidth.Millimeters(), cfg.RoomLength.Millimeters()
	}
	bbX := g.Measurement.BoundingBox.Max.X
	bbY := g.Measurement.BoundingBox.Max.Y
	fits := bbX <= cfg.RoomWidth.Millimeters()+1e-6 && bbY <= cfg.RoomLength.Millimeters()+1e-6
	return cfg, fits, bbX, bbY, cfg.RoomWidth.Millimeters(), cfg.RoomLength.Millimeters()
}

// TestForRoomFit_NarrowLandingAppliesForLU — регресс: для узкого марша
// (ширина < ~545 мм) варианты L/П ранее генерировали площадку < 600 мм
// (П-образный — вообще с глубиной 0), из-за чего форма фронтенда не проходила
// validateForm и вариант «не применялся» (применялась только спираль).
// Теперь formFromConfig и генератор гарантируют landing ≥ max(600, width).
func TestForRoomFit_NarrowLandingAppliesForLU(t *testing.T) {
	set := constraint.StandardProfile("STANDARD")
	for _, width := range []float64{500, 800} {
		c := baseCfg()
		c.Flight = engineering.FlightStraight
		c.Width = engineering.Length(width)
		c.Height = engineering.Length(3000)
		c.StepHeight = engineering.Length(180)
		c.TreadDepth = engineering.Length(280)
		c.Length = engineering.Length(640)
		c.LandingWidth = engineering.Length(0)
		c.RoomWidth = engineering.Length(5500)
		c.RoomLength = engineering.Length(5500)

		variations := ForRoomFit(context.Background(), c, set)
		for _, v := range variations {
			if v.Config["flight"] != "l_shape" && v.Config["flight"] != "u_shape" {
				continue
			}
			lw := atof(t, v.Config["landingWidthMM"])
			ld := atof(t, v.Config["landingDepthMM"])
			if lw < 600 {
				t.Fatalf("width=%v %s: landingWidthMM=%v < 600 (применение упадёт на validateForm)",
					width, v.Config["flight"], lw)
			}
			if ld < 600 {
				t.Fatalf("width=%v %s: landingDepthMM=%v < 600 (применение упадёт на validateForm)",
					width, v.Config["flight"], ld)
			}
		}
	}
}
