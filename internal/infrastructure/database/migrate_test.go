package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
)

// testURL возвращает URL тестовой БД или "" (тест пропускается).
// Миграции применяются к изолированной схеме, чтобы не мешать dev-БД.
func testURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	return url
}

func TestMigrationsApplyAndRollback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url := testURL(t)
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Откат к чистому состоянию перед тестом.
	if err := Migrate(ctx, pool, migrationsDir(t), "down"); err != nil {
		t.Fatalf("clean down: %v", err)
	}

	// Up — должны появиться таблицы.
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("up: %v", err)
	}
	var version uint
	if err := pool.QueryRow(ctx, "select max(version) from schema_migrations").Scan(&version); err != nil {
		t.Fatalf("version: %v", err)
	}
	if version == 0 {
		t.Fatal("expected at least one applied migration")
	}

	// Повторный up — no change (идемпотентность).
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("second up must be no-change: %v", err)
	}

	// Down — таблицы удалены.
	if err := Migrate(ctx, pool, migrationsDir(t), "down"); err != nil {
		t.Fatalf("down: %v", err)
	}
	var n int
	// schema_migrations остаётся (управляется мигратором); приложенные таблицы удалены.
	if err := pool.QueryRow(ctx,
		"select count(*) from information_schema.tables where table_schema='public' and table_name='projects'").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("projects table must be dropped after down, found %d", n)
	}

	// Восстанавливаем для dev-окружения.
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("restore up: %v", err)
	}
}

func TestMigrateInvalidDirection(t *testing.T) {
	ctx := context.Background()
	url := testURL(t)
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool, migrationsDir(t), "sideways"); err == nil {
		t.Fatal("invalid direction must be rejected")
	}
}

// migrationsDir — каталог миграций относительно пакета.
func migrationsDir(t *testing.T) string {
	t.Helper()
	return "../../../migrations"
}

var _ = migrate.ErrNoChange
