package database

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/infrastructure/database/migguard"
)

// DB-1 (forensic 2026-09-27) — регрессия на необратимое dirty-состояние.
//
// ПРОБЛЕМА. golang-migrate помечает версию dirty=true ДО выполнения файла.
// Любой отказ внутри миграции (чаще всего жёсткий CHECK на legacy-данных)
// оставляет базу в состоянии, из которого `migrate up` больше не выходит.
// Восстановления у проекта не было: в cmd/migrate не было ни -force, ни
// -steps, а cmd/api вызывал Migrate(up) при старте и делал os.Exit(1).
// Итог: одна строка старых данных навсегда блокировала подъём приложения.
//
// ЧТО ПРОВЕРЯЕМ. Три независимые меры:
//  1. preflight находит нарушение, НЕ трогая версию схемы;
//  2. dirty-состояние распознаётся типизированной ошибкой, а не сырым текстом;
//  3. после снятия dirty миграция доходит до конца.
//
// Требует STAIR_TEST_DATABASE_URL; без него тест скипается.

// TestDB001_PreflightDetectsViolationWithoutDirtying — ключевое свойство
// preflight: он read-only. Если бы он пачкал версию, диагностика сама
// стала бы источником необратимого состояния.
func TestDB001_PreflightDetectsViolationWithoutDirtying(t *testing.T) {
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	before, err := GetMigrationVersion(pool, migrationsDir(t))
	if err != nil {
		t.Fatalf("version before: %v", err)
	}
	if before.Dirty {
		t.Fatalf("precondition: schema must be clean, got version=%d dirty", before.Version)
	}

	// Воспроизводим legacy-состояние: ограничения ещё нет, данные уже есть.
	// Именно так выглядит БД, до которой дошла миграция 000031. Снимаем
	// CHECK, кладём плохую строку, проверяем preflight — он обязан её найти,
	// НЕ пачкая версию схемы.
	for _, c := range []string{"stair_configurations_flight_check", "stair_configurations_positive_geometry"} {
		if _, err := pool.Exec(ctx,
			`ALTER TABLE stair_configurations DROP CONSTRAINT IF EXISTS `+c); err != nil {
			t.Fatalf("drop %s: %v", c, err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `ALTER TABLE stair_configurations
			ADD CONSTRAINT stair_configurations_flight_check
			CHECK (flight IN ('straight','l_shape','u_shape','spiral'))`)
		_, _ = pool.Exec(ctx, `ALTER TABLE stair_configurations
			ADD CONSTRAINT stair_configurations_positive_geometry CHECK (
				width_mm > 0 AND height_mm > 0 AND step_height_mm > 0
				AND stringer_thickness_mm > 0 AND step_thickness_mm > 0
				AND clearance_mm >= 0 AND railing_height_mm >= 0
				AND comfort_step_mm >= 0 AND revision >= 1)`)
	})

	tenant, projectID, ownerID := insertLegacyConfig(t, ctx, pool, "db001-preflight")
	_ = tenant
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM stair_configurations WHERE id = $1`, legacyConfigID)
		_, _ = pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, projectID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, legacyTenantID)
	})

	rep, err := migguard.Preflight(ctx, pool)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if !rep.Failed() {
		t.Fatal("preflight must find the violating row")
	}
	var found bool
	for _, v := range rep.Violations {
		if v.Constraint == "stair_configurations_positive_geometry" || v.Constraint == "stair_configurations_flight_check" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a stair_configurations violation, got: %+v", rep.Violations)
	}
	if got := rep.String(); len(got) < 40 {
		t.Errorf("report must be actionable, got %q", got)
	}

	// ГЛАВНОЕ: версия схемы не тронута.
	after, err := GetMigrationVersion(pool, migrationsDir(t))
	if err != nil {
		t.Fatalf("version after: %v", err)
	}
	if after.Dirty {
		t.Fatal("preflight dirtied the schema — диагностика не должна быть необратимой")
	}
	if after.Version != before.Version {
		t.Errorf("preflight changed version %d -> %d", before.Version, after.Version)
	}
}

// TestDB001_CleanDataPassesPreflight — на нормальных данных preflight молчит.
// Иначе диагностика превратится в шум и перестанут читать её вывод.
func TestDB001_CleanDataPassesPreflight(t *testing.T) {
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	rep, err := migguard.Preflight(ctx, pool)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if rep.Failed() {
		t.Fatalf("clean data must pass preflight, found: %+v", rep.Violations)
	}
}

// TestDB001_PreflightRulesMatchMigration — правила preflight обязаны
// соответствовать ограничениям из текста миграции. Расхождение означало бы
// диагностику, которая врёт: нашла бы не то, что отвергнет миграция.
func TestDB001_PreflightRulesMatchMigration(t *testing.T) {
	// Правила preflight зеркалят ограничения 000029 и 000031. Расхождение
	// означало бы диагностику, которая врёт: нашла бы не то, что отвергнет
	// миграция.
	up := readMigrationFile(t, "000031_integrity_constraints.up.sql")
	a29 := readMigrationFile(t, "000029_audit_action_allowlist.up.sql")
	for _, c := range migguard.Constraints() {
		switch {
		case strings.Contains(a29, c):
			// Ограничение живёт в 000029 — ок.
		case strings.Contains(up, c):
			// Ограничение живёт в 000031 — ок.
		default:
			t.Errorf("migguard checks %q, but neither 000029 nor 000031 declares it", c)
		}
	}
}

// TestDB001_DirtyStateIsTypedError — «грязная» версия обязана опознаваться
// типизированной ошибкой, иначе вызывающий не сможет отличить её от сбоя БД
// и напечатает бесполезное «database migrate failed».
func TestDB001_DirtyStateIsTypedError(t *testing.T) {
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	v, err := GetMigrationVersion(pool, migrationsDir(t))
	if err != nil || v.Version < 1 {
		t.Skipf("no migrations applied (version=%d err=%v)", v.Version, err)
	}

	// Ставим dirty вручную (это то, что делает golang-migrate при отказе).
	if _, err := pool.Exec(ctx, `UPDATE schema_migrations SET dirty = true`); err != nil {
		t.Fatalf("set dirty: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `UPDATE schema_migrations SET dirty = false`)
	})

	for _, tc := range []struct {
		name string
		fn   func() error
	}{
		{"Migrate", func() error { return Migrate(ctx, pool, migrationsDir(t), "up") }},
		{"MigrateWithTable", func() error {
			return MigrateWithTable(ctx, pool, migrationsDir(t), "up", "schema_migrations")
		}},
		{"MigrateWithTableSteps", func() error {
			return MigrateWithTableSteps(ctx, pool, migrationsDir(t), "up", "schema_migrations", 1)
		}},
		{"MigrateToVersion", func() error {
			return MigrateToVersion(ctx, pool, migrationsDir(t), uint(v.Version))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			if err == nil {
				t.Fatal("dirty schema must be rejected")
			}
			if !errors.Is(err, migguard.ErrDirtyState) {
				t.Errorf("want migguard.ErrDirtyState, got %v", err)
			}
			// Сообщение обязано называть команды, иначе оператор не знает, что делать.
			for _, want := range []string{"-preflight", "-force"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error must mention %s: %s", want, err.Error())
				}
			}
		})
	}
}

// ---- вспомогательное для DB001 ----

const (
	legacyTenantID = "00000000-0000-0000-0000-0db100010001"
	legacyUserID   = "00000000-0000-0000-0000-0db100010002"
	legacyProject  = "00000000-0000-0000-0000-0db100010003"
	legacyConfigID = "00000000-0000-0000-0000-0db100010004"
)

// insertLegacyConfig создаёт связку tenant→user→project→config с заведомо
// нарушающей геометрией (width < 0 и недопустимый тип марша).
func insertLegacyConfig(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tag string) (tenant, project, owner string) {
	t.Helper()
	tenant, project, owner = legacyTenantID, legacyProject, legacyUserID
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id,name,slug) VALUES ($1,$2,$3)
		 ON CONFLICT (id) DO NOTHING`, tenant, "Legacy "+tag, "legacy-"+tag); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id,tenant_id,email,name,password_hash,role,status)
		 VALUES ($1,$2,$3,$4,$5,'user','active') ON CONFLICT (id) DO NOTHING`,
		owner, tenant, "db001-"+tag+"@test.dev", "L", "x"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO projects (id,tenant_id,owner_id,name,description,status)
		 VALUES ($1,$2,$3,$4,$5,'draft') ON CONFLICT (id) DO NOTHING`,
		project, tenant, owner, "DB-001 "+tag, "d"); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO stair_configurations
		   (id,project_id,width_mm,height_mm,flight,step_height_mm,
		    stringer_thickness_mm,step_thickness_mm,clearance_mm,railing_height_mm,comfort_step_mm)
		 VALUES ($1,$2,-100,2700,'spiral-v2',180,50,40,0,950,300)
		 ON CONFLICT (id) DO NOTHING`, legacyConfigID, project); err != nil {
		t.Fatalf("insert config: %v", err)
	}
	return tenant, project, owner
}
