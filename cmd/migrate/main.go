// Команда migrate применяет/откатывает миграции БД (DEV-0009).
// Примеры:
//
//	STAIR_DATABASE_URL="postgres://..." go run ./cmd/migrate -dir migrations
//	STAIR_DATABASE_URL="postgres://..." go run ./cmd/migrate -dir migrations -down
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"stairplatform/internal/infrastructure/database"
)

func main() {
	dir := flag.String("dir", "migrations", "каталог с SQL-миграциями")
	down := flag.Bool("down", false, "откатить миграции вместо применения")
	table := flag.String("table", "", "таблица версий golang-migrate (пусто — schema_migrations)")
	databaseURL := flag.String("database", os.Getenv("STAIR_DATABASE_URL"), "URL подключения к PostgreSQL")
	flag.Parse()

	if *databaseURL == "" {
		log.Fatal("STAIR_DATABASE_URL must be set (or -database)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, database.DefaultConfig(*databaseURL))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	direction := "up"
	if *down {
		direction = "down"
	}
	if err := database.MigrateWithTable(ctx, pool, *dir, direction, *table); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("migrations %s applied successfully", direction)
}
