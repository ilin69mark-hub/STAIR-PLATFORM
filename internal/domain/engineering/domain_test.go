package engineering

import (
	"testing"
)

func TestLengthValidation(t *testing.T) {
	l, err := NewLength(250.0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !l.Equals(Length(250)) {
		t.Fatal("length equality failed")
	}

	if _, err := NewLength(-1); err == nil {
		t.Fatal("negative length must be rejected")
	}
}

func TestAngleUnits(t *testing.T) {
	a, err := NewAngleDegrees(90)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := a.Degrees(); got != 90 {
		t.Fatalf("expected 90 degrees, got %.2f", got)
	}
	if a.Radians() <= 1.5 || a.Radians() >= 1.6 {
		t.Fatalf("expected ~pi/2 radians, got %f", a.Radians())
	}
}

func TestParameterLifecycle(t *testing.T) {
	p := NewParameter("p", "step.height", "mm", 180.0, SourceUser)
	if p.State != ParameterCreated {
		t.Fatal("new parameter must be created")
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate error: %v", err)
	}
	if p.State != ParameterValidated {
		t.Fatal("parameter must be validated")
	}
	if err := p.Apply(); err != nil {
		t.Fatalf("apply error: %v", err)
	}
	if p.State != ParameterApplied {
		t.Fatal("parameter must be applied")
	}

	// формульные параметры вычисляются, а не задаются вручную.
	calc := NewParameter("c", "count", "int", nil, SourceFormula)
	if err := calc.Calculate(nil); err != nil {
		t.Fatalf("formula calculate error: %v", err)
	}
	if calc.State != ParameterCalculated {
		t.Fatal("formula parameter must be calculated")
	}
}

func TestParameterValidateRejectsEmpty(t *testing.T) {
	p := NewParameter("", "", "mm", 1.0, SourceUser)
	if err := p.Validate(); err == nil {
		t.Fatal("parameter without id/name must fail validation")
	}
}

func TestRevisionImmutable(t *testing.T) {
	snap := "model-v1"
	r1, err := NewRevision("author", "initial", snap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r2, err := r1.Child("author", "change", "model-v2")
	if err != nil {
		t.Fatalf("child error: %v", err)
	}
	if r2.Parent != r1 {
		t.Fatal("child must reference parent")
	}
	if r1.State != RevisionDraft {
		t.Fatal("parent state must be preserved")
	}
}

func TestRevisionRelease(t *testing.T) {
	r, err := NewRevision("author", "release", "model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.Release(); err != nil {
		t.Fatalf("release error: %v", err)
	}
	if r.State != RevisionReleased {
		t.Fatalf("expected released, got %s", r.State)
	}

	// released-ревизия неизменяема — new Child запрещён.
	if _, err := r.Child("x", "y", "z"); err == nil {
		t.Fatal("child of released revision must fail")
	}
}

func TestEngineeringObjectStateMachine(t *testing.T) {
	obj, err := NewObject("proj-1", KindProject, "owner", "init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := obj.Transition(StateDraft); err != nil {
		t.Fatalf("draft transition error: %v", err)
	}
	if err := obj.Transition(StateEditing); err != nil {
		t.Fatalf("editing transition error: %v", err)
	}
	if err := obj.Transition(StateCalculated); err != nil {
		t.Fatalf("calculated transition error: %v", err)
	}
	if err := obj.Transition(StateValidated); err != nil {
		t.Fatalf("validated transition error: %v", err)
	}
	if err := obj.Transition(StateApproved); err != nil {
		t.Fatalf("approved transition error: %v", err)
	}
	if err := obj.Transition(StateReleased); err != nil {
		t.Fatalf("released transition error: %v", err)
	}

	// недопустимый переход (Released → Editing).
	if err := obj.Transition(StateEditing); err == nil {
		t.Fatal("illegal transition must be rejected")
	}
}

func TestEngineeringObjectCommitHistory(t *testing.T) {
	obj, err := NewObject("proj-1", KindProject, "owner", "init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := obj.Commit("owner", "second revision"); err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if len(obj.History()) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(obj.History()))
	}
	if len(obj.Events()) == 0 {
		t.Fatal("expected domain events recorded")
	}
}

func TestStairConfigurationValidation(t *testing.T) {
	w, _ := NewLength(900)
	h, _ := NewLength(2700)
	cfg, err := NewStairConfiguration(w, h, FlightStraight)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate error: %v", err)
	}

	if _, err := NewStairConfiguration(w, h, ""); err == nil {
		t.Fatal("empty flight type must be rejected")
	}
}

func TestSpiralStairConfigurationGuard(t *testing.T) {
	w, _ := NewLength(1000)
	h, _ := NewLength(2700)
	r, _ := NewLength(1500)

	// Валидная спираль: R > W.
	cfg := &StairConfiguration{Width: w, Height: h, Flight: FlightSpiral, StepCount: 15, OuterRadius: r}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate error: %v", err)
	}

	// R == W — колонна нулевого радиуса (EDR-0007 §4.4).
	bad, _ := NewLength(1000)
	cfg.OuterRadius = bad
	if err := cfg.Validate(); err == nil {
		t.Fatal("spiral with R == W must be rejected")
	}

	// R < W — отрицательный радиус колонны.
	small, _ := NewLength(800)
	cfg.OuterRadius = small
	if err := cfg.Validate(); err == nil {
		t.Fatal("spiral with R < W must be rejected")
	}
}

func TestProjectAggregate(t *testing.T) {
	proj, err := NewProject("proj-1", "owner", "my staircase", "create")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj.ID() != "proj-1" {
		t.Fatalf("unexpected id: %s", proj.ID())
	}
	if err := proj.Rename("renamed"); err != nil {
		t.Fatalf("rename error: %v", err)
	}
	if proj.Name() != "renamed" {
		t.Fatal("name not updated")
	}

	if _, err := NewProject("proj-2", "owner", "", "x"); err == nil {
		t.Fatal("project without name must be rejected")
	}
}

func TestNewObjectValidation(t *testing.T) {
	if _, err := NewObject("", KindProject, "owner", "init"); err == nil {
		t.Fatal("object without id must be rejected")
	}
	if _, err := NewObject("proj-1", KindProject, "", "init"); err == nil {
		t.Fatal("object without owner must be rejected")
	}
	obj, err := NewObject("proj-1", KindProject, "owner", "init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.State != StateCreated || obj.Current == nil {
		t.Fatalf("expected created state with initial revision, got %+v", obj.State)
	}
}

func TestObjectParameterLifecycle(t *testing.T) {
	obj, err := NewObject("proj-1", KindProject, "owner", "init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := NewParameter("w", "width", "mm", 900.0, SourceUser)
	if err := obj.AddParameter(p); err != nil {
		t.Fatalf("add parameter error: %v", err)
	}
	if err := obj.AddParameter(p); err == nil {
		t.Fatal("duplicate parameter must be rejected")
	}

	got, ok := obj.Parameter("w")
	if !ok || got.ID != "w" {
		t.Fatalf("parameter lookup failed: %+v, %v", got, ok)
	}
	if _, ok := obj.Parameter("missing"); ok {
		t.Fatal("missing parameter must not be found")
	}
}

func TestParameterModifyAndArchive(t *testing.T) {
	p := NewParameter("p", "step.height", "mm", 180.0, SourceUser)
	if err := p.Modify(200.0); err != nil {
		t.Fatalf("modify error: %v", err)
	}
	if p.State != ParameterModified {
		t.Fatalf("expected modified, got %s", p.State)
	}
	if err := p.Apply(); err != nil {
		t.Fatalf("apply after modify: %v", err)
	}
	if p.State != ParameterApplied {
		t.Fatalf("expected applied, got %s", p.State)
	}

	p.Archive()
	if p.State != ParameterArchived {
		t.Fatalf("expected archived, got %s", p.State)
	}
}

func TestParameterRecalculate(t *testing.T) {
	calc := NewParameter("c", "count", "int", nil, SourceFormula)
	if err := calc.Calculate(nil); err != nil {
		t.Fatalf("calculate error: %v", err)
	}
	calc.Recalculate(nil)
	if calc.State != ParameterRecalculated {
		t.Fatalf("expected recalculated, got %s", calc.State)
	}
}

func TestRevisionAdvance(t *testing.T) {
	r, err := NewRevision("author", "draft", "model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.Advance(); err != nil {
		t.Fatalf("advance error: %v", err)
	}
	if r.State != RevisionWorking {
		t.Fatalf("expected working, got %s", r.State)
	}
}
