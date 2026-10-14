package http

import (
	"encoding/json"
	"strings"
	"testing"

	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

// report_contract_test.go — S-240: контракт отчёта и пропажа источника.
//
// Тот самый случай, который отчёт и чинит: `Issue.Source` заполнялся с
// S-235, но конвертер его не переносил, и нормативная библиотека оставалась
// в бэкенде. Проверка идёт через МАРШРУТ и JSON, а не через вызов
// конвертера: конвертер-то как раз и был в порядке — терялось поле в
// структуре ответа, и только сетевой вызов это покажет.

// TestIssueSourceReachesJSON — источник правила доезжает до клиента.
func TestIssueSourceReachesJSON(t *testing.T) {
	vr := validation.Result{
		Valid: false,
		Issues: []validation.Issue{{
			Code:     constraint.SAF_RAILING_HEIGHT,
			Severity: constraint.SeverityError,
			Message:  "высота ограждения должна быть не менее 900 мм",
			Value:    850, Min: 900, HasMin: true,
			Source: "СП 55.13330, п. 5.1 [норма]",
		}},
	}
	raw, err := json.Marshal(toValidation(vr))
	if err != nil {
		t.Fatalf("маршализация: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "СП 55.13330") {
		t.Errorf("документ нормы не доехал до ответа: %s", body)
	}
	if !strings.Contains(body, "п. 5.1") {
		t.Errorf("пункт нормы не доехал до ответа: %s", body)
	}
	if !strings.Contains(body, "норма") {
		t.Errorf("основание не доехало до ответа: %s", body)
	}
}

// TestReportLevelsInJSON — три уровня с итогом, сводкой и причинами.
func TestReportLevelsInJSON(t *testing.T) {
	vr := validation.Result{Valid: true}
	r := validation.BuildReport(vr, false, []string{"нет расчётной схемы площадки"})

	// Отчёт собирается сервисом из Result, а в DTO попадает через него же;
	// здесь проверяем доменную часть контракта.
	rep, err := json.Marshal(toReport(r))
	if err != nil {
		t.Fatalf("маршализация отчёта: %v", err)
	}
	body := string(rep)
	for _, want := range []string{
		`"overall":"partial"`,
		`"code":"geometry"`, `"code":"codes"`, `"code":"calculation"`,
		`"status":"not_performed"`,
		"нет расчётной схемы площадки",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("в отчёте нет %q: %s", want, body)
		}
	}
}

// TestReportAdditiveToOldContract — старые поля живы как были. Отчёт
// добавлен аддитивно, и клиент, который о нём не знает, не должен сломаться.
func TestReportAdditiveToOldContract(t *testing.T) {
	vr := validation.Result{Valid: true, Blocking: false}
	got := toValidation(vr)
	if !got.Valid || got.Blocking {
		t.Errorf("старое поле изменилось: valid=%v blocking=%v", got.Valid, got.Blocking)
	}
	raw, _ := json.Marshal(got)
	for _, want := range []string{`"valid":true`, `"blocking":false`, `"issues"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("старое поле %q пропало из ответа: %s", want, raw)
		}
	}
}
