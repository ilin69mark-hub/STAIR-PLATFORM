package manufacturing

import (
	"fmt"
	"strconv"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// ЗЕРКАЛЬНЫЕ ДЕТАЛИ НЕ СКЛЕИВАЮТСЯ В ОДНУ ПОЗИЦИЮ РАСКРОЯ.
//
// Находка 2026-10-13. Позиция раскроя определялась габаритами детали. У
// Г- и П-марша верхний марш повёрнут на 180°, поэтому детали на нём
// зеркальны деталям нижнего: отверстия стоят на одних и тех же X, но
// выгибаются в противоположные стороны по Y. Габариты совпадают до
// миллиметра, и обе детали попадали в одну позицию.
//
// Пока в раскрое не было отверстий, это было не видно: позиция
// объединялась по габаритам, а документ реза отверстий вообще не содержал
// (писались 3DFACE меша). Оператор получал «26 одинаковых пластин 166×50»,
// ставил в стойку половину, а вторая половина не вставала на место.
//
// Теперь позиция учитывает рисунок отверстий, и зеркальность видна прямо в
// раскрое.

// TestMirroredPartsDoNotShareCutItem — детали одного размера с разным
// рисунком отверстий получают разные позиции раскроя.
func TestMirroredPartsDoNotShareCutItem(t *testing.T) {
	cfg := lshapeConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatalf("Manufacture: %v", err)
	}

	// Группируем позиции по «размер + число отверстий». Если две позиции
	// попали в одну группу, значит при одинаковых габаритах у них разный
	// рисунок — это и есть зеркальность (или иное отличие, которое раньше
	// терялось).
	groups := map[string][]string{}
	for _, it := range pkg.CutList.Items {
		if len(it.Holes) == 0 {
			continue
		}
		key := fmt.Sprintf("%s/%gx%g/отв=%d", it.MaterialCode,
			it.Length.Millimeters(), it.Width.Millimeters(), len(it.Holes))
		groups[key] = append(groups[key], string(it.PartNumber))
	}
	separated := 0
	for key, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		separated++
		t.Logf("%s разделён на %d позиции: %v", key, len(ids), ids)
	}
	if separated == 0 {
		t.Fatal("зеркальные позиции раскроя не разделены: детали верхнего и " +
			"нижнего марша склеились в одну позицию, и половина деталей встанет зеркально")
	}
	t.Logf("групп, разделённых по рисунку отверстий: %d", separated)
}

// TestCutItemIdentityIsUnambiguous — один рисунок отверстий не может быть
// размазан по нескольким позициям раскроя.
//
// Обратная сторона предыдущего теста: если разделение пошло неправильно, то
// одна и та же деталь описана двумя позициями, и лист посчитает её дважды.
func TestCutItemIdentityIsUnambiguous(t *testing.T) {
	cfg := lshapeConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatalf("Manufacture: %v", err)
	}
	seen := map[string]int{}
	for _, it := range pkg.CutList.Items {
		seen[cutItemID(it)]++
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("рисунок %s описан в %d позициях раскроя, а должен быть в одной", id, n)
		}
	}
}

// TestCutItemHolesLieInsideProfile — отверстия позиции раскроя лежат внутри
// её контура.
//
// Отверстие в поле заготовки — технологический вырез или запас под рельсы.
// Лазер его не ведёт, и в документе реза оно выглядело бы как дефект
// геометрии детали.
func TestCutItemHolesLieInsideProfile(t *testing.T) {
	c := testConfig(t)
	pkg, err := Manufacture(c, genResult(t, c), nil)
	if err != nil {
		t.Fatalf("Manufacture: %v", err)
	}
	withHoles := 0
	for _, it := range pkg.CutList.Items {
		if len(it.Holes) == 0 {
			continue
		}
		withHoles++
		if len(it.Profile) < 3 {
			t.Errorf("%s: отверстий %d, а контура нет — рез опишет только прямоугольник",
				it.PartNumber, len(it.Holes))
			continue
		}
		for i, h := range it.Holes {
			if got := dommfg.HolesWithinProfile([][]dommfg.Point2D{h}, it.Profile); len(got) != 1 {
				t.Errorf("%s: отверстие %d выходит за контур детали", it.PartNumber, i)
			}
		}
	}
	if withHoles == 0 {
		t.Fatal("ни у одной позиции раскроя нет отверстий — документ реза будет без отверстий")
	}
	t.Logf("позиций раскроя с отверстиями: %d", withHoles)
}

// cutItemID — подпись позиции раскроя: материал, габариты и весь рисунок
// отверстий. Ровно то, чем позиция обязана отличаться от любой другой.
func cutItemID(it dommfg.CutItem) string {
	s := fmt.Sprintf("%s/%gx%g", it.MaterialCode, it.Length.Millimeters(), it.Width.Millimeters())
	s += "/контур=" + strconv.Itoa(len(it.Profile))
	for _, h := range it.Holes {
		s += "/[" + strconv.Itoa(len(h))
		for _, p := range h {
			s += fmt.Sprintf("%.1f,%.1f", p.X, p.Y)
		}
		s += "]"
	}
	return s
}
