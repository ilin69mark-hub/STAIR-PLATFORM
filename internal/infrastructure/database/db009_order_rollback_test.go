package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"stairplatform/internal/application/order"
)

// DB-9 (forensic 2026-09-27) — откат 000018 падал на данных, которые сама же
// миграция UP-18 разрешала.
//
// ЦЕПОЧКА ДЕФЕКТА. UP-18 снял NOT NULL с orders.user_id и orders.price_json,
// чтобы консультации могли быть анонимными, — но снял ДЛЯ ВСЕХ строк и не
// связал с `kind`. Обычный заказ получил право иметь NULL в обоих полях.
// Down-18 удалял только `kind = 'consultation'` и затем делал
// `SET NOT NULL` — то есть падал на любом заказе с пустой ценой:
//
//   ERROR: column "price_json" of relation "orders" contains null values
//
// Падение внутри миграции помечает версию dirty, после чего `migrate up` не
// выполняется ВООБЩЕ. А cmd/api вызывает Migrate при старте и завершается
// os.Exit(1) — то есть неудачный откат блокировал не только накат, но и
// подъём приложения.
//
// Тест повторяет именно эту последовательность на живой БД.

// TestDB009_RollbackSurvivesNullableOrderRows — главный сценарий: поднять
// схему до 000018, создать заказ с NULL price_json, откатить до 000017 и
// убедиться, что откат не падает и версия не пачкается.
func TestDB009_RollbackSurvivesNullableOrderRows(t *testing.T) {
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()

	// Изолированная БД, а не схема: golang-migrate (драйвер pgx) квалифицирует
	// свою таблицу версий как public.schema_migrations, поэтому search_path не
	// даёт изоляции — миграции применялись бы к общей базе. Отдельная БД
	// единственный способ проверить откат, не ломая общую.
	base, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(base.Close)

	dbName := fmt.Sprintf("db009_%d", uniqueSuffix())
	if _, err := base.Exec(ctx, `CREATE DATABASE `+dbName); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = base.Exec(context.Background(), `DROP DATABASE IF EXISTS `+dbName+` WITH (FORCE)`)
	})

	// Заменяем имя БД в URL (путь после хоста).
	idx := strings.Index(url[strings.Index(url, "://")+3:], "/")
	adminURL := url
	if idx >= 0 {
		rest := url[strings.Index(url, "://")+3:]
		host := rest[:idx]
		adminURL = url[:strings.Index(url, "://")+3] + host + "/" + dbName
		if q := strings.Index(rest, "?"); q >= 0 {
			adminURL += rest[q:]
		}
	}
	pool, err := Connect(ctx, DefaultConfig(adminURL))
	if err != nil {
		t.Fatalf("connect to scratch db: %v (url=%s)", err, adminURL)
	}
	t.Cleanup(pool.Close)

	if err := MigrateToVersion(ctx, pool, migrationsDir(t), 18); err != nil {
		t.Fatalf("migrate to 18: %v", err)
	}

	var tenantID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug) VALUES ('DB-009', 'db009') RETURNING id`).Scan(&tenantID); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (tenant_id, email, name, password_hash, role, status)
		 VALUES ($1, 'db009@test.dev', 'DB009', 'x', 'user', 'active') RETURNING id`, tenantID).
		Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Заказ с NULL price_json — ровно то, что UP-18 разрешила, а Down-18
	// отвергал.
	if _, err := pool.Exec(ctx,
		`INSERT INTO orders (tenant_id, kind, status, contact, config_json, price_json, user_id)
		 VALUES ($1, 'order', 'new', '{"name":"X"}', '{}', NULL, $2)`, tenantID, userID); err != nil {
		t.Fatalf("insert order with NULL price_json: %v", err)
	}
	// И консультация — её Down-18 удаляет штатно.
	if _, err := pool.Exec(ctx,
		`INSERT INTO orders (tenant_id, kind, status, contact, config_json, price_json, user_id)
		 VALUES ($1, 'consultation', 'new', '{"name":"Y"}', '{}', NULL, NULL)`, tenantID); err != nil {
		t.Fatalf("insert consultation: %v", err)
	}

	// Откат. БЫЛО: падение на «price_json contains null values».
	if err := MigrateToVersion(ctx, pool, migrationsDir(t), 17); err != nil {
		t.Fatalf("DB-9: откат 18→17 упал на данных, которые сама UP-18 разрешила: %v", err)
	}

	v, err := GetMigrationVersion(pool, migrationsDir(t))
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if v.Dirty {
		t.Fatalf("откат оставил версию dirty — накат был бы заблокирован (version=%d)", v.Version)
	}
	if v.Version != 17 {
		t.Errorf("version = %d, want 17", v.Version)
	}

	// Заказ пережил откат и его данные не выглядят как валидные: после отката
	// колонка NOT NULL, значит значение чем-то заполнено, и это должно быть
	// помечено, а не принято за настоящую цену.
	var price string
	if err := pool.QueryRow(ctx,
		`SELECT price_json::text FROM orders WHERE tenant_id = $1`, tenantID).Scan(&price); err != nil {
		t.Fatalf("read price_json after rollback: %v", err)
	}
	if !strings.Contains(price, "__rolled_back_000018") {
		t.Errorf("цену откаченного заказа помечать надо явно, а не молча: %s", price)
	}
	if !json.Valid([]byte(price)) {
		t.Errorf("price_json после отката перестал быть валидным JSON: %s", price)
	}
}

// TestDB009_KindNullabilityConstraintHolds — миграция 000033 обязана не давать
// ситуации, из-за которой откат падал: обычный заказ не может иметь пустую
// цену или несуществующего владельца.
func TestDB009_KindNullabilityConstraintHolds(t *testing.T) {
	_, pr := integrationAuthRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	owner := testOwnerID(t, pr, tenant)
	or := NewOrderRepository(pr.pool)

	// Валидный заказ проходит.
	ok := &order.Order{
		TenantID: tenant, UserID: owner, Kind: order.KindOrder, Status: order.StatusNew,
		Contact:    order.Contact{Name: "ok", Email: "ok@test.dev"},
		ConfigJSON: []byte(`{}`), PriceJSON: []byte(`{"final_price":1}`),
	}
	if err := or.Create(ctx, ok); err != nil {
		t.Fatalf("валидный заказ отвергнут: %v", err)
	}

	// Заказ без цены — отвергается схемой.
	noPrice := &order.Order{
		TenantID: tenant, UserID: owner, Kind: order.KindOrder, Status: order.StatusNew,
		Contact:    order.Contact{Name: "no-price", Email: "np@test.dev"},
		ConfigJSON: []byte(`{}`),
	}
	if err := or.Create(ctx, noPrice); err == nil {
		t.Error("заказ без price_json обязан отвергаться — именно он ронял откат")
	}

	// Заказ без владельца — тоже.
	noOwner := &order.Order{
		TenantID: tenant, Kind: order.KindOrder, Status: order.StatusNew,
		Contact:    order.Contact{Name: "no-owner", Email: "no@test.dev"},
		ConfigJSON: []byte(`{}`), PriceJSON: []byte(`{"final_price":1}`),
	}
	if err := or.Create(ctx, noOwner); err == nil {
		t.Error("заказ без user_id обязан отвергаться — он делает невозможным откат")
	}

	// Консультация — наоборот, обязана быть анонимной.
	consult := &order.Order{
		TenantID: tenant, Kind: order.KindConsultation, Status: order.StatusNew,
		Contact:    order.Contact{Name: "consult", Email: "c@test.dev"},
		ConfigJSON: []byte(`{}`),
	}
	if err := or.Create(ctx, consult); err != nil {
		t.Fatalf("консультация обязана быть анонимной: %v", err)
	}
}
