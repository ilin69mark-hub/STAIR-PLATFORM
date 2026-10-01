// Package store реализует настройки публичного магазина и его прайс
// (волна 0 «admin store»).
//
// Модуль отвечает за две вещи:
//   - настройки точки (контакты, реквизиты, соцсети, SEO, параметры расчёта),
//     которые редактирует администратор магазина;
//   - цены материалов (₽/кг), которые участвуют в расчёте вместо зашитых
//     в движок ставок.
//
// Данные tenant-scoped по построению: это же станет основой мультитенанта
// (ROADMAP-0013), поэтому нигде не используется «default tenant» как
// подстановка — tenant приходит из аутентификации.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNotFound — настройки магазина отсутствуют (всегда есть дефолты).
	ErrNotFound = errors.New("store: settings not found")
	// ErrInvalid — настройки не прошли валидацию.
	ErrInvalid = errors.New("store: invalid settings")
)

// Contacts — контакты магазина для подвала, шапки и страниц.
type Contacts struct {
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Address   string `json:"address"`
	WorkHours string `json:"work_hours"`
}

// Company — реквизиты: нужны для чеков, квитанций и правовых страниц.
type Company struct {
	Name      string `json:"name"`
	LegalName string `json:"legal_name"`
	INN       string `json:"inn"`
	OGRN      string `json:"ogrn"`
	Email     string `json:"email"`
	Website   string `json:"website"`
}

// Social — ссылки на соцсети (пустые разделы скрываются).
type Social struct {
	Telegram string `json:"telegram"`
	VK       string `json:"vk"`
	WhatsApp string `json:"whatsapp"`
	YouTube  string `json:"youtube"`
}

// SEO — дефолты для страниц, которые не задали свой заголовок/описание.
type SEO struct {
	DefaultTitle       string `json:"default_title"`
	DefaultDescription string `json:"default_description"`
	OGImage            string `json:"og_image"`
}

// Counters — внешние счётчики (коды, а не скрипты): витрина подставляет их
// только при согласии пользователя на cookie.
type Counters struct {
	YandexMetrikaID  string `json:"yandex_metrika_id"`
	GA4MeasurementID string `json:"ga4_measurement_id"`
}

// Rates — параметры расчёта магазина. Ноль означает «взять дефолт движка»,
// поэтому частично настроенный магазин остаётся рабочим.
type Rates struct {
	MachinePerHourRub int64   `json:"machine_per_hour_rub"`
	LaborPerHourRub   int64   `json:"labor_per_hour_rub"`
	OverheadPercent   float64 `json:"overhead_percent"`
	MarginPercent     float64 `json:"margin_percent"`
	DiscountPercent   float64 `json:"discount_percent"`
	TaxPercent        float64 `json:"tax_percent"`
}

// Settings — настройки магазина целиком.
type Settings struct {
	Contacts Contacts `json:"contacts"`
	Company  Company  `json:"company"`
	Social   Social   `json:"social"`
	SEO      SEO      `json:"seo"`
	Counters Counters `json:"counters"`
	Rates    Rates    `json:"rates"`

	// UpdatedBy/UpdatedAt проставляются сервисом при сохранении.
	UpdatedBy string    `json:"updated_by,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultSettings — настройки по умолчанию: пустые контакты (магазин ещё не
// настроен) и параметры расчёта, совпадающие с DefaultRates движка. Пустой
// магазин обязан считать так же, как до появления этого модуля.
func DefaultSettings() Settings {
	return Settings{
		SEO: SEO{
			DefaultTitle:       "Лестницы на заказ",
			DefaultDescription: "Расчёт лестницы, 3D-модель и предварительная цена по вашим габаритам.",
		},
		Rates: Rates{
			OverheadPercent: 20,
			MarginPercent:   30,
			DiscountPercent: 5,
			TaxPercent:      20,
		},
	}
}

// WithDefaults заполняет пустые поля дефолтами: так настройки, сохранённые
// до появления нового поля, не ломают расчёт.
func (s Settings) WithDefaults() Settings {
	d := DefaultSettings()
	if s.SEO.DefaultTitle == "" {
		s.SEO.DefaultTitle = d.SEO.DefaultTitle
	}
	if s.SEO.DefaultDescription == "" {
		s.SEO.DefaultDescription = d.SEO.DefaultDescription
	}
	if s.Rates.OverheadPercent == 0 {
		s.Rates.OverheadPercent = d.Rates.OverheadPercent
	}
	if s.Rates.MarginPercent == 0 {
		s.Rates.MarginPercent = d.Rates.MarginPercent
	}
	if s.Rates.DiscountPercent == 0 {
		s.Rates.DiscountPercent = d.Rates.DiscountPercent
	}
	if s.Rates.TaxPercent == 0 {
		s.Rates.TaxPercent = d.Rates.TaxPercent
	}
	return s
}

// Validate проверяет диапазоны параметров расчёта: ставки неотрицательны,
// проценты в диапазоне 0–100. Ошибка оборачивает ErrInvalid и называет поле —
// так админка получает понятный текст вместо «некорректный запрос».
func (s Settings) Validate() error {
	for _, check := range []struct {
		field string
		value float64
	}{
		{"machine_per_hour_rub", float64(s.Rates.MachinePerHourRub)},
		{"labor_per_hour_rub", float64(s.Rates.LaborPerHourRub)},
	} {
		if check.value < 0 {
			return fmt.Errorf("%w: %s must not be negative", ErrInvalid, check.field)
		}
	}
	for _, check := range []struct {
		field string
		value float64
	}{
		{"overhead_percent", s.Rates.OverheadPercent},
		{"margin_percent", s.Rates.MarginPercent},
		{"discount_percent", s.Rates.DiscountPercent},
		{"tax_percent", s.Rates.TaxPercent},
	} {
		if check.value < 0 || check.value > 100 {
			return fmt.Errorf("%w: %s must be within 0..100", ErrInvalid, check.field)
		}
	}
	return nil
}

// PublicSettings — безопасное подмножество для витрины: контакты, реквизиты,
// соцсети, SEO и коды счётчиков. Цены материалов сюда не входят: витрина
// берёт их из каталога, где уже учтён прайс магазина.
type PublicSettings struct {
	Contacts Contacts `json:"contacts"`
	Company  Company  `json:"company"`
	Social   Social   `json:"social"`
	SEO      SEO      `json:"seo"`
	Counters Counters `json:"counters"`
}

// Public возвращает витринное подмножество настроек.
func (s Settings) Public() PublicSettings {
	return PublicSettings{
		Contacts: s.Contacts,
		Company:  s.Company,
		Social:   s.Social,
		SEO:      s.SEO,
		Counters: s.Counters,
	}
}

// MaterialPrice — цена материала магазина, ₽ за килограмм.
type MaterialPrice struct {
	Code string `json:"code"`
	// PricePerKgRub — цена в рублях за кг (не minor-единицы: это витринная
	// величина, округление до целых рублей).
	PricePerKgRub int64 `json:"price_per_kg_rub"`
	// Overridden — true, если цена задана магазином; false — действует
	// встроенная ставка движка.
	Overridden bool `json:"overridden"`
}

// Repository — порт хранения настроек и прайса магазина.
type Repository interface {
	// GetSettings возвращает настройки tenant; ErrNotFound — если их нет.
	GetSettings(ctx context.Context, tenantID string) (Settings, error)
	// SaveSettings сохраняет настройки (upsert).
	SaveSettings(ctx context.Context, tenantID string, s Settings, updatedBy string) error
	// ListMaterialPrices возвращает цены материалов, заданные магазином.
	ListMaterialPrices(ctx context.Context, tenantID string) ([]MaterialPrice, error)
	// SetMaterialPrice сохраняет цену одного материала (upsert).
	SetMaterialPrice(ctx context.Context, tenantID, materialCode string, pricePerKgRub int64, updatedBy string) error
	// DeleteMaterialPrice возвращает материал к встроенной ставке.
	DeleteMaterialPrice(ctx context.Context, tenantID, materialCode string) error
}
