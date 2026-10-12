package project

import "testing"

// Label — как участник показан в списке. Участник без имени виден только по
// почте, поэтому запасной вариант обязан работать всегда.

func TestProjectMemberLabel(t *testing.T) {
	// Есть и имя, и почта — показываем оба, чтобы отличить двух однофамильцев.
	m := &ProjectMember{Name: "Иван Петров", Email: "ivan@example.com"}
	if got := m.Label(); got != "Иван Петров — ivan@example.com" {
		t.Errorf("Label() = %q", got)
	}
	// Только имя — почты нет.
	m = &ProjectMember{Name: "Иван Петров"}
	if got := m.Label(); got != "Иван Петров" {
		t.Errorf("без почты Label() = %q", got)
	}
	// Только почта — участник добавлялся до того, как появилось поле имени.
	m = &ProjectMember{Email: "ivan@example.com"}
	if got := m.Label(); got != "ivan@example.com" {
		t.Errorf("без имени Label() = %q", got)
	}
	// Ничего нет — пустая строка, а не паника: так выглядит непрочитанный
	// участник в ответе списка.
	m = &ProjectMember{}
	if got := m.Label(); got != "" {
		t.Errorf("пустой участник дал Label() = %q", got)
	}
}
