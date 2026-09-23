package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testDBLockKey — ключ сессионного advisory lock, сериализующего прогоны
// database-пакета против общей stair_test. Без него два параллельных
// `go test ./...` оставляли «Dirty database version N» (S-120, S-137).
const testDBLockKey = 0x53743133 // "St13"

// TestMain — гигиена shared БД (S-137). Прод-миграции и down-пути не
// трогаются (S-136 закрыт), reset работает тем же мигратором, что и прод:
//   - серийный guard: advisory lock занят другим прогоном → понятная ошибка
//     вместо порчи общей БД; lock удерживается на всём прогоне;
//   - полный сброс (down+up): данные и грязные флаги предыдущих прогонов
//     удаляются до первого теста. Без сброса повторный прогон падал на up-10
//     (CREATE UNIQUE INDEX (project_id, revision)) поверх строк прогона №1,
//     оставшихся в stair_configurations после down-цепочки
//     TestMigrateToVersionIntegration (down до версии 1, а не 0: таблицы
//     миграции 000001 не дропаются) → dirty version 10 → красный сьют
//     на несвежей БД.
func TestMain(m *testing.M) {
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		// Без энва тесты сами скипаются; TestMain не должен им мешать.
		os.Exit(m.Run())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Серийный guard: один прогон против stair_test за раз. Session-level
	// lock снимается закрытием соединения ПОСЛЕ m.Run() — иначе второй
	// параллельный прогон мог бы сбросить БД во время чужих тестов.
	lock, err := pgx.Connect(ctx, url)
	if err != nil {
		log.Fatalf("database tests: connect for advisory lock: %v", err)
	}
	var got bool
	if err := lock.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", testDBLockKey).Scan(&got); err != nil {
		_ = lock.Close(ctx)
		log.Fatalf("database tests: advisory lock: %v", err)
	}
	if !got {
		_ = lock.Close(ctx)
		log.Fatalf("database tests: another test run already holds stair_test (advisory lock %#x); run go test serially", testDBLockKey)
	}

	if err := resetTestDatabase(ctx, url); err != nil {
		_ = lock.Close(ctx)
		log.Fatalf("database tests: reset stair_test: %v", err)
	}

	code := m.Run()
	_ = lock.Close(ctx) // снимает сессионный advisory lock
	os.Exit(code)
}

// resetTestDatabase приводит stair_test к детерминированному состоянию:
// последняя версия миграций, чистая схема, без dirty.
func resetTestDatabase(ctx context.Context, url string) error {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return fmt.Errorf("pool: %w", err)
	}
	defer pool.Close()

	// Свежая БД (DROP/CREATE): schema_migrations ещё нет — down-фаза не
	// нужна, сразу up. Иначе golang-migrate считает отсутствие таблицы
	// версий ошибкой, а не «nil version».
	var migrated bool
	if err := pool.QueryRow(ctx,
		"SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&migrated); err != nil {
		return fmt.Errorf("check schema_migrations: %w", err)
	}
	if migrated {
		m, err := newMigrateInstance(pool, testMigrationsDir, "schema_migrations")
		if err != nil {
			return err
		}

		// Self-heal: если предыдущий прогон упал на середине миграции (dirty),
		// force на ФАКТИЧЕСКУЮ dirty-версию снимает флаг, чтобы down мог
		// пройти до нуля. Force на «latest» был бы ошибкой: версии выше
		// dirty-версии ещё не применялись.
		version, dirty, err := m.Version()
		if err != nil && err != migrate.ErrNilVersion {
			_, _ = m.Close()
			return fmt.Errorf("version: %w", err)
		}
		if dirty {
			if ferr := m.Force(int(version)); ferr != nil {
				_, _ = m.Close()
				return fmt.Errorf("force %d: %w", version, ferr)
			}
		}

		// down до нуля (дропает таблицы миграции 000001 — данные предыдущих
		// прогонов удаляются), затем up до последней версии: чистая схема.
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			// Страховка от полусобранной схемы (dirty при упавшем up):
			// транзакционная откатка могла оставить состояние, которое не
			// проходит down. stair_test — ТЕСТОВАЯ БД: пересоздаём схему и
			// заново применяем up свежим инстансом (кэш версий старого устарел).
			if derr := dropAndRecreateSchema(ctx, pool); derr != nil {
				_, _ = m.Close()
				return fmt.Errorf("reset down: %v (schema reset: %v)", err, derr)
			}
			m2, merr := newMigrateInstance(pool, testMigrationsDir, "schema_migrations")
			_, _ = m.Close()
			return upAndVerify(ctx, pool, m2, merr)
		}
		_, _ = m.Close()
		return upAndVerify(ctx, pool, nil, nil)
	}
	return upAndVerify(ctx, pool, nil, nil)
}

// upAndVerify применяет миграции до последней версии (инстанс m может быть
// nil — тогда создаётся новый) и проверяет итоговое состояние schema_migrations.
func upAndVerify(ctx context.Context, pool *pgxpool.Pool, m *migrate.Migrate, merr error) error {
	// merr и m приходят парой из fallback-ветки resetTestDatabase.
	if merr != nil {
		return merr
	}
	if m == nil {
		var err error
		m, err = newMigrateInstance(pool, testMigrationsDir, "schema_migrations")
		if err != nil {
			return err
		}
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("reset up: %w", err)
	}

	var gotVersion int
	var gotDirty bool
	if err := pool.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations").Scan(&gotVersion, &gotDirty); err != nil {
		return fmt.Errorf("verify reset: %w", err)
	}
	if gotDirty || gotVersion == 0 {
		return fmt.Errorf("verify reset: unexpected state version=%d dirty=%v", gotVersion, gotDirty)
	}
	return nil
}

// dropAndRecreateSchema — полный сброс схемы stair_test (только тест-хелпер;
// прод-миграции не затрагиваются).
func dropAndRecreateSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE"); err != nil {
		return fmt.Errorf("drop schema: %w", err)
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA public"); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

// testMigrationsDir — каталог миграций относительно пакета, без *testing.T.
const testMigrationsDir = "../../../migrations"
