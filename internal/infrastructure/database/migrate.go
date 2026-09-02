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
	if dir == "" {
		return fmt.Errorf("database: migrations dir is required")
	}

	start := time.Now()
	m, err := newMigrateInstance(pool, dir)
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
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// GetMigrationVersion возвращает текущую версию миграций.
func GetMigrationVersion(pool *pgxpool.Pool, dir string) (*MigrationVersion, error) {
	m, err := newMigrateInstance(pool, dir)
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
	m, err := newMigrateInstance(pool, dir)
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

func newMigrateInstance(pool *pgxpool.Pool, dir string) (*migrate.Migrate, error) {
	if dir == "" {
		return nil, fmt.Errorf("database: migrations dir is required")
	}

	cc := pool.Config().ConnConfig
	query := ""
	if cc.TLSConfig == nil {
		query = "?sslmode=disable"
	}
	user := url.UserPassword(cc.User, cc.Password).String()
	dbURL := fmt.Sprintf("pgx5://%s@%s:%d/%s%s",
		user, cc.Host, cc.Port, cc.Database, query)

	m, err := migrate.New("file://"+dir, dbURL)
	if err != nil {
		return nil, fmt.Errorf("database: migrate init: %w", err)
	}
	return m, nil
}
