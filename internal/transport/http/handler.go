package http

import (
	"encoding/json"
	"net/http"

	"stairplatform/internal/application/stair"
)

// StairService — прикладной интерфейс расчёта лестницы, ожидаемый
// транспортным слоем (инверсия зависимостей, DOM-0008).
type StairService interface {
	Calculate(cfg stair.Config, opts stair.Options) (*stair.Result, error)
}

// handleCalculate — POST /api/v1/stairs:calculate.
// 200 — успешный расчёт (включая blocking-валидацию с отчётом);
// 400 — некорректный JSON;
// 422 — невалидный вход (нельзя выполнить расчёт);
// 500 — внутренняя ошибка конвейера.
func handleCalculate(svc StairService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req calculateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}

		cfg, err := toConfig(req)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
			return
		}
		opts, err := toOptions(req)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_rates", err.Error())
			return
		}

		res, err := svc.Calculate(cfg, opts)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, toResponse(res))
	}
}

func toResponse(res *stair.Result) calculateResponse {
	resp := calculateResponse{
		Validation: toValidationResult(res),
	}
	if res.Validation.Blocking || res.Package == nil || res.Price == nil {
		// Конвейер остановлен: производственных и финансовых данных нет.
		return resp
	}
	resp.Flight = toFlight(*res)
	resp.Geometry = toGeometry(*res)
	resp.Manufacturing = toManufacturing(res.Package)
	resp.Pricing = toPricing(res.Price)
	return resp
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
