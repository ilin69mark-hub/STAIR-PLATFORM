package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestStaticAssetsServedThroughRouter — сквозная проверка монтирования: путь
// приходит в обработчик с префиксом /static-assets/, иначе все текстуры 404.
func TestStaticAssetsServedThroughRouter(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pbr", "WOOD-OAK"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pbr", "WOOD-OAK", "color.jpg"), []byte("JPEG"), 0o600); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET "+staticAssetsMount+"/", StaticAssetsHandler(dir))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static-assets/pbr/WOOD-OAK/color.jpg", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 через роутер, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "JPEG" {
		t.Fatalf("не отдан файл: %q", rec.Body.String())
	}
}
