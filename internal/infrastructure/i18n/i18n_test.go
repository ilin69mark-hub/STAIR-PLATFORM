package i18n

import (
	"testing"
)

func TestLanguageIsValid(t *testing.T) {
	valid := []Language{LanguageEN, LanguageRU}
	for _, l := range valid {
		if !l.IsValid() {
			t.Errorf("expected language %q to be valid", l)
		}
	}
	if Language("invalid").IsValid() {
		t.Error("expected 'invalid' language to be invalid")
	}
}

func TestMapTranslatorTranslate(t *testing.T) {
	tr := NewMapTranslator()
	tr.Register(LanguageEN, map[string]string{
		"hello": "Hello",
	})
	tr.Register(LanguageRU, map[string]string{
		"hello": "Привет",
	})

	if got := tr.Translate("hello", LanguageEN); got != "Hello" {
		t.Errorf("expected 'Hello', got %q", got)
	}
	if got := tr.Translate("hello", LanguageRU); got != "Привет" {
		t.Errorf("expected 'Привет', got %q", got)
	}
}

func TestMapTranslatorFallback(t *testing.T) {
	tr := NewMapTranslator()
	tr.Register(LanguageEN, map[string]string{
		"hello": "Hello",
	})
	// RU не зарегистрирован — должен fallback на EN
	if got := tr.Translate("hello", LanguageRU); got != "Hello" {
		t.Errorf("expected fallback 'Hello', got %q", got)
	}
}

func TestMapTranslatorMissingKey(t *testing.T) {
	tr := NewMapTranslator()
	tr.Register(LanguageEN, map[string]string{
		"hello": "Hello",
	})
	// Отсутствующий ключ — возвращает сам ключ
	if got := tr.Translate("missing", LanguageEN); got != "missing" {
		t.Errorf("expected 'missing', got %q", got)
	}
}

func TestMapTranslatorTranslatef(t *testing.T) {
	tr := NewMapTranslator()
	tr.Register(LanguageEN, map[string]string{
		"greeting": "Hello, %s!",
	})

	got := tr.Translatef("greeting", LanguageEN, "World")
	if got != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", got)
	}
}

func TestDefaultMessages(t *testing.T) {
	msgs := DefaultMessages()
	if len(msgs[LanguageEN]) == 0 {
		t.Error("expected non-empty EN messages")
	}
	if len(msgs[LanguageRU]) == 0 {
		t.Error("expected non-empty RU messages")
	}
	// Проверяем наличие ключевых сообщений
	for _, lang := range []Language{LanguageEN, LanguageRU} {
		if _, ok := msgs[lang]["pipeline.completed"]; !ok {
			t.Errorf("expected 'pipeline.completed' for lang %q", lang)
		}
	}
}

func TestLocalizer(t *testing.T) {
	tr := NewMapTranslator()
	tr.Register(LanguageEN, map[string]string{
		"hello": "Hello",
	})
	tr.Register(LanguageRU, map[string]string{
		"hello": "Привет",
	})

	locEN := NewLocalizer(LanguageEN, tr)
	locRU := NewLocalizer(LanguageRU, tr)

	if got := locEN.T("hello"); got != "Hello" {
		t.Errorf("expected 'Hello', got %q", got)
	}
	if got := locRU.T("hello"); got != "Привет" {
		t.Errorf("expected 'Привет', got %q", got)
	}
}

func TestLocalizerTf(t *testing.T) {
	tr := NewMapTranslator()
	tr.Register(LanguageEN, map[string]string{
		"greeting": "Hello, %s!",
	})

	loc := NewLocalizer(LanguageEN, tr)
	got := loc.Tf("greeting", "World")
	if got != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", got)
	}
}
