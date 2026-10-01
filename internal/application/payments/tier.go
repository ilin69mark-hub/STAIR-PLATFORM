package payments

import (
	"fmt"
	"strings"
)

func normalizeCurrency(c string) string {
	return strings.ToUpper(strings.TrimSpace(c))
}

// Tier — тариф платной услуги: серверный источник истины по цене
// (S-150, red-team «платное бесплатно»). Клиент выбирает тариф, сумму
// определяет сервер — подменить amount_minor в теле больше нельзя.
type Tier struct {
	ID          string
	AmountMinor int64
	Currency    string
	// Title/Description — витринные подписи услуги. Пустые допустимы:
	// кабинет и API отдадут ID, витрина покажет то, что есть.
	Title       string
	Description string
}

// Catalog — набор доступных тарифов (server-side price authority).
type Catalog struct {
	tiers map[string]Tier
}

// NewCatalog создаёт каталог; дубли ID отвергаются (первый побеждает,
// чтобы опечатка в конфиге не тихо затеняла тариф).
func NewCatalog(tiers ...Tier) *Catalog {
	c := &Catalog{tiers: make(map[string]Tier, len(tiers))}
	for _, t := range tiers {
		t.Currency = normalizeCurrency(t.Currency)
		if t.ID == "" || t.AmountMinor <= 0 {
			continue
		}
		if _, dup := c.tiers[t.ID]; !dup {
			c.tiers[t.ID] = t
		}
	}
	return c
}

// DefaultCatalog — дефолтный прайс (рубли, копейки). Значения — стартовые:
// меняются конфигурацией (STAIR_PAYMENT_TIERS) в cmd/api/main.go, клиент на них
// не влияет.
func DefaultCatalog() *Catalog {
	return NewCatalog(
		Tier{
			ID: TierBasic, AmountMinor: 90_000, Currency: "RUB",
			Title:       "Выезд инженера и замер",
			Description: "Инженер приедет с образцами материалов, снимет размеры проёма и проверит, вписывается ли лестница. Смету по замерам — в тот же день.",
		},
		Tier{
			ID: TierPro, AmountMinor: 180_000, Currency: "RUB",
			Title:       "Проект и рабочая документация",
			Description: "Полный проект под ключ: планы, разрезы, спецификация деталей и раскрой. По этому проекту лестницу собирает любой мастер.",
		},
	)
}

// Идентификаторы тарифов.
const (
	TierBasic = "basic"
	TierPro   = "pro"
)

// Resolve возвращает тариф по ID. Неизвестный/пустой ID — ErrInvalid
// (S-150: цена только из серверного каталога, никаких amount из тела).
func (c *Catalog) Resolve(id string) (Tier, error) {
	if c == nil {
		return Tier{}, fmt.Errorf("%w: price catalog not configured", ErrInvalid)
	}
	t, ok := c.tiers[id]
	if !ok {
		return Tier{}, fmt.Errorf("%w: unknown tier %q", ErrInvalid, id)
	}
	return t, nil
}

// List возвращает тарифы для витрины (id+цена; порядок — как в NewCatalog).
func (c *Catalog) List() []Tier {
	if c == nil {
		return nil
	}
	out := make([]Tier, 0, len(c.tiers))
	for _, id := range []string{TierBasic, TierPro} {
		if t, ok := c.tiers[id]; ok {
			out = append(out, t)
		}
	}
	return out
}
