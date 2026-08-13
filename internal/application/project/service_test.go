package project

import (
	"context"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
)

// fakeRepo — тестовая реализация Repository в памяти.
type fakeRepo struct {
	projects     map[string]*Project
	configs      map[string][]*StairConfiguration
	calculations map[string][]*Calculation
	next         int
	err          error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		projects:     map[string]*Project{},
		configs:      map[string][]*StairConfiguration{},
		calculations: map[string][]*Calculation{},
	}
}

func (f *fakeRepo) CreateProject(ctx context.Context, p *Project) error {
	if f.err != nil {
		return f.err
	}
	f.next++
	p.ID = itoa(f.next)
	f.projects[p.ID] = p
	return nil
}

func (f *fakeRepo) GetProject(ctx context.Context, id string) (*Project, error) {
	p, ok := f.projects[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (f *fakeRepo) ListProjects(ctx context.Context) ([]*Project, error) {
	var out []*Project
	for _, p := range f.projects {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeRepo) SaveConfiguration(ctx context.Context, c *StairConfiguration) error {
	f.configs[c.ProjectID] = append(f.configs[c.ProjectID], c)
	return nil
}

func (f *fakeRepo) GetLatestConfiguration(ctx context.Context, projectID string) (*StairConfiguration, error) {
	cfgs := f.configs[projectID]
	if len(cfgs) == 0 {
		return nil, ErrNotFound
	}
	return cfgs[len(cfgs)-1], nil
}

func (f *fakeRepo) SaveCalculationWithConfig(ctx context.Context, cfg *StairConfiguration, snap Snapshot) (*Calculation, error) {
	if err := f.SaveConfiguration(ctx, cfg); err != nil {
		return nil, err
	}
	calc := &Calculation{
		ProjectID: cfg.ProjectID,
		Valid:     snap.Validation.Valid,
		Blocking:  snap.Validation.Blocking,
		Result:    []byte("{}"),
	}
	calc.ConfigurationID = cfg.ID
	if err := f.SaveCalculation(ctx, calc); err != nil {
		return nil, err
	}
	return calc, nil
}

func (f *fakeRepo) SaveCalculation(ctx context.Context, c *Calculation) error {
	if c.ConfigurationID == "" {
		c.ConfigurationID = "cfg-" + c.ProjectID
	}
	f.calculations[c.ProjectID] = append(f.calculations[c.ProjectID], c)
	return nil
}

func (f *fakeRepo) GetLatestCalculation(ctx context.Context, projectID string) (*Calculation, error) {
	cs := f.calculations[projectID]
	if len(cs) == 0 {
		return nil, ErrNotFound
	}
	return cs[len(cs)-1], nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func testConfig() stair.Config {
	return stair.Config{
		Width:             engineering.Length(900),
		Height:            engineering.Length(2700),
		Flight:            engineering.FlightStraight,
		StepHeight:        engineering.Length(180),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		Clearance:         engineering.Length(80),
		RailingHeight:     engineering.Length(900),
	}
}

func TestCreateProject(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())

	p, err := svc.CreateProject(context.Background(), "Лестница на 2 этаж", "Заказ 1")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected assigned ID")
	}
	if p.Status != "draft" {
		t.Fatalf("expected status draft, got %q", p.Status)
	}

	got, err := svc.GetProject(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "Лестница на 2 этаж" {
		t.Fatalf("name mismatch: %q", got.Name)
	}
}

func TestCreateProjectRequiresName(t *testing.T) {
	svc := NewService(newFakeRepo(), stair.NewService(), DefaultRules())
	if _, err := svc.CreateProject(context.Background(), "", ""); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGetProjectNotFound(t *testing.T) {
	svc := NewService(newFakeRepo(), stair.NewService(), DefaultRules())
	if _, err := svc.GetProject(context.Background(), "missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCalculateSavesConfigAndResult(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())

	p, err := svc.CreateProject(context.Background(), "Тест", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	calc, err := svc.Calculate(context.Background(), p.ID, testConfig(), stair.Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if !calc.Valid {
		t.Fatal("expected valid calculation")
	}
	if calc.Blocking {
		t.Fatal("expected non-blocking")
	}
	if len(calc.Result) == 0 {
		t.Fatal("expected result snapshot")
	}

	cfg, err := svc.GetLatestConfig(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetLatestConfig: %v", err)
	}
	if cfg.WidthMM != 900 || cfg.HeightMM != 2700 {
		t.Fatalf("config mismatch: %+v", cfg)
	}

	got, err := svc.GetResult(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if got.ConfigurationID != calc.ConfigurationID {
		t.Fatalf("calculation not linked to config: %s vs %s", got.ConfigurationID, calc.ConfigurationID)
	}
}

func TestCalculateProjectNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	if _, err := svc.Calculate(context.Background(), "missing", testConfig(), stair.Options{}); err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestCalculateBlockingValidation(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), "Тест", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	cfg := testConfig()
	cfg.StepHeight = engineering.Length(60) // вне допустимого диапазона
	calc, err := svc.Calculate(context.Background(), p.ID, cfg, stair.Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if calc.Valid || !calc.Blocking {
		t.Fatalf("expected blocking validation (Valid=%v Blocking=%v)", calc.Valid, calc.Blocking)
	}
}
