package manufacturing

import (
	"strings"
	"testing"
)

// TestRoleCodesExistInCatalog — коды, которыми расчёт помечает узлы, обязаны
// лежать в каталоге.
//
// Проверяет не «значения совпадают» (это повторило бы константы), а
// признак реальной связи: движек берёт эти три кода дословно, и если позиции
// в каталоге нет, add() молча её пропустит. Потребность в крепеже уедет в
// ноль, а расчёт не упадёт и тест зелёного каталога ничего не заметит —
// ровно тот класс отказа, из-за которого появился S-181.
func TestRoleCodesExistInCatalog(t *testing.T) {
	reg := DefaultFastenerRegistry()
	for _, code := range []string{ScrewTreadRiserCode, ScrewAnchorCode, PlugAnchorCode} {
		if _, ok := reg.Find(code); !ok {
			t.Errorf("код узла %q отсутствует во встроенном каталоге: смета потеряет крепёж молча", code)
		}
	}
}

// TestEveryAnchoringScrewHasPlug — у каждого глухаря, который можно вворачивать
// в бетон, есть парный дюбель.
//
// Правило крепления в бетон: на отверстие идёт глухарь И дюбель, и код
// дюбеля получается из кода глухаря заменой префикса (plugCodeFor). Позиция
// добавляется в каталог вручную, поэтому пару можно потерять: завели
// PLUG-8x80, но не SCREW-8x80 — и тогда при бетонном основании дюбель исчезает
// из сметы без единой ошибки, потому что несуществующий код add() пропускает
// молча.
//
// Условие — то же, что у продакшн-кода: addAnchoring зовёт дюбель по
// основанию, FixingBase.NeedsPlug(). Поэтому SCREW-6x35 (глухарь в дерево,
// основание BaseWood) проверка не требует: он к бетону не идёт, и PLUG-6x35
// в каталоге нет законно.
func TestEveryAnchoringScrewHasPlug(t *testing.T) {
	reg := DefaultFastenerRegistry()
	for _, f := range reg.Fasteners() {
		if f.Kind != FastenerScrew || !f.Base.NeedsPlug() {
			continue
		}
		if !strings.HasPrefix(f.Code, "SCREW-") {
			t.Errorf("глухарь с кодом %q без префикса SCREW- — plugCodeFor его не найдёт", f.Code)
			continue
		}
		plugCode := "PLUG-" + strings.TrimPrefix(f.Code, "SCREW-")
		p, ok := reg.Find(plugCode)
		if !ok {
			t.Errorf("к глухарю %q нет парного дюбеля %q: крепление в бетон останется без дюбеля",
				f.Code, plugCode)
			continue
		}
		if p.DiameterMM != f.DiameterMM || p.LengthMM != f.LengthMM {
			t.Errorf("дюбель %q (%g×%g) не пара глухарю %q (%g×%g)",
				plugCode, p.DiameterMM, p.LengthMM, f.Code, f.DiameterMM, f.LengthMM)
		}
	}
}
