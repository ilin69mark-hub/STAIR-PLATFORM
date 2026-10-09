package http

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// Винтовой марш удалён из проекта целиком (решение владельца 2026-10-06), и
// фича-флаг S-152 вместе с ним: отдельный код отключения «временно
// недоступно» больше не нужен, потому что типа марша просто нет.
//
// Что важно зафиксировать здесь: «spiral» не должен молча превращаться в
// прямой марш. Именно так работал код до DOM-001 — неизвестный тип марша
// трактовался как прямой, и клиент получал чужую геометрию. Теперь такой
// запрос возвращает блокирующую валидацию с перечнем допустимых типов.

const spiralRejectBody = `{
	"width_mm": 900, "height_mm": 2800, "flight": "spiral", "material": "STEEL-S235",
	"step_height_mm": 175, "step_thickness_mm": 6, "stringer_thickness_mm": 8,
	"clearance_mm": 2000, "railing_height_mm": 900
}`

// TestSpiralIsUnknownFlightType — публичные ручки отвечают блокирующей
// валидацией с кодом GEO-HEIGHT и перечислением оставшихся типов марша.
func TestSpiralIsUnknownFlightType(t *testing.T) {
	for _, path := range []string{"/api/v1/public/stairs:quote", "/api/v1/public/stairs:validate"} {
		req := httptest.NewRequest("POST", path, strings.NewReader(spiralRejectBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		testRouter().ServeHTTP(rec, req)

		if rec.Code != 200 {
			t.Fatalf("%s: want 200 с валидацией, got %d: %s", path, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"blocking":true`) {
			t.Errorf("%s: ответ должен быть блокирующим: %s", path, body)
		}
		if !strings.Contains(body, "GEO-HEIGHT") {
			t.Errorf("%s: ожидается код GEO-HEIGHT на неизвестный тип марша: %s", path, body)
		}
		if !strings.Contains(body, "Неизвестный тип лестницы") {
			t.Errorf("%s: ожидается сообщение о неизвестном типе лестницы: %s", path, body)
		}
		// Перечень допустимых типов не должен предлагать спираль.
		if strings.Contains(body, "спираль") || strings.Contains(body, "spiral (спиральный)") {
			t.Errorf("%s: в перечне типов марша не должно быть спирали: %s", path, body)
		}
		if !strings.Contains(body, "l_shape") || !strings.Contains(body, "u_shape") {
			t.Errorf("%s: перечень должен называть оставшиеся типы марша: %s", path, body)
		}
	}
}

// TestInternalCalculateRejectsSpiralToo — внутренняя ручка ведёт себя так же:
// фича-флаг был только публичным, но после удаления типа марша разница
// между ручками исчезла.
func TestInternalCalculateRejectsSpiralToo(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, authedRequest("POST", "/api/v1/stairs:calculate", spiralRejectBody))
	if rec.Code != 200 {
		t.Fatalf("want 200 с валидацией, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Неизвестный тип лестницы") {
		t.Errorf("внутренний расчёт тоже должен отвергать спираль как неизвестный тип: %s", body)
	}
}

// TestPublicQuoteOffersNoSpiralVariations — советник больше не строит
// спиральных вариантов, поэтому публичная смета не может их содержать.
// Проверка остаётся как защита от возврата типа марша через вариации.
func TestPublicQuoteOffersNoSpiralVariations(t *testing.T) {
	body := `{
		"width_mm": 1100, "height_mm": 2700, "flight": "straight", "material": "STEEL-S235",
		"step_height_mm": 168, "step_thickness_mm": 6, "stringer_thickness_mm": 8,
		"riser": true, "clearance_mm": 2000, "railing_height_mm": 900,
		"room_width_mm": 4500, "room_length_mm": 900
	}`
	req := httptest.NewRequest("POST", "/api/v1/public/stairs:quote", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"flight":"spiral"`) {
		t.Errorf("публичный ответ не должен предлагать спиральные варианты: %s", rec.Body.String())
	}
}
