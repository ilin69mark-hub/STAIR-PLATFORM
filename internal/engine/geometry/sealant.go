package geometry

import (
	"stairplatform/internal/domain/engineering"
)

// ПЛОЩАДЬ ГЕРМЕТИЗАЦИИ (CONF-SEALANT).
//
// Герметик идёт по швам, а не по площади ступени: он закрывает стык
// деталей, через который потом проходит вода. Швы здесь такие:
//
//   - ступень ↔ опорная пятка 1 (heel) — ступень лежит на пятке, шов по всей
//     верхней грани пятки;
//   - подступенок ↔ пятка 2 (web) — пятка одевает вертикальный срез зуба и
//     держит подступенок;
//   - подступенок №1 ↔ пластина 1-й ступени (first_web) — плоскость, которой
//     подступенок стоит на пятке пола;
//   - торец косоура — плоскость, которой косоур упирается в перекрытие и в пол.
//
// Площади берутся из тех же размеров обоймы выреза, по которым пятки
// построены (comb_heels.go), поэтому шов совпадает с деталью: если
// изменится HeelWidth или HeelInset, изменится и расход герметика.

// ContactAreas — площади герметизируемых стыков, мм², по зонам.
//
// Зоны раздельные, а не одна сумма: в смете и в прайсе видно, куда ушло.
// Иначе невозможно понять, от чего отступать, когда расход не совпал с
// фактом на объекте.
type ContactAreas struct {
	// TreadHeelsMM2 — ступень к опорной пятке 1.
	TreadHeelsMM2 float64
	// RiserWebsMM2 — подступенок к пятке 2.
	RiserWebsMM2 float64
	// RiserFirstWebsMM2 — подступенок №1 к пластине 1-й ступени.
	RiserFirstWebsMM2 float64
	// StringerEndsMM2 — торцы косоуров.
	StringerEndsMM2 float64
}

// TotalMM2 — суммарная площадь герметизируемых стыков.
func (c ContactAreas) TotalMM2() float64 {
	return c.TreadHeelsMM2 + c.RiserWebsMM2 + c.RiserFirstWebsMM2 + c.StringerEndsMM2
}

// TotalM2 — площадь в м² (для формулы расхода V = S × t, где t в мм даёт
// объём в литрах: 1 м² × 1 мм = 1 л).
func (c ContactAreas) TotalM2() float64 { return c.TotalMM2() / 1e6 }

// SealantContactAreas считает площади герметизируемых стыков лестницы.
//
// Число пяток соответствует геометрии: пятки 1 ставятся под каждой ступенью
// с двух сторон (2n), пятки 2 — по каждому подступенку кроме последнего
// (2(n−1)), пластина 1-й ступени — одна на сторону (2). Без подступенков
// (Riser = false) пятки 2 и пластина 1-й ступени не строятся, и их площади
// не считаются: герметик по несуществующему шву — это расход на
// nonexistent-деталь, то есть прямой переплаты.
func SealantContactAreas(cfg *engineering.StairConfiguration) ContactAreas {
	if cfg == nil {
		return ContactAreas{}
	}
	sp := cfg.CombOrDefault()
	s := combHeelSpecOf(cfg)

	var areas ContactAreas

	// Ступень ↔ пятка 1: вся верхняя грань пятки.
	heelL := s.b - 2*s.inset
	if heelL > 0 && sp.HeelWidth > 0 {
		areas.TreadHeelsMM2 = 2 * float64(s.n) * heelL * sp.HeelWidth
	}

	if cfg.Riser {
		// Подступенок ↔ пятка 2.
		wh := webHeight(cfg, sp)
		if wh > 0 && sp.HeelWidth > 0 {
			areas.RiserWebsMM2 = 2 * float64(s.n-1) * wh * sp.HeelWidth
		}
		// Подступенок №1 ↔ пластина 1-й ступени.
		if s.firstWebH > 0 && s.firstWebW > 0 {
			areas.RiserFirstWebsMM2 = 2 * s.firstWebH * s.firstWebW
		}
	}

	// Торцы косоуров. Только для пластинчатого каркаса: у гребёнки из
	// дерева торца профилируются по зубьям, и площадь примыкания там не
	// равна сечению пластины. Считать «на глаз» значило бы класть в смету
	// герметик на стыки, которых нет.
	if PlateStringerFrame(cfg) {
		depth := sp.StringerSpine
		th := cfg.StringerThickness.Millimeters()
		if depth > 0 && th > 0 {
			// Два косоура × два торца (низ у пола, верх у перекрытия).
			areas.StringerEndsMM2 = 2 * 2 * depth * th
		}
	}
	return areas
}
