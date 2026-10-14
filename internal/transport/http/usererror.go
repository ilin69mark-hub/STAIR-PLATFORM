package http

import (
	"errors"
	"net/http"
	"strings"

	"stairplatform/internal/engine/solver"
)

// userInputMessage возвращает понятное пользователю русское сообщение для
// ошибки ввода. Ошибки *solver.InputError уже несут готовый текст; остальные
// известные сообщения движков/доменов переводятся; неизвестное — обобщённое
// русское (детали уходят в лог сервера, а не клиенту).
func userInputMessage(err error) string {
	var inp *solver.InputError
	if errors.As(err, &inp) {
		return inp.Message
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid rate"), strings.Contains(msg, "rate must not be negative"),
		strings.Contains(msg, "overflows"):
		return "Некорректная ставка: ставки должны быть неотрицательными числами."
	case strings.Contains(msg, "invalid email"):
		return "Некорректный email."
	case strings.Contains(msg, "password too weak"):
		return "Пароль слишком слабый (минимум 8 символов)."
	case strings.Contains(msg, "invalid credentials"):
		return "Неверный email или пароль."
	case strings.Contains(msg, "invalid policy"):
		return "Некорректная политика безопасности."
	case strings.Contains(msg, "already approved"), strings.Contains(msg, "invalid status transition"),
		strings.Contains(msg, "invalid_status"):
		return "Некорректный статус: операция недопустима для текущего состояния."
	case strings.Contains(msg, "already registered"):
		return "Пользователь с таким email уже зарегистрирован."
	case strings.Contains(msg, "invalid granularity"):
		return "Некорректная гранулярность аналитики."
	case strings.Contains(msg, "invalid time range"):
		return "Некорректный временной диапазон."
	case strings.Contains(msg, "amount_minor must be positive"), strings.Contains(msg, "amount mismatch"):
		return "Некорректная сумма платежа."
	case strings.Contains(msg, "invalid webhook body"):
		return "Некорректное тело webhook-уведомления."
	case strings.Contains(msg, "contact.name required"):
		return "Укажите имя контактного лица."
	case strings.Contains(msg, "contact.email required"):
		return "Укажите email контактного лица."
	case strings.Contains(msg, "config required"):
		return "Не передана конфигурация лестницы."
	case strings.Contains(msg, "price required"):
		return "Не передана цена заказа."
	case strings.Contains(msg, "question required"):
		return "Укажите вопрос."
	case strings.Contains(msg, "author required"):
		return "Укажите автора отзыва."
	case strings.Contains(msg, "text required"):
		return "Укажите текст отзыва."
	case strings.Contains(msg, "rating must be 1..5"):
		return "Оценка должна быть от 1 до 5."
	case strings.Contains(msg, "invalid status"):
		return "Некорректный статус."
	case strings.Contains(msg, "name required"):
		return "Укажите имя."
	case strings.Contains(msg, "width must be positive"):
		return "Ширина марша должна быть положительной."
	case strings.Contains(msg, "height must be positive"):
		return "Высота подъёма должна быть положительной."
	case strings.Contains(msg, "flight type is required"):
		return "Не выбран тип лестницы."
	case strings.Contains(msg, "stringer thickness must not be negative"):
		return "Толщина косоура не может быть отрицательной."
	case strings.Contains(msg, "step thickness must not be negative"):
		return "Толщина ступени не может быть отрицательной."
	case strings.Contains(msg, "clearance must not be negative"):
		return "Просвет не может быть отрицательным."
	case strings.Contains(msg, "railing height must not be negative"):
		return "Высота перил не может быть отрицательной."
	case strings.Contains(msg, "landing width must not be negative"):
		return "Ширина площадки не может быть отрицательной."
	case strings.Contains(msg, "machine_per_hour_rub must not be negative"),
		strings.Contains(msg, "labor_per_hour_rub must not be negative"):
		return "Ставка станка или труда не может быть отрицательной."
	case strings.Contains(msg, "must be within 0..100"):
		return "Процент накладных, маржи, скидки или НДС должен быть от 0 до 100."
	case strings.Contains(msg, "unknown material"):
		return "Такого материала нет в каталоге."
	}
	return "Некорректный запрос. Проверьте переданные параметры."
}

// writeInputError пишет 422 с понятным русским сообщением для ошибки ввода.
func writeInputError(w http.ResponseWriter, code string, err error) {
	writeError(w, http.StatusUnprocessableEntity, code, userInputMessage(err))
}
