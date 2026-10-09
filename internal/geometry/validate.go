package geometry

import (
	"fmt"
	"slices"
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
	return NewTessellationCache().validate(solid)
}

// ValidateCached — вариант Validate, использующий разделяемый кеш
// триангуляций граней (EM-06: проверка положительного объёма не должна
// триангулировать грани повторно, если они уже триангулированы для объёма,
// площади и mesh).
func ValidateCached(solid *Solid, cache *TessellationCache) []ValidationIssue {
	return cache.validate(solid)
}

func (c *TessellationCache) validate(solid *Solid) []ValidationIssue {
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

	// количество вхождений каждого геометрического ребра в грани. Ключи
	// копятся в срезе и сортируются, а не кладутся в map: хеширование
	// [2]Point3 на металлокаркасе стоило дороже всей сортировки.
	keys := make([][2]Point3, 0, 16*len(shells))
	for faceIdx, shell := range shells {
		for _, face := range shell.Faces() {
			w := face.Outer()
			if w == nil || len(w.Edges()) < 3 {
				issues = append(issues, ValidationIssue{
					Code: "GEO-FACE-FEW-EDGES", Severity: SeverityError,
					Element: faceElement(faceIdx), Message: "face has fewer than 3 edges",
				})
				continue
			}
			if !wireIsClosed(w) {
				issues = append(issues, ValidationIssue{
					Code: "GEO-FACE-OPEN-WIRE", Severity: SeverityError,
					Element: faceElement(faceIdx), Message: "face contour is not closed",
				})
			}
			keys = appendWireEdges(keys, w)
			// Отверстия грани — тоже рёбра тела: каждое из них делится между
			// крышкой и стенкой отверстия, поэтому их тоже считаем. Без этого
			// грань с отверстием всегда «не manifold».
			for _, inner := range face.Inner() {
				if inner == nil || len(inner.Edges()) < 3 {
					issues = append(issues, ValidationIssue{
						Code: "GEO-FACE-FEW-EDGES", Severity: SeverityError,
						Element: faceElement(faceIdx), Message: "hole contour has fewer than 3 edges",
					})
					continue
				}
				if !wireIsClosed(inner) {
					issues = append(issues, ValidationIssue{
						Code: "GEO-FACE-OPEN-WIRE", Severity: SeverityError,
						Element: faceElement(faceIdx), Message: "hole contour is not closed",
					})
				}
				keys = appendWireEdges(keys, inner)
			}
		}
	}
	sortEdgeKeys(keys)

	// manifold: каждое ребро замкнутого тела ровно в двух гранях.
	for i := 0; i < len(keys); {
		j := i + 1
		for j < len(keys) && keys[j] == keys[i] {
			j++
		}
		if count := j - i; count != 2 {
			key := keys[i]
			issues = append(issues, ValidationIssue{
				Code: "GEO-SOLID-NON-MANIFOLD", Severity: SeverityError,
				Element: fmt.Sprintf("edge:(%.0f,%.0f,%.0f;%.0f,%.0f,%.0f)",
					key[0].X, key[0].Y, key[0].Z, key[1].X, key[1].Y, key[1].Z),
				Message: fmt.Sprintf("edge has %d incident faces, want 2", count),
			})
		}
		i = j
	}

	if vol, err := c.volume(solid); err != nil || vol <= Precision {
		issues = append(issues, ValidationIssue{
			Code: "GEO-SOLID-NON-POSITIVE-VOLUME", Severity: SeverityError,
			Element: "solid", Message: "solid volume must be positive",
		})
	}

	slices.SortFunc(issues, func(a, b ValidationIssue) int {
		if a.Code < b.Code {
			return -1
		}
		if a.Code > b.Code {
			return 1
		}
		return 0
	})
	return issues
}

// faceElement — имя грани для отчёта валидации. Строка собирается только
// когда проблема действительно найдена: раньше она форматировалась на КАЖДУЮ
// грань каждого тела, и на металлокаркасе это были миллионы Sprintf.
func faceElement(faceIdx int) string { return fmt.Sprintf("face:%d", faceIdx) }

// wireIsClosed проверяет замыкание провода БЕЗ построения среза точек.
func wireIsClosed(w *Wire) bool {
	edges := w.Edges()
	if len(edges) < 3 {
		return false
	}
	first, _ := edges[0].Endpoints()
	_, last := edges[len(edges)-1].Endpoints()
	return first.Point() == last.Point()
}

// appendWireEdges дописывает в срез геометрические ключи рёбер провода.
func appendWireEdges(keys [][2]Point3, w *Wire) [][2]Point3 {
	for _, e := range w.Edges() {
		v1, v2 := e.Endpoints()
		keys = append(keys, edgeKey(v1.Point(), v2.Point()))
	}
	return keys
}

// sortEdgeKeys сортирует ключи рёбер по значениям точек. slices.SortFunc
// вместо sort.Sort: сравнение конкретного типа без интерфейсных вызовов.
func sortEdgeKeys(keys [][2]Point3) {
	slices.SortFunc(keys, func(a, b [2]Point3) int {
		if c := comparePoint(a[0], b[0]); c != 0 {
			return c
		}
		return comparePoint(a[1], b[1])
	})
}

// comparePoint — детерминированное сравнение точек: -1, 0 или 1.
func comparePoint(a, b Point3) int {
	switch {
	case a.X != b.X:
		return cmpFloat(a.X, b.X)
	case a.Y != b.Y:
		return cmpFloat(a.Y, b.Y)
	case a.Z != b.Z:
		return cmpFloat(a.Z, b.Z)
	default:
		return 0
	}
}

func cmpFloat(a, b float64) int {
	if a < b {
		return -1
	}
	return 1
}

// contourPoints возвращает упорядоченные точки контура грани (без
// замыкающего дубликата) и признак замкнутости провода.
func contourPoints(w *Wire) ([]Point3, bool) {
	edges := w.Edges()
	if len(edges) < 3 {
		return nil, false
	}
	first, _ := edges[0].Endpoints()
	// Ёмкость сразу по числу рёбер: при append от одного элемента срез
	// перевыделялся log₂(N) раз на КАЖДЫЙ контур, и на металлокаркасе с
	// сотнями деталей это давало миллионы выделений.
	pts := make([]Point3, 0, len(edges)+1)
	pts = append(pts, first.Point())
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
