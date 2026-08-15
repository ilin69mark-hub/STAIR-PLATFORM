// Package analytics реализует прикладной слой Analytics bounded context
// (Phase F, BC-043): агрегации активности tenant (Usage Analytics) —
// EDR-0028. Application зависит от порта Repository (BE-0005); транспорт
// и БД — вне слоя (ADR-0006). Аналитика read-only: источник данных —
// операционные таблицы (users, projects, calculations, audit_events,
// payment_intents); новых миграций нет.
package analytics

import (
	"context"
	"errors"
	"time"
)

// Granularity — шаг временного бакета серии (EDR-0028 §3.1).
type Granularity string

const (
	GranularityDay   Granularity = "day"
	GranularityWeek  Granularity = "week"
	GranularityMonth Granularity = "month"
)

// Valid возвращает true, если гранулярность входит в каталог.
func (g Granularity) Valid() bool {
	switch g {
	case GranularityDay, GranularityWeek, GranularityMonth:
		return true
	}
	return false
}

// Interval возвращает SQL-шаг generate_series для гранулярности.
func (g Granularity) Interval() string {
	switch g {
	case GranularityDay:
		return "1 day"
	case GranularityWeek:
		return "1 week"
	default:
		return "1 month"
	}
}

var (
	// ErrInvalidRange — невалидный диапазон (from > to или пустые даты).
	ErrInvalidRange = errors.New("analytics: invalid time range")
	// ErrInvalidGranularity — гранулярность вне каталога.
	ErrInvalidGranularity = errors.New("analytics: invalid granularity")
)

// UsagePoint — счётчики активности в одном временном бакете (EDR-0028 §3.1).
type UsagePoint struct {
	Bucket          time.Time
	Logins          int
	ActiveUsers     int
	ProjectsCreated int
	Calculations    int
	Exports         int
	Payments        int
}

// UsageTotals — итоговые счётчики tenant за окно (EDR-0028 §3.1).
type UsageTotals struct {
	Users        int
	ActiveUsers  int
	Projects     int
	Calculations int
	Logins       int
	Exports      int
	Payments     int
}

// UsageReport — полный ответ Usage Analytics: итоги + серия по бакетам.
type UsageReport struct {
	From        time.Time
	To          time.Time
	Granularity Granularity
	Totals      UsageTotals
	Series      []UsagePoint
}

// Repository — порт доступа к данным аналитики (BE-0005).
type Repository interface {
	// UsageTotals возвращает итоговые счётчики tenant за окно [from, to].
	UsageTotals(ctx context.Context, tenantID string, from, to time.Time) (UsageTotals, error)
	// UsageSeries возвращает непрерывный ряд счётчиков по бакетам
	// гранулярности g в окне [from, to] (пустые бакеты заполнены нулями).
	UsageSeries(ctx context.Context, tenantID string, from, to time.Time, g Granularity) ([]UsagePoint, error)
	// ProjectTotals возвращает агрегаты по проектам tenant за окно [from, to].
	ProjectTotals(ctx context.Context, tenantID string, from, to time.Time) (ProjectTotals, error)
	// ProjectList возвращает сводку по каждому проекту tenant.
	ProjectList(ctx context.Context, tenantID string) ([]ProjectRow, error)
}

// ProjectRow — сводка по одному проекту tenant (EDR-0029 §3.1).
type ProjectRow struct {
	ID             string
	Name           string
	Status         string
	OwnerEmail     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Configurations int
	Calculations   int
	// LatestCalculationValid — валидность последнего расчёта; nil, если
	// у проекта нет ни одного расчёта.
	LatestCalculationValid *bool
	Comments               int
	Members                int
}

// ProjectTotals — агрегаты по проектам tenant (EDR-0029 §3.1).
type ProjectTotals struct {
	Projects                int
	ProjectsCreated         int
	ByStatus                map[string]int
	ProjectsWithCalculation int
	ValidProjects           int
	Configurations          int
	Calculations            int
	Comments                int
}

// ProjectReport — полный ответ Project Analytics: агрегаты + сводки.
type ProjectReport struct {
	From     time.Time
	To       time.Time
	Totals   ProjectTotals
	Projects []ProjectRow
}
