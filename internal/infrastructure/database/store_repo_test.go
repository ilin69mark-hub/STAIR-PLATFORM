package database

import (
	"context"
	"errors"
	"os"
	"testing"

	"stairplatform/internal/application/store"
)

func integrationStoreRepo(t *testing.T) (*StoreRepository, string) {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	repo := NewStoreRepository(pool)
	tenant := testTenantID(t, NewProjectRepository(pool))
	if _, err := repo.pool.Exec(ctx, `DELETE FROM store_rates WHERE tenant_id = $1`, tenant); err != nil {
		t.Fatalf("cleanup rates: %v", err)
	}
	if _, err := repo.pool.Exec(ctx, `DELETE FROM store_settings WHERE tenant_id = $1`, tenant); err != nil {
		t.Fatalf("cleanup settings: %v", err)
	}
	return repo, tenant
}

// TestStoreSettingsRoundtrip — upsert настроек магазина и чтение с
// простановкой автора правки.
func TestStoreSettingsRoundtrip(t *testing.T) {
	repo, tenant := integrationStoreRepo(t)
	ctx := context.Background()

	if _, err := repo.GetSettings(ctx, tenant); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("до первой настройки ожидался ErrNotFound, получено %v", err)
	}

	in := store.DefaultSettings()
	in.Contacts.Phone = "+7 900 000-00-00"
	in.Company.INN = "7701234567"
	in.Rates.MarginPercent = 18
	if err := repo.SaveSettings(ctx, tenant, in, ""); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	// Повторная запись — upsert, а не конфликт.
	in.Contacts.Phone = "+7 900 111-22-33"
	if err := repo.SaveSettings(ctx, tenant, in, ""); err != nil {
		t.Fatalf("SaveSettings (upsert): %v", err)
	}

	got, err := repo.GetSettings(ctx, tenant)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.Contacts.Phone != "+7 900 111-22-33" || got.Company.INN != "7701234567" {
		t.Fatalf("настройки искажены: %+v", got)
	}
	if got.Rates.MarginPercent != 18 || got.Rates.OverheadPercent != 20 {
		t.Fatalf("параметры расчёта потеряны: %+v", got.Rates)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("updated_at не проставлен")
	}
}

// TestStoreMaterialPricesCRUD — цены материалов сохраняются, читаются и
// удаляются по ключу (tenant, material).
func TestStoreMaterialPricesCRUD(t *testing.T) {
	repo, tenant := integrationStoreRepo(t)
	ctx := context.Background()

	if err := repo.SetMaterialPrice(ctx, tenant, "WOOD-OAK", 4200, ""); err != nil {
		t.Fatalf("SetMaterialPrice: %v", err)
	}
	if err := repo.SetMaterialPrice(ctx, tenant, "STEEL-S235", 120, ""); err != nil {
		t.Fatalf("SetMaterialPrice: %v", err)
	}
	// Upsert: повторная запись меняет цену, а не падает.
	if err := repo.SetMaterialPrice(ctx, tenant, "WOOD-OAK", 4500, ""); err != nil {
		t.Fatalf("SetMaterialPrice (upsert): %v", err)
	}

	got, err := repo.ListMaterialPrices(ctx, tenant)
	if err != nil {
		t.Fatalf("ListMaterialPrices: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидались 2 цены, получено %d: %+v", len(got), got)
	}
	// Сортировка по коду: STEEL-S235 раньше WOOD-OAK.
	if got[0].Code != "STEEL-S235" || got[0].PricePerKgRub != 120 {
		t.Fatalf("первая цена искажена: %+v", got[0])
	}
	if got[1].Code != "WOOD-OAK" || got[1].PricePerKgRub != 4500 || !got[1].Overridden {
		t.Fatalf("вторая цена искажена: %+v", got[1])
	}

	if err := repo.DeleteMaterialPrice(ctx, tenant, "WOOD-OAK"); err != nil {
		t.Fatalf("DeleteMaterialPrice: %v", err)
	}
	got, err = repo.ListMaterialPrices(ctx, tenant)
	if err != nil {
		t.Fatalf("ListMaterialPrices after delete: %v", err)
	}
	if len(got) != 1 || got[0].Code != "STEEL-S235" {
		t.Fatalf("удаление не сработало: %+v", got)
	}
}

// TestStorePricesRejectNegative — отрицательная цена отклоняется constraint'ом,
// а не молча проходит в прайс.
func TestStorePricesRejectNegative(t *testing.T) {
	repo, tenant := integrationStoreRepo(t)
	if err := repo.SetMaterialPrice(context.Background(), tenant, "WOOD-OAK", -1, ""); err == nil {
		t.Fatal("отрицательная цена не отклонена")
	}
}
