package http

import (
	"context"
	"errors"
	"net/http"

	appast "stairplatform/internal/application/assistant"
)

// AssistantService — прикладной интерфейс AI-ассистентов, ожидаемый
// транспортным слоем (инверсия зависимостей, DOM-0008).
type AssistantService interface {
	Ask(ctx context.Context, tenantID, userID string, kind appast.Kind, req appast.Request) (*appast.Result, error)
}

// assistantRequest — запрос к ассистенту. Поля конфигурации совпадают с
// calculateRequest; kind — из пути маршрута; priority — предпочтение
// (актуально для design; прочие kind игнорируют его на D1).
type assistantRequest struct {
	calculateRequest
	Priority string `json:"priority,omitempty"`
}

// handleAssistantAsk — POST /api/v1/assistant/{kind} (Phase D, EDR-0036).
// kind ∈ design|engineering|manufacturing|pricing.
// 200 — рекомендация ассистента (структурный ответ + комментарий);
// 400 — некорректный JSON;
// 404 — неизвестный kind;
// 422 — невалидный вход / недоступный ассистент / нет допустимой конфигурации;
// 499 — отмена контекста; 500 — внутренняя ошибка.
func handleAssistantAsk(svc AssistantService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := appast.Kind(r.PathValue("kind"))
		if !kind.Valid() {
			writeError(w, http.StatusNotFound, "not_found", "Неизвестный тип ассистента")
			return
		}

		var req assistantRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}

		cfg, err := toConfig(req.calculateRequest)
		if err != nil {
			writeInputError(w, "invalid_input", err)
			return
		}

		// Все четыре kind прикладного слоя реализованы (добавляются маршруты
		// при регистрации Config.Assistant).
		var areq appast.Request
		switch kind {
		case appast.KindDesign:
			areq = appast.DesignRequest{
				Config: cfg,
				Preferences: appast.DesignPreferences{
					Priority: appast.DesignPriority(req.Priority),
				},
			}
		case appast.KindEngineering, appast.KindManufacturing, appast.KindPricing:
			opts, oerr := toOptions(req.calculateRequest)
			if oerr != nil {
				writeInputError(w, "invalid_rates", oerr)
				return
			}
			areq = appast.AnalysisRequest{
				Config:  cfg,
				Options: opts,
			}
		}

		res, err := svc.Ask(r.Context(), tenantID(r.Context()), userID(r.Context()), kind, areq)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				writeError(w, 499, "cancelled", "Операция отменена")
				return
			}
			writeInputError(w, "invalid_input", err)
			return
		}

		writeJSON(w, http.StatusOK, res)
	}
}
