package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStaticAssetsHandler — публичная раздача PBR/HDRI (этап 1 шаг 5):
// файл отдаётся с иммутабельным кэшем, каталог и выход за пределы корня
// закрыты, листинга нет.
func TestStaticAssetsHandler(t *testing.T) {
	// Секрет лежит ВНЕ раздаваемого корня — иначе это не traversal.
	parent := t.TempDir()
	dir := filepath.Join(parent, "assets")
	if err := os.MkdirAll(filepath.Join(dir, "pbr", "WOOD-OAK"), 0o750); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(parent, "secret.env")
	if err := os.WriteFile(secret, []byte("SECRET=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pbr", "WOOD-OAK", "color.jpg"), []byte("JPEG"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := StaticAssetsHandler(dir)
	// Роутер монтирует обработчик на /static-assets/, поэтому пути приходят
	// с префиксом — именно этот случай ломался (404 на всех текстурах).
	const mount = "/static-assets"

	// 1) Файл отдаётся + иммутабельный кэш + CORS.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pbr/WOOD-OAK/color.jpg", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("want immutable cache, got %q", cc)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("textures must be loadable cross-origin")
	}

	// 2) Path traversal (../) — 404/403, секрет не утекает.
	for _, p := range []string{"/../secret.env", "/pbr/../../secret.env", "/%2e%2e/secret.env",
		"/..%2fsecret.env", "/pbr/WOOD-OAK/../../../secret.env"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "SECRET=1") {
			t.Fatalf("traversal %q leaked: code=%d body=%s", p, rec.Code, rec.Body.String())
		}
	}

	// 3) Каталог не листингуется.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pbr/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("directory listing must be 404, got %d", rec.Code)
	}

	// 4) Метод записи запрещён.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/pbr/WOOD-OAK/color.jpg", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405 for POST, got %d", rec.Code)
	}

	// 5) Отсутствующий файл — 404 без паники.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pbr/NOPE/color.jpg", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}
