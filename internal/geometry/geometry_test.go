package geometry

import (
	"errors"
	"math"
	"testing"
)

var errBadFormula = errors.New("bad formula")

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6
}

func TestVectorOps(t *testing.T) {
	a := NewVector3(1, 2, 3)
	b := NewVector3(4, 5, 6)
	if !a.Add(b).Equals(NewVector3(5, 7, 9)) {
		t.Fatal("add")
	}
	if !b.Sub(a).Equals(NewVector3(3, 3, 3)) {
		t.Fatal("sub")
	}
	if !a.Scale(2).Equals(NewVector3(2, 4, 6)) {
		t.Fatal("scale")
	}
	if a.Dot(b) != 32 {
		t.Fatalf("dot = %v, want 32", a.Dot(b))
	}
	if !a.Cross(b).Equals(NewVector3(-3, 6, -3)) {
		t.Fatalf("cross = %+v", a.Cross(b))
	}
	if !nearlyEqual(NewVector3(3, 4, 0).Norm(), 5) {
		t.Fatal("norm")
	}
	u, ok := NewVector3(0, 0, 0).Normalized()
	if ok {
		t.Fatal("zero vector must not normalize")
	}
	if _, ok := NewVector3(1, 0, 0).Normalized(); !ok {
		t.Fatal("unit vector must normalize")
	}
	_ = u
}

func TestPointDistance(t *testing.T) {
	d := NewPoint3(0, 0, 0).Distance(NewPoint3(3, 4, 0))
	if !nearlyEqual(d, 5) {
		t.Fatalf("distance = %v, want 5", d)
	}
}

func TestBBox(t *testing.T) {
	bb := NewBBox(NewPoint3(0, 0, 0), NewPoint3(10, 20, 30))
	if bb.IsEmpty() {
		t.Fatal("bbox must not be empty")
	}
	if !bb.Contains(NewPoint3(5, 10, 15)) {
		t.Fatal("point inside must be contained")
	}
	if bb.Contains(NewPoint3(11, 0, 0)) {
		t.Fatal("point outside must not be contained")
	}
	if !NewBBox().IsEmpty() {
		t.Fatal("empty bbox must be empty")
	}
	u := NewBBox(NewPoint3(0, 0, 0), NewPoint3(1, 1, 1)).Union(NewBBox(NewPoint3(5, 5, 5)))
	if !u.Contains(NewPoint3(4, 4, 4)) {
		t.Fatal("union must cover both boxes")
	}
}

func TestTransformCompose(t *testing.T) {
	// перенос(10,0,0) ∘ вращениеZ(90°) точки (1,0,0) → (10,1,0).
	tz, _ := Rotate(NewVector3(0, 0, 1), math.Pi/2)
	tr := Translate(10, 0, 0)
	composed := tr.Mul(tz)
	p := composed.Apply(NewPoint3(1, 0, 0))
	if !nearlyEqual(p.X, 10) || !nearlyEqual(p.Y, 1) {
		t.Fatalf("composed transform = %+v", p)
	}
}

func TestTransformInverse(t *testing.T) {
	tm := Translate(3, 4, 5)
	inv, err := tm.Inverse()
	if err != nil {
		t.Fatal(err)
	}
	p := NewPoint3(1, 2, 3)
	if _, err := inv.Inverse(); err != nil {
		t.Fatal("inverse of inverse must exist")
	}
	back := inv.Apply(tm.Apply(p))
	if !nearlyEqual(back.X, p.X) || !nearlyEqual(back.Y, p.Y) || !nearlyEqual(back.Z, p.Z) {
		t.Fatalf("inverse*apply = %+v, want %+v", back, p)
	}
	var zero Transform
	if _, err := zero.Inverse(); err == nil {
		t.Fatal("singular transform must error")
	}
}

func TestRotateAxis(t *testing.T) {
	// вращение вокруг X на 90°: (0,1,0) → (0,0,1).
	rx, err := Rotate(NewVector3(1, 0, 0), math.Pi/2)
	if err != nil {
		t.Fatal(err)
	}
	p := rx.Apply(NewPoint3(0, 1, 0))
	if !nearlyEqual(p.Z, 1) || !nearlyEqual(p.Y, 0) {
		t.Fatalf("rotateX = %+v", p)
	}
	if _, err := Rotate(ZeroVector(), 1); err == nil {
		t.Fatal("zero axis must error")
	}
}

func TestFrameRoundTrip(t *testing.T) {
	// система с переносом (100,200,300) и поворотом 90° вокруг Z.
	rx, _ := Rotate(NewVector3(0, 0, 1), math.Pi/2)
	f := NewFrame(
		NewPoint3(100, 200, 300),
		NewVector3(rx[0][0], rx[1][0], rx[2][0]),
		NewVector3(rx[0][1], rx[1][1], rx[2][1]),
		NewVector3(0, 0, 1),
	)
	local := f.WorldToLocal(NewPoint3(101, 200, 300))
	// глобальный вектор (1,0,0) в системе, повёрнутой на +90° вокруг Z,
	// имеет локальные координаты (0,-1,0).
	if !nearlyEqual(local.X, 0) || !nearlyEqual(local.Y, -1) {
		t.Fatalf("world->local = %+v", local)
	}
	back := f.LocalToWorld(local)
	if !nearlyEqual(back.X, 101) || !nearlyEqual(back.Y, 200) || !nearlyEqual(back.Z, 300) {
		t.Fatalf("round trip = %+v", back)
	}
	// матрица должна дать тот же результат, что и LocalToWorld, для той же точки.
	loc := NewPoint3(0, -1, 0)
	got := f.Matrix().Apply(loc)
	want := f.LocalToWorld(loc)
	if !nearlyEqual(got.X, want.X) || !nearlyEqual(got.Y, want.Y) || !nearlyEqual(got.Z, want.Z) {
		t.Fatalf("matrix vs frame mismatch: %+v vs %+v", got, want)
	}
}

func TestTopologyStructure(t *testing.T) {
	v0 := NewVertex(NewPoint3(0, 0, 0))
	v1 := NewVertex(NewPoint3(10, 0, 0))
	v2 := NewVertex(NewPoint3(10, 10, 0))
	e0 := NewEdge(v0, v1)
	e1 := NewEdge(v1, v2)
	e2 := NewEdge(v2, v0)
	w := NewWire(e0, e1, e2)
	if !w.IsClosed() {
		t.Fatal("triangle wire must be closed")
	}
	if NewWire(e0, e1).IsClosed() {
		t.Fatal("open wire must not be closed")
	}
	if !NewEdge(v0, v0).IsDegenerate() {
		t.Fatal("edge with same vertex must be degenerate")
	}
	f := NewFace(w)
	s := NewShell(f)
	solid := NewSolid(s)
	comp := NewCompound(solid)
	if len(comp.Solids()) != 1 || len(comp.Solids()[0].Shells()) != 1 {
		t.Fatal("containment structure")
	}
	if Kind(v0) != TopoVertex || Kind(e0) != TopoEdge || Kind(w) != TopoWire ||
		Kind(f) != TopoFace || Kind(s) != TopoShell || Kind(solid) != TopoSolid ||
		Kind(comp) != TopoCompound {
		t.Fatal("topo kinds")
	}
}

func TestParametricModelPartialRebuild(t *testing.T) {
	m := NewParametricModel()
	mustAdd := func(p *Parameter) {
		t.Helper()
		if err := m.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	mustAdd(NewParameter("A", "owner", TypeLength, 10.0))
	mustAdd(NewDerivedParameter("B", "owner", TypeLength, func(m *ParametricModel) (any, error) {
		a, _ := m.Parameter("A")
		return a.Value.(float64) * 2, nil
	}))
	mustAdd(NewDerivedParameter("C", "owner", TypeLength, func(m *ParametricModel) (any, error) {
		b, _ := m.Parameter("B")
		return b.Value.(float64) + 1, nil
	}))
	mustAdd(NewParameter("D", "owner", TypeLength, 100.0)) // независимый

	for _, dep := range [][2]string{{"A", "B"}, {"B", "C"}} {
		if err := m.AddDependency(dep[0], dep[1]); err != nil {
			t.Fatal(err)
		}
	}

	if err := m.Set("A", 10.0); err != nil {
		t.Fatal(err)
	}
	if err := m.Rebuild("A"); err != nil {
		t.Fatal(err)
	}
	c, _ := m.Parameter("C")
	if c.Value.(float64) != 21 {
		t.Fatalf("C = %v, want 21", c.Value)
	}
	d, _ := m.Parameter("D")
	dBefore := d.Value

	// изменяем только A: B и C должны пересчитаться, D — нет.
	if err := m.Set("A", 50.0); err != nil {
		t.Fatal(err)
	}
	if err := m.Rebuild("A"); err != nil {
		t.Fatal(err)
	}
	b, _ := m.Parameter("B")
	c, _ = m.Parameter("C")
	if b.Value.(float64) != 100 {
		t.Fatalf("B = %v, want 100", b.Value)
	}
	if c.Value.(float64) != 101 {
		t.Fatalf("C = %v, want 101", c.Value)
	}
	if b.State != StateCalculated || c.State != StateCalculated {
		t.Fatal("derived params must be calculated")
	}
	d, _ = m.Parameter("D")
	if d.Value != dBefore {
		t.Fatal("independent parameter D must not be recalculated")
	}
}

func TestParametricModelCycleRejected(t *testing.T) {
	m := NewParametricModel()
	if err := m.Add(NewParameter("A", "owner", TypeFloat, 1.0)); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(NewParameter("B", "owner", TypeFloat, 2.0)); err != nil {
		t.Fatal(err)
	}
	if err := m.AddDependency("A", "B"); err != nil {
		t.Fatal(err)
	}
	if err := m.AddDependency("B", "A"); err == nil {
		t.Fatal("cycle must be rejected")
	}
}

func TestParametricModelInvariants(t *testing.T) {
	m := NewParametricModel()
	if err := m.Add(NewParameter("", "owner", TypeFloat, 1.0)); err == nil {
		t.Fatal("empty id must be rejected")
	}
	if err := m.Add(NewParameter("X", "", TypeFloat, 1.0)); err == nil {
		t.Fatal("missing owner must be rejected")
	}
	if err := m.Add(NewParameter("X", "owner", TypeFloat, 1.0)); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(NewParameter("X", "owner", TypeFloat, 2.0)); err == nil {
		t.Fatal("duplicate id must be rejected")
	}
	locked := NewParameter("L", "owner", TypeFloat, 1.0)
	locked.State = StateLocked
	if err := m.Add(locked); err != nil {
		t.Fatal(err)
	}
	if err := m.Set("L", 5.0); err == nil {
		t.Fatal("locked parameter must not be settable")
	}
}

func TestParametricModelFormulaError(t *testing.T) {
	m := NewParametricModel()
	if err := m.Add(NewParameter("A", "owner", TypeFloat, 1.0)); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(NewDerivedParameter("B", "owner", TypeFloat, func(m *ParametricModel) (any, error) {
		return nil, errBadFormula
	})); err != nil {
		t.Fatal(err)
	}
	if err := m.AddDependency("A", "B"); err != nil {
		t.Fatal(err)
	}
	if err := m.Rebuild("A"); err == nil {
		t.Fatal("formula error must propagate")
	}
	b, _ := m.Parameter("B")
	if b.State != StateInvalid {
		t.Fatal("parameter must be marked invalid on formula error")
	}
}
