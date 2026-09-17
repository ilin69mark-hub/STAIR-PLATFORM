package database

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
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
