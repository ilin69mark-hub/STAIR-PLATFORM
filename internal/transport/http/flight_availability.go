package http

import (
	"net/http"

	"stairplatform/internal/domain/engineering"
)

// spiralTemporarilyDisabled — винтовой марш временно выведен из расчётов
// публичного API (S-152): нормы EDR-0007 внутренне противоречивы, ни одна
// конфигурация не проходит проверку проступи, а предложения советника сами
// невалидны. Код движка, каталог и тесты НЕ удалены — вернём вместе с
// исправлением норм.
//
// Проверка живёт в слое HTTP: внутренний /api/v1/stairs:calculate и движок
// продолжают считать спираль (их используют существующие тесты и черновики
// проектов), публичный расчёт — нет.
const spiralDisabledMessage = "Винтовые лестницы временно недоступны: тип марша проходит техническое обновление. Выберите прямой, L-образный или П-образный марш."

// rejectDisabledFlight возвращает true, если тип марша в запросе отключён, и
// уже ответил клиенту.
func rejectDisabledFlight(w http.ResponseWriter, flight string) bool {
	if engineering.FlightType(flight).Valid() && flight != string(engineering.FlightSpiral) {
		return false
	}
	if flight == string(engineering.FlightSpiral) {
		writeError(w, http.StatusUnprocessableEntity, "flight_temporarily_disabled", spiralDisabledMessage)
		return true
	}
	return false
}
