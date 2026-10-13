package geometry

// railing_layout.go — места покупных деталей ограждения (S-192).
//
// Тела ограждения строятся как декор и в Model не входят, поэтому decompose
// про них не знает, и покупные детали получали Placement.Known=false.
// Здесь — второй источник мест, но НЕ вторая правда: раскладка считается
// теми же формулами и константами, что straightRailingSolids
// (balusterSteps, edgeInset, xoff, fractions ригелей), в том же пакете.
// Разойтись с геометрией ей не из чего — кроме ручной правки одного
// места без другого, которую ловит TestRailingLayoutMatchesGeometry
// (якорь обязан лежать внутри тела той же роли).
//
// Область честности: ТОЛЬКО прямой марш со стальным ограждением.
// L/U, стекло и выключенные перила возвращают nil — «не знаю», а не
// выдуманные точки. manufacturing оставляет такие детали без места,
// и сборочный чертёж честно показывает их списком под планом.
//
// Порядок элементов ТОЧНО повторяет циклы purchasedRailingParts
// (manufacturing): поручни, стойки, ригели, фитинги, заглушки.
// Связь проверяет TestPurchasedLayoutZipped: расхождение — падение.

import (
	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// RailingPlaced — место одной покупной детали.
//
// Ref — опорная точка: у стоек и фитингов база на ступени, у поручня
// и ригелей начало отрезка A, у заглушек торец. U — единичный вектор,
// вдоль которого откладывается Part.Length (у стоек +Z, у поручня
// и ригелей — направление отрезка, у входной заглушки — внутрь трубы).
// manufacturing достраивает Origin так, чтобы Center(L, W) вернулся
// ровно в якорь (Ref + U·L/2): чертёж ставит номер в физический центр.
type RailingPlaced struct {
	// Kind — роль dommfg.PartKind строкой (ребро geometry→manufacturing
	// запрещено архитектурой, поэтому не константа, а её значение).
	Kind string
	Ref  kerngeo.Point3
	U    kerngeo.Vector3
	V    kerngeo.Vector3
	// Side — «А» (у Y=0) / «Б» (у дальней грани): то же соглашение,
	// что whereLabel у каркаса.
	Side string
	// Step — ступень стойки (1-based); 0 у поручня и торцов.
	Step int
	// PostIdx — номер стойки на своей стороне (1-based); 0 у поручня.
	PostIdx int
	// Row — ряд ригеля (0/1). Читается только при Kind == "rigel",
	// у остальных значение не используется.
	Row int
	// End — «начало»/«конец» у заглушек поручня; "" у остальных.
	End string
}

// RailingPurchasedLayout — места покупных деталей прямого стального
// ограждения в мировом кадре изделия (включая сдвиг approachShift:
// декор сдвигается синхронно с маршем, см. appendRailingMesh).
// nil — конфигурация вне области честности (см. шапку файла).
func RailingPurchasedLayout(cfg *engineering.StairConfiguration) []RailingPlaced {
	if cfg == nil || cfg.Flight != engineering.FlightStraight {
		return nil
	}
	if cfg.RailingType.Effective() != engineering.RailingTypeSteel {
		return nil
	}
	rh := cfg.RailingHeight.Millimeters()
	if !railingEnabled(rh, cfg.Railing) {
		return nil
	}
	n := cfg.StepCount
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	w := cfg.Width.Millimeters()
	if n <= 0 || b <= 0 || h <= 0 || w <= 0 {
		return nil
	}
	nosing := cfgProfileNosing(cfg)
	shift := approachShift(cfg)
	sides := railingSides(cfg.Railing, w)
	if len(sides) == 0 {
		return nil
	}
	H := float64(n) * h
	// xoff — та же поправка на свес, что в straightRailingSolids:
	// середина ступени, а не середина шага.
	xoff := -nosing
	steps := balusterSteps(n, b)
	zUp := kerngeo.NewVector3(0, 0, 1)
	xAx := kerngeo.NewVector3(1, 0, 0)
	yAx := kerngeo.NewVector3(0, 1, 0)

	sideName := func(edge float64) string {
		if edge < w/2 {
			return "А"
		}
		return "Б"
	}

	var out []RailingPlaced
	// 1. Поручни — по одному на сторону, как в straightRailingSolids:
	// A=(0,railY,rh), B=(n·b,railY,H+rh), без вылета за 0..n·b.
	for _, edge := range sides {
		railY := edgeInset(edge, w, railWidth/2)
		a := kerngeo.NewPoint3(shift, railY, rh)
		bp := kerngeo.NewPoint3(shift+float64(n)*b, railY, H+rh)
		u, ok := bp.Sub(a).Normalized()
		if !ok {
			return nil
		}
		out = append(out, RailingPlaced{
			Kind: "rail_top", Ref: a, U: u, V: yAx,
			Side: sideName(edge),
		})
	}
	// 2. Стойки — s-major, как плоский цикл purchasedRailingParts.
	for _, edge := range sides {
		balY := edgeInset(edge, w, balusterEdgeInset+capOD/2)
		for j, k := range steps {
			x := shift + (float64(k)-0.5)*b + xoff
			out = append(out, RailingPlaced{
				Kind: "post",
				Ref:  kerngeo.NewPoint3(x, balY, float64(k)*h),
				U:    zUp, V: xAx,
				Side: sideName(edge), Step: k, PostIdx: j + 1,
			})
		}
	}
	// 3. Ригели — s/r/i, как цикл purchasedRailingParts (ряды 33 %/66 %).
	if len(steps) >= 2 {
		for _, edge := range sides {
			balY := edgeInset(edge, w, balusterEdgeInset+capOD/2)
			for r, frac := range []float64{rigelFrac0, rigelFrac1} {
				for i := 0; i+1 < len(steps); i++ {
					k, kNext := steps[i], steps[i+1]
					x0 := shift + (float64(k)-0.5)*b + xoff
					x1 := shift + (float64(kNext)-0.5)*b + xoff
					a := kerngeo.NewPoint3(x0, balY, float64(k)*h+rh*frac)
					bp := kerngeo.NewPoint3(x1, balY, float64(kNext)*h+rh*frac)
					u, ok := bp.Sub(a).Normalized()
					if !ok {
						return nil
					}
					out = append(out, RailingPlaced{
						Kind: "rigel", Ref: a, U: u, V: yAx,
						Side: sideName(edge), Step: k, Row: r,
					})
				}
			}
		}
	}
	// 4. Фитинги — по комплекту на стойку, порядок стоек как в п. 2:
	// наконечник, фланец, муфта, накладка.
	for _, edge := range sides {
		balY := edgeInset(edge, w, balusterEdgeInset+capOD/2)
		for j, k := range steps {
			x := shift + (float64(k)-0.5)*b + xoff
			base := kerngeo.NewPoint3(x, balY, float64(k)*h)
			top := kerngeo.NewPoint3(x, balY, float64(k)*h+rh)
			side, post := sideName(edge), j+1
			out = append(out,
				RailingPlaced{Kind: "tip", Ref: top, U: zUp, V: xAx,
					Side: side, Step: k, PostIdx: post},
				RailingPlaced{Kind: "flange", Ref: base, U: zUp, V: xAx,
					Side: side, Step: k, PostIdx: post},
				RailingPlaced{Kind: "slip", Ref: base, U: zUp, V: xAx,
					Side: side, Step: k, PostIdx: post},
				RailingPlaced{Kind: "deco", Ref: base, U: zUp, V: xAx,
					Side: side, Step: k, PostIdx: post},
			)
		}
	}
	// 5. Заглушки — по две на нитку (вход и выход), U внутрь трубы:
	// якорь встаёт в физический центр заглушки, а не на срез.
	for _, edge := range sides {
		railY := edgeInset(edge, w, railWidth/2)
		a := kerngeo.NewPoint3(shift, railY, rh)
		bp := kerngeo.NewPoint3(shift+float64(n)*b, railY, H+rh)
		u, ok := bp.Sub(a).Normalized()
		if !ok {
			return nil
		}
		out = append(out,
			RailingPlaced{Kind: "end_cap", Ref: a, U: u, V: yAx,
				Side: sideName(edge), End: "начало"},
			RailingPlaced{Kind: "end_cap", Ref: bp, U: u.Scale(-1), V: yAx,
				Side: sideName(edge), End: "конец"},
		)
	}
	return out
}
