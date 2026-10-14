package database

import (
	"context"
	"testing"

	"stairplatform/internal/application/audit"
)

// AUDIT-002 (2026-09-27): забытое поле Result не должно «съедать» событие.
//
// Проверка на живой БД обязательна: в audit_events стоит CHECK
// audit_events_result_check (result IN ('ok','denied','failed')), то есть
// пустая строка отвергается и СОРОНОЙ валидации, и САМОЙ БД. Прежде это
// означало тихую потерю события (см. application-audit тест), теперь —
// запись с result='ok'.
//
// Без STAIR_TEST_DATABASE_URL тест скипается.

// c002CleanupProbe удаляет записанные пробы.
//
// audit_events — append-only таблица, но тесты делят ОДИН tenant 'default',
// и TestAuditInsertAndList проверяет точное число его событий. Оставленная
// проба ломает чужой тест, поэтому за собой убираем: иначе «моя запись
// создалась» доказывается ценой падения несвязанной проверки.
func c002CleanupProbe(t *testing.T, repo *ProjectRepository, details ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, d := range details {
			if _, err := repo.pool.Exec(context.Background(),
				`DELETE FROM audit_events WHERE detail = $1`, d); err != nil {
				t.Logf("cleanup probe %q: %v", d, err)
			}
		}
	})
}

// TestAUDIT002_ForgottenResultIsPersisted — сквозная проверка:
// Service.Record с незаполненным Result обязан реально записаться.
func TestAUDIT002_ForgottenResultIsPersisted(t *testing.T) {
	ar, repo := integrationAuditRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	c002CleanupProbe(t, repo, "AUDIT-002 forgot-result probe")

	svc := audit.NewService(ar)
	e := &audit.Event{
		TenantID: tenant,
		Action:   audit.ActionSettingsUpdated,
		Detail:   "AUDIT-002 forgot-result probe",
		// Result намеренно не задан.
	}
	if err := svc.Record(ctx, e); err != nil {
		t.Fatalf("AUDIT-002: событие с забытым Result не записалось: %v", err)
	}

	events, err := ar.ListByTenant(ctx, tenant, 50)
	if err != nil {
		t.Fatalf("ListByTenant: %v", err)
	}
	var found *audit.Event
	for _, ev := range events {
		if ev.Detail == "AUDIT-002 forgot-result probe" {
			found = ev
			break
		}
	}
	if found == nil {
		t.Fatal("событие отсутствует в audit_events — запись потеряна")
	}
	if found.Result != audit.ResultOK {
		t.Errorf("result = %q, want ok", found.Result)
	}
	if found.ID == "" {
		t.Error("id не возвращён — событие не было записано по-настоящему")
	}
}

// TestAUDIT002_RepositoryNormalizesForDirectCallers — страховка на прямых
// вызовах порта в обход Service (тесты, будущий код). Репозиторий обязан
// нормализовать Result сам, иначе пустое поле упало бы технической ошибкой
// CHECK (23514) вместо записи события — то есть прямая вставка потеряла бы
// событие так же, как раньше терял сервисный путь.
func TestAUDIT002_RepositoryNormalizesForDirectCallers(t *testing.T) {
	ar, repo := integrationAuditRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	c002CleanupProbe(t, repo, "AUDIT-002 empty-result probe")

	// Прямой вызов порта в обход Service: нормализации там нет.
	e := &audit.Event{
		TenantID: tenant,
		Action:   audit.ActionSettingsUpdated,
		Detail:   "AUDIT-002 empty-result probe",
	}
	err := ar.Insert(ctx, e)
	if err != nil {
		t.Fatalf("Insert обязан нормализовать пустой Result, получено %v", err)
	}
	if e.Result != audit.ResultOK {
		t.Errorf("после Insert result = %q, want ok (репозиторий нормализует)", e.Result)
	}
}

// TestAUDIT002_GarbageResultStillRejectedEndToEnd — сквозная проверка
// строгости: непустой мусор обязан быть отвергнут и приложением, и БД.
func TestAUDIT002_GarbageResultStillRejectedEndToEnd(t *testing.T) {
	ar, repo := integrationAuditRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)

	svc := audit.NewService(ar)
	e := &audit.Event{
		TenantID: tenant,
		Action:   audit.ActionSettingsUpdated,
		Result:   audit.Result("teapot"),
	}
	if err := svc.Record(ctx, e); err == nil {
		t.Error("мусорный Result обязан отвергаться — иначе в журнал попадёт чушь")
	}
}
