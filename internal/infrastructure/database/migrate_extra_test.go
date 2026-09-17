package database

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrateEmptyDir(t *testing.T) {
	if err := Migrate(context.Background(), nil, "", "up"); err == nil {
		t.Fatal("want error for empty dir")
	}
	if err := MigrateWithTable(context.Background(), nil, "", "up", ""); err == nil {
		t.Fatal("want error")
	}
}

func TestGetMigrationVersionIntegration(t *testing.T) {
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	// ensure migrated
	_ = Migrate(context.Background(), pool, "../../../migrations", "up")
	v, err := GetMigrationVersion(pool, "../../../migrations")
	if err != nil {
		t.Fatalf("GetMigrationVersion: %v", err)
	}
	if v.Version == 0 {
		t.Fatal("want non-zero version")
	}
	v2, err := GetMigrationVersionWithTable(pool, "../../../migrations", "schema_migrations")
	if err != nil || v2.Version == 0 {
		t.Fatalf("GetMigrationVersionWithTable: %v %v", v2, err)
	}
}

func TestMigrateToVersionIntegration(t *testing.T) {
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	if err := MigrateToVersion(context.Background(), pool, "../../../migrations", 1); err != nil {
		t.Fatalf("MigrateToVersion 1: %v", err)
	}
	// back to latest
	if err := Migrate(context.Background(), pool, "../../../migrations", "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
}

func TestNewMigrateInstanceEmptyDir(t *testing.T) {
	if _, err := newMigrateInstance(nil, "", "schema_migrations"); err == nil {
		t.Fatal("want error")
	}
}
