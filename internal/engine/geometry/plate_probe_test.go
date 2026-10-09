package geometry

import (
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

func TestPlateWithHolesErr(t *testing.T) {
	p0 := kerngeo.NewPoint3(0, 0, 0)
	p1 := kerngeo.NewPoint3(70, 0, 10)
	s := plateWithHoles(p0, p1, 50.8, 3, 15, 5, "probe")
	if s == nil {
		// повторить вручную, чтобы увидеть ошибку
		axis := p1.Sub(p0)
		L := axis.Norm()
		u, _ := axis.Normalized()
		wd, _ := perpTo(u)
		td, ok := u.Cross(wd).Normalized()
		t.Logf("L=%v u=%v wd=%v td_ok=%v", L, u, wd, ok)
		prof := []kerngeo.Point3{
			p0.Add(wd.Scale(50.8 / 2)).Add(td.Scale(1.5)),
			p1.Add(wd.Scale(50.8 / 2)).Add(td.Scale(1.5)),
			p1.Add(wd.Scale(-50.8 / 2)).Add(td.Scale(1.5)),
			p0.Add(wd.Scale(-50.8 / 2)).Add(td.Scale(1.5)),
		}
		_, err := kerngeo.ExtrudeWithHoles(prof, nil, td, 3)
		t.Fatalf("plate nil; plain extrude err = %v", err)
	}
	v, verr := kerngeo.Volume(s)
	t.Logf("plate ok, vol %v err %v", v, verr)
}
