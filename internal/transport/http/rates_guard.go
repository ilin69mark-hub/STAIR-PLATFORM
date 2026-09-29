package http

import (
	"net/http"

	"stairplatform/internal/application/stair"
)

// clientRatesNotAllowedMessage — текст отклонения клиентских ставок цены
// (SEC-001). Один текст для всех маршрутов, чтобы клиент не мог отличить
// публичный путь от авторизованного и «играть в разные правила».
const clientRatesNotAllowedMessage = "Ставки цены задаются на сервере (каталог/настройки магазина); переопределять их в запросе нельзя."

// rejectClientRates — единая точка отказа для клиентского переопределения
// цен (SEC-001).
//
// Возвращает true, если запрос содержит непустое поле `rates`; в этом случае
// ответ 422 rates_not_allowed УЖЕ записан и вызывающий обязан выйти.
//
// Вызывается ВСЕМИ маршрутами, которые декодируют calculateRequest
// (в т.ч. вложенный в optimizeRequest), ДО любого вызова toOptions/calculate:
//
//	POST /api/v1/stairs:calculate
//	POST /api/v1/stairs:validate
//	POST /api/v1/stairs:optimize
//	POST /api/v1/stairs:calculate/async
//	POST /api/v1/projects/{id}/calculate
//	POST /api/v1/projects/{id}/preview
//	POST /api/v1/projects/{id}/optimize
//	POST /api/v1/assistant/{kind}
//	POST /api/v1/public/stairs:quote
//
// До 2026-09-26 защиту имел только публичный quote (public.go), а
// авторизованные пути принимали любые ставки — это позволяло обнулить цену.
func rejectClientRates(w http.ResponseWriter, req calculateRequest) bool {
	if !clientRatesPresent(req.Rates) {
		return false
	}
	writeError(w, http.StatusUnprocessableEntity, "rates_not_allowed", clientRatesNotAllowedMessage)
	return true
}

// optionsOrReject — декодирование опций расчёта с проверкой запрещённого
// поля `rates` и простановкой tenant-контекста для цен. Возвращает ok=false,
// если клиент прислал ставки (ответ 422 уже записан).
//
// Единственная разрешённая точка сборки Options из тела запроса: любой новый
// маршрут расчёта обязан вызывать её, а не toOptions напрямую, иначе
// регрессия SEC-001 вернётся.
//
// CRITICAL-03 (2026-09-27): функция также проставляет Options.TenantID —
// tenant аутентифицированного запроса. Раньше транспорт на этом маршруте НЕ
// сообщал application-слою, чьи ставки использовать, и девять авторизованных
// маршрутов молча падали в engprc.DefaultRates() в то время, как витрина
// считала по ставкам магазина. Одна и та же лестница стоила по-разному в
// зависимости от маршрута. Теперь transport только называет контекст, а
// политику выбора источника применяет stair.Service.resolveRates.
func optionsOrReject(r *http.Request, w http.ResponseWriter, req calculateRequest) (stair.Options, bool) {
	if rejectClientRates(w, req) {
		return stair.Options{}, false
	}
	opts, ok := toOptions(req)
	if !ok {
		return stair.Options{}, false
	}
	opts.TenantID = tenantID(r.Context())
	return opts, true
}
