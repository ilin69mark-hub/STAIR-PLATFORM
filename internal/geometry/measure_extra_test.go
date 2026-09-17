package geometry

import "testing"

func openContourSolid() *Solid {
	v0 := NewVertex(NewPoint3(0, 0, 0))
	v1 := NewVertex(NewPoint3(10, 0, 0))
	v2 := NewVertex(NewPoint3(10, 10, 0))
	v3 := NewVertex(NewPoint3(20, 10, 0))
	return NewSolid(NewShell(NewFace(NewWire(
		NewEdge(v0, v1), NewEdge(v1, v2), NewEdge(v2, v3),
	))))
}

func fewEdgesSolid() *Solid {
	v0 := NewVertex(NewPoint3(0, 0, 0))
	v1 := NewVertex(NewPoint3(10, 0, 0))
	return NewSolid(NewShell(NewFace(NewWire(NewEdge(v0, v1)))))
}

func collinearSolid() *Solid {
	v0 := NewVertex(NewPoint3(0, 0, 0))
	v1 := NewVertex(NewPoint3(10, 0, 0))
	v2 := NewVertex(NewPoint3(20, 0, 0))
	return NewSolid(NewShell(NewFace(NewWire(
		NewEdge(v0, v1), NewEdge(v1, v2), NewEdge(v2, v0),
	))))
}

func TestSolidBoundingBox(t *testing.T) {
	if bb := SolidBoundingBox(nil); !bb.IsEmpty() {
		t.Fatalf("nil solid must give empty bbox, got %+v", bb)
	}
	bb := SolidBoundingBox(boxSolid())
	if bb.IsEmpty() || !bb.Contains(NewPoint3(5, 5, 2.5)) {
		t.Fatalf("box bbox = %+v", bb)
	}
}

func TestVolumeCached(t *testing.T) {
	cache := NewTessellationCache()
	v1, err := VolumeCached(boxSolid(), cache)
	if err != nil || !nearlyEqual(v1, 500) {
		t.Fatalf("cached volume = %v, err %v", v1, err)
	}
	v2, err := VolumeCached(boxSolid(), cache)
	if err != nil || v2 != v1 {
		t.Fatalf("reused cache volume = %v, err %v", v2, err)
	}
}

func TestSurfaceAreaCached(t *testing.T) {
	cache := NewTessellationCache()
	a1, err := SurfaceAreaCached(boxSolid(), cache)
	if err != nil || !nearlyEqual(a1, 400) {
		t.Fatalf("cached area = %v, err %v", a1, err)
	}
	a2, err := SurfaceAreaCached(boxSolid(), cache)
	if err != nil || a2 != a1 {
		t.Fatalf("reused cache area = %v, err %v", a2, err)
	}
}

func TestSurfaceAreaErrors(t *testing.T) {
	if _, err := SurfaceArea(NewSolid()); err == nil {
		t.Fatal("solid without shells must error")
	}
}

func TestVolumeOpenContour(t *testing.T) {
	if _, err := Volume(openContourSolid()); err == nil {
		t.Fatal("open face contour must error")
	}
}

func TestSurfaceAreaOpenContour(t *testing.T) {
	if _, err := SurfaceArea(openContourSolid()); err == nil {
		t.Fatal("open face contour must error")
	}
}

func TestVolumeWireErr(t *testing.T) {
	if _, err := Volume(fewEdgesSolid()); err == nil {
		t.Fatal("face with fewer than 3 edges must error")
	}
}

func TestSurfaceAreaWireErr(t *testing.T) {
	if _, err := SurfaceArea(fewEdgesSolid()); err == nil {
		t.Fatal("face with fewer than 3 edges must error")
	}
}

func TestVolumeTrisErr(t *testing.T) {
	if _, err := Volume(collinearSolid()); err == nil {
		t.Fatal("collinear contour must error")
	}
}

func TestSurfaceAreaTrisErr(t *testing.T) {
	if _, err := SurfaceArea(collinearSolid()); err == nil {
		t.Fatal("collinear contour must error")
	}
}
