package database

import (
	"context"
	"testing"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/project"
)

// TestNarrowingRollbackLosesNoRows — страж класса «откат сужающей CHECK
// миграции молча теряет строки» (инцидент 2026-10-07, INFRA-0028 п.2).
//
// Тогда откат 000041 выполняли вручную, мимо транзакции мигратора: резервная
// таблица и DELETE зафиксировались, а ADD CONSTRAINT упал — 8 событий
// остались только в audit_events_action_restore_backup. Тест повторяет
// сценарий на тестовой БД и требует:
//  1. сид: события из «возвращённого» словаря 000041;
//  2. откат на 2 шага (000042.down + 000041.down): строки уходят в резерв,
//     суммарно журнал+резерв совпадают с исходным числом (дельтами —
//     соседние тесты тоже пишут в журнал, абсолютные числа не годятся);
//  3. подъём: 000041 расширяет CHECK, 000042 возвращает строки и удаляет
//     резервную таблицу;
//  4. финал: журнал содержит все исходные строки, резервной таблицы нет,
//     версия та же, dirty=false.
func TestNarrowingRollbackLosesNoRows(t *testing.T) {
	ctx := context.Background()
	url := testURL(t)
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	// Hermetic: более ранние тесты пакета тоже пишут в журнал (в т.ч.
	// действия из сужаемого словаря). Чистим журнал и возможный резерв
	// сброшенного down-цикла, иначе 000041.down унесёт чужие строки и
	// дельты перестанут сходиться. Тестовая БД одноразовая (TestMain
	// делает то же самое перед прогоном), соседние тесты свои данные
	// создают сами и журнал не перечитывают.
	if _, err := pool.Exec(ctx, "DELETE FROM audit_events"); err != nil {
		t.Fatalf("pre-wipe journal: %v", err)
	}
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS audit_events_action_restore_backup"); err != nil {
		t.Fatalf("pre-wipe reserve: %v", err)
	}

	repo := NewProjectRepository(pool)
	ar := NewAuditRepository(pool)
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)
	p := &project.Project{Name: "Откат без потерь", Description: "тест", Status: project.StatusDraft}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	seedActions := []audit.Action{
		audit.ActionProjectModified, // 'project.modified'
		audit.ActionDataExported,    // 'data.exported'
		audit.ActionMemberAdded,     // 'member.added'
	}
	for _, a := range seedActions {
		e := &audit.Event{ActorID: owner, TenantID: tenant, ProjectID: p.ID, Action: a, Result: audit.ResultOK}
		if err := ar.Insert(ctx, e); err != nil {
			t.Fatalf("Insert %s: %v", a, err)
		}
	}

	count := func(table string) int {
		t.Helper()
		var n int
		var exists bool
		if err := pool.QueryRow(ctx,
			"SELECT to_regclass($1) IS NOT NULL", "public."+table).Scan(&exists); err != nil {
			t.Fatalf("to_regclass %s: %v", table, err)
		}
		if !exists {
			return 0
		}
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	journalBefore, backupBefore := count("audit_events"), count("audit_events_action_restore_backup")
	vBefore, err := GetMigrationVersion(pool, migrationsDir(t))
	if err != nil {
		t.Fatalf("version before: %v", err)
	}

	// Откат на 2 шага: 000042.down (пересоздаёт пустой резерв),
	// затем 000041.down (резерв+удаление сужаемых строк).
	if err := MigrateWithTableSteps(ctx, pool, migrationsDir(t), "down", "schema_migrations", 2); err != nil {
		t.Fatalf("steps down 2: %v", err)
	}
	if got := count("audit_events"); got != journalBefore-len(seedActions) {
		t.Fatalf("после отката в журнале %d, хотели %d", got, journalBefore-len(seedActions))
	}
	if got := count("audit_events_action_restore_backup"); got != backupBefore+len(seedActions) {
		t.Fatalf("в резерве %d, хотели %d", got, backupBefore+len(seedActions))
	}

	// Подъём: 000041 расширяет CHECK, 000042 возвращает строки и чистит резерв.
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if got := count("audit_events"); got != journalBefore {
		t.Fatalf("после подъёма в журнале %d, хотели %d — строки потеряны", got, journalBefore)
	}
	var backupExists bool
	if err := pool.QueryRow(ctx,
		"SELECT to_regclass('public.audit_events_action_restore_backup') IS NOT NULL").Scan(&backupExists); err != nil {
		t.Fatalf("backup exists: %v", err)
	}
	if backupExists {
		t.Fatal("резервная таблица пережила подъём — 000042 обязана её удалять")
	}
	vAfter, err := GetMigrationVersion(pool, migrationsDir(t))
	if err != nil {
		t.Fatalf("version after: %v", err)
	}
	if vAfter.Version != vBefore.Version || vAfter.Dirty {
		t.Fatalf("версия после цикла: %+v, была %+v", vAfter, vBefore)
	}
}
