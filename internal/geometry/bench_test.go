package geometry

import (
	"math"
	"testing"
)

// BenchmarkTriangulate измеряет базовый примитив триангуляции (ear
// clipping, O(n²)) — главный CPU-хотспот конвейера (EM-06 Performance).
// Baseline для B2.2 (устранение 4× избыточной триангуляции граней).
func BenchmarkTriangulate(b *testing.B) {
	cases := []struct {
		name string
		pts  []Point3
	}{
		{"face5", polygonOnZ(5)},
		{"face18", polygonOnZ(18)},
		{"face41", polygonOnZ(41)},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Triangulate(c.pts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// polygonOnZ возвращает правильный многоугольник n-вершин в плоскости Z
// (грань, перпендикулярная оси Z) — типичный случай граней ступени.
func polygonOnZ(n int) []Point3 {
	pts := make([]Point3, 0, n)
	for i := 0; i < n; i++ {
		ang := 2 * math.Pi * float64(i) / float64(n)
		pts = append(pts, NewPoint3(100*math.Cos(ang), 100*math.Sin(ang), 0))
	}
	return pts
}
