package http

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/project"
	appstorage "stairplatform/internal/application/storage"
	"stairplatform/internal/infrastructure/cad"
)

// StorageService — прикладной интерфейс объектного хранилища (EDR-0026
// §3.4), ожидаемый транспортным слоем.
type StorageService interface {
	SaveExport(ctx context.Context, tenantID, category, filename string, data []byte, contentType string) (*appstorage.ExportRef, error)
	Load(ctx context.Context, tenantID, key string) ([]byte, string, error)
	Delete(ctx context.Context, tenantID, key string) error
}

// ---- DTO ----

type exportRefDTO struct {
	Key         string `json:"key"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

// ---- handlers ----

// handleStoreExportCAD — POST /api/v1/projects/{id}/export/cad/store
// (auth, член проекта). Экспортирует CAD (E1) и сохраняет в Storage
// (EDR-0026 §3.5). 201 — сохранён; 400 — неверный format;
// 403 — нет прав; 404 — нет проекта/конфигурации; 422 — ошибка хранилища.
func handleStoreExportCAD(projects ProjectService, svc StorageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		format, err := cad.ParseFormat(r.URL.Query().Get("format"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", userInputMessage(err))
			return
		}
		projectID := r.PathValue("id")
		ctx := r.Context()
		user := userID(ctx)
		tenant := tenantID(ctx)

		mesh, err := projects.ExportCAD(ctx, tenant, user, projectID)
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Для проекта нет конфигурации")
			return
		}
		if errors.Is(err, project.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		var buf bytes.Buffer
		if err := cad.Write(&buf, mesh, format); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Не удалось выполнить экспорт чертежа")
			return
		}
		ref, err := svc.SaveExport(ctx, tenant, "cad", "project-"+projectID+format.Extension(), buf.Bytes(), format.MIME())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusCreated, exportRefDTO{Key: ref.Key, ContentType: ref.ContentType, Size: ref.Size})
	}
}

// handleGetObject — GET /api/v1/storage/{key} (auth). Отдаёт объект,
// скоуп tenant (EDR-0026 §3.5). 200 — данные; 403 — ключ чужого tenant;
// 404 — нет объекта.
func handleGetObject(svc StorageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		ctx := r.Context()
		tenant := tenantID(ctx)

		data, _, err := svc.Load(ctx, tenant, key)
		if err != nil {
			if strings.HasPrefix(key, tenant+"/") {
				writeError(w, http.StatusNotFound, "not_found", "Объект не найден.")
				return
			}
			writeError(w, http.StatusForbidden, "forbidden", "Объект вне области доступа.")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		// G705: произвольные байты из object storage; octet-stream + nosniff
		// заставляют браузер скачивать файл, а не рендерить его как HTML.
		_, _ = w.Write(data) //nolint:gosec // G705: octet-stream download, см. выше
	}
}

// handleDeleteObject — DELETE /api/v1/storage/{key} (admin). Удаляет объект
// tenant (EDR-0026 §3.5). 204 — удалено; 403 — нет прав/чужой tenant;
// 404 — нет объекта.
func handleDeleteObject(svc StorageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionIntegrationsManage) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуется право integrations.manage.")
			return
		}
		key := r.PathValue("key")
		tenant := tenantID(r.Context())
		if err := svc.Delete(r.Context(), tenant, key); err != nil {
			if strings.HasPrefix(key, tenant+"/") {
				writeError(w, http.StatusNotFound, "not_found", "Объект не найден.")
				return
			}
			writeError(w, http.StatusForbidden, "forbidden", "Объект вне области доступа.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
