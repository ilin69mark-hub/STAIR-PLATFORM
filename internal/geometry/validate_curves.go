package geometry

import (
	"fmt"
	"math"
)

// ValidateArc проверяет корректность дуги (ENG-GEO-0020).
func ValidateArc(a *Arc) []ValidationIssue {
	var issues []ValidationIssue
	if a == nil {
		return []ValidationIssue{{Code: "GEO-ARC-MISSING", Severity: SeverityError, Element: "arc", Message: "arc is required"}}
	}
	if a.Radius <= Precision {
		issues = append(issues, ValidationIssue{Code: "GEO-ARC-RADIUS", Severity: SeverityError, Element: "arc", Message: fmt.Sprintf("arc radius must be positive, got %v", a.Radius)})
	}
	if a.Normal.Norm() <= Precision {
		issues = append(issues, ValidationIssue{Code: "GEO-ARC-NORMAL", Severity: SeverityError, Element: "arc", Message: "arc normal must be non-zero"})
	}
	if math.Abs(a.Start-a.End) < Precision {
		issues = append(issues, ValidationIssue{Code: "GEO-ARC-ANGLES", Severity: SeverityError, Element: "arc", Message: "arc start and end angles must differ"})
	}
	return issues
}

// ValidateBezier проверяет корректность кривой Безье (ENG-GEO-0021).
func ValidateBezier(b *Bezier) []ValidationIssue {
	var issues []ValidationIssue
	if b == nil {
		return []ValidationIssue{{Code: "GEO-BEZIER-MISSING", Severity: SeverityError, Element: "bezier", Message: "bezier curve is required"}}
	}
	// Проверяем вырожденность: все точки совпадают
	if b.P0 == b.P1 && b.P1 == b.P2 && b.P2 == b.P3 {
		issues = append(issues, ValidationIssue{Code: "GEO-BEZIER-DEGENERATE", Severity: SeverityWarning, Element: "bezier", Message: "all control points coincide"})
	}
	return issues
}

// ValidateFillet проверяет корректность скругления (ENG-GEO-0022).
func ValidateFillet(f *Fillet) []ValidationIssue {
	var issues []ValidationIssue
	if f == nil {
		return []ValidationIssue{{Code: "GEO-FILLET-MISSING", Severity: SeverityError, Element: "fillet", Message: "fillet is required"}}
	}
	if f.Radius <= Precision {
		issues = append(issues, ValidationIssue{Code: "GEO-FILLET-RADIUS", Severity: SeverityError, Element: "fillet", Message: fmt.Sprintf("fillet radius must be positive, got %v", f.Radius)})
	}
	if f.Normal.Norm() <= Precision {
		issues = append(issues, ValidationIssue{Code: "GEO-FILLET-NORMAL", Severity: SeverityError, Element: "fillet", Message: "fillet normal must be non-zero"})
	}
	// Проверяем, что ребро не вырождено
	if f.Edge[0] == f.Edge[1] {
		issues = append(issues, ValidationIssue{Code: "GEO-FILLET-EDGE", Severity: SeverityError, Element: "fillet", Message: "fillet edge is degenerate"})
	}
	return issues
}

// ValidateChamfer проверяет корректность фаски (ENG-GEO-0023).
func ValidateChamfer(c *Chamfer) []ValidationIssue {
	var issues []ValidationIssue
	if c == nil {
		return []ValidationIssue{{Code: "GEO-CHAMFER-MISSING", Severity: SeverityError, Element: "chamfer", Message: "chamfer is required"}}
	}
	if c.Width <= Precision {
		issues = append(issues, ValidationIssue{Code: "GEO-CHAMFER-WIDTH", Severity: SeverityError, Element: "chamfer", Message: fmt.Sprintf("chamfer width must be positive, got %v", c.Width)})
	}
	if c.Normal.Norm() <= Precision {
		issues = append(issues, ValidationIssue{Code: "GEO-CHAMFER-NORMAL", Severity: SeverityError, Element: "chamfer", Message: "chamfer normal must be non-zero"})
	}
	if c.Edge[0] == c.Edge[1] {
		issues = append(issues, ValidationIssue{Code: "GEO-CHAMFER-EDGE", Severity: SeverityError, Element: "chamfer", Message: "chamfer edge is degenerate"})
	}
	return issues
}

// ValidateNGon проверяет корректность n-угольника (ENG-GEO-0024).
func ValidateNGon(n *NGon) []ValidationIssue {
	var issues []ValidationIssue
	if n == nil {
		return []ValidationIssue{{Code: "GEO-NGON-MISSING", Severity: SeverityError, Element: "ngon", Message: "ngon is required"}}
	}
	if len(n.Vertices) < 3 {
		issues = append(issues, ValidationIssue{Code: "GEO-NGON-VERTICES", Severity: SeverityError, Element: "ngon", Message: fmt.Sprintf("ngon must have at least 3 vertices, got %d", len(n.Vertices))})
		return issues
	}
	if n.Normal.Norm() <= Precision {
		issues = append(issues, ValidationIssue{Code: "GEO-NGON-NORMAL", Severity: SeverityError, Element: "ngon", Message: "ngon normal must be non-zero"})
	}
	// Проверяем планарность (все точки в одной плоскости)
	origin := n.Vertices[0]
	for i := 1; i < len(n.Vertices)-1; i++ {
		p1 := n.Vertices[i].Sub(origin)
		p2 := n.Vertices[i+1].Sub(origin)
		cross := p1.Cross(p2)
		if cross.Norm() > Precision {
			// Нормаль полигона
			polyNormal, _ := cross.Normalized()
			// Проверяем, что нормаль совпадает с заданной
			if math.Abs(polyNormal.Dot(n.Normal)-1) > Precision {
				issues = append(issues, ValidationIssue{Code: "GEO-NGON-NORMAL-MISMATCH", Severity: SeverityWarning, Element: "ngon", Message: "ngon normal does not match polygon normal"})
			}
			break
		}
	}
	// Проверяем вырожденность (все точки на одной линии)
	collinear := true
	for i := 2; i < len(n.Vertices); i++ {
		p1 := n.Vertices[1].Sub(n.Vertices[0])
		p2 := n.Vertices[i].Sub(n.Vertices[0])
		if p1.Cross(p2).Norm() > Precision {
			collinear = false
			break
		}
	}
	if collinear {
		issues = append(issues, ValidationIssue{Code: "GEO-NGON-COLLINEAR", Severity: SeverityError, Element: "ngon", Message: "ngon vertices are collinear"})
	}
	return issues
}
