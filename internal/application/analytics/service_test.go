package analytics

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeRepo — тестовый репозиторий аналитики.
type fakeRepo struct {
	totals    UsageTotals
	series    []UsagePoint
	totalsErr error
	seriesErr error
	gotFrom   time.Time
	gotTo     time.Time
	gotGran   Granularity

	projTotals  ProjectTotals
	projRows    []ProjectRow
	projTotErr  error
	projListErr error
}

func (f *fakeRepo) UsageTotals(_ context.Context, _ string, from, to time.Time) (UsageTotals, error) {
	if f.totalsErr != nil {
		return UsageTotals{}, f.totalsErr
	}
	f.gotFrom = from
	f.gotTo = to
	return f.totals, nil
}

func (f *fakeRepo) UsageSeries(_ context.Context, _ string, from, to time.Time, g Granularity) ([]UsagePoint, error) {
	if f.seriesErr != nil {
		return nil, f.seriesErr
	}
	f.gotFrom = from
	f.gotTo = to
	f.gotGran = g
	return f.series, nil
}

func (f *fakeRepo) ProjectTotals(_ context.Context, _ string, from, to time.Time) (ProjectTotals, error) {
	if f.projTotErr != nil {
		return ProjectTotals{}, f.projTotErr
	}
	f.gotFrom = from
	f.gotTo = to
	return f.projTotals, nil
}

func (f *fakeRepo) ProjectList(_ context.Context, _ string) ([]ProjectRow, error) {
	if f.projListErr != nil {
		return nil, f.projListErr
	}
	return f.projRows, nil
}

func TestUsage(t *testing.T) {
	repo := &fakeRepo{
		totals: UsageTotals{Users: 3, ActiveUsers: 2, Projects: 1, Calculations: 4, Logins: 5, Exports: 1, Payments: 1},
		series: []UsagePoint{{Bucket: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Logins: 1}},
	}
	svc := NewService(repo)
	from := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	rep, err := svc.Usage(context.Background(), "t-1", from, to, GranularityDay)
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if rep.Totals.Users != 3 || rep.Totals.Payments != 1 {
		t.Fatalf("unexpected totals: %+v", rep.Totals)
	}
	if len(rep.Series) != 1 || rep.Series[0].Logins != 1 {
		t.Fatalf("unexpected series: %+v", rep.Series)
	}
	if !rep.From.Equal(from.UTC()) || !rep.To.Equal(to.UTC()) {
		t.Fatalf("from/to not normalized: %v %v", rep.From, rep.To)
	}
}

func TestUsageInvalidGranularity(t *testing.T) {
	svc := NewService(&fakeRepo{})
	_, err := svc.Usage(context.Background(), "t-1",
		time.Now(), time.Now().Add(time.Hour), Granularity("hour"))
	if !errors.Is(err, ErrInvalidGranularity) {
		t.Fatalf("expected ErrInvalidGranularity, got %v", err)
	}
}

func TestUsageInvalidRange(t *testing.T) {
	svc := NewService(&fakeRepo{})
	_, err := svc.Usage(context.Background(), "t-1",
		time.Now().Add(time.Hour), time.Now(), GranularityDay)
	if !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("expected ErrInvalidRange, got %v", err)
	}
}

func TestUsagePropagatesRepoErrors(t *testing.T) {
	svc := NewService(&fakeRepo{totalsErr: errors.New("db down")})
	_, err := svc.Usage(context.Background(), "t-1",
		time.Now(), time.Now().Add(time.Hour), GranularityDay)
	if err == nil {
		t.Fatal("expected error from totals")
	}
}

func TestGranularityInterval(t *testing.T) {
	cases := map[Granularity]string{
		GranularityDay: "1 day", GranularityWeek: "1 week", GranularityMonth: "1 month",
	}
	for g, want := range cases {
		if g.Interval() != want {
			t.Fatalf("Interval(%s) = %q, want %q", g, g.Interval(), want)
		}
	}
	if !GranularityDay.Valid() || Granularity("hour").Valid() {
		t.Fatal("Valid() misbehaves")
	}
}

func TestProjects(t *testing.T) {
	valid := true
	repo := &fakeRepo{
		projTotals: ProjectTotals{
			Projects: 2, ProjectsCreated: 1, ByStatus: map[string]int{"draft": 1, "approved": 1},
			ProjectsWithCalculation: 1, ValidProjects: 1, Configurations: 2,
			Calculations: 1, Comments: 3,
		},
		projRows: []ProjectRow{
			{ID: "p-1", Name: "A", Status: "approved", OwnerEmail: "o@e.com",
				Configurations: 2, Calculations: 1, LatestCalculationValid: &valid, Comments: 3, Members: 2},
		},
	}
	svc := NewService(repo)
	from := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	rep, err := svc.Projects(context.Background(), "t-1", from, to)
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if rep.Totals.Projects != 2 || rep.Totals.ByStatus["approved"] != 1 {
		t.Fatalf("unexpected totals: %+v", rep.Totals)
	}
	if len(rep.Projects) != 1 || rep.Projects[0].Name != "A" ||
		rep.Projects[0].LatestCalculationValid == nil || !*rep.Projects[0].LatestCalculationValid {
		t.Fatalf("unexpected rows: %+v", rep.Projects)
	}
	if !rep.From.Equal(from.UTC()) || !rep.To.Equal(to.UTC()) {
		t.Fatalf("from/to not normalized: %v %v", rep.From, rep.To)
	}
}

func TestProjectsInvalidRange(t *testing.T) {
	svc := NewService(&fakeRepo{})
	_, err := svc.Projects(context.Background(), "t-1",
		time.Now().Add(time.Hour), time.Now())
	if !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("expected ErrInvalidRange, got %v", err)
	}
}

func TestProjectsPropagatesRepoErrors(t *testing.T) {
	svc := NewService(&fakeRepo{projTotErr: errors.New("db down")})
	_, err := svc.Projects(context.Background(), "t-1",
		time.Now(), time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected error from project totals")
	}
}
