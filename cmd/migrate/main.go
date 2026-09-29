// Команда migrate применяет/откатывает миграции БД (DEV-0009).
//
//	STAIR_DATABASE_URL="postgres://..." go run ./cmd/migrate -dir migrations
//	STAIR_DATABASE_URL="postgres://..." go run ./cmd/migrate -dir migrations -steps 1
//	STAIR_DATABASE_URL="postgres://..." go run ./cmd/migrate -dir migrations -preflight
//	STAIR_DATABASE_URL="postgres://..." go run ./cmd/migrate -dir migrations -down -steps 1 -yes
//
// DB-001 (forensic 2026-09-27): -preflight, -force и -steps.
//
// Раньше откат требовал одного флага -down, который откатывал ВСЕ миграции
// до нуля, а восстановить «грязную» версию было нечем: golang-migrate
// помечает версию dirty ДО выполнения файла, поэтому падение на
// legacy-данных делало `migrate up` неисполнимым навсегда, и cmd/api
// (вызывающий Migrate при каждом старте) переставал подниматься.
// -preflight показывает проблему ДО того, как она станет необратимой,
// -force даёт штатный выход, -steps ограничивает шаг.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/infrastructure/database"
	"stairplatform/internal/infrastructure/database/migguard"
)

func main() {
	dir := flag.String("dir", "migrations", "каталог с SQL-миграциями")
	down := flag.Bool("down", false, "откатить миграции вместо применения")
	table := flag.String("table", "", "таблица версий golang-migrate (пусто — schema_migrations)")
	databaseURL := flag.String("database", os.Getenv("STAIR_DATABASE_URL"), "URL подключения к PostgreSQL")

	// DB-001: безопасность миграций.
	preflight := flag.Bool("preflight", false,
		"read-only проверка данных против ограничений будущих миграций; ничего не меняет")
	forceVersion := flag.Int("force", -1,
		"принудительно проставить версию (восстановление после dirty); -1 = выключено")
	steps := flag.Int("steps", 0, "применить/откатить не более N миграций (0 = все)")
	assumeYes := flag.Bool("yes", false, "подтверждение необратимых операций (обязательно для -down и -force)")

	flag.Parse()

	if *databaseURL == "" {
		log.Fatal("STAIR_DATABASE_URL must be set (or -database)")
	}
	// -down откатывает до нуля и необратим: часть down-миграций удаляет
	// данные (000018_orders_kind.down.sql — DELETE ... WHERE kind='consultation').
	// Один флаг без подтверждения — это путь к тихой потере лидов по опечатке.
	if *down && !*assumeYes {
		log.Fatal("-down откатывает миграции (часть down-миграций удаляет данные) и необратим.\n" +
			"  Повторите с -yes, если это намеренно. Для безопасного отката одной миграции: -down -steps 1 -yes")
	}
	if *forceVersion >= 0 && !*assumeYes {
		log.Fatal("-force принудительно меняет версию схемы и требует -yes.\n" +
			"  Использовать только после preflight и ручного устранения нарушений.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := database.Connect(ctx, database.DefaultConfig(*databaseURL))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Preflight идёт ДО любых изменений: его единственная задача — дать
	// оператору увидеть нарушающие данные, пока версия ещё чистая.
	if *preflight {
		runPreflight(ctx, pool)
		return
	}

	// -force: снять dirty-отметку. Идемпотентно ставит версию и больше
	// ничего не делает — применение миграций остаётся отдельной операцией.
	if *forceVersion >= 0 {
		if err := forceVersionNow(ctx, pool, *table, *forceVersion); err != nil {
			log.Fatalf("force: %v", err)
		}
		return
	}

	direction := "up"
	if *down {
		direction = "down"
	}

	if err := database.MigrateWithTableSteps(ctx, pool, *dir, direction, *table, *steps); err != nil {
		// DB-001: dirty-состояние — это не «миграция не прошла», а
		// «миграция не пройдёт, пока не снимут отметку». Сообщение должно
		// называть команду, иначе оператор идёт в SQL руками.
		if errors.Is(err, migguard.ErrDirtyState) {
			log.Fatalf("%v\n\n  Диагностика (read-only):\n    go run ./cmd/migrate -preflight\n"+
				"  Восстановление после устранения нарушений:\n    go run ./cmd/migrate -force <version> -yes",
				err)
		}
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("migrations %s applied successfully", direction)
}

// runPreflight печатает отчёт и завершает процесс кодом 1 при нарушениях.
func runPreflight(ctx context.Context, pool *pgxpool.Pool) {
	rep, err := migguard.Preflight(ctx, pool)
	if err != nil {
		log.Fatalf("preflight: %v", err)
	}
	fmt.Print(rep.String())
	if rep.Failed() {
		log.Fatalf("preflight: %d нарушений — миграция будет отвергнута (схема останется чистой)", len(rep.Violations))
	}
}

// forceVersionNow ставит версию и снимает dirty-отметку напрямую.
//
// Обращаемся к таблице версий обычным SQL, а не через golang-migrate:
// его API force есть только у драйвера-специфичного экземпляра, и
// дублировать его логику здесь рискованнее, чем выполнить один UPDATE.
func forceVersionNow(ctx context.Context, pool *pgxpool.Pool, table string, version int) error {
	if table == "" {
		table = "schema_migrations"
	}
	if !validVersionTable(table) {
		return fmt.Errorf("недопустимое имя таблицы версий: %q", table)
	}
	tag, err := pool.Exec(ctx,
		fmt.Sprintf(`UPDATE %s SET version = $1, dirty = false`, table), version)
	if err != nil {
		return fmt.Errorf("проставить версию %d: %w", version, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("таблица %s пуста — нечего восстанавливать (или её нет)", table)
	}
	log.Printf("версия схемы принудительно выставлена в %d, dirty снята", version)
	log.Printf("  Следующий шаг: go run ./cmd/migrate -dir migrations   (или -down -steps 1 -yes)")
	return nil
}

// validVersionTable — таблица версий подставляется в SQL, поэтому имя
// проверяется по белому списку, а не экранируется.
func validVersionTable(t string) bool {
	for _, ok := range []string{"schema_migrations", "schema_migrations_seeds"} {
		if t == ok {
			return true
		}
	}
	return false
}
