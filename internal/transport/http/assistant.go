package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
// project_id/history_limit — скоуп и глубина conversation-memory (S-135):
// project_id пустой — память для запроса не используется; history_limit
// 0 → default 10, cap 50, <0 → без истории.
type assistantRequest struct {
	calculateRequest
	Priority     string `json:"priority,omitempty"`
	ProjectID    string `json:"project_id,omitempty"`
	HistoryLimit *int   `json:"history_limit,omitempty"`
}

// handleAssistantAsk — POST /api/v1/assistant/{kind} (Phase D, EDR-0036).
// kind ∈ design|engineering|manufacturing|pricing.
// 200 — рекомендация ассистента (структурный ответ + комментарий);
// 400 — некорректный JSON;
// 403 — project_id из тела, но вызывающий не член проекта (S-142);
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

		cfg := toConfig(req.calculateRequest)

		// Все четыре kind прикладного слоя реализованы (добавляются маршруты
		// при регистрации Config.Assistant).
		historyLimit := 0
		if req.HistoryLimit != nil {
			historyLimit = *req.HistoryLimit
		}
		var areq appast.Request
		switch kind {
		case appast.KindDesign:
			areq = appast.DesignRequest{
				Config: cfg,
				Preferences: appast.DesignPreferences{
					Priority: appast.DesignPriority(req.Priority),
				},
				ProjectID:    req.ProjectID,
				HistoryLimit: historyLimit,
			}
		case appast.KindEngineering, appast.KindManufacturing, appast.KindPricing:
			opts, oerr := toOptions(req.calculateRequest)
			if oerr != nil {
				writeInputError(w, "invalid_rates", oerr)
				return
			}
			areq = appast.AnalysisRequest{
				Config:       cfg,
				Options:      opts,
				ProjectID:    req.ProjectID,
				HistoryLimit: historyLimit,
			}
		}

		res, err := svc.Ask(r.Context(), tenantID(r.Context()), userID(r.Context()), kind, areq)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				writeError(w, 499, "cancelled", "Операция отменена")
				return
			}
			// ErrInvalid / ErrNoFeasible — ошибка входных данных (клиент):
			// 422. ErrForbidden — проект вне членства вызывающего (S-142):
			// 403. Прочие (сбой модели, инфраструктуры, неожиданное) —
			// 500, чтобы не маскировать внутренние проблемы под ошибку ввода.
			if errors.Is(err, appast.ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
				return
			}
			if errors.Is(err, appast.ErrInvalid) || errors.Is(err, appast.ErrNoFeasible) {
				writeInputError(w, "invalid_input", err)
			} else {
				wrapped := fmt.Sprintf("assistant: %v", err)
				slog.Error("assistant request failed", "kind", kind, "error", err)
				writeError(w, http.StatusInternalServerError, "internal", wrapped)
			}
			return
		}

		writeJSON(w, http.StatusOK, res)
	}
}
