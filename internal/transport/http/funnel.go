package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/funnel"
)

// Приём клиентских событий витрины и отчёт воронки.
//
// Эндпоинт ПУБЛИЧНЫЙ и без аутентификации: события шлёт анонимный
// посетитель. Отсюда три правила, и все три проверяются на сервере, а не
// «по договорённости с клиентом»:
//   - согласие: без consent_version события не пишутся вовсе;
//   - каталог имён: произвольное имя не примется, иначе отчёт наполнялся бы
//     чужим мусором;
//   - visitor: хеш от IP и User-Agent, сами IP и UA не сохраняются. Без
//     соли (STAIR_ANALYTICS_SALT пуст) visitor пустой и пересечь визиты
//     невозможно.

// FunnelService — прикладной интерфейс воронки, ожидаемый транспортом.
type FunnelService interface {
	Ingest(ctx context.Context, sessionID string, events []funnel.Event, consentVersion int) (funnel.IngestResult, error)
	Report(ctx context.Context, from, to time.Time) (*funnel.FunnelReport, error)
}

// ---- DTO ----

// analyticsEventDTO — событие в теле запроса витрины.
type analyticsEventDTO struct {
	Name      string         `json:"name"`
	Props     map[string]any `json:"props"`
	Path      string         `json:"path"`
	Browser   string         `json:"browser"`
	Timestamp string         `json:"ts"`
}

// ingestRequest — пачка событий одного визита.
type ingestRequest struct {
	SessionID      string              `json:"session_id"`
	ConsentVersion int                 `json:"consent_version"`
	Events         []analyticsEventDTO `json:"events"`
}

// funnelStepDTO — шаг воронки.
type funnelStepDTO struct {
	Name      string  `json:"name"`
	Sessions  int     `json:"sessions"`
	Share     float64 `json:"share"`
	StepShare float64 `json:"step_share"`
}

// blockerDTO — точка затруднения.
type blockerDTO struct {
	Event  string `json:"event"`
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// abandonPointDTO — где бросают.
type abandonPointDTO struct {
	LastEvent string  `json:"last_event"`
	Sessions  int     `json:"sessions"`
	Share     float64 `json:"share"`
	AvgSec    float64 `json:"avg_seconds"`
}

// funnelReportDTO — ответ админского отчёта.
type funnelReportDTO struct {
	From      string            `json:"from"`
	To        string            `json:"to"`
	Sessions  int               `json:"sessions"`
	Events    int               `json:"events"`
	Steps     []funnelStepDTO   `json:"steps"`
	Blockers  []blockerDTO      `json:"blockers"`
	Abandons  []abandonPointDTO `json:"abandons"`
	AvgSecAll float64           `json:"avg_seconds"`
}

func toFunnelReportDTO(rep *funnel.FunnelReport) funnelReportDTO {
	d := funnelReportDTO{
		From:      rep.From.Format("2006-01-02"),
		To:        rep.To.Format("2006-01-02"),
		Sessions:  rep.Sessions,
		Events:    rep.Events,
		Steps:     make([]funnelStepDTO, 0, len(rep.Steps)),
		Blockers:  make([]blockerDTO, 0, len(rep.Blockers)),
		Abandons:  make([]abandonPointDTO, 0, len(rep.Abandons)),
		AvgSecAll: rep.AvgSecAll,
	}
	for _, s := range rep.Steps {
		d.Steps = append(d.Steps, funnelStepDTO{
			Name: s.Name, Sessions: s.Sessions, Share: s.Share, StepShare: s.StepShare,
		})
	}
	for _, b := range rep.Blockers {
		d.Blockers = append(d.Blockers, blockerDTO{Event: b.Event, Reason: b.Reason, Count: b.Count})
	}
	for _, a := range rep.Abandons {
		share := 0.0
		if rep.Sessions > 0 {
			share = float64(a.Sessions) / float64(rep.Sessions)
		}
		d.Abandons = append(d.Abandons, abandonPointDTO{
			LastEvent: a.LastEvent, Sessions: a.Sessions, Share: share, AvgSec: a.AvgSec,
		})
	}
	return d
}

// ---- приём ----

// visitorHasher считает псевдоним посетителя. Пустая соль — посетитель не
// идентифицируется вовсе, и это штатный режим: соль живёт в переменной
// окружения, и её можно не задавать.
type visitorHasher struct{ key []byte }

func newVisitorHasher(salt string) visitorHasher {
	return visitorHasher{key: []byte(salt)}
}

// hash возвращает 16 hex-символов HMAC(ip+ua) или "" без соли.
func (h visitorHasher) hash(ip, ua string) string {
	if len(h.key) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, h.key)
	mac.Write([]byte(ip))
	mac.Write([]byte("|"))
	mac.Write([]byte(ua))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

// browserFamily — семейство браузера из User-Agent. НЕ сохраняется сам
// User-Agent: полная строка версии и ОС персональными данными не является
// по смыслу, но формально является, и нужна нам ровно настолько, насколько
// хватает «Chrome»/«Safari»/«Firefox» для разбора «где у людей ломается».
func browserFamily(ua string) string {
	switch {
	case ua == "":
		return "unknown"
	case strings.Contains(ua, "Edg/"), strings.Contains(ua, "Edge/"):
		return "edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		return "opera"
	case strings.Contains(ua, "YaBrowser"):
		return "yandex"
	case strings.Contains(ua, "Firefox/"):
		return "firefox"
	case strings.Contains(ua, "Chrome/"), strings.Contains(ua, "Chromium/"):
		return "chrome"
	case strings.Contains(ua, "Safari/"):
		return "safari"
	default:
		return "other"
	}
}

// handleIngestAnalytics — POST /api/v1/public/analytics:events.
//
// 202 — принято (в теле сколько accepted/dropped). Ответ 202, а не 200:
// событие не меняет состояние бизнеса, витрина не должна на нём ничего
// проверять. 400 — битый JSON/нет session_id; 413 — пачка больше лимита.
func handleIngestAnalytics(svc FunnelService, hasher visitorHasher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ingestRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		if len(req.Events) > funnel.MaxBatchEvents {
			writeError(w, http.StatusRequestEntityTooLarge, "too_many_events",
				"В пачке больше событий, чем допустимо за один запрос")
			return
		}
		ua := r.Header.Get("User-Agent")
		visitor := hasher.hash(clientIP(r), ua)

		events := make([]funnel.Event, 0, len(req.Events))
		family := browserFamily(ua)
		for _, e := range req.Events {
			ev := funnel.Event{
				Name:  e.Name,
				Props: e.Props,
				Path:  e.Path,
				// visitor и browser считает СЕРВЕР. Значения из тела запроса
				// намеренно игнорируются: иначе клиент подмешивал бы в отчёт
				// произвольную строку, и «где ломается» считалось бы по
				// тому, что клиент сам написал.
				Visitor: visitor,
				Browser: family,
			}
			if ts, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
				ev.OccurredAt = ts
			}
			events = append(events, ev)
		}

		res, err := svc.Ingest(r.Context(), req.SessionID, events, req.ConsentVersion)
		if err != nil {
			switch {
			case errors.Is(err, funnel.ErrBatchTooLarge):
				writeError(w, http.StatusRequestEntityTooLarge, "too_many_events",
					"В пачке больше событий, чем допустимо за один запрос")
			case errors.Is(err, funnel.ErrInvalidSession):
				writeError(w, http.StatusBadRequest, "invalid_session",
					"session_id должен быть UUID")
			case errors.Is(err, funnel.ErrConsentRequired):
				// Не согласие — не ошибка клиента: витрина просто не должна
				// была слать. Отвечаем 202, чтобы она не повторяла и не
				// засоряла лог ошибками.
				writeJSON(w, http.StatusAccepted, map[string]int{"accepted": 0, "dropped": len(events)})
			default:
				writeServiceError(w, r, err, "Внутренняя ошибка сервера")
			}
			return
		}
		writeJSON(w, http.StatusAccepted, res)
	}
}

// ---- отчёт ----

// handleFunnelAnalytics — GET /api/v1/admin/analytics/funnel (auth+admin).
// 200 — воронка; 403 — нет права analytics.read; 422 — неверное окно.
func handleFunnelAnalytics(svc FunnelService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionAnalyticsRead) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора")
			return
		}
		from, err := queryTime(r, "from", time.Now().UTC().AddDate(0, 0, -30))
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_from", "Параметр from должен быть в формате YYYY-MM-DD или RFC3339.")
			return
		}
		to, err := queryTime(r, "to", time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_to", "Параметр to должен быть в формате YYYY-MM-DD или RFC3339.")
			return
		}
		rep, err := svc.Report(r.Context(), from, to)
		if err != nil {
			if errors.Is(err, funnel.ErrInvalidRange) {
				writeError(w, http.StatusUnprocessableEntity, "invalid_range", "Параметр from не может быть позже to.")
				return
			}
			writeServiceError(w, r, err, "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusOK, toFunnelReportDTO(rep))
	}
}
