// Package geometry implements the domain-agnostic Geometry Platform
// (STAIR-KERNEL, ENG-GEO-0001). Coordinates are float64 in millimetres
// and radians per ADR-0008; all computations are deterministic, use no
// internal rounding and target 1e-6 precision.
// The package is independent of any application domain (invariant 6):
// a staircase is just one use case.
package geometry

import "math"

// Precision — геометрическая точность платформы (ADR-0008): 0.000001 мм.
const Precision = 1e-6

// Point3 — точка в трёхмерном пространстве (координаты в мм).
type Point3 struct {
	X, Y, Z float64
}

// NewPoint3 создаёт точку.
func NewPoint3(x, y, z float64) Point3 { return Point3{X: x, Y: y, Z: z} }

// Add смещает точку на вектор.
func (p Point3) Add(v Vector3) Point3 {
	return Point3{X: p.X + v.X, Y: p.Y + v.Y, Z: p.Z + v.Z}
}

// Sub возвращает вектор от o к p.
func (p Point3) Sub(o Point3) Vector3 {
	return Vector3{X: p.X - o.X, Y: p.Y - o.Y, Z: p.Z - o.Z}
}

// Distance возвращает евклидово расстояние между точками.
func (p Point3) Distance(o Point3) float64 {
	return math.Sqrt((p.X-o.X)*(p.X-o.X) + (p.Y-o.Y)*(p.Y-o.Y) + (p.Z-o.Z)*(p.Z-o.Z))
}

// Vector3 — вектор в трёхмерном пространстве.
type Vector3 struct {
	X, Y, Z float64
}

// NewVector3 создаёт вектор.
func NewVector3(x, y, z float64) Vector3 { return Vector3{X: x, Y: y, Z: z} }

// ZeroVector возвращает нулевой вектор.
func ZeroVector() Vector3 { return Vector3{} }

// Add складывает векторы.
func (v Vector3) Add(o Vector3) Vector3 {
	return Vector3{X: v.X + o.X, Y: v.Y + o.Y, Z: v.Z + o.Z}
}

// Sub вычитает векторы.
func (v Vector3) Sub(o Vector3) Vector3 {
	return Vector3{X: v.X - o.X, Y: v.Y - o.Y, Z: v.Z - o.Z}
}

// Scale масштабирует вектор.
func (v Vector3) Scale(s float64) Vector3 {
	return Vector3{X: v.X * s, Y: v.Y * s, Z: v.Z * s}
}

// Dot возвращает скалярное произведение.
func (v Vector3) Dot(o Vector3) float64 {
	return v.X*o.X + v.Y*o.Y + v.Z*o.Z
}

// Cross возвращает векторное произведение.
func (v Vector3) Cross(o Vector3) Vector3 {
	return Vector3{
		X: v.Y*o.Z - v.Z*o.Y,
		Y: v.Z*o.X - v.X*o.Z,
		Z: v.X*o.Y - v.Y*o.X,
	}
}

// Norm возвращает длину вектора.
func (v Vector3) Norm() float64 {
	return math.Sqrt(v.Dot(v))
}

// Normalized возвращает единичный вектор; ok=false для нулевого вектора.
func (v Vector3) Normalized() (Vector3, bool) {
	n := v.Norm()
	if n == 0 {
		return Vector3{}, false
	}
	return v.Scale(1 / n), true
}

// Equals сравнивает векторы с инженерной точностью (1e-6).
func (v Vector3) Equals(o Vector3) bool {
	return math.Abs(v.X-o.X) <= Precision &&
		math.Abs(v.Y-o.Y) <= Precision &&
		math.Abs(v.Z-o.Z) <= Precision
}

// BBox — минимальный осеориентированный ограничивающий параллелепипед.
type BBox struct {
	Min, Max Point3
}

// NewBBox строит ограничивающий параллелепипед по набору точек.
// Пустой набор даёт пустой BBox.
func NewBBox(points ...Point3) BBox {
	if len(points) == 0 {
		return BBox{}
	}
	min, max := points[0], points[0]
	for _, p := range points[1:] {
		if p.X < min.X {
			min.X = p.X
		}
		if p.Y < min.Y {
			min.Y = p.Y
		}
		if p.Z < min.Z {
			min.Z = p.Z
		}
		if p.X > max.X {
			max.X = p.X
		}
		if p.Y > max.Y {
			max.Y = p.Y
		}
		if p.Z > max.Z {
			max.Z = p.Z
		}
	}
	return BBox{Min: min, Max: max}
}

// IsEmpty возвращает true для пустого BBox.
func (b BBox) IsEmpty() bool {
	return b.Min == b.Max && b == BBox{}
}

// Contains проверяет попадание точки в параллелепипед.
func (b BBox) Contains(p Point3) bool {
	if b.IsEmpty() {
		return false
	}
	return p.X >= b.Min.X && p.X <= b.Max.X &&
		p.Y >= b.Min.Y && p.Y <= b.Max.Y &&
		p.Z >= b.Min.Z && p.Z <= b.Max.Z
}

// Union возвращает минимальный параллелепипед, содержащий оба.
func (b BBox) Union(o BBox) BBox {
	if b.IsEmpty() {
		return o
	}
	if o.IsEmpty() {
		return b
	}
	return NewBBox(b.Min, b.Max, o.Min, o.Max)
}
