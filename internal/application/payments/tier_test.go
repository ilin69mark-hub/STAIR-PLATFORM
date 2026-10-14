package payments

import "testing"

// Терминальные статусы интента используются SQL-guard'ом перехода: из них
// выход запрещён. Ошибка здесь означает либо зависший платёж, либо двойное
// списание.

func TestTerminalStatusesIsTerminalGuard(t *testing.T) {
	list := TerminalStatuses()
	if len(list) == 0 {
		t.Fatal("список терминальных статусов пуст")
	}
	// Ожидаемое содержимое зафиксировано намеренно: SQL-guard в инфраструктуре
	// опирается именно на этот набор.
	want := map[Status]bool{StatusPaid: true, StatusFailed: true, StatusRefunded: true}
	for _, s := range list {
		if !want[s] {
			t.Errorf("в списке терминальных неожиданный статус %q", s)
		}
		if !isTerminalStatus(s) {
			t.Errorf("статус %q в списке, но isTerminalStatus о нём ложит", s)
		}
		delete(want, s)
	}
	for s := range want {
		t.Errorf("статус %q должен быть терминальным, но не попал в список", s)
	}
}

func TestNonTerminalStatuses(t *testing.T) {
	for _, s := range []Status{StatusPending, Status("processing"), Status("")} {
		if isTerminalStatus(s) {
			t.Errorf("статус %q не терминальный, но isTerminalStatus сказал иначе", s)
		}
	}
}

func TestCatalogListOrderIsStable(t *testing.T) {
	c := DefaultCatalog()
	got := c.List()
	// Порядок задан явно (Basic, Pro), а не обходом карты: витрина показывает
	// тарифы сверху вниз, и случайный порядок дёргал бы цены подвалом.
	if len(got) != 2 {
		t.Fatalf("тарифов %d, ждали 2", len(got))
	}
	if got[0].ID != TierBasic || got[1].ID != TierPro {
		t.Fatalf("порядок тарифов нарушен: %s, %s", got[0].ID, got[1].ID)
	}

	// nil-каталог возвращает nil, а не паникует: витрина зовёт List до
	// загрузки прайса.
	var nilCat *Catalog
	if nilCat.List() != nil {
		t.Error("nil-каталог вернул не nil")
	}
}

func TestNewCatalogDropsUnusableTiers(t *testing.T) {
	// Тариф без ID или с неположительной суммой в прайс не попадает:
	// предлагать клиенту бесплатный тариф хуже, чем не предлагать ничего.
	c := NewCatalog(
		Tier{ID: TierBasic, AmountMinor: 100, Currency: "RUB"},
		Tier{ID: TierPro, AmountMinor: 0, Currency: "RUB"},
		Tier{ID: "", AmountMinor: 500, Currency: "RUB"},
	)
	got := c.List()
	if len(got) != 1 || got[0].ID != TierBasic {
		t.Fatalf("в каталог попало не то: %+v", got)
	}
}

func TestNewCatalogKeepsFirstOfDuplicate(t *testing.T) {
	// Дубль ID не должен перетирать уже загруженный тариф: при стартовых
	// значениях из конфигурации это тихо подменило бы цену.
	c := NewCatalog(
		Tier{ID: TierBasic, AmountMinor: 100, Currency: "RUB"},
		Tier{ID: TierBasic, AmountMinor: 999_999, Currency: "RUB"},
	)
	got := c.List()
	if len(got) != 1 {
		t.Fatalf("дубль ID попал в каталог дважды: %+v", got)
	}
	if got[0].AmountMinor != 100 {
		t.Errorf("дубль перетирал первый тариф: сумма %d вместо 100", got[0].AmountMinor)
	}
}

func TestNewCatalogNormalizesCurrency(t *testing.T) {
	// Валюта нормализуется при загрузке, иначе платёж ушёл бы в валюту,
	// которую провайдер не понимает.
	c := NewCatalog(Tier{ID: TierBasic, AmountMinor: 100, Currency: "rub"})
	if got := c.List(); len(got) != 1 || got[0].Currency != "RUB" {
		t.Errorf("валюта не нормализована: %+v", got)
	}
}
