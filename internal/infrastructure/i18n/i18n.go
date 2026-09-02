// Package i18n реализует интернационализацию (ENG-I18N-0001).
// Поддерживает en и ru языки.
package i18n

import (
	"fmt"
	"sync"
)

// Language — поддерживаемый язык.
type Language string

const (
	LanguageEN Language = "en"
	LanguageRU Language = "ru"
)

// IsValid проверяет, что язык поддерживается.
func (l Language) IsValid() bool {
	switch l {
	case LanguageEN, LanguageRU:
		return true
	}
	return false
}

// Translator — интерфейс перевода.
type Translator interface {
	Translate(key string, lang Language) string
	Translatef(key string, lang Language, args ...interface{}) string
}

// MapTranslator — переводчик на основе карты ключей.
type MapTranslator struct {
	messages map[Language]map[string]string
	mu       sync.RWMutex
}

// NewMapTranslator создаёт переводчик.
func NewMapTranslator() *MapTranslator {
	return &MapTranslator{
		messages: make(map[Language]map[string]string),
	}
}

// Register регистрирует сообщения для языка.
func (t *MapTranslator) Register(lang Language, messages map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messages[lang] = messages
}

// Translate переводит ключ на указанный язык.
func (t *MapTranslator) Translate(key string, lang Language) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if msgs, ok := t.messages[lang]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	// Fallback на English
	if msgs, ok := t.messages[LanguageEN]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	return key
}

// Translatef переводит ключ с форматированием.
func (t *MapTranslator) Translatef(key string, lang Language, args ...interface{}) string {
	msg := t.Translate(key, lang)
	return fmt.Sprintf(msg, args...)
}

// DefaultMessages возвращает сообщения по умолчанию.
func DefaultMessages() map[Language]map[string]string {
	return map[Language]map[string]string{
		LanguageEN: {
			"stair.created":            "Stair configuration created",
			"stair.updated":            "Stair configuration updated",
			"stair.deleted":            "Stair configuration deleted",
			"analysis.started":         "Analysis started",
			"analysis.completed":       "Analysis completed",
			"analysis.failed":          "Analysis failed",
			"optimization.started":     "Optimization started",
			"optimization.completed":   "Optimization completed",
			"pipeline.started":         "Pipeline started",
			"pipeline.completed":       "Pipeline completed",
			"pipeline.failed":          "Pipeline failed",
			"document.generated":       "Document generated",
			"export.started":           "Export started",
			"export.completed":         "Export completed",
			"validation.error":         "Validation error",
			"validation.warning":       "Validation warning",
			"error.not_found":          "Resource not found",
			"error.unauthorized":       "Unauthorized",
			"error.forbidden":          "Forbidden",
			"error.internal":           "Internal server error",
		},
		LanguageRU: {
			"stair.created":            "Конфигурация лестницы создана",
			"stair.updated":            "Конфигурация лестницы обновлена",
			"stair.deleted":            "Конфигурация лестницы удалена",
			"analysis.started":         "Анализ запущен",
			"analysis.completed":       "Анализ завершён",
			"analysis.failed":          "Анализ завершён с ошибкой",
			"optimization.started":     "Оптимизация запущена",
			"optimization.completed":   "Оптимизация завершена",
			"pipeline.started":         "Pipeline запущен",
			"pipeline.completed":       "Pipeline завершён",
			"pipeline.failed":          "Pipeline завершён с ошибкой",
			"document.generated":       "Документ сгенерирован",
			"export.started":           "Экспорт запущен",
			"export.completed":         "Экспорт завершён",
			"validation.error":         "Ошибка валидации",
			"validation.warning":       "Предупреждение валидации",
			"error.not_found":          "Ресурс не найден",
			"error.unauthorized":       "Не авторизован",
			"error.forbidden":          "Доступ запрещён",
			"error.internal":           "Внутренняя ошибка сервера",
		},
	}
}

// Localizer — локализатор для конкретного языка.
type Localizer struct {
	lang       Language
	translator Translator
}

// NewLocalizer создаёт локализатор.
func NewLocalizer(lang Language, translator Translator) *Localizer {
	return &Localizer{
		lang:       lang,
		translator: translator,
	}
}

// T переводит ключ.
func (l *Localizer) T(key string) string {
	return l.translator.Translate(key, l.lang)
}

// Tf переводит ключ с форматированием.
func (l *Localizer) Tf(key string, args ...interface{}) string {
	return l.translator.Translatef(key, l.lang, args...)
}
