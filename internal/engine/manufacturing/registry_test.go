package manufacturing

import (
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// mustMaterials возвращает каталог материалов реестра (P2-12: Default* уже
// возвращает ошибку вместо паники; в тестах недоступность каталога — фейл).
func mustMaterials(t testing.TB) *dommfg.MaterialRegistry {
	t.Helper()
	reg, err := DefaultMaterialRegistry()
	if err != nil {
		t.Fatalf("DefaultMaterialRegistry: %v", err)
	}
	return reg
}

// mustSheets возвращает каталог листов реестра (аналог mustMaterials).
func mustSheets(t testing.TB) *dommfg.StockSheetRegistry {
	t.Helper()
	reg, err := DefaultStockSheetRegistry()
	if err != nil {
		t.Fatalf("DefaultStockSheetRegistry: %v", err)
	}
	return reg
}
