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
	Projects(ctx context.Context, tenantID string, from, to time.Time) (*analytics.ProjectReport, error)
	Manufacturing(ctx context.Context, tenantID string, from, to time.Time, g analytics.Granularity) (*analytics.ManufacturingReport, error)
	Cost(ctx context.Context, tenantID string, from, to time.Time, g analytics.Granularity) (*analytics.CostReport, error)
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

// projectRowDTO — сводка по одному проекту (EDR-0029 §3.4).
type projectRowDTO struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Status                 string `json:"status"`
	OwnerEmail             string `json:"owner_email"`
	CreatedAt              string `json:"created_at"`
	UpdatedAt              string `json:"updated_at"`
	Configurations         int    `json:"configurations"`
	Calculations           int    `json:"calculations"`
	LatestCalculationValid *bool  `json:"latest_calculation_valid"`
	Comments               int    `json:"comments"`
	Members                int    `json:"members"`
}

// projectTotalsDTO — агрегаты по проектам tenant (EDR-0029 §3.4).
type projectTotalsDTO struct {
	Projects                int            `json:"projects"`
	ProjectsCreated         int            `json:"projects_created"`
	ByStatus                map[string]int `json:"by_status"`
	ProjectsWithCalculation int            `json:"projects_with_calculation"`
	ValidProjects           int            `json:"valid_projects"`
	Configurations          int            `json:"configurations"`
	Calculations            int            `json:"calculations"`
	Comments                int            `json:"comments"`
}

// projectReportDTO — ответ Project Analytics.
type projectReportDTO struct {
	From     string           `json:"from"`
	To       string           `json:"to"`
	Totals   projectTotalsDTO `json:"totals"`
	Projects []projectRowDTO  `json:"projects"`
}

// toProjectReportDTO конвертирует отчёт в представление API.
func toProjectReportDTO(rep *analytics.ProjectReport) projectReportDTO {
	d := projectReportDTO{
		From: rep.From.Format("2006-01-02"),
		To:   rep.To.Format("2006-01-02"),
		Totals: projectTotalsDTO{
			Projects:                rep.Totals.Projects,
			ProjectsCreated:         rep.Totals.ProjectsCreated,
			ByStatus:                rep.Totals.ByStatus,
			ProjectsWithCalculation: rep.Totals.ProjectsWithCalculation,
			ValidProjects:           rep.Totals.ValidProjects,
			Configurations:          rep.Totals.Configurations,
			Calculations:            rep.Totals.Calculations,
			Comments:                rep.Totals.Comments,
		},
		Projects: make([]projectRowDTO, 0, len(rep.Projects)),
	}
	for _, p := range rep.Projects {
		d.Projects = append(d.Projects, projectRowDTO{
			ID:                     p.ID,
			Name:                   p.Name,
			Status:                 p.Status,
			OwnerEmail:             p.OwnerEmail,
			CreatedAt:              p.CreatedAt.Format(time.RFC3339),
			UpdatedAt:              p.UpdatedAt.Format(time.RFC3339),
			Configurations:         p.Configurations,
			Calculations:           p.Calculations,
			LatestCalculationValid: p.LatestCalculationValid,
			Comments:               p.Comments,
			Members:                p.Members,
		})
	}
	return d
}

// manufacturingPointDTO — производственные метрики одного бакета
// (EDR-0030 §3.4).
type manufacturingPointDTO struct {
	Bucket       string  `json:"bucket"`
	Calculations int     `json:"calculations"`
	Parts        int     `json:"parts"`
	Sheets       int     `json:"sheets"`
	Utilization  float64 `json:"utilization"`
}

// manufacturingTotalsDTO — агрегаты производства tenant (EDR-0030 §3.4).
type manufacturingTotalsDTO struct {
	Calculations int            `json:"calculations"`
	Parts        int            `json:"parts"`
	BomLines     int            `json:"bom_lines"`
	CutItems     int            `json:"cut_items"`
	Sheets       int            `json:"sheets"`
	PartArea     float64        `json:"part_area"`
	SheetArea    float64        `json:"sheet_area"`
	WasteArea    float64        `json:"waste_area"`
	Utilization  float64        `json:"utilization"`
	Materials    map[string]int `json:"materials"`
}

// manufacturingReportDTO — ответ Manufacturing Analytics.
type manufacturingReportDTO struct {
	From        string                  `json:"from"`
	To          string                  `json:"to"`
	Granularity string                  `json:"granularity"`
	Totals      manufacturingTotalsDTO  `json:"totals"`
	Series      []manufacturingPointDTO `json:"series"`
}

// toManufacturingReportDTO конвертирует отчёт в представление API.
func toManufacturingReportDTO(rep *analytics.ManufacturingReport) manufacturingReportDTO {
	d := manufacturingReportDTO{
		From:        rep.From.Format("2006-01-02"),
		To:          rep.To.Format("2006-01-02"),
		Granularity: string(rep.Granularity),
		Totals: manufacturingTotalsDTO{
			Calculations: rep.Totals.Calculations,
			Parts:        rep.Totals.Parts,
			BomLines:     rep.Totals.BomLines,
			CutItems:     rep.Totals.CutItems,
			Sheets:       rep.Totals.Sheets,
			PartArea:     rep.Totals.PartArea,
			SheetArea:    rep.Totals.SheetArea,
			WasteArea:    rep.Totals.WasteArea,
			Utilization:  rep.Totals.Utilization,
			Materials:    rep.Totals.Materials,
		},
		Series: make([]manufacturingPointDTO, 0, len(rep.Series)),
	}
	for _, p := range rep.Series {
		d.Series = append(d.Series, manufacturingPointDTO{
			Bucket:       p.Bucket.Format("2006-01-02"),
			Calculations: p.Calculations,
			Parts:        p.Parts,
			Sheets:       p.Sheets,
			Utilization:  p.Utilization,
		})
	}
	return d
}

// costPointDTO — стоимостные метрики одного бакета (EDR-0031 §3.4).
type costPointDTO struct {
	Bucket        string  `json:"bucket"`
	Calculations  int     `json:"calculations"`
	FinalPrice    int64   `json:"final_price"`
	AvgFinalPrice float64 `json:"avg_final_price"`
}

// costTotalsDTO — агрегаты стоимости tenant (EDR-0031 §3.4).
type costTotalsDTO struct {
	Calculations   int     `json:"calculations"`
	Material       int64   `json:"material"`
	Machine        int64   `json:"machine"`
	Labor          int64   `json:"labor"`
	Overhead       int64   `json:"overhead"`
	ProductionCost int64   `json:"production_cost"`
	Margin         int64   `json:"margin"`
	Discount       int64   `json:"discount"`
	PreTax         int64   `json:"pre_tax"`
	Tax            int64   `json:"tax"`
	FinalPrice     int64   `json:"final_price"`
	AvgFinalPrice  float64 `json:"avg_final_price"`
	Currency       string  `json:"currency"`
}

// costReportDTO — ответ Cost Analytics.
type costReportDTO struct {
	From        string         `json:"from"`
	To          string         `json:"to"`
	Granularity string         `json:"granularity"`
	Totals      costTotalsDTO  `json:"totals"`
	Series      []costPointDTO `json:"series"`
}

// toCostReportDTO конвертирует отчёт в представление API.
func toCostReportDTO(rep *analytics.CostReport) costReportDTO {
	d := costReportDTO{
		From:        rep.From.Format("2006-01-02"),
		To:          rep.To.Format("2006-01-02"),
		Granularity: string(rep.Granularity),
		Totals: costTotalsDTO{
			Calculations:   rep.Totals.Calculations,
			Material:       rep.Totals.Material,
			Machine:        rep.Totals.Machine,
			Labor:          rep.Totals.Labor,
			Overhead:       rep.Totals.Overhead,
			ProductionCost: rep.Totals.ProductionCost,
			Margin:         rep.Totals.Margin,
			Discount:       rep.Totals.Discount,
			PreTax:         rep.Totals.PreTax,
			Tax:            rep.Totals.Tax,
			FinalPrice:     rep.Totals.FinalPrice,
			AvgFinalPrice:  rep.Totals.AvgFinalPrice,
			Currency:       rep.Totals.Currency,
		},
		Series: make([]costPointDTO, 0, len(rep.Series)),
	}
	for _, p := range rep.Series {
		d.Series = append(d.Series, costPointDTO{
			Bucket:        p.Bucket.Format("2006-01-02"),
			Calculations:  p.Calculations,
			FinalPrice:    p.FinalPrice,
			AvgFinalPrice: p.AvgFinalPrice,
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

// handleProjectsAnalytics — GET /api/v1/admin/analytics/projects (auth+admin).
// 200 — отчёт Project Analytics; 403 — нет права analytics.read;
// 422 — невалидный диапазон; 500 — внутренняя ошибка.
func handleProjectsAnalytics(svc AnalyticsService) http.HandlerFunc {
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

		rep, err := svc.Projects(r.Context(), tenantID(r.Context()), from, to)
		if err != nil {
			switch {
			case errors.Is(err, analytics.ErrInvalidRange):
				writeError(w, http.StatusUnprocessableEntity, "invalid_range", "from must not be after to")
			default:
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
			return
		}
		writeJSON(w, http.StatusOK, toProjectReportDTO(rep))
	}
}

// handleManufacturingAnalytics — GET /api/v1/admin/analytics/manufacturing
// (auth+admin). 200 — отчёт Manufacturing Analytics; 403 — нет права
// analytics.read; 422 — невалидные from/to/granularity; 500 — ошибка.
func handleManufacturingAnalytics(svc AnalyticsService) http.HandlerFunc {
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

		rep, err := svc.Manufacturing(r.Context(), tenantID(r.Context()), from, to, g)
		if err != nil {
			switch {
			case errors.Is(err, analytics.ErrInvalidRange):
				writeError(w, http.StatusUnprocessableEntity, "invalid_range", "from must not be after to")
			default:
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
			return
		}
		writeJSON(w, http.StatusOK, toManufacturingReportDTO(rep))
	}
}

// handleCostAnalytics — GET /api/v1/admin/analytics/cost (auth+admin).
// 200 — отчёт Cost Analytics; 403 — нет права analytics.read;
// 422 — невалидные from/to/granularity; 500 — ошибка.
func handleCostAnalytics(svc AnalyticsService) http.HandlerFunc {
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

		rep, err := svc.Cost(r.Context(), tenantID(r.Context()), from, to, g)
		if err != nil {
			switch {
			case errors.Is(err, analytics.ErrInvalidRange):
				writeError(w, http.StatusUnprocessableEntity, "invalid_range", "from must not be after to")
			default:
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
			return
		}
		writeJSON(w, http.StatusOK, toCostReportDTO(rep))
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
