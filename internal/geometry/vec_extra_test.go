package geometry

import "testing"

func TestVecCoverage(t *testing.T) {
	p0 := NewPoint3(0, 0, 0)
	p1 := NewPoint3(3, 4, 0)
	v := NewVector3(1, 2, 3)
	_ = p0.Add(v)
	_ = p1.Sub(p0)
	_ = p0.Distance(p1)
	_ = ZeroVector()
	_ = v.Add(v)
	_ = v.Sub(v)
	_ = v.Scale(2)
	_ = v.Dot(v)
	_ = v.Cross(v)
	_ = v.Norm()
	if _, ok := v.Normalized(); !ok {
		t.Fatal("norm")
	}
	if _, ok := ZeroVector().Normalized(); ok {
		t.Fatal("zero should fail")
	}
	_ = v.Equals(v)
	b := NewBBox(p0, p1, NewPoint3(1, 1, 1))
	_ = b.IsEmpty()
	_ = b.Contains(p0)
	_ = b.Union(NewBBox())
	_ = NewBBox().IsEmpty()
	_ = b.Contains(NewPoint3(10, 10, 10))
	if !NewBBox().Union(b).Contains(p0) {
		t.Fatal("union")
	}
}
