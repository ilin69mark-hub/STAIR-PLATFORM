package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	storeapp "stairplatform/internal/application/store"
)

// DOM-007 (2026-09-26): молчаливый успех при опечатке в имени поля.
//
// json.Decoder без DisallowUnknownFields игнорирует незнакомые ключи. Для
// DTO, где тело целиком заменяет состояние, это приводит к потере данных при
// ответе 200 — самый опасный класс дефектов API, потому что клиент показывает
// «сохранено».

// dom007Router — роутер с админскими правами и сервисом магазина.
func dom007Router() http.Handler {
	return testRouterWithStore(storeapp.NewService(newHTTPFakeStoreRepo()), true)
}

// TestDOM007_UnknownFieldRejectedOnDangerousEndpoints — опечатка в имени поля
// обязана давать 400, а не 200.
func TestDOM007_UnknownFieldRejectedOnDangerousEndpoints(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
	}{
		{
			name: "настройки магазина: опечатка в имени секции",
			path: "/api/v1/admin/store/settings",
			body: `{"contacts":{"telephone":"+7 900 000-00-00"}}`,
		},
		{
			name: "цена материала: опечатка в имени поля",
			path: "/api/v1/admin/store/prices",
			body: `{"code":"STEEL-S235","pricePerKgRub":2500}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := authedRequest(http.MethodPut, tc.path, tc.body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			dom007Router().ServeHTTP(rec, req)
			if rec.Code == http.StatusOK || rec.Code == http.StatusCreated {
				t.Fatalf("DOM-007: опечатка в поле принята (HTTP %d): %s — настройки потеряны молча",
					rec.Code, rec.Body.String())
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("ожидался 400 invalid_json, получено %d: %s", rec.Code, rec.Body.String())
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			low := strings.ToLower(body.Error.Message)
			if !strings.Contains(low, "unknown field") && !strings.Contains(low, "неизвестное поле") {
				t.Errorf("сообщение должно называть проблемное поле, получено %q", body.Error.Message)
			}
			if strings.Contains(body.Error.Message, `\"`) {
				t.Errorf("сообщение не должно содержать экранированных кавычек: %q", body.Error.Message)
			}
		})
	}
}

// TestDOM007_KnownFieldsStillAccepted — строгий разбор не должен ломать
// корректные запросы (обратная совместимость).
func TestDOM007_KnownFieldsStillAccepted(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
	}{
		{"настройки магазина", "/api/v1/admin/store/settings", `{"contacts":{"email":"shop@example.com"}}`},
		{"цена материала", "/api/v1/admin/store/prices", `{"code":"STEEL-S235","price_per_kg_rub":2500}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := authedRequest(http.MethodPut, tc.path, tc.body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			dom007Router().ServeHTTP(rec, req)
			if rec.Code >= 400 {
				t.Fatalf("корректное тело отклонено (%d): %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestDOM007_DecodeJSONStrictRejectsUnknownField — поведение самого хелпера.
func TestDOM007_DecodeJSONStrictRejectsUnknownField(t *testing.T) {
	type dto struct {
		Known string `json:"known"`
	}
	var d dto
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"known":"a","typo":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	err := decodeJSONStrict(rec, req, &d)
	if err == nil {
		t.Fatal("decodeJSONStrict must reject unknown fields")
	}
	if !strings.Contains(err.Error(), "typo") {
		t.Errorf("ошибка должна называть поле, получено %q", err.Error())
	}
	// Обычный decodeJSON, наоборот, обязан молчать (это документированное
	// поведение для остальных 29 DTO).
	var d2 dto
	req2 := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"known":"a","typo":"b"}`))
	req2.Header.Set("Content-Type", "application/json")
	if err := decodeJSON(httptest.NewRecorder(), req2, &d2); err != nil {
		t.Errorf("decodeJSON должен остаться нестрогим: %v", err)
	}
}
