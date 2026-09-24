package http

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// StaticAssetsHandler — публичная раздача версионированных статических
// ассетов (этап 1 «студийный 3D»): PBR-текстуры материалов и HDRI.
//
// Требования безопасности:
//   - раздаётся ТОЛЬКО из одного каталога (root); путь нормализуется и
//     проверяется на выход за пределы root (path traversal);
//   - никаких листингов: отдаём только существующие файлы;
//   - только GET/HEAD (для HEAD — тот же FileServer, без тела);
//   - файлы иммутабельны (content-hash в имени), поэтому кэш на год.
//
// Каталог задаётся Config.StaticAssetsDir (cmd/api: STAIR_STATIC_ASSETS_DIR),
// по умолчанию ./assets. Если каталога нет — 404 (без ошибок в логах).
func StaticAssetsHandler(root string) http.Handler {
	if root == "" {
		root = "assets"
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	fileServer := http.FileServer(http.Dir(absRoot))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Метод не поддерживается")
			return
		}
		// Ассеты версионируются по содержимому → можно кэшировать надолго.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Access-Control-Allow-Origin", "*") // текстуры грузятся с другого origin (Next.js)
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")

		upath := filepath.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		target := filepath.Join(absRoot, upath)
		// Двойная проверка: после Join путь обязан остаться внутри root.
		if !strings.HasPrefix(target, absRoot+string(os.PathSeparator)) && target != absRoot {
			writeError(w, http.StatusForbidden, "forbidden", "Доступ запрещён")
			return
		}
		info, statErr := os.Stat(target)
		if statErr != nil || info.IsDir() {
			// Нет файла или это каталог (листинга не делаем).
			writeError(w, http.StatusNotFound, "not_found", "Ассет не найден")
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
