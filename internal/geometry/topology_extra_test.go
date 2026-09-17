package geometry

import "testing"

func TestTopologyCoverage(t *testing.T) {
	p0 := NewPoint3(0, 0, 0)
	p1 := NewPoint3(1, 0, 0)
	p2 := NewPoint3(0, 1, 0)
	v0 := NewVertex(p0)
	v1 := NewVertex(p1)
	v2 := NewVertex(p2)
	_ = v0.Point()
	e0 := NewEdge(v0, v1)
	e1 := NewEdge(v1, v2)
	e2 := NewEdge(v2, v0)
	if _, _ = e0.Endpoints(); e0.IsDegenerate() {
		t.Fatal("degenerate")
	}
	deg := NewEdge(v0, v0)
	if !deg.IsDegenerate() {
		t.Fatal("want degenerate")
	}
	w := NewWire(e0, e1, e2)
	_ = w.Edges()
	_ = w.IsClosed()
	// not closed case
	w2 := NewWire(e0, e1)
	if w2.IsClosed() {
		t.Fatal("want not closed")
	}
	f := NewFace(w)
	_ = f.Outer()
	_ = f.Inner()
	sh := NewShell(f)
	_ = sh.Faces()
	s := NewSolid(sh)
	s2 := NewSolidRole("beam", sh)
	_ = s2.Role()
	_ = s.Shells()
	s3 := s.WithRole("col")
	if s3.Role() != "col" {
		t.Fatal("role")
	}
	c := NewCompound(s, s2)
	_ = c.Solids()
	// Transform
	tr := Identity()
	got := TransformSolid(s, tr)
	if got == nil || len(got.Shells()) != 1 {
		t.Fatal("transform")
	}
	if TransformSolid(nil, tr) != nil {
		t.Fatal("nil")
	}
	// Kind
	if Kind(v0) != TopoVertex || Kind(e0) != TopoEdge || Kind(w) != TopoWire || Kind(f) != TopoFace || Kind(sh) != TopoShell || Kind(s) != TopoSolid || Kind(c) != TopoCompound || Kind(nil) != "" {
		t.Fatal("kind")
	}
}
