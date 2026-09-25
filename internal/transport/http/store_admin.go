package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/store"
	engprc "stairplatform/internal/engine/pricing"
)

// StoreService — порт настроек магазина для HTTP-слоя.
type StoreService interface {
	Settings(ctx context.Context, tenantID string) (store.Settings, error)
	UpdateSettings(ctx context.Context, tenantID string, in store.Settings) (store.Settings, error)
	PublicSettings(ctx context.Context, tenantID string) (store.PublicSettings, error)
	MaterialPrices(ctx context.Context, tenantID string) ([]store.MaterialPrice, error)
	SetMaterialPrice(ctx context.Context, tenantID, code string, pricePerKgRub int64, updatedBy string) error
	DeleteMaterialPrice(ctx context.Context, tenantID, code string) error
	// ResolveRates — ставки расчёта магазина: встроенные значения плюс
	// настройки и цены tenant.
	ResolveRates(ctx context.Context, tenantID string) (*engprc.Rates, error)
}

// ---- admin ----

// requireStorePermission — проверка права магазина: настройки и прайс меняет
// только администратор tenant'а (SEC-0003). Права отдельные от settings.*,
// потому что security-политика и коммерческий прайс — разные решения.
func requireStorePermission(w http.ResponseWriter, r *http.Request, perm auth.Permission) bool {
	if hasPermission(r, perm) {
		return true
	}
	writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора магазина")
	return false
}

// purgePublicStoreCache сбрасывает кэш публичных ответов после смены настроек
// или прайса: иначе витрина до TTL (5 мин) показывала бы старую цену, а
// расчёт считал бы по новой.
func purgePublicStoreCache(invalidate CacheInvalidator) {
	if invalidate == nil {
		return
	}
	if n := invalidate.InvalidatePrefix("/api/v1/public/"); n > 0 {
		slog.Debug("store: public response cache purged", "entries", n)
	}
}

// handleGetStoreSettings — GET /api/v1/admin/store/settings.
func handleGetStoreSettings(svc StoreService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.PermissionStoreSettingsRead) {
			return
		}
		got, err := svc.Settings(r.Context(), tenantID(r.Context()))
		if err != nil {
			mapStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, got)
	}
}

// handleUpdateStoreSettings — PUT /api/v1/admin/store/settings.
func handleUpdateStoreSettings(svc StoreService, invalidate CacheInvalidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.PermissionStoreSettingsWrite) {
			return
		}
		var in store.Settings
		if err := decodeJSON(w, r, &in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		in.UpdatedBy = userID(r.Context())
		got, err := svc.UpdateSettings(r.Context(), tenantID(r.Context()), in)
		if err != nil {
			mapStoreError(w, err)
			return
		}
		purgePublicStoreCache(invalidate)
		writeJSON(w, http.StatusOK, got)
	}
}

// materialPriceDTO — цена материала магазина: админка и витрина видят одно.
type materialPriceDTO struct {
	Code          string `json:"code"`
	PricePerKgRub int64  `json:"price_per_kg_rub"`
	Overridden    bool   `json:"overridden"`
}

func toMaterialPriceDTO(p store.MaterialPrice) materialPriceDTO {
	return materialPriceDTO{Code: p.Code, PricePerKgRub: p.PricePerKgRub, Overridden: p.Overridden}
}

// handleListMaterialPrices — GET /api/v1/admin/store/prices.
func handleListMaterialPrices(svc StoreService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.PermissionStorePricesRead) {
			return
		}
		prices, err := svc.MaterialPrices(r.Context(), tenantID(r.Context()))
		if err != nil {
			mapStoreError(w, err)
			return
		}
		out := make([]materialPriceDTO, 0, len(prices))
		for _, p := range prices {
			out = append(out, toMaterialPriceDTO(p))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// setMaterialPriceRequest — тело сохранения цены материала.
type setMaterialPriceRequest struct {
	Code          string `json:"code"`
	PricePerKgRub int64  `json:"price_per_kg_rub"`
}

// handleSetMaterialPrice — PUT /api/v1/admin/store/prices.
func handleSetMaterialPrice(svc StoreService, invalidate CacheInvalidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.PermissionStorePricesWrite) {
			return
		}
		var req setMaterialPriceRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		tenant := tenantID(r.Context())
		err := svc.SetMaterialPrice(r.Context(), tenant, req.Code, req.PricePerKgRub, userID(r.Context()))
		if err != nil {
			mapStoreError(w, err)
			return
		}
		purgePublicStoreCache(invalidate)
		// Возвращаем актуальную строку прайса, чтобы админка не гадала.
		prices, err := svc.MaterialPrices(r.Context(), tenant)
		if err != nil {
			mapStoreError(w, err)
			return
		}
		for _, p := range prices {
			if p.Code == req.Code {
				writeJSON(w, http.StatusOK, toMaterialPriceDTO(p))
				return
			}
		}
		writeJSON(w, http.StatusOK, toMaterialPriceDTO(store.MaterialPrice{
			Code: req.Code, PricePerKgRub: req.PricePerKgRub, Overridden: true,
		}))
	}
}

// handleDeleteMaterialPrice — DELETE /api/v1/admin/store/prices/{code}.
// Возвращает материал к встроенной ставке движка.
func handleDeleteMaterialPrice(svc StoreService, invalidate CacheInvalidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.PermissionStorePricesWrite) {
			return
		}
		if err := svc.DeleteMaterialPrice(r.Context(), tenantID(r.Context()), r.PathValue("code")); err != nil {
			mapStoreError(w, err)
			return
		}
		purgePublicStoreCache(invalidate)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---- public ----

// handlePublicStoreSettings — GET /api/v1/public/store-settings.
// Витрине отдаём безопасное подмножество: контакты, реквизиты, соцсети, SEO и
// коды счётчиков. Цены материалов и услуг живут в каталогах, где уже учтён
// прайс магазина.
func handlePublicStoreSettings(svc StoreService, authSvc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Витрина работает без авторизации, поэтому tenant берётся из сервиса
		// (сейчас — default). При переходе на мультитенант (ROADMAP-0013) это
		// станет host-резолвером.
		tenant, err := authSvc.DefaultTenant(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		got, err := svc.PublicSettings(r.Context(), tenant.ID)
		if err != nil {
			mapStoreError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=60")
		writeJSON(w, http.StatusOK, got)
	}
}

func mapStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrInvalid):
		writeInputError(w, "invalid_input", err)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Настройки магазина не найдены")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
	}
}

// publicStoreRates — ставки расчёта магазина для публичного ручки. ok=false,
// если сервис настроек не подключён или его не удалось прочитать: тогда
// действуют встроенные ставки движка (витрина не отдаёт 500 из-за прайса).
func publicStoreRates(r *http.Request, storeSvc StoreService, authSvc AuthService) (*engprc.Rates, bool) {
	if storeSvc == nil {
		return nil, false
	}
	tenant, err := authSvc.DefaultTenant(r.Context())
	if err != nil {
		slog.Error("store: default tenant unavailable, using engine defaults", "error", err)
		return nil, false
	}
	rates, err := storeSvc.ResolveRates(r.Context(), tenant.ID)
	if err != nil {
		slog.Error("store: rates unavailable, using engine defaults", "error", err)
		return nil, false
	}
	return rates, true
}
