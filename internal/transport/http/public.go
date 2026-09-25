package http

import (
	"net/http"

	"stairplatform/internal/application/stair"
	kerngeo "stairplatform/internal/geometry"
)

// publicQuoteDTO — публичный результат расчёта для клиентского сайта
// (store). Отличается от calculateResponse тем, что НЕ содержит
// производственный пакет (BOM/CutList/Nesting) и детали деталей: розничный
// клиент получает только геометрию, валидацию, предварительную цену,
// габаритную ширину и preview-сетку (mesh) для 3D-визуализации.
type publicQuoteDTO struct {
	Validation  validationDTO `json:"validation"`
	Flight      flightDTO     `json:"flight"`
	LShape      *lshapeDTO    `json:"lshape,omitempty"`
	UShape      *ushapeDTO    `json:"ushape,omitempty"`
	Spiral      *spiralDTO    `json:"spiral,omitempty"`
	Geometry    geometryDTO   `json:"geometry"`
	Pricing     *pricingDTO   `json:"pricing,omitempty"`
	Mesh        *kerngeo.Mesh `json:"mesh,omitempty"`
	RailingMesh *kerngeo.Mesh `json:"railing_mesh,omitempty"`
	RoomMesh    *kerngeo.Mesh `json:"room_mesh,omitempty"`
}

// handlePublicQuote — POST /api/v1/public/stairs:quote.
// Публичный (без аутентификации) расчёт предварительной цены и геометрии
// для розничного клиента. Идентичен /stairs:calculate, но ответ исключает
// производственную документацию (BOM/CutList/Nesting). Ограничен rate-limiter
// для защиты от злоупотреблений.
//
//	200 — успешный расчёт;
//	400 — некорректный JSON;
//	422 — невалидный вход;
//	429 — превышен rate-limit;
//	500 — внутренняя ошибка.
func handlePublicQuote(svc StairService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req calculateRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}

		if rejectDisabledFlight(w, req.Flight) {
			return
		}

		cfg := toConfig(req)
		opts, err := toOptions(req)
		if err != nil {
			writeInputError(w, "invalid_rates", err)
			return
		}

		res, err := svc.Calculate(r.Context(), cfg, opts)
		if err != nil {
			mapStairError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, toPublicQuote(res, cfg))
	}
}

// toPublicQuote конвертирует полный результат в публичный DTO без
// производственного пакета. Если конвейер остановлен (blocking/нет цены),
// pricing и mesh отсутствуют (omitempty). Габаритная ширина марша
// проставляется во все варианты flight-результата из конфигурации.
func toPublicQuote(res *stair.Result, cfg stair.Config) publicQuoteDTO {
	out := publicQuoteDTO{
		Validation: toValidationResult(res, true),
	}
	if res.Validation.Blocking || res.Price == nil {
		return out
	}
	w := cfg.Width.Millimeters()
	e := flightEcho(*res)
	f := toFlight(*res)
	f.WidthMm = w
	out.Flight = f
	out.LShape = toLShape(res.LShape, e)
	out.UShape = toUShape(res.UShape, e)
	out.Spiral = toSpiral(res.Spiral, e)
	if out.LShape != nil {
		out.LShape.WidthMm = w
	}
	if out.UShape != nil {
		out.UShape.WidthMm = w
	}
	if out.Spiral != nil {
		out.Spiral.WidthMm = w
	}
	out.Geometry = toGeometry(*res)
	price := toPricing(res.Price)
	out.Pricing = &price
	out.Mesh = res.Mesh
	out.RailingMesh = res.RailingMesh
	out.RoomMesh = res.RoomMesh
	return out
}
