package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/engine/solver"
) // maxBodyBytes — предельный размер тела запроса (защита от DoS,
// SEC-0003): применяется в decodeJSON.
var maxBodyBytes int64 = 1 << 20 // 1 MiB

// decodeJSON декодирует тело запроса в dst с ограничением размера
// (MaxBodyBytes). Возвращает ошибку при невалидном JSON.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	return json.NewDecoder(r.Body).Decode(dst)
}

// StairService — прикладной интерфейс расчёта лестницы, ожидаемый
// транспортным слоем (инверсия зависимостей, DOM-0008).
type StairService interface {
	Calculate(ctx context.Context, cfg stair.Config, opts stair.Options) (*stair.Result, error)
	Optimize(ctx context.Context, cfg stair.Config, opts stair.Options, req stair.OptimizeRequest) (*stair.OptimizeResult, error)
}

// mapStairError преобразует ошибку конвейера в HTTP-ответ для клиента:
// отмена/дедлайн контекста — 499; оставшаяся пользовательская проблема
// (InputError) — 422 с русским текстом; прочее — внутренний сбой 500 с
// обобщённым русским сообщением (детали — в лог сервера).
func mapStairError(w http.ResponseWriter, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		writeError(w, 499, "cancelled", "Операция отменена")
		return
	}
	var inp *solver.InputError
	if errors.As(err, &inp) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", inp.Message)
		return
	}
	if strings.Contains(err.Error(), "unknown optimization target") {
		writeError(w, http.StatusUnprocessableEntity, "invalid_input",
			"Неизвестная цель оптимизации. Доступны: price, cost, material, comfort.")
		return
	}
	slog.Error("calculation pipeline error", "error", err.Error())
	writeError(w, http.StatusInternalServerError, "internal_error",
		"Не удалось выполнить расчёт. Попробуйте позже.")
}

// handleCalculate — POST /api/v1/stairs:calculate.
// 200 — успешный расчёт (включая blocking-валидацию с отчётом);
// 400 — некорректный JSON;
// 422 — невалидный вход (нельзя выполнить расчёт);
// 500 — внутренняя ошибка конвейера.
func handleCalculate(svc StairService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req calculateRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}

		cfg, err := toConfig(req)
		if err != nil {
			writeInputError(w, "invalid_input", err)
			return
		}
		opts, err := toOptions(req)
		if err != nil {
			writeInputError(w, "invalid_rates", err)
			return
		}

		res, err := svc.Calculate(r.Context(), cfg, opts)
		if err != nil {
			mapStairError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, toResponse(res))
	}
}

// handleOptimize — POST /api/v1/stairs:optimize (EDR-0032).
// 200 — итог поиска (valid:false — нет допустимой конфигурации в диапазоне);
// 400 — некорректный JSON;
// 422 — невалидный вход или неизвестная целевая метрика;
// 500 — внутренняя ошибка конвейера.
func handleOptimize(svc StairService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req optimizeRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}

		cfg, err := toConfig(req.calculateRequest)
		if err != nil {
			writeInputError(w, "invalid_input", err)
			return
		}
		opts, err := toOptions(req.calculateRequest)
		if err != nil {
			writeInputError(w, "invalid_rates", err)
			return
		}

		out, err := svc.Optimize(r.Context(), cfg, opts, toOptimizeRequest(req))
		if err != nil {
			mapStairError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toOptimizeResponse(out))
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
	e := flightEcho(*res)
	resp.Flight = toFlight(*res)
	resp.LShape = toLShape(res.LShape, e)
	resp.UShape = toUShape(res.UShape, e)
	resp.Spiral = toSpiral(res.Spiral, e)
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
