package geometry

import (
	"fmt"
	"math"
)

// Transform — матрица преобразования 4x4 (ENG-GEO-0101).
// Конвенция: точки — однородные (x, y, z, 1), применяются как column-vector:
// t * p. Матрица хранится в row-major порядке: m[Row][Col].
type Transform [4][4]float64

// Identity возвращает единичное преобразование.
func Identity() Transform {
	return Transform{
		{1, 0, 0, 0},
		{0, 1, 0, 0},
		{0, 0, 1, 0},
		{0, 0, 0, 1},
	}
}

// Mul возвращает произведение t * o (сначала применяется o, затем t).
func (t Transform) Mul(o Transform) Transform {
	var r Transform
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			var sum float64
			for k := 0; k < 4; k++ {
				sum += t[i][k] * o[k][j]
			}
			r[i][j] = sum
		}
	}
	return r
}

// Apply применяет преобразование к точке (однородные координаты).
func (t Transform) Apply(p Point3) Point3 {
	x := t[0][0]*p.X + t[0][1]*p.Y + t[0][2]*p.Z + t[0][3]
	y := t[1][0]*p.X + t[1][1]*p.Y + t[1][2]*p.Z + t[1][3]
	z := t[2][0]*p.X + t[2][1]*p.Y + t[2][2]*p.Z + t[2][3]
	return NewPoint3(x, y, z)
}

// Inverse возвращает обратное преобразование; ошибка при вырожденной матрице.
func (t Transform) Inverse() (Transform, error) {
	// расширенная матрица [t | I], метод Гаусса-Жордана.
	var m [4][8]float64
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			m[i][j] = t[i][j]
		}
		m[i][4+i] = 1
	}
	for col := 0; col < 4; col++ {
		// поиск ведущего элемента.
		pivot := col
		for row := col + 1; row < 4; row++ {
			if math.Abs(m[row][col]) > math.Abs(m[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(m[pivot][col]) < Precision {
			return Transform{}, fmt.Errorf("geometry: singular transform")
		}
		if pivot != col {
			m[col], m[pivot] = m[pivot], m[col]
		}
		scale := m[col][col]
		for j := 0; j < 8; j++ {
			m[col][j] /= scale
		}
		for row := 0; row < 4; row++ {
			if row == col {
				continue
			}
			factor := m[row][col]
			for j := 0; j < 8; j++ {
				m[row][j] -= factor * m[col][j]
			}
		}
	}
	var inv Transform
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			inv[i][j] = m[i][4+j]
		}
	}
	return inv, nil
}

// Translate возвращает преобразование переноса.
func Translate(x, y, z float64) Transform {
	t := Identity()
	t[0][3] = x
	t[1][3] = y
	t[2][3] = z
	return t
}

// RotateX возвращает вращение вокруг оси X на угол (радианы).
func RotateX(angle float64) Transform {
	t := Identity()
	c, s := math.Cos(angle), math.Sin(angle)
	t[1][1], t[1][2] = c, -s
	t[2][1], t[2][2] = s, c
	return t
}

// RotateY возвращает вращение вокруг оси Y на угол (радианы).
func RotateY(angle float64) Transform {
	t := Identity()
	c, s := math.Cos(angle), math.Sin(angle)
	t[0][0], t[0][2] = c, s
	t[2][0], t[2][2] = -s, c
	return t
}

// RotateZ возвращает вращение вокруг оси Z на угол (радианы).
func RotateZ(angle float64) Transform {
	t := Identity()
	c, s := math.Cos(angle), math.Sin(angle)
	t[0][0], t[0][1] = c, -s
	t[1][0], t[1][1] = s, c
	return t
}

// Rotate возвращает вращение вокруг оси, проходящей через начало координат.
// Ошибка при нулевой оси.
func Rotate(axis Vector3, angle float64) (Transform, error) {
	u, ok := axis.Normalized()
	if !ok {
		return Transform{}, fmt.Errorf("geometry: zero rotation axis")
	}
	// формула Родригеса.
	c, s := math.Cos(angle), math.Sin(angle)
	ux, uy, uz := u.X, u.Y, u.Z
	t := Identity()
	t[0][0] = c + ux*ux*(1-c)
	t[0][1] = ux*uy*(1-c) - uz*s
	t[0][2] = ux*uz*(1-c) + uy*s
	t[1][0] = uy*ux*(1-c) + uz*s
	t[1][1] = c + uy*uy*(1-c)
	t[1][2] = uy*uz*(1-c) - ux*s
	t[2][0] = uz*ux*(1-c) - uy*s
	t[2][1] = uz*uy*(1-c) + ux*s
	t[2][2] = c + uz*uz*(1-c)
	return t, nil
}

// Frame — локальная система координат (ENG-GEO-0101):
// начало координат + ортонормированный базис осей.
// Преобразования между системами выполняются только через матрицы.
type Frame struct {
	Origin Point3
	Basis  [3]Vector3 // [0]=X, [1]=Y, [2]=Z
}

// NewFrame создаёт систему координат с ортонормированным базисом.
func NewFrame(origin Point3, xAxis, yAxis, zAxis Vector3) Frame {
	return Frame{Origin: origin, Basis: [3]Vector3{xAxis, yAxis, zAxis}}
}

// WorldToLocal переводит точку из глобальной системы в локальную.
func (f Frame) WorldToLocal(p Point3) Point3 {
	d := p.Sub(f.Origin)
	return NewPoint3(d.Dot(f.Basis[0]), d.Dot(f.Basis[1]), d.Dot(f.Basis[2]))
}

// LocalToWorld переводит точку из локальной системы в глобальную.
func (f Frame) LocalToWorld(p Point3) Point3 {
	o := f.Basis[0].Scale(p.X)
	o = o.Add(f.Basis[1].Scale(p.Y))
	o = o.Add(f.Basis[2].Scale(p.Z))
	return f.Origin.Add(o)
}

// Matrix возвращает матрицу преобразования локальных координат в глобальные.
func (f Frame) Matrix() Transform {
	t := Identity()
	t[0][0], t[1][0], t[2][0] = f.Basis[0].X, f.Basis[0].Y, f.Basis[0].Z
	t[0][1], t[1][1], t[2][1] = f.Basis[1].X, f.Basis[1].Y, f.Basis[1].Z
	t[0][2], t[1][2], t[2][2] = f.Basis[2].X, f.Basis[2].Y, f.Basis[2].Z
	t[0][3] = f.Origin.X
	t[1][3] = f.Origin.Y
	t[2][3] = f.Origin.Z
	return t
}
