package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"stairplatform/internal/application/analytics"
	"stairplatform/internal/application/auth"
)

// AnalyticsService — прикладной интерфейс аналитики (EDR-0028 §3.4),
// ожидаемый транспортным слоем (инверсия зависимостей, DOM-0008).
type AnalyticsService interface {
	Usage(ctx context.Context, tenantID string, from, to time.Time, g analytics.Granularity) (*analytics.UsageReport, error)
}

// ---- DTO ----

// usagePointDTO — счётчики одного бакета (EDR-0028 §3.4).
type usagePointDTO struct {
	Bucket          string `json:"bucket"`
	Logins          int    `json:"logins"`
	ActiveUsers     int    `json:"active_users"`
	ProjectsCreated int    `json:"projects_created"`
	Calculations    int    `json:"calculations"`
	Exports         int    `json:"exports"`
	Payments        int    `json:"payments"`
}

// usageTotalsDTO — итоговые счётчики tenant за окно.
type usageTotalsDTO struct {
	Users        int `json:"users"`
	ActiveUsers  int `json:"active_users"`
	Projects     int `json:"projects"`
	Calculations int `json:"calculations"`
	Logins       int `json:"logins"`
	Exports      int `json:"exports"`
	Payments     int `json:"payments"`
}

// usageReportDTO — ответ Usage Analytics.
type usageReportDTO struct {
	From        string          `json:"from"`
	To          string          `json:"to"`
	Granularity string          `json:"granularity"`
	Totals      usageTotalsDTO  `json:"totals"`
	Series      []usagePointDTO `json:"series"`
}

// toUsageReportDTO конвертирует отчёт в представление API.
func toUsageReportDTO(rep *analytics.UsageReport) usageReportDTO {
	d := usageReportDTO{
		From:        rep.From.Format("2006-01-02"),
		To:          rep.To.Format("2006-01-02"),
		Granularity: string(rep.Granularity),
		Totals: usageTotalsDTO{
			Users:        rep.Totals.Users,
			ActiveUsers:  rep.Totals.ActiveUsers,
			Projects:     rep.Totals.Projects,
			Calculations: rep.Totals.Calculations,
			Logins:       rep.Totals.Logins,
			Exports:      rep.Totals.Exports,
			Payments:     rep.Totals.Payments,
		},
		Series: make([]usagePointDTO, 0, len(rep.Series)),
	}
	for _, p := range rep.Series {
		d.Series = append(d.Series, usagePointDTO{
			Bucket:          p.Bucket.Format("2006-01-02"),
			Logins:          p.Logins,
			ActiveUsers:     p.ActiveUsers,
			ProjectsCreated: p.ProjectsCreated,
			Calculations:    p.Calculations,
			Exports:         p.Exports,
			Payments:        p.Payments,
		})
	}
	return d
}

// ---- handlers ----

// handleUsageAnalytics — GET /api/v1/admin/analytics/usage (auth+admin).
// 200 — отчёт Usage Analytics; 403 — нет права analytics.read;
// 422 — невалидные from/to/granularity; 500 — внутренняя ошибка.
func handleUsageAnalytics(svc AnalyticsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionAnalyticsRead) {
			writeError(w, http.StatusForbidden, "forbidden", "admin required")
			return
		}

		from, err := queryTime(r, "from", time.Now().UTC().AddDate(0, 0, -30))
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_from", "from must be YYYY-MM-DD or RFC3339")
			return
		}
		to, err := queryTime(r, "to", time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_to", "to must be YYYY-MM-DD or RFC3339")
			return
		}
		g := analytics.Granularity(r.URL.Query().Get("granularity"))
		if g == "" {
			g = analytics.GranularityDay
		}
		if !g.Valid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_granularity", "granularity must be day, week or month")
			return
		}

		rep, err := svc.Usage(r.Context(), tenantID(r.Context()), from, to, g)
		if err != nil {
			switch {
			case errors.Is(err, analytics.ErrInvalidRange):
				writeError(w, http.StatusUnprocessableEntity, "invalid_range", "from must not be after to")
			default:
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
			return
		}
		writeJSON(w, http.StatusOK, toUsageReportDTO(rep))
	}
}

// queryTime читает параметр как YYYY-MM-DD или RFC3339; пустое значение —
// дефолт def.
func queryTime(r *http.Request, name string, def time.Time) (time.Time, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	return time.Time{}, errors.New("invalid time")
}
