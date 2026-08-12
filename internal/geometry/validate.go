package geometry

import (
	"fmt"
	"sort"
)

// Severity — серьёзность проблемы валидации геометрии (ENG-GEO-0018).
type Severity string

const (
	// SeverityError — нарушение, делающее модель некорректной.
	SeverityError Severity = "error"
	// SeverityWarning — потенциальная проблема, не ломающая модель.
	SeverityWarning Severity = "warning"
)

// ValidationIssue — запись диагностического отчёта (ENG-GEO-0018):
// код, серьёзность, идентификатор элемента и описание.
type ValidationIssue struct {
	Code     string
	Severity Severity
	Element  string
	Message  string
}

// Validate проверяет корректность твёрдого тела на уровне Standard
// (ENG-GEO-0018): замкнутость контуров граней, грань из ≥3 рёбер,
// manifold-структура рёбер (каждое геометрическое ребро ровно в двух
// гранях) и положительный объём. Результат детерминирован: issues
// отсортированы по Code; тела без ошибок дают пустой список.
func Validate(solid *Solid) []ValidationIssue {
	var issues []ValidationIssue
	if solid == nil {
		return []ValidationIssue{{
			Code: "GEO-SOLID-MISSING", Severity: SeverityError,
			Element: "solid", Message: "solid is required",
		}}
	}
	shells := solid.Shells()
	if len(shells) == 0 {
		issues = append(issues, ValidationIssue{
			Code: "GEO-SOLID-EMPTY", Severity: SeverityError,
			Element: "solid", Message: "solid has no shells",
		})
		return issues
	}

	// количество вхождений каждого геометрического ребра в грани.
	edgeCount := make(map[[2]Point3]int)
	for faceIdx, shell := range shells {
		for _, face := range shell.Faces() {
			el := fmt.Sprintf("face:%d", faceIdx)
			w := face.Outer()
			if w == nil || len(w.Edges()) < 3 {
				issues = append(issues, ValidationIssue{
					Code: "GEO-FACE-FEW-EDGES", Severity: SeverityError,
					Element: el, Message: "face has fewer than 3 edges",
				})
				continue
			}
			_, closed := contourPoints(w)
			if !closed {
				issues = append(issues, ValidationIssue{
					Code: "GEO-FACE-OPEN-WIRE", Severity: SeverityError,
					Element: el, Message: "face contour is not closed",
				})
			}
			for _, e := range w.Edges() {
				v1, v2 := e.Endpoints()
				key := edgeKey(v1.Point(), v2.Point())
				edgeCount[key]++
			}
		}
	}

	// manifold: каждое ребро замкнутого тела ровно в двух гранях.
	for key, count := range edgeCount {
		if count != 2 {
			issues = append(issues, ValidationIssue{
				Code: "GEO-SOLID-NON-MANIFOLD", Severity: SeverityError,
				Element: fmt.Sprintf("edge:(%.0f,%.0f,%.0f;%.0f,%.0f,%.0f)",
					key[0].X, key[0].Y, key[0].Z, key[1].X, key[1].Y, key[1].Z),
				Message: fmt.Sprintf("edge has %d incident faces, want 2", count),
			})
		}
	}

	if vol, err := Volume(solid); err != nil || vol <= Precision {
		issues = append(issues, ValidationIssue{
			Code: "GEO-SOLID-NON-POSITIVE-VOLUME", Severity: SeverityError,
			Element: "solid", Message: "solid volume must be positive",
		})
	}

	sort.Slice(issues, func(i, j int) bool { return issues[i].Code < issues[j].Code })
	return issues
}

// contourPoints возвращает упорядоченные точки контура грани (без
// замыкающего дубликата) и признак замкнутости провода.
func contourPoints(w *Wire) ([]Point3, bool) {
	edges := w.Edges()
	if len(edges) < 3 {
		return nil, false
	}
	first, _ := edges[0].Endpoints()
	pts := []Point3{first.Point()}
	for _, e := range edges {
		_, second := e.Endpoints()
		pts = append(pts, second.Point())
	}
	closed := pts[len(pts)-1] == pts[0]
	return pts[:len(pts)-1], closed
}

// edgeKey строит неупорядоченный ключ ребра: пару точек с одинаковым
// геометрическим представлением независимо от направления ребра.
func edgeKey(a, b Point3) [2]Point3 {
	if lessPoint(a, b) {
		return [2]Point3{a, b}
	}
	return [2]Point3{b, a}
}

// lessPoint задаёт детерминированный порядок точек для edgeKey.
func lessPoint(a, b Point3) bool {
	if a.X != b.X {
		return a.X < b.X
	}
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.Z < b.Z
}
