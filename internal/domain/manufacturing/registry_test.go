package manufacturing

import "testing"

// Каталог крепежа — то, из которого собирается смета. Ошибка здесь означает
// либо позицию нулевой длины в раскрое, либо крепёж, который не на что
// заменить.

func TestFastenerKindIsValid(t *testing.T) {
	for _, k := range []FastenerKind{FastenerScrew, FastenerPlug} {
		if !k.IsValid() {
			t.Errorf("вид крепежа %q отвергнут как недопустимый", k)
		}
	}
	for _, k := range []FastenerKind{"", "nail", "weld"} {
		if k.IsValid() {
			t.Errorf("вид крепежа %q прошёл как допустимый", k)
		}
	}
}

func TestFastenerValidate(t *testing.T) {
	good := &Fastener{Code: "SCREW-6x35", Kind: FastenerScrew, DiameterMM: 6, LengthMM: 35}
	if err := good.Validate(); err != nil {
		t.Fatalf("корректная позиция отвергнута: %v", err)
	}

	// Порядок проверок важен: пустой код и неизвестный вид отвергаются прежде
	// размеров, иначе в смете появилась бы позиция без кода.
	cases := []struct {
		name string
		f    *Fastener
	}{
		{"nil", nil},
		{"пустой код", &Fastener{Code: "  ", Kind: FastenerScrew, DiameterMM: 6, LengthMM: 35}},
		{"неизвестный вид", &Fastener{Code: "X", Kind: "weld", DiameterMM: 6, LengthMM: 35}},
		{"нулевой диаметр", &Fastener{Code: "X", Kind: FastenerScrew, DiameterMM: 0, LengthMM: 35}},
		{"нулевая длина", &Fastener{Code: "X", Kind: FastenerScrew, DiameterMM: 6, LengthMM: -1}},
	}
	for _, c := range cases {
		if err := c.f.Validate(); err == nil {
			t.Errorf("%s: принято без ошибки", c.name)
		}
	}
}

func TestFastenerRegistryFindAndCodes(t *testing.T) {
	reg := DefaultFastenerRegistry()
	if reg == nil {
		t.Fatal("каталог крепежа не собран")
	}
	all := reg.Fasteners()
	if len(all) == 0 {
		t.Fatal("каталог пуст")
	}

	// Каждая позиция каталога обязана находиться по своему коду: иначе
	// подстановка крепежа в смету молча не нашла бы деталь.
	for _, f := range all {
		got, ok := reg.Find(f.Code)
		if !ok {
			t.Errorf("позиция %q есть в каталоге, но Find её не находит", f.Code)
			continue
		}
		if got.Code != f.Code {
			t.Errorf("Find(%q) вернул %q", f.Code, got.Code)
		}
		if err := got.Validate(); err != nil {
			t.Errorf("позиция %q в каталоге не проходит Validate: %v", f.Code, err)
		}
	}

	// Коды отдаются отсортированными: интерфейс и выгрузка сметы должны быть
	// устойчивы к порядку обхода карты.
	codes := reg.Codes()
	for i := 1; i < len(codes); i++ {
		if codes[i-1] > codes[i] {
			t.Fatalf("коды не отсортированы: %q перед %q", codes[i-1], codes[i])
		}
	}
	if len(codes) != len(all) {
		t.Errorf("Codes вернул %d при %d позициях", len(codes), len(all))
	}
}

func TestFastenerRegistryFindMisses(t *testing.T) {
	reg := DefaultFastenerRegistry()
	if _, ok := reg.Find("SCREW-НЕТ-ТАКОГО"); ok {
		t.Error("Find нашёл несуществующий код")
	}
	if _, ok := reg.Find(""); ok {
		t.Error("Find нашёл позицию по пустому коду")
	}

	// nil-реестр не должен паниковать: вызывающий не обязан проверять его сам.
	var nilReg *FastenerRegistry
	if _, ok := nilReg.Find("SCREW-6x35"); ok {
		t.Error("nil-реестр что-то нашёл")
	}
	if nilReg.Fasteners() != nil || nilReg.Codes() != nil {
		t.Error("nil-реестр вернул не nil")
	}
}

func TestSealantKindIsValid(t *testing.T) {
	// Вид герметика проверяется так же, как крепёж: неизвестный вид не должен
	// доезжать до сметы.
	for _, k := range []SealantKind{SealantSilicone, SealantPU} {
		if !k.IsValid() {
			t.Errorf("вид герметика %q отвергнут", k)
		}
	}
	if SealantKind("").IsValid() {
		t.Error("пустой вид герметика прошёл")
	}
	if SealantKind("bitumen").IsValid() {
		t.Error("выдуманный вид герметика прошёл")
	}
}

func TestSealantRegistry(t *testing.T) {
	reg := DefaultSealantRegistry()
	if reg == nil {
		t.Fatal("каталог герметиков не собран")
	}
	all := reg.Sealants()
	if len(all) == 0 {
		t.Fatal("каталог герметиков пуст")
	}
	for _, s := range all {
		if _, ok := reg.Find(s.Code); !ok {
			t.Errorf("позиция %q есть в каталоге, но Find её не находит", s.Code)
		}
		if err := s.Validate(); err != nil {
			t.Errorf("позиция %q не проходит Validate: %v", s.Code, err)
		}
	}
	if _, ok := reg.Find("НЕТ-ТАКОГО"); ok {
		t.Error("Find нашёл несуществующий код герметика")
	}
	var nilReg *SealantRegistry
	if _, ok := nilReg.Find("SILICONE"); ok {
		t.Error("nil-реестр герметиков что-то нашёл")
	}
}