package cad

import (
	"fmt"
	"io"

	kerngeo "stairplatform/internal/geometry"
)

// writeDXF сериализует сетку в ASCII DXF R12 (EDR-0022 §3.3). Каждая грань
// — сущность 3DFACE с четырьмя углами (последний повторяет третий, т.к.
// грань треугольная). Координаты в мм без преобразования; числовой формат
// %.6f обеспечивает детерминированность.
func writeDXF(w io.Writer, m *kerngeo.Mesh) error {
	if _, err := io.WriteString(w, "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "0\nSECTION\n2\nENTITIES\n"); err != nil {
		return err
	}
	for _, t := range m.Triangles {
		a, b, c := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
		// Треугольная грань: 4-я точка повторяет 3-ю.
		if _, err := io.WriteString(w, fmt.Sprintf(
			"0\n3DFACE\n8\n0\n10\n%.6f\n20\n%.6f\n30\n%.6f\n"+
				"11\n%.6f\n21\n%.6f\n31\n%.6f\n"+
				"12\n%.6f\n22\n%.6f\n32\n%.6f\n"+
				"13\n%.6f\n23\n%.6f\n33\n%.6f\n",
			a.X, a.Y, a.Z,
			b.X, b.Y, b.Z,
			c.X, c.Y, c.Z,
			c.X, c.Y, c.Z,
		)); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "0\nENDSEC\n0\nEOF\n")
	return err
}
