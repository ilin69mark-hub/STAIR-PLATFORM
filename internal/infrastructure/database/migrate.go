package database

import (
	"context"
	"fmt"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate применяет миграции из каталога dir (файлы *.up.sql/*.down.sql).
// direction: "up" — применить все; "down" — откатить все (до нуля).
func Migrate(ctx context.Context, pool *pgxpool.Pool, dir, direction string) error {
	if dir == "" {
		return fmt.Errorf("database: migrations dir is required")
	}

	// golang-migrate ожидает postgres://-URL; восстанавливаем из частей
	// подключения пула в URL-форме (ConnString даёт keyword/value).
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
		return fmt.Errorf("database: migrate init: %w", err)
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
	return nil
}
