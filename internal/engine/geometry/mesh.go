package geometry

import (
	"fmt"
	"math"

	kerngeo "stairplatform/internal/geometry"
)

// ToPreviewMesh строит полигональную сетку для отображения модели
// (ENG-GEO-0008): каждая грань B-Rep модели триангулируется. Mesh
// является производной величиной и никогда не является источником истины
// геометрии; источник — параметрическая модель.
func ToPreviewMesh(model *kerngeo.Compound) (*kerngeo.Mesh, error) {
	return ToPreviewMeshCached(model, kerngeo.NewTessellationCache())
}

// ToPreviewMeshCached — вариант ToPreviewMesh, разделяющий кеш триангуляций
// с валидацией и измерениями (EM-06, B2.2): каждая грань триангулируется
// один раз за вызов Generate.
func ToPreviewMeshCached(model *kerngeo.Compound, tess *kerngeo.TessellationCache) (*kerngeo.Mesh, error) {
	if model == nil {
		return nil, fmt.Errorf("geometry: model is required")
	}
	mesh := &kerngeo.Mesh{}
	origin := kerngeo.BoundingBox(model).Min
	for i, solid := range model.Solids() {
		verts, uvs, tris, err := meshSolid(solid, tess, origin)
		if err != nil {
			return nil, err
		}
		base := len(mesh.Vertices)
		mesh.Vertices = append(mesh.Vertices, verts...)
		mesh.UV = append(mesh.UV, uvs...)
		start := len(mesh.Triangles)
		for _, tr := range tris {
			if err := mesh.AddTriangle(base+tr[0], base+tr[1], base+tr[2]); err != nil {
				return nil, err
			}
		}
		// Роль тела сохраняем для материала в 3D (этап 1): ступени, косоуры,
		// площадка и ограждение получают разные PBR-пресеты.
		mesh.PartRanges = append(mesh.PartRanges, kerngeo.PartRange{
			Solid: i,
			Role:  solid.Role(),
			Start: start,
			End:   len(mesh.Triangles),
		})
	}
	return mesh, nil
}

// mmPerMeter — перевод миллиметров в метры. UV считаются в метрах, чтобы
// масштаб текстуры задавался повтором в материале, а не размером детали.
const mmPerMeter = 1000.0

// triplanarUV проецирует точки грани на её доминирующую ось и возвращает
// координаты в метрах.
//
// Проекция по доминирующей оси (а не по нормали, как в полном трипланаре)
// выбрана сознательно: наша модель — полиэдр из ПЛОСКИХ граней, у каждой
// грани своя нормаль, поэтому выбор оси даёт то же развёртывание, что и
// трипланар с блендингом, но без швов и лишней работы на кадр. Швов не
// видно, потому что соседние грани детали почти всегда ортогональны, и
// проекция меняется ровно на ребре.
//
// offset (смещение детали) нужен, чтобы у двух одинаковых ступеней рисунок
// не был зеркальным копированием: соседние ступени сдвинуты вдоль марша.
func triplanarUV(points []kerngeo.Point3, faceNormal kerngeo.Vector3, offset kerngeo.Point3) []kerngeo.Point2 {
	ax := math.Abs(faceNormal.X)
	ay := math.Abs(faceNormal.Y)
	az := math.Abs(faceNormal.Z)
	uv := make([]kerngeo.Point2, len(points))
	for i, p := range points {
		q := p.Sub(offset)
		var u, v float64
		switch {
		case ax >= ay && ax >= az:
			// Грань смотрит по X — разворачиваем в плоскости ZY.
			u, v = q.Z/mmPerMeter, q.Y/mmPerMeter
		case ay >= az:
			// По Y — горизонтальные плиты (проступи, площадка): XZ.
			u, v = q.X/mmPerMeter, q.Z/mmPerMeter
		default:
			// По Z — вертикальные полотна (косоуры, подступенки): XY.
			u, v = q.X/mmPerMeter, q.Y/mmPerMeter
		}
		uv[i] = kerngeo.Point2{U: u, V: v}
	}
	return uv
}

// faceNormalOf — нормаль грани по её вершинам (плоская грань, поэтому
// достаточно векторного произведения двух рёбер).
func faceNormalOf(points []kerngeo.Point3) kerngeo.Vector3 {
	if len(points) < 3 {
		return kerngeo.Vector3{X: 0, Y: 0, Z: 1}
	}
	a := points[1].Sub(points[0])
	b := points[2].Sub(points[0])
	n := a.Cross(b)
	if n.Norm() <= kerngeo.Precision {
		return kerngeo.Vector3{X: 0, Y: 0, Z: 1}
	}
	u, _ := n.Normalized()
	return u
}

// meshSolid собирает вершины, текстурные координаты и треугольники (локальные
// индексы) одного тела Solid с разделяемым кешем триангуляций. Локальные
// индексы позволяют собирать mesh из параллельных результатов (result-slot,
// EM-06).
//
// origin — ОБЩАЯ точка отсчёта UV для всех деталей модели (обычно её
// минимальный угол габарита). Отсчёт от габарита КАЖДОЙ детали давал бы
// каждой ступени собственное начало координат, и все ступени выходили бы
// пиксель в пиксель одинаковыми. Общая точка делает рисунок непрерывным вдоль
// марша и одновременно убирает зависимость от размещения: сдвиг лестницы от
// стены двигает и габарит, поэтому UV не «прыгают» при движении ползунка.
func meshSolid(solid *kerngeo.Solid, tess *kerngeo.TessellationCache, origin kerngeo.Point3) ([]kerngeo.Point3, []kerngeo.Point2, [][3]int, error) {
	var verts []kerngeo.Point3
	var uvs []kerngeo.Point2
	var tris [][3]int
	offset := origin
	for _, shell := range solid.Shells() {
		for _, face := range shell.Faces() {
			t := tess.Face(face.Outer())
			if t.WireErr != nil {
				return nil, nil, nil, fmt.Errorf("geometry: face wire: %w", t.WireErr)
			}
			if t.TrisErr != nil {
				return nil, nil, nil, fmt.Errorf("geometry: face triangulation: %w", t.TrisErr)
			}
			base := len(verts)
			verts = append(verts, t.Points...)
			uvs = append(uvs, triplanarUV(t.Points, faceNormalOf(t.Points), offset)...)
			for _, tr := range t.Tris {
				tris = append(tris, [3]int{base + tr[0], base + tr[1], base + tr[2]})
			}
		}
	}
	return verts, uvs, tris, nil
}
