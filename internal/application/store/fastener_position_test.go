package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// errBoom — искусственный сбой чтения справочника позиций.
var errBoom = errors.New("store: boom")

// Позиции крепежа, добавленные магазином (000048, решение владельца
// 2026-10-13).
//
// Главное, что проверяется здесь, — не CRUD, а СВЯЗЬ: позиция, добавленная в
// админке, обязана попасть в каталог расчёта и в прайс. Если этого нет,
// владелец добавит крепёж, увидит его в интерфейсе и получит смету без него —
// ровно тот дефект, что закрывал S-179 у ограждения и который молчалив.

// newPosition — своя позиция с заполненными полями.
func newPosition() FastenerPosition {
	return FastenerPosition{
		Code: "SCREW-6x16", NameRu: "Глухарь 6х16 (свой)", Kind: "screw",
		DiameterMM: 6, LengthMM: 16, Base: "wood", PricePerPcRub: 8,
	}
}

// TestFastenerRegistryIncludesCustomPosition — своя позиция входит в каталог
// расчёта и имеет те же размеры, что заданы в админке.
func TestFastenerRegistryIncludesCustomPosition(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	p := newPosition()
	if err := svc.CreateFastenerPosition(context.Background(), "t-1", p, "u-1"); err != nil {
		t.Fatalf("CreateFastenerPosition: %v", err)
	}
	reg, err := svc.FastenerRegistry(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("FastenerRegistry: %v", err)
	}
	f, ok := reg.Find(p.Code)
	if !ok {
		t.Fatalf("позиция %q отсутствует в каталоге расчёта — крепёж молча выпадет из сметы", p.Code)
	}
	if f.NameRu != p.NameRu || f.DiameterMM != p.DiameterMM || f.LengthMM != p.LengthMM {
		t.Errorf("позиция искажена: %+v, want код/название/размер как в админке", f)
	}
	if f.Kind != dommfg.FastenerScrew {
		t.Errorf("вид = %q, want screw", f.Kind)
	}
	// Встроенный каталог должен остаться на месте: своя позиция его
	// дополняет, а не заменяет.
	if _, ok := reg.Find(dommfg.ScrewTreadRiserCode); !ok {
		t.Errorf("встроенная позиция %q пропала из каталога после добавления своей", dommfg.ScrewTreadRiserCode)
	}
}

// TestFastenerPositionReachesPrice — у своей позиции есть цена, и прайс её
// показывает. Позиция без цены попала бы в смету нулём.
func TestFastenerPositionReachesPrice(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	p := newPosition()
	if err := svc.CreateFastenerPosition(context.Background(), "t-1", p, "u-1"); err != nil {
		t.Fatalf("CreateFastenerPosition: %v", err)
	}
	prices, err := svc.FastenerPrices(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("FastenerPrices: %v", err)
	}
	for _, pr := range prices {
		if pr.Code != p.Code {
			continue
		}
		if pr.PricePerPcRub != p.PricePerPcRub {
			t.Fatalf("цена позиции = %d, want %d", pr.PricePerPcRub, p.PricePerPcRub)
		}
		if !pr.Overridden {
			t.Error("у своей позиции Overridden = false: цена задана магазином, это переопределение")
		}
		return
	}
	t.Fatalf("позиция %q не попала в прайс", p.Code)
}

// TestFastenerPositionIsTenantScoped — своя позиция одного tenant'а не
// появляется у другого. Иначе позиция, добавленная в одной админке, тихо
// меняла бы сметы чужих проектов.
func TestFastenerPositionIsTenantScoped(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	p := newPosition()
	if err := svc.CreateFastenerPosition(context.Background(), "t-1", p, "u-1"); err != nil {
		t.Fatalf("CreateFastenerPosition: %v", err)
	}
	reg, err := svc.FastenerRegistry(context.Background(), "t-2")
	if err != nil {
		t.Fatalf("FastenerRegistry: %v", err)
	}
	if _, ok := reg.Find(p.Code); ok {
		t.Fatalf("позиция %q tenant t-1 попала в каталог tenant t-2", p.Code)
	}
}

// TestFastenerPositionCannotShadowBuiltin — своя позиция не может занять код
// встроенной. Иначе «глухарь 6х35» с другим размером тихо испортил бы расчёт
// всем, кто считает по встроенному коду.
func TestFastenerPositionCannotShadowBuiltin(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	p := newPosition()
	p.Code = dommfg.ScrewTreadRiserCode // SCREW-6x35
	p.LengthMM = 99
	err := svc.CreateFastenerPosition(context.Background(), "t-1", p, "u-1")
	if err == nil {
		t.Fatal("своя позиция заняла код встроенной — расчёт по этому коду изменился бы молча")
	}
	if !strings.Contains(err.Error(), "встроенном каталоге") {
		t.Errorf("ошибка не объясняет причину: %v", err)
	}
	reg, _ := svc.FastenerRegistry(context.Background(), "t-1")
	f, _ := reg.Find(p.Code)
	if f.LengthMM == 99 {
		t.Fatal("подмена всё-таки прошла: встроенная позиция изменилась")
	}
}

// TestBuiltinPositionCannotBeDeleted — встроенную позицию удалить нельзя: она
// в коде, и удаление из справочника tenant'а ничего бы не убрало.
func TestBuiltinPositionCannotBeDeleted(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	err := svc.DeleteFastenerPosition(context.Background(), "t-1", dommfg.ScrewTreadRiserCode)
	if err == nil {
		t.Fatal("встроенная позиция удалилась — удаление не имело бы смысла, код вернулся бы из кода")
	}
}

// TestDeleteFastenerPositionRemovesPrice — удаление позиции убирает и цену.
// Иначе код снова можно занять, и новая позиция молча получила бы цену
// удалённой.
func TestDeleteFastenerPositionRemovesPrice(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	p := newPosition()
	if err := svc.CreateFastenerPosition(context.Background(), "t-1", p, "u-1"); err != nil {
		t.Fatalf("CreateFastenerPosition: %v", err)
	}
	if err := svc.DeleteFastenerPosition(context.Background(), "t-1", p.Code); err != nil {
		t.Fatalf("DeleteFastenerPosition: %v", err)
	}
	if _, ok := repo.fasteners["t-1"][p.Code]; ok {
		t.Errorf("цена удалённой позиции %q осталась в прайсе", p.Code)
	}
	reg, _ := svc.FastenerRegistry(context.Background(), "t-1")
	if _, ok := reg.Find(p.Code); ok {
		t.Errorf("удалённая позиция %q осталась в каталоге расчёта", p.Code)
	}
}

// TestCreateFastenerPositionRejectsBadInput — позиция с негодными полями
// отвергается ДО записи, с внятным текстом и упоминанием поля.
func TestCreateFastenerPositionRejectsBadInput(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(p *FastenerPosition)
		field string
	}{
		{"пустой код", func(p *FastenerPosition) { p.Code = "" }, "код"},
		{"пустое название", func(p *FastenerPosition) { p.NameRu = "" }, "название"},
		{"неизвестный вид", func(p *FastenerPosition) { p.Kind = "weld" }, "вид крепежа"},
		{"нулевой диаметр", func(p *FastenerPosition) { p.DiameterMM = 0 }, "диаметр"},
		{"диаметр-изменя", func(p *FastenerPosition) { p.DiameterMM = 1e6 }, "диаметр"},
		{"нулевая длина", func(p *FastenerPosition) { p.LengthMM = 0 }, "длина"},
		{"длина-изменя", func(p *FastenerPosition) { p.LengthMM = 1e6 }, "длина"},
		{"неизвестный материал", func(p *FastenerPosition) { p.Base = "plastic" }, "материал"},
		{"нулевая цена", func(p *FastenerPosition) { p.PricePerPcRub = 0 }, "цена"},
		{"отрицательная цена", func(p *FastenerPosition) { p.PricePerPcRub = -5 }, "цена"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := NewService(repo)
			p := newPosition()
			c.mut(&p)
			err := svc.CreateFastenerPosition(context.Background(), "t-1", p, "u-1")
			if err == nil {
				t.Fatalf("позиция с %s принята", c.name)
			}
			if !strings.Contains(strings.ToLower(err.Error()), c.field) {
				t.Errorf("ошибка не называет поле %q: %v", c.field, err)
			}
			if got, _ := repo.ListFastenerPositions(context.Background(), "t-1"); len(got) != 0 {
				t.Errorf("негодная позиция всё-таки записана: %+v", got)
			}
		})
	}
}

// TestCreateFastenerPositionRejectsDuplicate — повторная позиция с тем же
// кодом отвергается: тихо перезаписать размеры означало бы, что уже
// посчитанные проекты поедут без предупреждения.
func TestCreateFastenerPositionRejectsDuplicate(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	p := newPosition()
	if err := svc.CreateFastenerPosition(context.Background(), "t-1", p, "u-1"); err != nil {
		t.Fatalf("CreateFastenerPosition: %v", err)
	}
	dup := p
	dup.LengthMM = 40
	dup.PricePerPcRub = 99
	if err := svc.CreateFastenerPosition(context.Background(), "t-1", dup, "u-1"); err == nil {
		t.Fatal("повторная позиция с тем же кодом принята — размеры сменились молча")
	}
	got, _ := repo.ListFastenerPositions(context.Background(), "t-1")
	if len(got) != 1 || got[0].LengthMM != p.LengthMM {
		t.Fatalf("исходная позиция искажена повтором: %+v", got)
	}
}

// TestFastenerRegistryFallsBackOnStoreFailure — сбой чтения справочника не
// роняет расчёт, а возвращает встроенный каталог. Иначе недоступность базы
// означала бы невозможность посчитать лестницу вообще.
func TestFastenerRegistryFallsBackOnStoreFailure(t *testing.T) {
	repo := newFakeRepo()
	repo.errPositions = errBoom
	svc := NewService(repo)
	reg, err := svc.FastenerRegistry(context.Background(), "t-1")
	if err == nil {
		t.Error("ошибка чтения справочника не сообщена — вызывающий не узнает, что считает по встроенному каталогу")
	}
	if reg == nil {
		t.Fatal("каталог не вернулся: расчёт упал бы вместо отката")
	}
	if _, ok := reg.Find(dommfg.ScrewTreadRiserCode); !ok {
		t.Error("во встроенном каталоге после сбоя нет штатной позиции")
	}
}

// TestFastenerRegistryWithoutTenantIsBuiltin — без tenant-контекста справочник
// магазина не читается вовсе (внутренние вызовы без контекста).
func TestFastenerRegistryWithoutTenantIsBuiltin(t *testing.T) {
	svc := NewService(newFakeRepo())
	reg, err := svc.FastenerRegistry(context.Background(), "")
	if err != nil {
		t.Fatalf("FastenerRegistry: %v", err)
	}
	if len(reg.Fasteners()) != len(dommfg.DefaultFastenerRegistry().Fasteners()) {
		t.Error("без tenant-контекста каталог отличается от встроенного")
	}
}
