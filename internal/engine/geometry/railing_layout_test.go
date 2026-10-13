package geometry

// railing_layout_test.go — гейты раскладки покупных деталей (S-192).
//
// Раскладка считается теми же формулами, что straightRailingSolids, но
// «теми же» — это утверждение, а не факт, пока его не проверяет тест.
// TestRailingLayoutMatchesGeometry сверяет якоря с построенной геометрией
// (а не с повторением формул): якорь обязан лежать в пределах 12 мм
// от тела той же роли.

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// layoutRoles — какие тела геометрии соответствуют якорю каждого вида.
// Фитинги узла стоят на одной базе: цанга (clamp) и колпачок/держатель
// (holder) делят точку, поэтому якорю достаточно близости к любому
// телу узла.
var layoutRoles = map[string][]string{
	"rail_top": {"railing"},
	"post":     {"baluster"},
	"rigel":    {"rigel"},
	"tip":      {"holder"},
	"flange":   {"clamp", "holder"},
	"slip":     {"clamp", "holder"},
	"deco":     {"clamp", "holder"},
	"end_cap":  {"end_cap"},
}

// distToBox — расстояние от точки до габарита (0 — внутри).
func distToBox(p kerngeo.Point3, b kerngeo.BBox) float64 {
	dx := 0.0
	if p.X < b.Min.X {
		dx = b.Min.X - p.X
	} else if p.X > b.Max.X {
		dx = p.X - b.Max.X
	}
	dy := 0.0
	if p.Y < b.Min.Y {
		dy = b.Min.Y - p.Y
	} else if p.Y > b.Max.Y {
		dy = p.Y - b.Max.Y
	}
	dz := 0.0
	if p.Z < b.Min.Z {
		dz = b.Min.Z - p.Z
	} else if p.Z > b.Max.Z {
		dz = p.Z - b.Max.Z
	}
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// TestRailingLayoutMatchesGeometry — якоря в пределах 12 мм от тел.
//
// 12 мм — радиус бессмысленности для НОМЕРА НА ЧЕРТЕЖЕ, а не допуск
// контакта: номер — текст сантиметрового кегля. Ошибка кадра (потерянный
// approachShift, перепутанные X/Y) даёт сотни миллиметров и валится
// с запасом. Требовать «строго внутри тела» было бы ложной точностью:
// якорь заглушки осознанно стоит на 0,8 мм снаружи торца (см. layout).
func TestRailingLayoutMatchesGeometry(t *testing.T) {
	cfg := seedConfig()
	layout := RailingPurchasedLayout(cfg)
	if len(layout) == 0 {
		t.Fatal("раскладка пуста на прямом стальном марше сида")
	}
	// Та же геометрия теми же входами (nosing — через тот же helper,
	// а не числом: число здесь разъедется с cfgProfileNosing молча).
	sols := straightRailingSolids(ProductionQuality,
		seedStepCount, seedTreadDepth, seedStepHeight, seedRailingHeight,
		seedWidth, cfgProfileNosing(cfg), engineering.RailingLeft)
	if len(sols) == 0 {
		t.Fatal("геометрия перил сида пуста")
	}
	// Сдвиг approach: в проде декор сдвигается в appendRailingMesh,
	// здесь повторяем его руками — иначе все якоря «улетят» на 1000 мм.
	shift := approachShift(cfg)
	byRole := map[string][]kerngeo.BBox{}
	for _, s := range sols {
		bb := kerngeo.SolidBoundingBox(s)
		bb.Min.X += shift
		bb.Max.X += shift
		byRole[s.Role()] = append(byRole[s.Role()], bb)
	}
	for _, el := range layout {
		roles, ok := layoutRoles[el.Kind]
		if !ok {
			t.Errorf("якорь %q: нет отображения на роли геометрии", el.Kind)
			continue
		}
		best := math.Inf(1)
		for _, r := range roles {
			for _, bb := range byRole[r] {
				if d := distToBox(el.Ref, bb); d < best {
					best = d
				}
			}
		}
		if math.IsInf(best, 1) {
			t.Errorf("якорь %q: в геометрии нет тел ролей %v", el.Kind, roles)
			continue
		}
		if best > 12 {
			t.Errorf("якорь %q (%.1f, %.1f, %.1f) в %.1f мм от ближайшего тела %v",
				el.Kind, el.Ref.X, el.Ref.Y, el.Ref.Z, best, roles)
		}
	}
}

// TestSteelPostMatchesCatalog — геометрическая стойка длиной ровно
// по каталогу (POST-001 Len), а не rh.
//
// Замерено в эталоне: стойка 805,7 от ступени. Строить трубу длиной rh
// (900) — значит резать не по каталогу: в смете одна длина, в изделии
// другая (S-206, третий пункт — закрыт этим изменением).
func TestSteelPostMatchesCatalog(t *testing.T) {
	spec, ok := engineering.PartByID("POST-001")
	if !ok {
		t.Fatal("в каталоге нет POST-001")
	}
	sols := straightRailingSolids(ProductionQuality,
		seedStepCount, seedTreadDepth, seedStepHeight, seedRailingHeight,
		seedWidth, cfgProfileNosing(seedConfig()), engineering.RailingLeft)
	for _, s := range sols {
		if s.Role() != "baluster" {
			continue
		}
		bb := kerngeo.SolidBoundingBox(s)
		if got := bb.Max.Z - bb.Min.Z; math.Abs(got-spec.Len) > 0.5 {
			t.Errorf("стойка высотой %.1f, каталог %.1f", got, spec.Len)
		}
	}
}

// TestRailPassesOverPosts — поручень проходит над стойками: точка
// (верх стойки + railLiftAbovePostTop) лежит ВНУТРИ трубы поручня.
//
// Это гейт контакта S-208: раньше поручень шёл мимо верхов стоек
// (ниже на 103 мм и в 34 мм в стороне), наконечник висел в воздухе.
// Точка — analytic-центр трубы (в 21 мм от стенок), поэтому строгое
// containing, а не допуск: ошибка кадра даёт сотни миллиметров.
func TestRailPassesOverPosts(t *testing.T) {
	cfg := seedConfig()
	sols := straightRailingSolids(ProductionQuality,
		seedStepCount, seedTreadDepth, seedStepHeight, seedRailingHeight,
		seedWidth, cfgProfileNosing(cfg), engineering.RailingLeft)
	var rails []kerngeo.BBox
	for _, s := range sols {
		if s.Role() == "railing" {
			rails = append(rails, kerngeo.SolidBoundingBox(s))
		}
	}
	if len(rails) == 0 {
		t.Fatal("в геометрии нет поручня-трубы")
	}
	steps := balusterSteps(seedStepCount, seedTreadDepth)
	balY := edgeInset(0, seedWidth, balusterEdgeInset+capOD/2)
	for _, k := range steps {
		x := (float64(k)-0.5)*seedTreadDepth - cfgProfileNosing(cfg)
		top := float64(k)*seedStepHeight + steelPostHeight
		p := kerngeo.NewPoint3(x, balY, top+railLiftAbovePostTop)
		best := math.Inf(1)
		for _, bb := range rails {
			if d := distToBox(p, bb); d < best {
				best = d
			}
		}
		if best > 0.5 {
			t.Errorf("над стойкой ступени %d (%.1f) нет трубы поручня (%.1f мм)",
				k, top, best)
		}
	}
}

// TestRailingLayoutCounts — число якорей сходится с числом тел геометрии
// там, где соответствие один-к-одному: стойки, поручни, ригели, заглушки.
// Держатели и цанги состоят из нескольких тел (наконечник — 7 тел),
// поэтому для них — только близость (тест выше), а не счёт.
func TestRailingLayoutCounts(t *testing.T) {
	cfg := seedConfig()
	layout := RailingPurchasedLayout(cfg)
	sols := straightRailingSolids(ProductionQuality,
		seedStepCount, seedTreadDepth, seedStepHeight, seedRailingHeight,
		seedWidth, cfgProfileNosing(cfg), engineering.RailingLeft)
	countSol := map[string]int{}
	for _, s := range sols {
		countSol[s.Role()]++
	}
	countLay := map[string]int{}
	for _, el := range layout {
		countLay[el.Kind]++
	}
	// Роль геометрии → вид раскладки: baluster/post — один-к-одному.
	pairs := [][2]string{
		{"baluster", "post"},
		{"railing", "rail_top"},
		{"rigel", "rigel"},
		{"end_cap", "end_cap"},
	}
	for _, p := range pairs {
		if countSol[p[0]] != countLay[p[1]] {
			t.Errorf("тел %q: %d, якорей %q: %d", p[0], countSol[p[0]], p[1], countLay[p[1]])
		}
	}
}
