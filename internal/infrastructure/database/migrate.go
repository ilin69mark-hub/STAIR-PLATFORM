package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/infrastructure/database/migguard"
)

// MigrationVersion — информация о текущей версии миграций.
type MigrationVersion struct {
	Version  int
	Dirty    bool
	Sequence int
}

// Migrate применяет миграции из каталога dir (файлы *.up.sql/*.down.sql).
// direction: "up" — применить все; "down" — откатить все (до нуля).
func Migrate(ctx context.Context, pool *pgxpool.Pool, dir, direction string) error {
	return MigrateWithTable(ctx, pool, dir, direction, "schema_migrations")
}

// MigrateWithTable то же, что Migrate, но с произвольным именем таблицы
// версий golang-migrate. Требуется для каталогов-сателлитов (seeds):
// они используют собственную таблицу версий и не конфликтуют с основными
// миграциями в schema_migrations.
func MigrateWithTable(ctx context.Context, pool *pgxpool.Pool, dir, direction, table string) error {
	if dir == "" {
		return fmt.Errorf("database: migrations dir is required")
	}
	if table == "" {
		table = "schema_migrations"
	}

	start := time.Now()
	m, err := newMigrateInstance(pool, dir, table)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	// DB-001: dirty-состояние отвергаем ДО попытки шага. Без этой проверки
	// m.Up() вернул бы сырое «Dirty database version 31», и вызывающий не
	// знал бы, что делать.
	if err := rejectDirty(m); err != nil {
		return err
	}

	switch direction {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("database: migrate up: %w", err)
		}
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("database: migrate down: %w", err)
		}
	default:
		return fmt.Errorf("database: unknown migration direction %q", direction)
	}

	slog.Info("database migration completed",
		"direction", direction,
		"dir", dir,
		"table", table,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// rejectDirty превращает «грязную» версию в типизированную ошибку.
//
// DB-001 (2026-09-27): golang-migrate помечает версию dirty ДО выполнения
// файла. Любой отказ внутри миграции (чаще всего — жёсткое ограничение на
// legacy-данных, вроде CHECK из 000031) оставляет базу в состоянии, из
// которого `migrate up` больше не выйдет. Сырое «Dirty database version 31»
// не говорит оператору, что делать, поэтому возвращаем ошибку с командами:
// диагностика (-preflight) и восстановление (-force).
func rejectDirty(m *migrate.Migrate) error {
	v, dirty, err := m.Version()
	// ErrNilVersion — свежая БД без таблицы версий; dirty там невозможен.
	if err != nil || !dirty {
		return nil
	}
	return fmt.Errorf("%w: version %d помечена dirty; обычная миграция невозможна. "+
		"Диагностика (read-only): cmd/migrate -preflight. "+
		"Восстановление после устранения нарушений: cmd/migrate -force %d -yes",
		migguard.ErrDirtyState, v, v)
}

// MigrateWithTableSteps то же, что MigrateWithTable, но ограничивает число
// применённых/откатанных миграций (steps <= 0 — без ограничения).
//
// DB-001 (2026-09-27): ограничение шагов добавлено вместе с -down -yes.
// Раньше -down откатывал ВСЕ миграции до нуля одним флагом, а часть
// down-миграций удаляет данные, — то есть откат «на одну миграцию» был
// невозможен в принципе.
func MigrateWithTableSteps(ctx context.Context, pool *pgxpool.Pool, dir, direction, table string, steps int) error {
	if steps <= 0 {
		return MigrateWithTable(ctx, pool, dir, direction, table)
	}
	if dir == "" {
		return fmt.Errorf("database: migrations dir is required")
	}
	if table == "" {
		table = "schema_migrations"
	}

	start := time.Now()
	m, err := newMigrateInstance(pool, dir, table)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	if err := rejectDirty(m); err != nil {
		return err
	}

	var applyErr error
	switch direction {
	case "up":
		applyErr = m.Steps(steps)
	case "down":
		applyErr = m.Steps(-steps)
	default:
		return fmt.Errorf("database: unknown migration direction %q", direction)
	}
	if applyErr != nil && !errors.Is(applyErr, migrate.ErrNoChange) {
		return fmt.Errorf("database: migrate %s by %d step(s): %w", direction, steps, applyErr)
	}

	slog.Info("database migration completed",
		"direction", direction, "steps", steps,
		"dir", dir, "table", table,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// GetMigrationVersion возвращает текущую версию миграций.
func GetMigrationVersion(pool *pgxpool.Pool, dir string) (*MigrationVersion, error) {
	return GetMigrationVersionWithTable(pool, dir, "schema_migrations")
}

// GetMigrationVersionWithTable то же, что GetMigrationVersion, но для
// произвольной таблицы версий (seeds и другие сателлитные каталоги).
func GetMigrationVersionWithTable(pool *pgxpool.Pool, dir, table string) (*MigrationVersion, error) {
	m, err := newMigrateInstance(pool, dir, table)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = m.Close() }()

	version, dirty, err := m.Version()
	if err != nil {
		if err == migrate.ErrNilVersion {
			return &MigrationVersion{Version: 0, Dirty: false}, nil
		}
		return nil, fmt.Errorf("database: get version: %w", err)
	}

	return &MigrationVersion{
		Version:  int(version),
		Dirty:    dirty,
		Sequence: int(version),
	}, nil
}

// MigrateToVersion применяет миграции до конкретной версии.
func MigrateToVersion(ctx context.Context, pool *pgxpool.Pool, dir string, version uint) error {
	start := time.Now()
	m, err := newMigrateInstance(pool, dir, "schema_migrations")
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	if err := rejectDirty(m); err != nil {
		return err
	}

	if err := m.Migrate(version); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("database: migrate to version %d: %w", version, err)
	}

	slog.Info("database migration to version completed",
		"version", version,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

func newMigrateInstance(pool *pgxpool.Pool, dir, table string) (*migrate.Migrate, error) {
	if dir == "" {
		return nil, fmt.Errorf("database: migrations dir is required")
	}

	cc := pool.Config().ConnConfig
	query := ""
	if cc.TLSConfig == nil {
		query = "?sslmode=disable"
	}

	dbURL, err := url.Parse(fmt.Sprintf("pgx5://%s@%s:%d/%s%s",
		url.UserPassword(cc.User, cc.Password).String(), cc.Host, cc.Port, cc.Database, query))
	if err != nil {
		return nil, fmt.Errorf("database: parse db url: %w", err)
	}
	q := dbURL.Query()
	q.Set("x-migrations-table", fmt.Sprintf("%q", table))
	q.Set("x-migrations-table-quoted", "true")
	dbURL.RawQuery = q.Encode()

	m, err := migrate.New("file://"+dir, dbURL.String())
	if err != nil {
		return nil, fmt.Errorf("database: migrate init: %w", err)
	}
	return m, nil
}
