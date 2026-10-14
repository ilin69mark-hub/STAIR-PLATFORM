package http

import (
	"context"
	"errors"
	"net/http"

	"stairplatform/internal/application/jobs"
	"stairplatform/internal/application/stair"
)

// JobsService — прикладной интерфейс фоновых заданий (EDR-0035),
// ожидаемый транспортным слоем.
type JobsService interface {
	SubmitCalculate(ctx context.Context, tenantID, userID string, payload jobs.Payload) (*jobs.Job, error)
	// GetJobForUser возвращает задание в скоупе tenant'а И владельца.
	// SEC-004: userID обязателен — tenant-only доступ означал бы, что любой
	// пользователь дефолтного tenant'а читает чужой расчёт.
	GetJobForUser(ctx context.Context, tenantID, userID, id string) (*jobs.Job, error)
}

// jobStatusResponse — ответ GET /api/v1/jobs/{id}: статус и, при успехе,
// результат в том же DTO-формате, что синхронный расчёт.
type jobStatusResponse struct {
	ID     string             `json:"id"`
	Type   string             `json:"type"`
	Status string             `json:"status"`
	Result *calculateResponse `json:"result,omitempty"`
	Error  string             `json:"error,omitempty"`
}

// handleCalculateAsync — POST /api/v1/stairs:calculate/async (EDR-0035).
// Вход — как у синхронного расчёта; валидируется сразу (422 — невалидный
// вход в очередь не попадает). Ответ 202: {job_id, type, status:"pending"}.
func handleCalculateAsync(svc JobsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req calculateRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		cfg := toConfig(req)
		opts, ok := optionsOrReject(r, w, req)
		if !ok {
			return
		}
		// Дешёвая синхронная валидация (EDR-0035 §3.4): заведомо невалидный
		// вход отвергается 422 и в очередь не попадает; полный расчёт —
		// в воркере.
		if err := stair.ValidateConfig(cfg); err != nil {
			writeInputError(w, "invalid_input", err)
			return
		}

		j, err := svc.SubmitCalculate(r.Context(), tenantID(r.Context()), userID(r.Context()), jobs.Payload{Config: cfg, Options: opts})
		if err != nil {
			mapStairError(w, r, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{
			"job_id": j.ID,
			"type":   j.Type,
			"status": string(j.Status),
		})
	}
}

// handleGetJob — GET /api/v1/jobs/{id} (EDR-0035). Детерминированная
// отдача статуса; при succeeded — результат в формате calculateResponse.
//
// SEC-004: доступ ограничен владельцем задания. Чужое задание отдаёт 404
// (не 403), чтобы не раскрывать факт его существования.
func handleGetJob(svc JobsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		// SEC-004: userID из контекста — скоуп по владельцу.
		j, err := svc.GetJobForUser(r.Context(), tenantID(r.Context()), userID(r.Context()), id)
		if err != nil {
			if errors.Is(err, jobs.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "Задание не найдено")
				return
			}
			mapStairError(w, r, err)
			return
		}
		resp := jobStatusResponse{ID: j.ID, Type: j.Type, Status: string(j.Status), Error: j.Error}
		if j.Result != nil {
			rr := toResponse(j.Result)
			resp.Result = &rr
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
