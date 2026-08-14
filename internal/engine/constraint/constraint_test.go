package constraint

import "testing"

func TestStandardProfileIntegrity(t *testing.T) {
	set := StandardProfile("standard")
	violations := ValidateIntegrity(set)
	if len(violations) > 0 {
		t.Fatalf("integrity violations: %v", violations)
	}
	// одна активная версия на код.
	ruleCount := 0
	activeCount := 0
	for _, versions := range set.Constraints {
		ruleCount++
		for _, c := range versions {
			if c.Active {
				activeCount++
			}
		}
	}
	if activeCount != ruleCount {
		t.Fatalf("expected one active version per rule, got %d/%d", activeCount, ruleCount)
	}
}

func TestDuplicateVersionRejected(t *testing.T) {
	set := NewSet("cs", "test")
	if err := set.Add(&Constraint{Code: "A", Category: "g", Version: 1}); err != nil {
		t.Fatalf("add error: %v", err)
	}
	if err := set.Add(&Constraint{Code: "A", Category: "g", Version: 1}); err == nil {
		t.Fatal("duplicate (code, version) must be rejected")
	}
	if err := set.Add(&Constraint{Code: "A", Category: "g", Version: 2}); err != nil {
		t.Fatalf("another version of the same code must be allowed: %v", err)
	}
}

func TestActivateSingleVersion(t *testing.T) {
	set := NewSet("cs", "test")
	a1 := &Constraint{Code: "A", Category: "g", Version: 1, Active: true}
	a2 := &Constraint{Code: "A", Category: "g", Version: 2}
	b := &Constraint{Code: "B", Category: "g", Version: 1, Active: true}
	for _, c := range []*Constraint{a1, a2, b} {
		if err := set.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	if err := set.Activate("A", 2); err != nil {
		t.Fatalf("activate error: %v", err)
	}
	activeA, ok := set.Active("A")
	if !ok || activeA.Version != 2 {
		t.Fatal("A must have version 2 active")
	}
	if a1.Active {
		t.Fatal("A version 1 must be inactive")
	}
	if _, ok := set.Active("B"); !ok {
		t.Fatal("B must stay active: activation of A must not touch other codes")
	}
	if err := set.Activate("A", 999); err == nil {
		t.Fatal("activate with unknown version must fail")
	}
	if err := set.Activate("X", 1); err == nil {
		t.Fatal("activate with unknown code must fail")
	}
}

func TestRangeContainsWithTolerance(t *testing.T) {
	r := Range{Min: 150, Max: 200, HasMin: true, HasMax: true, Tolerance: 0.1}
	if !r.Contains(175) {
		t.Fatal("175 must be in range")
	}
	if r.Contains(120) {
		t.Fatal("120 must be out of range")
	}
	if !r.Contains(150) {
		t.Fatal("boundary min must be in range")
	}
	// допуск: 200.05 входит (200 + 0.1).
	if !r.Contains(200.05) {
		t.Fatal("200.05 within tolerance must be in range")
	}
}

func TestResolve(t *testing.T) {
	set := StandardProfile("standard")
	c, ok := set.Resolve(GEO_STEP_HEIGHT, 180)
	if !ok {
		t.Fatalf("expected compliance, got violation %+v", c)
	}
	if c.Severity != SeverityError {
		t.Fatalf("step height must be error severity, got %s", c.Severity)
	}
	if _, ok := set.Resolve(GEO_STEP_HEIGHT, 250); ok {
		t.Fatal("250mm step height must violate the rule")
	}
}

func TestOverlappingRangesDetected(t *testing.T) {
	set := NewSet("cs", "test")
	// две версии одного правила с пересекающимися диапазонами.
	if err := set.Add(&Constraint{Code: "X", Version: 1, Category: "g", Range: Range{Min: 0, Max: 100, HasMin: true, HasMax: true}}); err != nil {
		t.Fatal(err)
	}
	if err := set.Add(&Constraint{Code: "X", Version: 2, Category: "g", Range: Range{Min: 50, Max: 200, HasMin: true, HasMax: true}}); err != nil {
		t.Fatal(err)
	}
	violations := ValidateIntegrity(set)
	found := false
	for _, v := range violations {
		if v != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("overlapping ranges between rule versions must be reported")
	}
}

func TestNonOverlappingVersionsClean(t *testing.T) {
	set := NewSet("cs", "test")
	// две версии с непересекающимися диапазонами.
	if err := set.Add(&Constraint{Code: "X", Version: 1, Category: "g", Active: true, Range: Range{Min: 0, Max: 100, HasMin: true, HasMax: true}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := set.Add(&Constraint{Code: "X", Version: 2, Category: "g", Range: Range{Min: 200, Max: 300, HasMin: true, HasMax: true}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if violations := ValidateIntegrity(set); len(violations) > 0 {
		t.Fatalf("non-overlapping versions must be clean, got: %v", violations)
	}
}

func TestMultipleActiveVersionsDetected(t *testing.T) {
	set := NewSet("cs", "test")
	// две активные версии одного правила — нарушение инварианта.
	if err := set.Add(&Constraint{Code: "X", Version: 1, Category: "g", Active: true, Range: Range{Min: 0, Max: 100, HasMin: true, HasMax: true}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := set.Add(&Constraint{Code: "X", Version: 2, Category: "g", Active: true, Range: Range{Min: 200, Max: 300, HasMin: true, HasMax: true}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	violations := ValidateIntegrity(set)
	found := false
	for _, v := range violations {
		if v != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("multiple active versions must be reported")
	}
}
