package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	// UpdateSettingsGroup меняет одну группу настроек (EDR-0041): контакты,
	// реквизиты, соцсети, SEO или ставки расчёта. Нужен, чтобы можно было
	// выдать «менять SEO», не выдавая «менять реквизиты».
	UpdateSettingsGroup(ctx context.Context, tenantID string, group store.SettingsGroup, patch json.RawMessage, actorID string) (store.Settings, error)
	PublicSettings(ctx context.Context, tenantID string) (store.PublicSettings, error)
	MaterialPrices(ctx context.Context, tenantID string) ([]store.MaterialPrice, error)
	SetMaterialPrice(ctx context.Context, tenantID, code string, pricePerKgRub int64, updatedBy string) error
	DeleteMaterialPrice(ctx context.Context, tenantID, code string) error
	// Крепёж (MNT-0001): прайс в рублях за штуку.
	FastenerPrices(ctx context.Context, tenantID string) ([]store.FastenerPrice, error)
	SetFastenerPrice(ctx context.Context, tenantID, code string, pricePerPcRub int64, updatedBy string) error
	DeleteFastenerPrice(ctx context.Context, tenantID, code string) error
	// Герметик (CONF-SEALANT): прайс в рублях за картридж.
	SealantPrices(ctx context.Context, tenantID string) ([]store.SealantPrice, error)
	SetSealantPrice(ctx context.Context, tenantID, code string, pricePerCartridgeRub int64, updatedBy string) error
	DeleteSealantPrice(ctx context.Context, tenantID, code string) error
	// ResolveRates — ставки расчёта магазина: встроенные значения плюс
	// настройки и цены tenant.
	ResolveRates(ctx context.Context, tenantID string) (*engprc.Rates, error)
}

// ---- admin ----

// requireStorePermission — проверка права магазина: настройки и прайс меняет
// только администратор tenant'а (SEC-0003). Права отдельные от settings.*,
// потому что security-политика и коммерческий прайс — разные решения.
func requireStorePermission(w http.ResponseWriter, r *http.Request, cap auth.Capability, level auth.Level) bool {
	if hasCapability(r, cap, level) {
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
		if !requireStorePermission(w, r, auth.CapSiteContacts, auth.LevelRead) {
			return
		}
		got, err := svc.Settings(r.Context(), tenantID(r.Context()))
		if err != nil {
			mapStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, got)
	}
}

// handleUpdateStoreSettings — PUT /api/v1/admin/store/settings.
func handleUpdateStoreSettings(svc StoreService, invalidate CacheInvalidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.CapSiteContacts, auth.LevelWrite) {
			return
		}
		var in store.Settings
		// DOM-007: строгий разбор — опечатка в имени секции настроек не должна
		// молча сбрасывать ВСЕ настройки магазина на дефолты (ответ 200).
		if err := decodeJSONStrict(w, r, &in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", jsonErrorMessage(err))
			return
		}
		in.UpdatedBy = userID(r.Context())
		got, err := svc.UpdateSettings(r.Context(), tenantID(r.Context()), in)
		if err != nil {
			mapStoreError(w, r, err)
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
		if !requireStorePermission(w, r, auth.CapPriceMaterials, auth.LevelRead) {
			return
		}
		prices, err := svc.MaterialPrices(r.Context(), tenantID(r.Context()))
		if err != nil {
			mapStoreError(w, r, err)
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
		if !requireStorePermission(w, r, auth.CapPriceMaterials, auth.LevelWrite) {
			return
		}
		// DOM-007: строгий разбор — опечатка в имени поля цены давала
		// price_per_kg_rub = 0 и ответ 200 «успешно сохранено».
		var req setMaterialPriceRequest
		if err := decodeJSONStrict(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", jsonErrorMessage(err))
			return
		}
		tenant := tenantID(r.Context())
		err := svc.SetMaterialPrice(r.Context(), tenant, req.Code, req.PricePerKgRub, userID(r.Context()))
		if err != nil {
			mapStoreError(w, r, err)
			return
		}
		purgePublicStoreCache(invalidate)
		// Возвращаем актуальную строку прайса, чтобы админка не гадала.
		prices, err := svc.MaterialPrices(r.Context(), tenant)
		if err != nil {
			mapStoreError(w, r, err)
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
		if !requireStorePermission(w, r, auth.CapPriceMaterials, auth.LevelWrite) {
			return
		}
		if err := svc.DeleteMaterialPrice(r.Context(), tenantID(r.Context()), r.PathValue("code")); err != nil {
			mapStoreError(w, r, err)
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
			// API-002: доменные ошибки больше не превращаются в 500 —
			// статус и код определяет единый контракт (error_contract.go).
			writeServiceError(w, r, err, "Внутренняя ошибка сервера")
			return
		}
		got, err := svc.PublicSettings(r.Context(), tenant.ID)
		if err != nil {
			mapStoreError(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=60")
		writeJSON(w, http.StatusOK, got)
	}
}

func mapStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrInvalid):
		writeInputError(w, "invalid_input", err)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Настройки магазина не найдены")
	default:
		writeServiceError(w, r, err, "Внутренняя ошибка сервера")
	}
}

// publicStoreTenant — tenant витрины для публичных (неаутентифицированных)
// маршрутов. Витрина однопринадлежна: это дефолтный tenant платформы. Пустая
// строка означает «контекст неизвестен» — считаем встроенными ставками движка
// (витрина не отдаёт 500 из-за недоступности tenant'а).
//
// CRITICAL-03 (2026-09-27): функция только НАЗЫВАЕТ контекст. Разрешение
// ставок делает stair.Service.resolveRates — иначе политика выбора источника
// расходилась бы между витриной и авторизованными маршрутами.
func publicStoreTenant(r *http.Request, authSvc AuthService) string {
	if authSvc == nil {
		return ""
	}
	tenant, err := authSvc.DefaultTenant(r.Context())
	// nil-tenant при nil-ошибке — не нарушение контракта репозитория
	// (auth_repo.go всегда возвращает ErrNotFound), но интерфейс AuthService
	// этого не запрещает, а разыменование nil здесь давало 500 на публичной
	// витрине. Пустой tenant = встроенные ставки движка, как и при ошибке.
	if err != nil || tenant == nil {
		if err != nil {
			slog.Error("store: default tenant unavailable, using engine defaults", "error", err)
		} else {
			slog.Error("store: default tenant is nil, using engine defaults")
		}
		return ""
	}
	return tenant.ID
}

// publicStoreRates — ставки расчёта магазина для СПРАВОЧНЫХ публичных ручек,
// которым ставки нужны как данные, а не как вход в конвейер (сейчас — каталог
// материалов, которому надо показать ₽/кг). ok=false, если сервис настроек не
// подключён или его не удалось прочитать: тогда действуют встроенные ставки
// движка (витрина не отдаёт 500 из-за прайса).
//
// В расчёте лестницы эта функция НЕ используется — см. CRITICAL-03.
func publicStoreRates(r *http.Request, storeSvc StoreService, authSvc AuthService) (*engprc.Rates, bool) {
	if storeSvc == nil {
		return nil, false
	}
	tenantID := publicStoreTenant(r, authSvc)
	if tenantID == "" {
		return nil, false
	}
	rates, err := storeSvc.ResolveRates(r.Context(), tenantID)
	if err != nil {
		slog.Error("store: rates unavailable, using engine defaults", "error", err)
		return nil, false
	}
	return rates, true
}

// handleUpdateStoreSettingsGroup — PUT /api/v1/admin/store/settings/{group}.
// Группа приходит из пути и задаёт проверяемое право: contacts → site.contacts,
// rates → price.rates. Отказ 403 — нет права на эту группу, 404 — группа не
// известна (иначе опечатка в пути отвечала бы «ничего не изменилось»).
func handleUpdateStoreSettingsGroup(svc StoreService, invalidate CacheInvalidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		group, ok := store.ParseSettingsGroup(r.PathValue("group"))
		if !ok {
			writeError(w, http.StatusNotFound, "not_found",
				"Такой группы настроек нет. Доступны: contacts, company, social, seo, rates.")
			return
		}
		if !requireStorePermission(w, r, group.Capability(), auth.LevelWrite) {
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Не удалось прочитать тело запроса")
			return
		}
		// Группу вырезаем из присланного документа, чтобы клиент присылал
		// {"phone": "..."}, а не {"contacts": {...}}.
		patch, err := groupPatch(body)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
			return
		}
		got, err := svc.UpdateSettingsGroup(r.Context(), tenantID(r.Context()), group, patch, userID(r.Context()))
		if err != nil {
			mapStoreError(w, r, err)
			return
		}
		purgePublicStoreCache(invalidate)
		writeJSON(w, http.StatusOK, got)
	}
}

// groupPatch достаёт из тела только свою группу.
func groupPatch(body []byte) (json.RawMessage, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("тело запроса должно быть объектом JSON")
	}
	patch, ok := doc[string(groupKeyOf(doc))]
	if !ok {
		return nil, fmt.Errorf("в теле нет секции с настройками группы")
	}
	return patch, nil
}

// groupKeyOf находит имя единственной секции в теле: у групп contacts,
// company, social, seo, rates секция называется так же, а счётчики входят в
// seo. Определяем по ключам, чтобы не заставлять клиента знать про счётчики.
func groupKeyOf(doc map[string]json.RawMessage) string {
	for _, known := range []string{"contacts", "company", "social", "seo", "rates", "counters"} {
		if _, ok := doc[known]; ok {
			return known
		}
	}
	return ""
}

// ---- крепёж (MNT-0001) ----

// fastenerPriceDTO — позиция прайса крепежа для админки.
type fastenerPriceDTO struct {
	Code          string  `json:"code"`
	NameRu        string  `json:"name_ru"`
	Kind          string  `json:"kind"`
	DiameterMM    float64 `json:"diameter_mm"`
	LengthMM      float64 `json:"length_mm"`
	PricePerPcRub int64   `json:"price_per_pc_rub"`
	Overridden    bool    `json:"overridden"`
}

func toFastenerPriceDTO(p store.FastenerPrice) fastenerPriceDTO {
	return fastenerPriceDTO{
		Code: p.Code, NameRu: p.NameRu, Kind: p.Kind,
		DiameterMM: p.DiameterMM, LengthMM: p.LengthMM,
		PricePerPcRub: p.PricePerPcRub, Overridden: p.Overridden,
	}
}

// handleListFastenerPrices — GET /api/v1/admin/store/fasteners.
func handleListFastenerPrices(svc StoreService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.CapPriceMaterials, auth.LevelRead) {
			return
		}
		prices, err := svc.FastenerPrices(r.Context(), tenantID(r.Context()))
		if err != nil {
			mapStoreError(w, r, err)
			return
		}
		out := make([]fastenerPriceDTO, 0, len(prices))
		for _, p := range prices {
			out = append(out, toFastenerPriceDTO(p))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// setFastenerPriceRequest — тело сохранения цены крепежа.
type setFastenerPriceRequest struct {
	Code          string `json:"code"`
	PricePerPcRub int64  `json:"price_per_pc_rub"`
}

// handleSetFastenerPrice — PUT /api/v1/admin/store/fasteners.
func handleSetFastenerPrice(svc StoreService, invalidate CacheInvalidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.CapPriceMaterials, auth.LevelWrite) {
			return
		}
		// DOM-007: строгий разбор, иначе опечатка в имени поля дала бы
		// price_per_pc_rub = 0 и ответ 200 «успешно сохранено».
		var req setFastenerPriceRequest
		if err := decodeJSONStrict(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", jsonErrorMessage(err))
			return
		}
		tenant := tenantID(r.Context())
		if err := svc.SetFastenerPrice(r.Context(), tenant, req.Code, req.PricePerPcRub, userID(r.Context())); err != nil {
			mapStoreError(w, r, err)
			return
		}
		purgePublicStoreCache(invalidate)
		prices, err := svc.FastenerPrices(r.Context(), tenant)
		if err != nil {
			mapStoreError(w, r, err)
			return
		}
		for _, p := range prices {
			if p.Code == req.Code {
				writeJSON(w, http.StatusOK, toFastenerPriceDTO(p))
				return
			}
		}
		writeJSON(w, http.StatusOK, toFastenerPriceDTO(store.FastenerPrice{
			Code: req.Code, PricePerPcRub: req.PricePerPcRub, Overridden: true,
		}))
	}
}

// handleDeleteFastenerPrice — DELETE /api/v1/admin/store/fasteners/{code}.
func handleDeleteFastenerPrice(svc StoreService, invalidate CacheInvalidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.CapPriceMaterials, auth.LevelWrite) {
			return
		}
		if err := svc.DeleteFastenerPrice(r.Context(), tenantID(r.Context()), r.PathValue("code")); err != nil {
			mapStoreError(w, r, err)
			return
		}
		purgePublicStoreCache(invalidate)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---- герметик (CONF-SEALANT) ----

// sealantPriceDTO — позиция прайса герметика для админки.
type sealantPriceDTO struct {
	Code                 string  `json:"code"`
	NameRu               string  `json:"name_ru"`
	Kind                 string  `json:"kind"`
	VolumeML             float64 `json:"volume_ml"`
	LayerMM              float64 `json:"layer_mm"`
	DensityKgPerL        float64 `json:"density_kg_per_l"`
	PricePerCartridgeRub int64   `json:"price_per_cartridge_rub"`
	Overridden           bool    `json:"overridden"`
}

func toSealantPriceDTO(p store.SealantPrice) sealantPriceDTO {
	return sealantPriceDTO{
		Code: p.Code, NameRu: p.NameRu, Kind: p.Kind,
		VolumeML: p.VolumeML, LayerMM: p.LayerMM, DensityKgPerL: p.DensityKgPerL,
		PricePerCartridgeRub: p.PricePerCartridgeRub, Overridden: p.Overridden,
	}
}

// handleListSealantPrices — GET /api/v1/admin/store/sealants.
func handleListSealantPrices(svc StoreService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.CapPriceMaterials, auth.LevelRead) {
			return
		}
		prices, err := svc.SealantPrices(r.Context(), tenantID(r.Context()))
		if err != nil {
			mapStoreError(w, r, err)
			return
		}
		out := make([]sealantPriceDTO, 0, len(prices))
		for _, p := range prices {
			out = append(out, toSealantPriceDTO(p))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// setSealantPriceRequest — тело сохранения цены герметика.
type setSealantPriceRequest struct {
	Code                 string `json:"code"`
	PricePerCartridgeRub int64  `json:"price_per_cartridge_rub"`
}

// handleSetSealantPrice — PUT /api/v1/admin/store/sealants.
func handleSetSealantPrice(svc StoreService, invalidate CacheInvalidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.CapPriceMaterials, auth.LevelWrite) {
			return
		}
		// DOM-007: строгий разбор, иначе опечатка в имени поля дала бы
		// price_per_cartridge_rub = 0 и ответ 200 «успешно сохранено».
		var req setSealantPriceRequest
		if err := decodeJSONStrict(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", jsonErrorMessage(err))
			return
		}
		tenant := tenantID(r.Context())
		if err := svc.SetSealantPrice(r.Context(), tenant, req.Code, req.PricePerCartridgeRub, userID(r.Context())); err != nil {
			mapStoreError(w, r, err)
			return
		}
		purgePublicStoreCache(invalidate)
		prices, err := svc.SealantPrices(r.Context(), tenant)
		if err != nil {
			mapStoreError(w, r, err)
			return
		}
		for _, p := range prices {
			if p.Code == req.Code {
				writeJSON(w, http.StatusOK, toSealantPriceDTO(p))
				return
			}
		}
		writeJSON(w, http.StatusOK, toSealantPriceDTO(store.SealantPrice{
			Code: req.Code, PricePerCartridgeRub: req.PricePerCartridgeRub, Overridden: true,
		}))
	}
}

// handleDeleteSealantPrice — DELETE /api/v1/admin/store/sealants/{code}.
func handleDeleteSealantPrice(svc StoreService, invalidate CacheInvalidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireStorePermission(w, r, auth.CapPriceMaterials, auth.LevelWrite) {
			return
		}
		if err := svc.DeleteSealantPrice(r.Context(), tenantID(r.Context()), r.PathValue("code")); err != nil {
			mapStoreError(w, r, err)
			return
		}
		purgePublicStoreCache(invalidate)
		w.WriteHeader(http.StatusNoContent)
	}
}
