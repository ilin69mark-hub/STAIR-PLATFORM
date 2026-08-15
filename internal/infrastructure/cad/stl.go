package cad

import (
	"fmt"
	"io"
	"math"

	kerngeo "stairplatform/internal/geometry"
)

// writeSTL сериализует сетку в ASCII STL (EDR-0022 §3.3). Нормаль — единичный
// вектор правой тройки (b−a) × (c−a); для вырожденных граней (нулевая длина)
// пишется (0,0,0). Числовой формат %.6f.
func writeSTL(w io.Writer, m *kerngeo.Mesh) error {
	if _, err := io.WriteString(w, "solid STAIR\n"); err != nil {
		return err
	}
	for _, t := range m.Triangles {
		a, b, c := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
		nx, ny, nz := faceNormal(a, b, c)
		facet := fmt.Sprintf(
			"facet normal %.6f %.6f %.6f\nouter loop\n"+
				"vertex %.6f %.6f %.6f\n"+
				"vertex %.6f %.6f %.6f\n"+
				"vertex %.6f %.6f %.6f\n"+
				"endloop\nendfacet\n",
			nx, ny, nz,
			a.X, a.Y, a.Z,
			b.X, b.Y, b.Z,
			c.X, c.Y, c.Z,
		)
		if _, err := io.WriteString(w, facet); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "endsolid STAIR\n")
	return err
}

// faceNormal возвращает единичную нормаль (b−a) × (c−a); вырожденная грань → (0,0,0).
func faceNormal(a, b, c kerngeo.Point3) (float64, float64, float64) {
	abx, aby, abz := b.X-a.X, b.Y-a.Y, b.Z-a.Z
	acx, acy, acz := c.X-a.X, c.Y-a.Y, c.Z-a.Z
	nx, ny, nz := aby*acz-abz*acy, abz*acx-abx*acz, abx*acy-aby*acx
	len := math.Hypot(nx, math.Hypot(ny, nz))
	if len == 0 {
		return 0, 0, 0
	}
	return nx / len, ny / len, nz / len
}
