package order

import "testing"

// IsTerminal запрещает переход из уже завершённого заказа: без него отменённый
// заказ можно было бы снова оплатить, а завершённый — переоткрыть.
//
// Файл назван status_test.go, а не entity_test.go: в последнем уже лежит
// fakeRepo для consultation_test.go, и перезапись унесла бы чужие тесты.

func TestStatusIsTerminal(t *testing.T) {
	for _, s := range TerminalStatuses {
		if !s.IsTerminal() {
			t.Errorf("статус %q в TerminalStatuses, но IsTerminal() ложит", s)
		}
	}
	for _, s := range []Status{Status(""), Status("pending"), Status("new")} {
		if s.IsTerminal() {
			t.Errorf("статус %q объявлен терминальным", s)
		}
	}
}
