package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"stairplatform/internal/application/analytics"
	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/payments"
	"stairplatform/internal/application/project"
	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/domain/pricing"
	"stairplatform/internal/engine/validation"
)

func newAnalyticsRepo(t *testing.T) (*AnalyticsRepository, *ProjectRepository) {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return NewAnalyticsRepository(pool), NewProjectRepository(pool)
}

// seedUsage создаёт собственный tenant с активностью: пользователь, проект,
// расчёт, аудит-события (login, export) и оплаченный платёжный интент —
// все в окне [t0-1h, t0+1h]. Возвращает tenant ID. Изолированный tenant
// нужен, потому что дефолтный tenant стойкий между прогонами и его не
// должны загрязнять другие интеграционные тесты (порядок исполнения).
func seedUsage(t *testing.T, ctx context.Context, pr *ProjectRepository, t0 time.Time) string {
	t.Helper()
	tenant := createTestTenant(t, pr, fmt.Sprintf("usage-%d", time.Now().UnixNano()))
	owner := testOwnerID(t, pr, tenant)
	projID := testProject(t, ctx, pr, tenant, "Usage project")

	// Расчёт: конфигурация + снапшот.
	cfg := &project.StairConfiguration{
		ProjectID: projID, WidthMM: 900, HeightMM: 3000, Flight: "straight",
		StepHeightMM: 180, StringerThicknessMM: 40, StepThicknessMM: 30,
		ClearanceMM: 30, RailingHeightMM: 900, ComfortStepMM: 300,
	}
	if _, err := pr.SaveCalculationWithConfig(ctx, tenant, cfg, sampleSnapshot(projID)); err != nil {
		t.Fatalf("SaveCalculationWithConfig: %v", err)
	}

	// Аудит: вход и экспорт (автор — владелец).
	ar := NewAuditRepository(pr.pool)
	for _, act := range []audit.Action{audit.ActionAuthLogin, audit.ActionDataExported} {
		ev := &audit.Event{TenantID: tenant, ActorID: owner, ProjectID: projID,
			Action: act, Result: audit.ResultOK, CreatedAt: t0}
		if err := ar.Insert(ctx, ev); err != nil {
			t.Fatalf("audit insert: %v", err)
		}
	}

	// Оплаченный интент в окне.
	payRepo := NewPaymentRepository(pr.pool)
	if err := payRepo.CreateIntent(ctx, &payments.PaymentIntent{
		TenantID: tenant, ProjectID: projID, UserID: owner, AmountMinor: 10000,
		Currency: "USD", Status: payments.StatusPaid, Provider: "mock",
		ProviderCheckoutID: "chk-analytics-" + itoaUD(),
	}); err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}
	return tenant
}

func TestUsageTotals(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	tenant := seedUsage(t, ctx, pr, t0)

	totals, err := repo.UsageTotals(ctx, tenant, t0.Add(-time.Hour), t0.Add(time.Hour))
	if err != nil {
		t.Fatalf("UsageTotals: %v", err)
	}
	if totals.Users < 1 {
		t.Fatalf("expected at least 1 user, got %d", totals.Users)
	}
	if totals.ActiveUsers < 1 {
		t.Fatalf("expected at least 1 active user, got %d", totals.ActiveUsers)
	}
	if totals.Projects < 1 {
		t.Fatalf("expected at least 1 project, got %d", totals.Projects)
	}
	if totals.Calculations < 1 {
		t.Fatalf("expected at least 1 calculation, got %d", totals.Calculations)
	}
	if totals.Logins < 1 {
		t.Fatalf("expected at least 1 login, got %d", totals.Logins)
	}
	if totals.Exports < 1 {
		t.Fatalf("expected at least 1 export, got %d", totals.Exports)
	}
	if totals.Payments < 1 {
		t.Fatalf("expected at least 1 payment, got %d", totals.Payments)
	}
}

func TestUsageSeriesContinuous(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	tenant := seedUsage(t, ctx, pr, t0)

	from := t0.Add(-48 * time.Hour)
	to := t0.Add(24 * time.Hour)
	series, err := repo.UsageSeries(ctx, tenant, from, to, analytics.GranularityDay)
	if err != nil {
		t.Fatalf("UsageSeries: %v", err)
	}
	// Окно 48h+24h => не менее 3 дневных бакетов; непрерывность (нули в пустых).
	if len(series) < 3 {
		t.Fatalf("expected continuous series >= 3 buckets, got %d", len(series))
	}
	for i := 1; i < len(series); i++ {
		prev := series[i-1].Bucket
		cur := series[i].Bucket
		if !cur.After(prev) || cur.Sub(prev).Hours() != 24 {
			t.Fatalf("buckets not 24h apart: %v -> %v", prev, cur)
		}
	}
	// Хотя бы в одном бакете есть активность.
	found := false
	for _, p := range series {
		if p.Logins > 0 && p.Payments > 0 && p.Calculations > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a bucket with seeded activity: %+v", series)
	}
}

func TestUsageSeriesEmptyTenant(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	// Свежий tenant без активности (дефолтный tenant стойкий между прогонами).
	tenant := createTestTenant(t, pr, fmt.Sprintf("empty-%d", time.Now().UnixNano()))

	from := time.Now().UTC().Add(-time.Hour)
	to := time.Now().UTC().Add(time.Hour)
	totals, err := repo.UsageTotals(ctx, tenant, from, to)
	if err != nil {
		t.Fatalf("UsageTotals: %v", err)
	}
	if totals.Logins != 0 || totals.Payments != 0 {
		t.Fatalf("expected zero activity for empty tenant: %+v", totals)
	}
}

// createTestTenant вставляет новый tenant и возвращает его ID.
func createTestTenant(t *testing.T, pr *ProjectRepository, slug string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := pr.pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`, slug, slug).Scan(&id); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	return id
}

// TestUsageTotalsOtherTenantIsolation — активность в одном tenant не видна
// из другого (SEC-0005).
func TestUsageTotalsIsolation(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	tenantA := seedUsage(t, ctx, pr, t0)

	// Чужой tenant без активности.
	other := createTestTenant(t, pr, fmt.Sprintf("other-%d", time.Now().UnixNano()))

	from := t0.Add(-time.Hour)
	to := t0.Add(time.Hour)
	totals, err := repo.UsageTotals(ctx, other, from, to)
	if err != nil {
		t.Fatalf("UsageTotals: %v", err)
	}
	if totals.Logins != 0 || totals.Payments != 0 || totals.Calculations != 0 {
		t.Fatalf("tenant isolation violated: %+v", totals)
	}
	_ = tenantA
}

// seedProjects создаёт tenant с проектами и активностью для F2 (EDR-0029):
// два проекта (draft и approved), у approved — конфигурация + расчёт
// (валидный), комментарий и второй участник. Возвращает tenant ID.
func seedProjects(t *testing.T, ctx context.Context, pr *ProjectRepository) string {
	t.Helper()
	tenant := createTestTenant(t, pr, fmt.Sprintf("proj-%d", time.Now().UnixNano()))
	owner := testOwnerID(t, pr, tenant)

	// draft-проект без конфигураций.
	draftID := testProject(t, ctx, pr, tenant, "Draft project")

	// approved-проект: конфигурация + валидный расчёт.
	approvedID := testProject(t, ctx, pr, tenant, "Approved project")
	cfg := &project.StairConfiguration{
		ProjectID: approvedID, WidthMM: 900, HeightMM: 3000, Flight: "straight",
		StepHeightMM: 180, StringerThicknessMM: 40, StepThicknessMM: 30,
		ClearanceMM: 30, RailingHeightMM: 900, ComfortStepMM: 300,
	}
	if _, err := pr.SaveCalculationWithConfig(ctx, tenant, cfg, sampleSnapshot(approvedID)); err != nil {
		t.Fatalf("SaveCalculationWithConfig: %v", err)
	}

	// Ревью-поток: draft → in_review → approved (рецензент ≠ автор запроса).
	rv, err := pr.RequestReview(ctx, tenant, approvedID, owner, "F2 review request")
	if err != nil {
		t.Fatalf("RequestReview: %v", err)
	}
	reviewer := testOwnerID(t, pr, tenant)
	if _, err := pr.DecideReview(ctx, tenant, approvedID, rv.ID, reviewer, project.ReviewApproved, "looks good"); err != nil {
		t.Fatalf("DecideReview: %v", err)
	}

	// Второй участник + комментарий (только на approved-проекте).
	memberID := testOwnerID(t, pr, tenant)
	if err := pr.AddMember(ctx, tenant, approvedID, &project.ProjectMember{
		ProjectID: approvedID, UserID: memberID, Role: project.RoleEditor,
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if _, err := pr.AddComment(ctx, tenant, approvedID, &project.Comment{
		ProjectID: approvedID, AuthorID: owner, Body: "F2 seed comment",
	}); err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	_ = draftID
	return tenant
}

func TestProjectTotals(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	tenant := seedProjects(t, ctx, pr)

	from := t0.Add(-time.Hour)
	to := t0.Add(time.Hour)
	totals, err := repo.ProjectTotals(ctx, tenant, from, to)
	if err != nil {
		t.Fatalf("ProjectTotals: %v", err)
	}
	if totals.Projects != 2 {
		t.Fatalf("expected 2 projects, got %d", totals.Projects)
	}
	if totals.ProjectsCreated != 2 {
		t.Fatalf("expected 2 created in window, got %d", totals.ProjectsCreated)
	}
	if totals.ByStatus["draft"] != 1 || totals.ByStatus["approved"] != 1 {
		t.Fatalf("unexpected by_status: %+v", totals.ByStatus)
	}
	if totals.ProjectsWithCalculation != 1 {
		t.Fatalf("expected 1 project with calculation, got %d", totals.ProjectsWithCalculation)
	}
	if totals.ValidProjects != 1 {
		t.Fatalf("expected 1 valid project, got %d", totals.ValidProjects)
	}
	if totals.Configurations != 1 || totals.Calculations != 1 || totals.Comments != 1 {
		t.Fatalf("unexpected counts: configs=%d calcs=%d comments=%d",
			totals.Configurations, totals.Calculations, totals.Comments)
	}
}

func TestProjectList(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	tenant := seedProjects(t, ctx, pr)

	rows, err := repo.ProjectList(ctx, tenant)
	if err != nil {
		t.Fatalf("ProjectList: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	byName := map[string]analytics.ProjectRow{}
	for _, r := range rows {
		byName[r.Name] = r
	}
	appr, ok := byName["Approved project"]
	if !ok {
		t.Fatalf("missing approved project: %+v", rows)
	}
	if appr.Configurations != 1 || appr.Calculations != 1 || appr.Comments != 1 || appr.Members != 2 {
		t.Fatalf("unexpected approved row: %+v", appr)
	}
	if appr.LatestCalculationValid == nil || !*appr.LatestCalculationValid {
		t.Fatalf("expected valid latest calculation: %+v", appr)
	}
	draft, ok := byName["Draft project"]
	if !ok {
		t.Fatalf("missing draft project: %+v", rows)
	}
	if draft.Configurations != 0 || draft.Calculations != 0 || draft.Members != 1 {
		t.Fatalf("unexpected draft row: %+v", draft)
	}
	if draft.LatestCalculationValid != nil {
		t.Fatalf("expected nil latest calculation for draft: %+v", draft)
	}
}

func TestProjectTotalsIsolation(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	seedProjects(t, ctx, pr)

	// Чужой tenant без проектов.
	other := createTestTenant(t, pr, fmt.Sprintf("otherproj-%d", time.Now().UnixNano()))
	from := t0.Add(-time.Hour)
	to := t0.Add(time.Hour)
	totals, err := repo.ProjectTotals(ctx, other, from, to)
	if err != nil {
		t.Fatalf("ProjectTotals: %v", err)
	}
	if totals.Projects != 0 || totals.ProjectsWithCalculation != 0 || totals.ValidProjects != 0 {
		t.Fatalf("tenant isolation violated: %+v", totals)
	}
	rows, err := repo.ProjectList(ctx, other)
	if err != nil {
		t.Fatalf("ProjectList: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows for foreign tenant, got %d", len(rows))
	}
}

// mfgSnapshot строит снапшот с производственным пакетом: 2 детали
// STEEL-S235 (тред + стрингер), BOM на 2 строки, карта раскроя и
// раскрой (PartCount=2, Sheets=1, Utilization=0.5).
func mfgSnapshot(projectID string) project.Snapshot {
	return project.Snapshot{
		ProjectID:  projectID,
		Validation: validation.Result{Valid: true, Blocking: false},
		Manufacturing: &manufacturing.ManufacturingPackage{
			Parts: []manufacturing.Part{
				{Number: "P-1", Kind: manufacturing.PartTread, Material: "STEEL-S235",
					Thickness: engineering.Length(40), Length: engineering.Length(800),
					Width: engineering.Length(270), SolidIndex: 0},
				{Number: "P-2", Kind: manufacturing.PartStringer, Material: "STEEL-S235",
					Thickness: engineering.Length(40), Length: engineering.Length(3000),
					Width: engineering.Length(270), SolidIndex: 1},
			},
			BOM: manufacturing.BOM{Lines: []manufacturing.BOMLine{
				{Number: 1, PartNumber: "P-1", Description: "tread", MaterialCode: "STEEL-S235",
					Thickness: engineering.Length(40), Quantity: 1,
					Length: engineering.Length(800), Width: engineering.Length(270)},
				{Number: 2, PartNumber: "P-2", Description: "stringer", MaterialCode: "STEEL-S235",
					Thickness: engineering.Length(40), Quantity: 1,
					Length: engineering.Length(3000), Width: engineering.Length(270)},
			}},
			CutList: manufacturing.CutList{Items: []manufacturing.CutItem{{
				PartNumber: "P-1", MaterialCode: "STEEL-S235",
				Thickness: engineering.Length(40), Length: engineering.Length(800),
				Width: engineering.Length(270), Quantity: 1,
			}}},
			Nesting: &manufacturing.NestingResult{
				Sheets: []manufacturing.SheetLayout{{
					MaterialCode: "STEEL-S235", Thickness: engineering.Length(40),
					Length: engineering.Length(2500), Width: engineering.Length(1250),
				}},
				PartCount: 2, PartArea: 1000, SheetArea: 2000, WasteArea: 1000, Utilization: 0.5,
			},
		},
	}
}

// seedManufacturing создаёт tenant с двумя расчётами, имеющими
// производственный пакет (по 2 детали STEEL-S235 каждый). Возвращает tenant.
func seedManufacturing(t *testing.T, ctx context.Context, pr *ProjectRepository) string {
	t.Helper()
	tenant := createTestTenant(t, pr, fmt.Sprintf("mfg-%d", time.Now().UnixNano()))
	for i := 0; i < 2; i++ {
		projID := testProject(t, ctx, pr, tenant, fmt.Sprintf("Mfg project %d", i))
		cfg := &project.StairConfiguration{
			ProjectID: projID, WidthMM: 900, HeightMM: 3000, Flight: "straight",
			StepHeightMM: 180, StringerThicknessMM: 40, StepThicknessMM: 30,
			ClearanceMM: 30, RailingHeightMM: 900, ComfortStepMM: 300,
		}
		if _, err := pr.SaveCalculationWithConfig(ctx, tenant, cfg, mfgSnapshot(projID)); err != nil {
			t.Fatalf("SaveCalculationWithConfig: %v", err)
		}
	}
	return tenant
}

func TestManufacturingTotals(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	tenant := seedManufacturing(t, ctx, pr)

	from := t0.Add(-time.Hour)
	to := t0.Add(time.Hour)
	totals, err := repo.ManufacturingTotals(ctx, tenant, from, to)
	if err != nil {
		t.Fatalf("ManufacturingTotals: %v", err)
	}
	if totals.Calculations != 2 {
		t.Fatalf("expected 2 calculations, got %d", totals.Calculations)
	}
	if totals.Parts != 4 {
		t.Fatalf("expected 4 parts, got %d", totals.Parts)
	}
	if totals.BomLines != 4 {
		t.Fatalf("expected 4 bom lines, got %d", totals.BomLines)
	}
	if totals.CutItems != 2 {
		t.Fatalf("expected 2 cut items, got %d", totals.CutItems)
	}
	if totals.Sheets != 2 {
		t.Fatalf("expected 2 sheets, got %d", totals.Sheets)
	}
	if totals.Materials["STEEL-S235"] != 4 {
		t.Fatalf("expected 4 STEEL-S235 parts, got %+v", totals.Materials)
	}
	if totals.Utilization != 0.5 {
		t.Fatalf("expected utilization 0.5, got %v", totals.Utilization)
	}
}

func TestManufacturingSeriesContinuous(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	tenant := seedManufacturing(t, ctx, pr)

	from := t0.Add(-48 * time.Hour)
	to := t0.Add(24 * time.Hour)
	series, err := repo.ManufacturingSeries(ctx, tenant, from, to, analytics.GranularityDay)
	if err != nil {
		t.Fatalf("ManufacturingSeries: %v", err)
	}
	if len(series) < 3 {
		t.Fatalf("expected continuous series >= 3 buckets, got %d", len(series))
	}
	found := false
	for _, p := range series {
		if p.Calculations > 0 && p.Parts > 0 {
			found = true
			if p.Parts != 4 {
				t.Fatalf("expected 4 parts in active bucket, got %d", p.Parts)
			}
		}
	}
	if !found {
		t.Fatalf("expected a bucket with seeded manufacturing: %+v", series)
	}
}

func TestManufacturingTotalsIsolation(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	seedManufacturing(t, ctx, pr)

	other := createTestTenant(t, pr, fmt.Sprintf("othermfg-%d", time.Now().UnixNano()))
	from := t0.Add(-time.Hour)
	to := t0.Add(time.Hour)
	totals, err := repo.ManufacturingTotals(ctx, other, from, to)
	if err != nil {
		t.Fatalf("ManufacturingTotals: %v", err)
	}
	if totals.Calculations != 0 || totals.Parts != 0 || len(totals.Materials) != 0 {
		t.Fatalf("tenant isolation violated: %+v", totals)
	}
}

// costSnapshot строит снапшот с ценовым брейкдауном (EDR-0031 §3.1):
// себестоимость 160, итоговая цена 220 RUB (2 расчёта → суммы ×2).
func costSnapshot(projectID string) project.Snapshot {
	return project.Snapshot{
		ProjectID:  projectID,
		Validation: validation.Result{Valid: true, Blocking: false},
		Pricing: &pricing.PriceBreakdown{
			Currency:       pricing.CurrencyRUB,
			Material:       100,
			Machine:        20,
			Labor:          30,
			Overhead:       10,
			ProductionCost: 160,
			Margin:         40,
			Discount:       0,
			PreTax:         200,
			Tax:            20,
			FinalPrice:     220,
		},
	}
}

// seedCost создаёт tenant с двумя расчётами, имеющими ценовой брейкдаун
// (по 220 RUB итоговая цена каждый). Возвращает tenant ID.
func seedCost(t *testing.T, ctx context.Context, pr *ProjectRepository) string {
	t.Helper()
	tenant := createTestTenant(t, pr, fmt.Sprintf("cost-%d", time.Now().UnixNano()))
	for i := 0; i < 2; i++ {
		projID := testProject(t, ctx, pr, tenant, fmt.Sprintf("Cost project %d", i))
		cfg := &project.StairConfiguration{
			ProjectID: projID, WidthMM: 900, HeightMM: 3000, Flight: "straight",
			StepHeightMM: 180, StringerThicknessMM: 40, StepThicknessMM: 30,
			ClearanceMM: 30, RailingHeightMM: 900, ComfortStepMM: 300,
		}
		if _, err := pr.SaveCalculationWithConfig(ctx, tenant, cfg, costSnapshot(projID)); err != nil {
			t.Fatalf("SaveCalculationWithConfig: %v", err)
		}
	}
	return tenant
}

func TestCostTotals(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	tenant := seedCost(t, ctx, pr)

	from := t0.Add(-time.Hour)
	to := t0.Add(time.Hour)
	totals, err := repo.CostTotals(ctx, tenant, from, to)
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	if totals.Calculations != 2 {
		t.Fatalf("expected 2 calculations, got %d", totals.Calculations)
	}
	if totals.FinalPrice != 440 {
		t.Fatalf("expected final_price 440, got %d", totals.FinalPrice)
	}
	if totals.ProductionCost != 320 {
		t.Fatalf("expected production_cost 320, got %d", totals.ProductionCost)
	}
	if totals.Material != 200 || totals.Machine != 40 || totals.Labor != 60 {
		t.Fatalf("unexpected cost elements: %+v", totals)
	}
	if totals.Tax != 40 || totals.Margin != 80 {
		t.Fatalf("unexpected margin/tax: %+v", totals)
	}
	if totals.Currency != "RUB" {
		t.Fatalf("expected currency RUB, got %q", totals.Currency)
	}
	if totals.AvgFinalPrice != 220 {
		t.Fatalf("expected avg final price 220, got %v", totals.AvgFinalPrice)
	}
}

func TestCostSeriesContinuous(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	tenant := seedCost(t, ctx, pr)

	from := t0.Add(-48 * time.Hour)
	to := t0.Add(24 * time.Hour)
	series, err := repo.CostSeries(ctx, tenant, from, to, analytics.GranularityDay)
	if err != nil {
		t.Fatalf("CostSeries: %v", err)
	}
	if len(series) < 3 {
		t.Fatalf("expected continuous series >= 3 buckets, got %d", len(series))
	}
	found := false
	for _, p := range series {
		if p.Calculations > 0 && p.FinalPrice > 0 {
			found = true
			if p.FinalPrice != 440 {
				t.Fatalf("expected 440 final_price in active bucket, got %d", p.FinalPrice)
			}
		}
	}
	if !found {
		t.Fatalf("expected a bucket with seeded cost: %+v", series)
	}
}

func TestCostTotalsIsolation(t *testing.T) {
	repo, pr := newAnalyticsRepo(t)
	ctx := context.Background()
	t0 := time.Now().UTC()
	seedCost(t, ctx, pr)

	other := createTestTenant(t, pr, fmt.Sprintf("othercost-%d", time.Now().UnixNano()))
	from := t0.Add(-time.Hour)
	to := t0.Add(time.Hour)
	totals, err := repo.CostTotals(ctx, other, from, to)
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	if totals.Calculations != 0 || totals.FinalPrice != 0 {
		t.Fatalf("tenant isolation violated: %+v", totals)
	}
}
