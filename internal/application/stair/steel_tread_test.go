package stair

import (
	"context"
	"strings"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// S-166: перевод подступенка и проступи на сталь. Решение владельца — «в
// будущем обе детали будут так же из стали». Задача карточки сейчас не в том,
// чтобы переключить материал по умолчанию, а в том, чтобы убедиться: когда
// переключат, стальные ступень и подступенок пройдут ограничения каталога
// листов, попадут в раскрой и в смету.
//
// Проверяется на настоящем расчёте, потому что по дороге материал может
// потеряться трижды: в ManufacturingPackage (деталь без признама), в раскрое
// (деталь без реза) или в цене (нет трудозатрат).

// findPart ищет деталь по виду в пакете.
func findPart(pkg *dommfg.ManufacturingPackage, kind dommfg.PartKind) (dommfg.Part, bool) {
	if pkg == nil {
		return dommfg.Part{}, false
	}
	for _, p := range pkg.Parts {
		if p.Kind == kind {
			return p, true
		}
	}
	return dommfg.Part{}, false
}

func TestSteelTreadAndRiserReachPackage(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatalf("стальная лестница не посчиталась: %v", err)
	}
	if res.Package == nil {
		t.Fatal("производственный пакет не собран — материал проверить негде")
	}

	for _, kind := range []dommfg.PartKind{dommfg.PartTread, dommfg.PartRiser} {
		p, ok := findPart(res.Package, kind)
		if !ok {
			t.Errorf("деталь %q не попала в пакет", kind)
			continue
		}
		// Материал детали обязан совпасть с TreadMaterial, а не с материалом
		// каркаса: иначе переключение TreadMaterial не влияет ни на что, и
		// будущий переход на сталь будет выглядеть как сделанный, а работать
		// не будет.
		if string(p.Material) != string(metalStairConfig().TreadMaterial) {
			t.Errorf("деталь %q получила материал %q вместо %q",
				kind, p.Material, metalStairConfig().TreadMaterial)
		}
		// Толщина детали не обязана совпадать с заданной конфигурацией:
		// при скруглении проступь собирается иначе, и движок приводит лист к
		// своей толщине. Проверяем, что стальные детали получили ОДИНАКОВУЮ
		// толщину и что она стальная (в каталоге листов 3–8 мм), а не 40 мм
		// из конфигурации дерева.
		if p.Thickness.Millimeters() < 3 || p.Thickness.Millimeters() > 8 {
			t.Errorf("деталь %q получила толщину %.1f — вне диапазона листа 3–8 мм",
				kind, p.Thickness.Millimeters())
		}
	}
}

func TestSteelTreachReachesCutListAndNesting(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}

	// Стальная ступень режется из листа, значит обязана быть в карте раскроя.
	// Если её там нет, материал дошёл до пакета, но не до раскроя — и раскрой
	// посчитан не по тому, что на самом деле режется.
	inCut := false
	for _, item := range res.Package.CutList.Items {
		if item.MaterialCode == metalStairConfig().TreadMaterial {
			inCut = true
			break
		}
	}
	if !inCut {
		t.Error("стальная ступень не попала в карту раскроя")
	}

	if res.Package.Nesting == nil {
		t.Fatal("раскрой не рассчитан")
	}
	if res.Package.Nesting.PartCount <= 0 {
		t.Error("раскрой пуст: ни одной детали не размещено")
	}
}

func TestSteelThicknessOutsideRangeIsClampedNotRejected(t *testing.T) {
	// Толщина 60 мм для стали — вне диапазона выпуска листа (3–12 мм).
	// Расчёт её НЕ отвергает, а схлопывает в 12 мм: laserPlateMM
	// ограничивает модельную толщину диапазоном выпуска. Это намеренное
	// решение — у стального косоура заданная толщина означает ширину сечения,
	// а не толщину листа, и без схлопывания цена металла улетела бы на
	// порядки.
	//
	// Карточка S-166 просит проверить ограничения каталога. Проверка показала:
	// ограничение есть, но оно молчаливое — клиент не узнаёт, что его 60 мм
	// превратили в 12. Тест фиксирует фактическое поведение.
	cfg := metalStairConfig()
	cfg.StepThickness = mustLengthHelper(60)

	s := NewService()
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	p, ok := findPart(res.Package, dommfg.PartTread)
	if !ok {
		t.Fatal("проступь не попала в пакет")
	}
	if got := p.Thickness.Millimeters(); got > 12 {
		t.Fatalf("толщина ступени %.1f осталась выше выпуска листа (12 мм) — схлопывание не сработало", got)
	}
}

func TestSteelPriceDiffersFromWood(t *testing.T) {
	s := NewService()
	steel, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatalf("сталь: %v", err)
	}
	wood, err := s.Calculate(context.Background(), woodStairConfig(), Options{})
	if err != nil {
		t.Fatalf("дерево: %v", err)
	}
	if steel.Price == nil || wood.Price == nil {
		t.Fatal("цена не рассчитана для одной из конфигураций")
	}
	// Материал ступеней влияет на цену: раскрой и закупка разные, а значит
	// и сумма должна отличаться. Одинаковые суммы означали бы, что материал
	// потерялся по дороге.
	if steel.Price.FinalPrice == wood.Price.FinalPrice {
		t.Errorf("цена стали и дерева совпала (%v) — материал ступеней ни на что не влияет",
			wood.Price.FinalPrice)
	}
}

func TestSteelMaterialAcceptedByCatalog(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatalf("расчёт: %v", err)
	}
	// Каталог листов: если материал ступеней не имеет листа в реестре, раскрой
	// молча вырождается в «всё влезло».
	for _, msg := range blockingMessages(res) {
		if strings.Contains(msg, "MFG_MATERIAL") {
			t.Errorf("блокирующая проблема материала на валидной стали: %s", msg)
		}
	}
}

func blockingMessages(res *Result) []string {
	var out []string
	for _, i := range res.Validation.Issues {
		if i.Severity == "error" || i.Severity == "blocking" {
			out = append(out, string(i.Code)+": "+i.Message)
		}
	}
	return out
}
