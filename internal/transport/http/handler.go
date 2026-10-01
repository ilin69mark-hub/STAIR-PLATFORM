package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/engine/solver"
)

// maxBodyBytes — предельный размер тела запроса (защита от DoS,
// SEC-0003): применяется в decodeJSON. Всегда 1 MiB (см. DefaultConfig и
// cmd/api/main.go); константа, чтобы у рантайма не было глобала-мутанта.
const maxBodyBytes int64 = 1 << 20 // 1 MiB

// decodeJSON декодирует тело запроса в dst с ограничением размера
// (MaxBodyBytes). Возвращает ошибку при невалидном JSON.
// S-144 (S-141 №7): для непустых тел требуется Content-Type:
// application/json (префикс-match — пропускаем параметры вроде charset).
// Браузерная форма text/plain (без CORS-preflight) больше не выполнит
// JSON-тело — CSRF-вектор из аудита закрыт; пустые тела/GET не
// затрагиваются. Callers маппят ошибку в 400 invalid_json.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if hasBody(r) && !contentTypeIsJSON(r.Header.Get("Content-Type")) {
		return errors.New("decodeJSON: Content-Type must be application/json")
	}
	return json.NewDecoder(r.Body).Decode(dst)
}

// decodeJSONStrict — как decodeJSON, но ОТКЛОНЯЕТ НЕИЗВЕСТНЫЕ ПОЛЯ тела.
//
// DOM-007 (2026-09-26): обычный Decode молча игнорирует опечатки в именах
// полей. Для DTO, где тело целиком заменяет состояние, это приводит к
// «молчаливому успеху» с потерей данных (доказанные случаи):
//   - PUT /admin/store/settings с {"contacts":{"telephone":…}} (опечатка) →
//     все секции настроек сбрасываются на дефолты, ответ 200;
//   - PUT /admin/store/prices с {"code":…,"pricePerKgRub":0} → цена 0;
//   - PATCH /admin/users/{id} с {"sttus":"disabled"} → пользователь НЕ
//     отключается, ответ 200;
//   - PUT /admin/settings с опечаткой в политике → политика сброшена.
//
// Применяется точечно (4 DTO), а не глобально: строгий разбор ломает
// клиентов, которые присылают лишние поля (обратная совместимость важнее).
func decodeJSONStrict(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if hasBody(r) && !contentTypeIsJSON(r.Header.Get("Content-Type")) {
		return errors.New("decodeJSON: Content-Type must be application/json")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		// Ошибка об неизвестном поле возвращаем как есть: транспорт отдаёт
		// 400 invalid_json с понятным текстом, а не «молчаливый успех».
		return err
	}
	return nil
}

// jsonErrorMessage — понятное сообщение об ошибке разбора тела.
//
// DOM-007: для неизвестного поля текст должен НАЗЫВАТЬ проблемный ключ,
// иначе клиент не понимает, что исправлять (тело ответа не содержит иных
// подсказок, а запрос «просто не применился» выглядит как сбой сервера).
// Текст json-декодера содержит только имена полей из тела запроса, поэтому
// утечки внутренних деталей нет. Остальные ошибки разбора остаются
// обобщёнными.
func jsonErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	const unknownField = "unknown field "
	if i := strings.Index(err.Error(), unknownField); i >= 0 {
		field := err.Error()[i+len(unknownField):]
		if j := strings.IndexAny(field, " \""); j > 0 {
			field = field[:j]
		}
		return "Неизвестное поле в теле запроса: " + field +
			". Проверьте название поля (сверьтесь со схемой в /docs/openapi/swagger.yaml)."
	}
	return "Некорректный JSON в теле запроса"
}

// hasBody — есть ли у запроса тело (Content-Length > 0; chunked считаем
// непустым). Пустые тела ведут себя как раньше (Decode вернёт EOF → 400).
func hasBody(r *http.Request) bool {
	return r.ContentLength > 0 || len(r.TransferEncoding) > 0
}

// contentTypeIsJSON — media type без параметров (после ";") равен
// application/json (регистронезависимо).
func contentTypeIsJSON(ct string) bool {
	media, _, _ := strings.Cut(ct, ";")
	return strings.EqualFold(strings.TrimSpace(media), "application/json")
}

// StairService — прикладной интерфейс расчёта лестницы, ожидаемый
// транспортным слоем (инверсия зависимостей, DOM-0008).
type StairService interface {
	Calculate(ctx context.Context, cfg stair.Config, opts stair.Options) (*stair.Result, error)
	Validate(ctx context.Context, cfg stair.Config, opts stair.Options) (*stair.Result, error)
	Optimize(ctx context.Context, cfg stair.Config, opts stair.Options, req stair.OptimizeRequest) (*stair.OptimizeResult, error)
}

// mapStairError преобразует ошибку конвейера в HTTP-ответ для клиента:
// отмена/дедлайн контекста — 499; оставшаяся пользовательская проблема
// (InputError) — 422 с русским текстом; прочее — внутренний сбой 500 с
// обобщённым русским сообщением (детали — в лог сервера).
func mapStairError(w http.ResponseWriter, r *http.Request, err error) {
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
	// SEC-003: клиент запросил пространство поиска шире потолка
	// (нормативное окно по числу ступеней, сетка шага комфорта, перебор
	// нижнего марша либо итоговое число кандидатов). Это ошибка ввода —
	// 422 с перечнем фактических потолков, чтобы клиент их видел.
	if errors.Is(err, stair.ErrSearchSpaceTooLarge) {
		writeError(w, http.StatusUnprocessableEntity, "search_space_too_large",
			fmt.Sprintf("Запрошенные границы перебора слишком широки: окно числа ступеней не более %d значений, сетка шага комфорта не чаще %.1f мм, перебор нижнего марша не более %d значений, итоговое число кандидатов не более %d.",
				stair.MaxStepCountSpan, stair.MinComfortStepGrid, stair.MaxLowerStepSpan, stair.MaxCandidates))
		return
	}
	slog.Error("calculation pipeline error", "error", err.Error(), "path", r.URL.Path)
	// API-002/API-004: 500 обязан нести request_id — по нему пользователь
	// сообщает о сбое, инженер находит строку лога.
	writeErrorWithRequestID(w, r, http.StatusInternalServerError, "internal_error",
		"Не удалось выполнить расчёт. Попробуйте позже.")
}

// handleCalculate — POST /api/v1/stairs:calculate.
// 200 — успешный расчёт (включая blocking-валидацию с отчётом);
// 400 — некорректный JSON;
// 422 — невалидный вход (нельзя выполнить расчёт);
// 500 — внутренняя ошибка конвейера.
func handleCalculate(svc StairService, auditSvc AuditService) http.HandlerFunc {
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

		res, err := svc.Calculate(r.Context(), cfg, opts)
		if err != nil {
			mapStairError(w, r, err)
			return
		}

		if auditSvc != nil {
			recordCalculationAudit(r, auditSvc, cfg, res)
		}

		writeJSON(w, http.StatusOK, toResponse(res))
	}
}

// handleValidate — POST /api/v1/public/stairs:validate и
// POST /api/v1/stairs:validate. Живая валидация при вводе (S-P5): выполняет
// ровно ту секцию validation, которую фронтенд показывает при расчёте, но без
// геометрии, производства, цены и записей. Блокирующее состояние — это не
// ошибка: 200 с valid:false и готовыми Suggestions/Variations (A/B/C).
func handleValidate(svc StairService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req calculateRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		if rejectDisabledFlight(w, req.Flight) {
			return
		}

		cfg := toConfig(req)
		opts, ok := optionsOrReject(r, w, req)
		if !ok {
			return
		}

		res, err := svc.Validate(r.Context(), cfg, opts)
		if err != nil {
			mapStairError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"validation": toValidationResult(res, false)})
	}
}

// recordCalculationAudit фиксирует событие расчёта лестницы (server-side
// аудит). best-effort: ошибка журнала не ломает ответ расчёта.
func recordCalculationAudit(r *http.Request, auditSvc AuditService, cfg stair.Config, res *stair.Result) {
	detail := map[string]any{
		"flight":   string(cfg.Flight),
		"blocking": res.Validation.Blocking,
	}
	if res.Validation.Blocking {
		issues := make([]map[string]any, 0, len(res.Validation.Issues))
		for _, it := range res.Validation.Issues {
			variants := make([]map[string]any, 0, len(it.Variations))
			for _, v := range it.Variations {
				variants = append(variants, map[string]any{
					"id":           v.ID,
					"fits":         v.Fits,
					"passes_norms": v.PassesNorms,
				})
			}
			issues = append(issues, map[string]any{
				"code":     it.Code,
				"variants": variants,
			})
		}
		detail["issues"] = issues
	}
	b, _ := json.Marshal(detail)
	e := &audit.Event{
		ActorID:   userID(r.Context()),
		TenantID:  tenantID(r.Context()),
		Action:    audit.ActionStairCalculated,
		Result:    audit.ResultOK,
		Detail:    string(b),
		RequestID: auditRequestID(r.Context()),
		IP:        clientIP(r),
	}
	// AUDIT-002 (2026-09-27): ошибка записи в журнал логируется, а не
	// выбрасывается. Расчёт уже состоялся, отменять его из-за аудита нельзя —
	// но и молчать о пробеле нельзя: «действие произошло, а записи нет»
	// обнаруживается только при расследовании, и то по косвенным признакам.
	recordAudit(r.Context(), auditSvc, e)
}

// recordAudit пишет событие аудита, логируя отказ. Единственная разрешённая
// обёртка для best-effort аудита в транспорте: бизнес-операция к этому
// моменту уже завершена, поэтому ошибка записи не должна её отменять, но
// обязана быть видна.
//
// До AUDIT-002 девять вызовов из двенадцати использовали `_ =` и роняли
// ошибку на пол: забытое поле или сбой БД давали «успешную» операцию без
// единой записи в журнале и без единого признака, что что-то пошло не так.
func recordAudit(ctx context.Context, svc AuditService, e *audit.Event) {
	if svc == nil {
		return
	}
	if err := svc.Record(ctx, e); err != nil {
		slog.Error("audit: event not recorded",
			"action", string(e.Action), "tenant_id", e.TenantID,
			"actor_id", e.ActorID, "request_id", e.RequestID, "error", err)
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

		cfg := toConfig(req.calculateRequest)
		opts, ok := optionsOrReject(r, w, req.calculateRequest)
		if !ok {
			return
		}

		out, err := svc.Optimize(r.Context(), cfg, opts, toOptimizeRequest(req))
		if err != nil {
			mapStairError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toOptimizeResponse(out))
	}
}

func toResponse(res *stair.Result) calculateResponse {
	resp := calculateResponse{
		Validation: toValidationResult(res, false),
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

// writeErrorWithRequestID — тело ошибки с request_id (API-002/SEC-005).
// Нужен для 5xx: по request_id пользователь сообщает о сбое, а инженер —
// находит его в логах. Для 4xx request_id не добавляется, чтобы не раздувать
// контракт успешных ответов.
func writeErrorWithRequestID(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	// r может быть nil в тестах, вызывающих хендлер напрямую
	// (handleMetrics и т. п.): тогда request_id просто отсутствует, но тело
	// ошибки остаётся корректным.
	rid := ""
	if r != nil {
		rid = RequestID(r.Context())
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":       code,
			"message":    message,
			"request_id": rid,
		},
	})
}
