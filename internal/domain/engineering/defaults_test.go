package engineering

import (
	"sync"
	"testing"
)

// Effective и OrDefault — функции подстановки «пустое значит стандартное».
// Они вызываются на каждом расчёте, поэтому пустое значение обязано
// разрешаться в осмысленный вариант, а не теряться.

// RailingType: пусто = стальное. Ошибка здесь тихо превратила бы заказ на
// стекло в заказ на сталь, и цена была бы другой.
func TestRailingTypeEffective(t *testing.T) {
	if got := RailingTypeGlass.Effective(); got != RailingTypeGlass {
		t.Errorf("стекло разрешилось в %q", got)
	}
	for _, in := range []RailingType{"", RailingTypeSteel, RailingType("unknown")} {
		if got := in.Effective(); got != RailingTypeSteel {
			t.Errorf("RailingType(%q).Effective() = %q, ждали сталь", in, got)
		}
	}
}

// FixingBase: пусто = бетон.
func TestFixingBaseOrDefault(t *testing.T) {
	if got := FixingBase("").OrDefault(); got != BaseConcrete {
		t.Errorf("пустое основание разрешилось в %q, ждали бетон", got)
	}
	// Оснований два — бетон и дерево. Пустое подставляет бетон.
	for _, in := range []FixingBase{BaseWood} {
		if got := in.OrDefault(); got != in {
			t.Errorf("основание %q подменилось на %q", in, got)
		}
	}
}

// CorrelationID ходит под мьютексом: он ставится при старте pipeline и
// читается при трассировке, то есть из разных горутин.
func TestCorrelationIDRoundTrip(t *testing.T) {
	o := &EngineeringObject{}
	if got := o.CorrelationID(); got != "" {
		t.Errorf("у нового объекта correlation ID = %q, ждали пусто", got)
	}
	o.SetCorrelationID("corr-1")
	if got := o.CorrelationID(); got != "corr-1" {
		t.Errorf("CorrelationID() = %q после SetCorrelationID", got)
	}
	o.SetCorrelationID("corr-2")
	if got := o.CorrelationID(); got != "corr-2" {
		t.Errorf("второй SetCorrelationID не перезаписал: %q", got)
	}
}

func TestCorrelationIDConcurrent(t *testing.T) {
	o := &EngineeringObject{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); o.SetCorrelationID("x") }()
		go func() { defer wg.Done(); _ = o.CorrelationID() }()
	}
	wg.Wait()
	// Тест падает сам при гонке: -race в CI это увидит. Здесь проверяем лишь
	// что параллельные обращения не оставили объект в негодном состоянии.
	o.SetCorrelationID("final")
	if got := o.CorrelationID(); got != "final" {
		t.Errorf("после параллельных обращений ID = %q", got)
	}
}
