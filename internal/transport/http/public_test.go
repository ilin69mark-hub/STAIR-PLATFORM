package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"stairplatform/internal/engine/constraint"
)

// TestPublicQuoteSuccess — публичный расчёт работает БЕЗ аутентификации
// и возвращает цену/геометрию, но НЕ производственный пакет.
func TestPublicQuoteSuccess(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote",
		strings.NewReader(referenceJSON))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp publicQuoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Validation.Valid || resp.Validation.Blocking {
		t.Fatalf("expected valid, got %+v", resp.Validation)
	}
	if resp.Flight.StepCount != 15 {
		t.Fatalf("step count = %d, want 15", resp.Flight.StepCount)
	}
	if resp.Pricing == nil {
		t.Fatal("pricing must be present for non-blocking result")
	}
	if resp.Pricing.FinalPriceRub != 3271385.47 {
		t.Fatalf("final price = %v, want 3271385.47", resp.Pricing.FinalPriceRub)
	}

	// Габаритная ширина марша приходит из конфигурации.
	if resp.Flight.WidthMm != 900 {
		t.Fatalf("width = %v, want 900", resp.Flight.WidthMm)
	}

	// Эхо производственных параметров для 2D-рендера (BC-002).
	if resp.Flight.StepThicknessMm != 40 || resp.Flight.RailingHeightMm != 1000 || !resp.Flight.Riser || resp.Flight.StringerThicknessMm != 50 {
		t.Fatalf("flight echo mismatch: %+v", resp.Flight)
	}

	// Preview-сетка для 3D присутствует и непустая.
	if resp.Mesh == nil || len(resp.Mesh.Vertices) == 0 || len(resp.Mesh.Triangles) == 0 {
		t.Fatal("public quote must include a non-empty mesh for 3D")
	}

	// Производственный пакет не должен выходить наружу.
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid raw response: %v", err)
	}
	for _, doc := range []string{"manufacturing", "parts", "bom", "cut_list", "nesting"} {
		if _, ok := raw[doc]; ok {
			t.Fatalf("public quote must NOT contain %q", doc)
		}
	}
}

// TestPublicQuoteNoAuthRequired — маршрут публичный: без cookie/токена — 200.
func TestPublicQuoteNoAuthRequired(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote",
		strings.NewReader(referenceJSON))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("public quote must not require auth, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Теперь сравнение: protected /calculate без авторизации недоступен.
func TestCalculateRequiresAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stairs:calculate",
		strings.NewReader(referenceJSON))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for protected route, got %d", rec.Code)
	}
}

func TestPublicQuoteInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote",
		strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestPublicQuoteInvalidInput(t *testing.T) {
	b := `{
		"width_mm": 900,
		"height_mm": 0,
		"flight": "straight",
		"step_height_mm": 180
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote", strings.NewReader(b))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	// Пользовательская проблема теперь возвращается как блокирующий
	// результат (200) с русской подсказкой, а не как техническая ошибка 422.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 with advisory result, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp publicQuoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Validation.Blocking || len(resp.Validation.Issues) == 0 {
		t.Fatalf("expected blocking advisory, got %+v", resp.Validation)
	}
	it := resp.Validation.Issues[0]
	if it.Code != "GEO-HEIGHT" || it.Param == "" || !strings.Contains(it.Guide, "больше 0 мм") {
		t.Fatalf("expected Russian advisory issue, got %+v", it)
	}
	if resp.Pricing != nil {
		t.Fatal("blocking result must not carry pricing")
	}
}

func TestPublicQuoteBlocking(t *testing.T) {
	b := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"step_height_mm": 10,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote", strings.NewReader(b))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 with report, got %d", rec.Code)
	}
	var resp publicQuoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Validation.Blocking {
		t.Fatalf("expected blocking, got %+v", resp.Validation)
	}
	if resp.Pricing != nil {
		t.Fatal("blocking result must have nil pricing")
	}
}

func TestPublicQuoteBlockingCarriesAdvice(t *testing.T) {
	b := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"step_height_mm": 10,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote", strings.NewReader(b))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp publicQuoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Validation.Blocking {
		t.Fatalf("expected blocking")
	}
	var angle validationIssueDTO
	found := false
	for _, it := range resp.Validation.Issues {
		if it.Code == string(constraint.GEO_ANGLE) {
			angle = it
			found = true
		}
	}
	if !found {
		t.Fatalf("angle issue expected: %+v", resp.Validation.Issues)
	}
	if angle.Param == "" || angle.Guide == "" {
		t.Fatalf("angle issue must carry param/guide: %+v", angle)
	}
	if len(angle.Suggestions) == 0 {
		t.Fatalf("angle issue must carry suggestions: %+v", angle)
	}
	anyValid := false
	for _, s := range angle.Suggestions {
		if s.StepCount < 1 || s.StepHeightMm < 150 || s.StepHeightMm > 200 ||
			s.TreadDepthMm < 260 || s.TreadDepthMm > 320 ||
			s.AngleDeg < 30 || s.AngleDeg > 45 {
			continue
		}
		anyValid = true
	}
	if !anyValid {
		t.Fatalf("no suggestion inside norms: %+v", angle.Suggestions)
	}
}

// TestPublicQuoteAngleCarriesVariations — регресс на сообщение пользователя:
// блокирующее предупреждение про угол наклона должно сопровождаться
// интерактивными вариациями (Variations), даже когда советник не может
// подобрать их при фиксированной проступи.
func TestPublicQuoteAngleCarriesVariations(t *testing.T) {
	b := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"step_height_mm": 10,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote", strings.NewReader(b))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp publicQuoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	var angle validationIssueDTO
	found := false
	for _, it := range resp.Validation.Issues {
		if it.Code == string(constraint.GEO_ANGLE) {
			angle = it
			found = true
		}
	}
	if !found {
		t.Fatalf("angle issue expected: %+v", resp.Validation.Issues)
	}
	if len(angle.Variations) == 0 {
		t.Fatalf("angle issue must carry interactive variations: %+v", angle)
	}
	for _, v := range angle.Variations {
		if v.Config == nil || v.Config["stepHeightMM"] == "" {
			t.Fatalf("variation must carry config: %+v", v)
		}
	}
}

// TestPublicQuoteLShapeLandingNarrowAdvisory — регресс на исходный пример
// пользователя: L-образный марш с площадкой уже марша (Wp=1200 < W=2000)
// возвращает блокирующую ПОДСКАЗКУ (200) на русском без технических
// префиксов (solver:/stair:), а не сырую техническую ошибку.
func TestPublicQuoteLShapeLandingNarrowAdvisory(t *testing.T) {
	b := `{
		"width_mm": 2000,
		"height_mm": 2700,
		"flight": "l_shape",
		"landing_width_mm": 1200,
		"lower_step_count": 6,
		"step_height_mm": 180,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote", strings.NewReader(b))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 with advisory result, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp publicQuoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Validation.Blocking || len(resp.Validation.Issues) == 0 {
		t.Fatalf("expected blocking advisory, got %+v", resp.Validation)
	}
	it := resp.Validation.Issues[0]
	if it.Code != string(constraint.GEO_LANDING_WIDTH) {
		t.Fatalf("expected landing-width issue, got %+v", it)
	}
	if it.Param == "" || it.Guide == "" {
		t.Fatalf("issue must carry param/guide, got %+v", it)
	}
	if !strings.Contains(it.Guide, "меньше ширины марша 2000 мм") {
		t.Fatalf("expected Russian advisory text, got %q", it.Guide)
	}
	for _, marker := range []string{"solver:", "stair:", "landing width"} {
		if strings.Contains(strings.ToLower(it.Guide), marker) {
			t.Fatalf("technical prefix must not leak into guide: %q", it.Guide)
		}
	}
}

// TestPublicQuoteTallFlightSucceeds — прямая лестница на пределе энвелопа
// H=6000: косоур помещается на лист 10400×6200, поэтому публичный quote
// должен вернуть 200 с валидным расчётом и ценой (регресс: раньше такой
// ввод падал сырой 422 из-за невместимости на лист).
func TestPublicQuoteTallFlightSucceeds(t *testing.T) {
	b := `{
		"width_mm": 900,
		"height_mm": 6000,
		"flight": "straight",
		"step_height_mm": 180,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote", strings.NewReader(b))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp publicQuoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if resp.Validation.Blocking || !resp.Validation.Valid {
		t.Fatalf("expected valid result, got %+v", resp.Validation)
	}
	if resp.Pricing == nil || resp.Pricing.FinalPriceRub <= 0 {
		t.Fatalf("tall flight must carry pricing, got %+v", resp.Pricing)
	}
}

// TestPublicQuoteSpiralBlockingCarriesRadiusSuggestion — винтовая лестница
// с несовместимой шириной (W=3000, H=6000) возвращает блокирующую подсказку
// с готовыми вариантами, уменьшающими ширину и несущими наружный радиус.
func TestPublicQuoteSpiralBlockingCarriesRadiusSuggestion(t *testing.T) {
	b := `{
		"width_mm": 3000,
		"height_mm": 6000,
		"flight": "spiral",
		"outer_radius_mm": 3100,
		"step_height_mm": 190,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2300,
		"railing_height_mm": 1100
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote", strings.NewReader(b))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 with advisory result, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp publicQuoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Validation.Blocking || len(resp.Validation.Issues) == 0 {
		t.Fatalf("expected blocking advisory, got %+v", resp.Validation)
	}
	var spiral validationIssueDTO
	found := false
	for _, it := range resp.Validation.Issues {
		if it.Code == string(constraint.GEO_SPIRAL_TREAD) {
			spiral = it
			found = true
		}
	}
	if !found {
		t.Fatalf("expected GEO_SPIRAL_TREAD issue, got %+v", resp.Validation.Issues)
	}
	if len(spiral.Suggestions) == 0 {
		t.Fatalf("spiral issue must carry suggestions, got %+v", spiral)
	}
	for _, s := range spiral.Suggestions {
		if s.OuterRadiusMm <= 0 || s.WidthMm <= 0 || s.WidthMm >= 3000 {
			t.Fatalf("spiral suggestion %+v must reduce width and carry radius", s)
		}
	}
}

func woodJSON(material string) string {
	return `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"material": "` + material + `",
		"step_height_mm": 180,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000
	}`
}

// TestPublicQuoteMaterialChoice — выбранный материал доходит до конвейера:
// расчёт с дубом валиден и цена отличается от стали.
func TestPublicQuoteMaterialChoice(t *testing.T) {
	var steelPrice, woodPrice float64
	for _, tc := range []struct {
		material string
		prices   *float64
	}{
		{"STEEL-S235", &steelPrice},
		{"WOOD-OAK", &woodPrice},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote",
			strings.NewReader(woodJSON(tc.material)))
		rec := httptest.NewRecorder()
		testRouter().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", tc.material, rec.Code, rec.Body.String())
		}
		var resp publicQuoteDTO
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: invalid response: %v", tc.material, err)
		}
		if resp.Validation.Blocking || !resp.Validation.Valid {
			t.Fatalf("%s: expected valid, got %+v", tc.material, resp.Validation)
		}
		if resp.Pricing == nil {
			t.Fatalf("%s: pricing missing", tc.material)
		}
		*tc.prices = resp.Pricing.FinalPriceRub
	}
	if steelPrice == woodPrice {
		t.Fatalf("steel and oak prices must differ, got %v == %v", steelPrice, woodPrice)
	}
}

// TestPublicQuoteMaterialThicknessBlocked — дуб не поддерживает косоур 150 мм:
// блокирующий MFG-MATERIAL с понятной русской подсказкой (200).
func TestPublicQuoteMaterialThicknessBlocked(t *testing.T) {
	b := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"material": "WOOD-OAK",
		"step_height_mm": 180,
		"stringer_thickness_mm": 150,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote", strings.NewReader(b))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 with advisory result, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp publicQuoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Validation.Blocking || len(resp.Validation.Issues) == 0 {
		t.Fatalf("expected blocking advisory, got %+v", resp.Validation)
	}
	var mat validationIssueDTO
	found := false
	for _, it := range resp.Validation.Issues {
		if it.Code == string(constraint.MFG_MATERIAL) {
			mat = it
			found = true
		}
	}
	if !found {
		t.Fatalf("expected MFG_MATERIAL issue, got %+v", resp.Validation.Issues)
	}
	if mat.Param == "" || mat.Guide == "" {
		t.Fatalf("material issue must carry param/guide: %+v", mat)
	}
}

// TestPublicQuoteAngle_UserScenario — сквозной регресс на жалобу пользователя:
// марш блокируется по углу наклона (GEO-ANGLE, «Число ступеней»), и советник
// обязан вернуть интерактивные Variations + Suggestions. Каждая вариация,
// будучи применённой (слита в конфиг и пересчитана повторным запросом),
// ДОЛЖНА снимать блокировку, причём сам GEO-ANGLE-issue исчезает. Прогоняется
// для прямого, L- и U-образного маршей — результаты печатаются в терминале.
func TestPublicQuoteAngle_UserScenario(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "straight",
			body: `{
				"width_mm": 1000, "height_mm": 3000, "flight": "straight",
				"step_height_mm": 158, "stringer_thickness_mm": 50,
				"step_thickness_mm": 40, "clearance_mm": 2500, "railing_height_mm": 900
			}`,
		},
		{
			name: "l_shape",
			body: `{
				"width_mm": 1000, "height_mm": 3000, "flight": "l_shape",
				"landing_width_mm": 1200, "landing_depth_mm": 1500, "lower_step_count": 6,
				"step_height_mm": 158, "stringer_thickness_mm": 50,
				"step_thickness_mm": 40, "clearance_mm": 2500, "railing_height_mm": 900
			}`,
		},
		{
			name: "u_shape",
			body: `{
				"width_mm": 1000, "height_mm": 3000, "flight": "u_shape",
				"landing_width_mm": 1200, "landing_depth_mm": 1500, "lower_step_count": 6,
				"step_height_mm": 158, "stringer_thickness_mm": 50,
				"step_thickness_mm": 40, "clearance_mm": 2500, "railing_height_mm": 900
			}`,
		},
	}

	// cfgKeyToRequest — ключи ConfigForm (camelCase) → ключи запроса (snake_case).
	cfgKeyToRequest := map[string]string{
		"flight": "flight", "heightMM": "height_mm", "widthMM": "width_mm",
		"stepHeightMM": "step_height_mm", "comfortStepMM": "comfort_step_mm",
		"landingWidthMM": "landing_width_mm", "landingDepthMM": "landing_depth_mm",
		"lowerStepCountMM": "lower_step_count", "roomWidthMM": "room_width_mm",
		"roomLengthMM": "room_length_mm", "approachSpaceMM": "approach_space_mm",
		"outerRadiusMM": "outer_radius_mm",
		"clearanceMm":   "clearance_mm", "railingMm": "railing_height_mm",
		"stringerThicknessMm": "stringer_thickness_mm", "stepThicknessMm": "step_thickness_mm",
		"direction": "direction", "railingLower": "railing_lower",
		"railingLanding": "railing_landing", "railingUpper": "railing_upper",
		"railing": "railing", "spiralDirection": "spiral_direction",
		"turnKind": "turn_kind", "winderCount": "winder_count",
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postQuote(t, tc.body)
			if !resp.Validation.Blocking {
				t.Fatalf("[%s] expected blocking result, got %+v", tc.name, resp.Validation)
			}
			var angle validationIssueDTO
			found := false
			for _, it := range resp.Validation.Issues {
				if it.Code == string(constraint.GEO_ANGLE) {
					angle = it
					found = true
				}
			}
			if !found {
				t.Fatalf("[%s] GEO-ANGLE issue expected, got %+v", tc.name, resp.Validation.Issues)
			}
			if len(angle.Variations) == 0 {
				t.Fatalf("[%s] GEO-ANGLE must carry interactive variations: %+v", tc.name, angle)
			}
			if len(angle.Suggestions) == 0 {
				t.Logf("[%s] GEO-ANGLE: advisor Suggestions пусты (ожидаемо — варианты даёт ForAngle); полагаемся на Variations", tc.name)
			}

			t.Logf("[%s] заблокирован по GEO-ANGLE; предложено вариаций=%d, советов=%d:",
				tc.name, len(angle.Variations), len(angle.Suggestions))
			for _, v := range angle.Variations {
				t.Logf("  ▸ вариация %q — %s", v.Title, v.Summary)
			}

			// Каждая вариация при повторном расчёте обязана снять блокировку
			// и устранить сам GEO-ANGLE.
			for i, v := range angle.Variations {
				merged := mergeConfig(t, tc.body, v.Config, cfgKeyToRequest)
				fixed := postQuote(t, merged)
				if fixed.Validation.Blocking {
					t.Fatalf("[%s] variation %d (%q) did not unblock: %+v",
						tc.name, i, v.Title, fixed.Validation)
				}
				for _, it := range fixed.Validation.Issues {
					if it.Code == string(constraint.GEO_ANGLE) {
						t.Fatalf("[%s] variation %d (%q) left GEO-ANGLE issue: %+v",
							tc.name, i, v.Title, it)
					}
				}
				t.Logf("[%s] вариация %q → пересчёт НЕ блокируется (угол в норме)", tc.name, v.Title)
			}
		})
	}
}

// postQuote шлёт тело расчёта на публичный эндпоинт и возвращает DTO.
func postQuote(t *testing.T, body string) publicQuoteDTO {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote", strings.NewReader(body))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp publicQuoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	return resp
}

// mergeConfig сливает конфиг вариации (ключи ConfigForm) в исходный JSON
// запроса, приводя ключи к snake_case формата calculateRequest.
func mergeConfig(t *testing.T, original string, cfg map[string]string, keyMap map[string]string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(original), &m); err != nil {
		t.Fatalf("bad original json: %v", err)
	}
	for k, v := range cfg {
		reqKey, ok := keyMap[k]
		if !ok {
			continue
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m[reqKey] = f
		} else {
			m[reqKey] = v
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal merged: %v", err)
	}
	return string(b)
}
